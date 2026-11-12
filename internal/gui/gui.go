// Package gui is ng's window: SDL3 for the window, input and drawing (with
// SDL_Renderer, so it runs on OpenGL, Vulkan or SDL's software renderer
// under xrdp), and the paint package for what to draw.
package gui

/*
#include <stdlib.h>
#include "sdl.h"
*/
import "C"

import (
	"fmt"
	"math"
	"os"
	"slices"
	"sync"
	"time"
	"unsafe"

	"github.com/Shougo/ng/internal/atlas"
	"github.com/Shougo/ng/internal/font"
	"github.com/Shougo/ng/internal/input"
	"github.com/Shougo/ng/internal/nvim"
	"github.com/Shougo/ng/internal/paint"
	"github.com/Shougo/ng/internal/screen"
)

const atlasSize = 2048

// bootstrap runs in Neovim before the UI attaches. ng installs nothing on
// the runtimepath; everything it needs is sent here.
const bootstrap = `vim.g.ng = 1`

// Options configure Run.
type Options struct {
	Cols, Rows int
}

type rect struct{ x, y, w, h int }

// App is the running GUI. All fields are owned by the main thread except
// the redraw queue.
type App struct {
	nv  *nvim.Nvim
	scr *screen.Screen

	win       *C.SDL_Window
	ren       *C.SDL_Renderer
	screenTex *C.SDL_Texture
	texW      int
	texH      int
	atlasTex  *C.SDL_Texture
	packer    *atlas.Packer
	glyphs    map[*font.Glyph]rect
	upload    []byte

	fonts      *font.Set
	fontKey    string
	pixW, pixH int
	focused    bool
	title      string
	drawn      bool // painted since the last present

	verts []C.SDL_Vertex
	idx   []C.int

	attached     bool
	gridCols     int // grid size that fits the window
	gridRows     int
	suppressText bool
	mouseButton  string
	mouseRow     int
	mouseCol     int

	mu      sync.Mutex
	batches [][]any
	ctl     []func() // work posted to the main thread
	closed  bool
	fatal   error
}

// post runs f on the main thread. Safe to call from any goroutine.
func (a *App) post(f func()) {
	a.mu.Lock()
	a.ctl = append(a.ctl, f)
	a.mu.Unlock()
	wake()
}

// New prepares an App for a connection that has not attached yet. Redraw
// notifications must be passed to Enqueue.
func New() *App {
	return &App{glyphs: map[*font.Glyph]rect{}, focused: true}
}

// Enqueue is the redraw notification handler. It runs on the RPC reader
// goroutine and only hands the batch to the main thread.
func (a *App) Enqueue(method string, params []any) {
	if method != "redraw" {
		return
	}
	a.mu.Lock()
	a.batches = append(a.batches, params)
	a.mu.Unlock()
	wake()
}

func (a *App) markClosed() {
	a.mu.Lock()
	a.closed = true
	a.mu.Unlock()
	wake()
}

// Run creates the window, attaches to nv and runs until Neovim closes the
// connection. It must be called from the main goroutine.
func (a *App) Run(nv *nvim.Nvim, opt Options) error {
	a.nv = nv
	if !C.ng_init() {
		return sdlError("SDL_Init")
	}
	defer C.SDL_Quit()

	title := C.CString("ng")
	a.win = C.ng_create_window(title, 800, 600)
	C.free(unsafe.Pointer(title))
	if a.win == nil {
		return sdlError("SDL_CreateWindow")
	}
	defer C.SDL_DestroyWindow(a.win)
	a.ren = C.SDL_CreateRenderer(a.win, nil)
	if a.ren == nil {
		return sdlError("SDL_CreateRenderer")
	}
	defer C.SDL_DestroyRenderer(a.ren)
	if a.atlasTex = C.ng_create_texture(a.ren, C.SDL_TEXTUREACCESS_STATIC, atlasSize, atlasSize); a.atlasTex == nil {
		return sdlError("atlas texture")
	}
	a.packer = atlas.NewPacker(atlasSize, atlasSize)

	a.scr = screen.New(opt.Cols, opt.Rows)
	if err := a.loadFonts("", "", 0); err != nil {
		return err
	}
	a.resizeWindowToGrid(opt.Cols, opt.Rows)
	C.SDL_ShowWindow(a.win)
	C.SDL_StartTextInput(a.win)

	go func() {
		a.nv.Serve()
		a.markClosed()
	}()
	// Attaching may wait for Neovim (see nvim.Prepare), so it runs on its
	// own goroutine and the window stays responsive: keys typed meanwhile
	// reach Neovim and can answer the prompt it is waiting at. The window
	// manager may not have given the size we asked for, so attach with the
	// grid that really fits.
	cols, rows := a.gridCols, a.gridRows
	go func() {
		err := a.nv.Prepare(bootstrap, cols, rows, func() { a.post(a.showWaiting) })
		a.post(func() { a.attachDone(err) })
	}()
	return a.loop()
}

// attachDone runs on the main thread when nvim.Prepare has finished.
func (a *App) attachDone(err error) {
	if err != nil {
		a.fatal = fmt.Errorf("attach: %w", err)
		return
	}
	a.attached = true
	if a.gridCols != a.scr.Width || a.gridRows != a.scr.Height {
		a.nv.TryResize(a.gridCols, a.gridRows)
	}
	// Errors from init.lua etc. that a headless Neovim could not show.
	a.nv.ReportStartupErrors()
}

// waitingMessage is drawn while Neovim is blocked at a prompt and has not
// accepted the UI yet. Neovim's own screen replaces it after attaching.
var waitingMessage = []string{
	"ng: Neovim is waiting for input and cannot show its screen yet.",
	"Type the answer to its prompt (for example <Enter> or <Esc>).",
}

func (a *App) showWaiting() {
	if a.attached {
		return
	}
	events := []any{[]any{"grid_clear", []any{int64(1)}}}
	for i, line := range waitingMessage {
		cells := []any{}
		for _, r := range line {
			cells = append(cells, []any{string(r)})
		}
		events = append(events, []any{"grid_line", []any{int64(1), int64(i), int64(0), cells, false}})
	}
	a.scr.HandleRedraw(events)
	a.paintDirty()
	a.present()
}

// Frame coalescing. Neovim sometimes sends several flushes for one
// action (for example entering the command line with cmdheight=0 first
// draws the command line, then moves the windows). A terminal shows only
// the last of them because it reads them together; ng presenting each
// flush at once made the intermediate screens flicker. After a flush, ng
// waits until Neovim has been quiet for quietMs before presenting, but
// never more than maxDelayMs, so typing stays well under a frame of
// latency.
const (
	quietMs    = 2
	maxDelayMs = 8
)

func (a *App) loop() error {
	var ev C.SDL_Event
	for {
		if C.ng_wait(&ev) == 0 {
			return sdlError("SDL_WaitEvent")
		}
		present, closed, err := a.drain(&ev)
		if err != nil || closed {
			return err
		}
		if a.drawn {
			deadline := time.Now().Add(maxDelayMs * time.Millisecond)
			for time.Now().Before(deadline) && C.ng_wait_timeout(&ev, quietMs) != 0 {
				p, closed, err := a.drain(&ev)
				if err != nil || closed {
					return err
				}
				present = present || p
			}
		}
		if a.drawn || present {
			a.drawn = false
			a.present()
		}
	}
}

// drain handles ev and every other pending event, then applies the queued
// redraws. Painting into the screen texture happens at each flush; only
// presenting is deferred.
func (a *App) drain(ev *C.SDL_Event) (present, closed bool, err error) {
	wakeType := uint32(C.ng_get_wake_type())
	for {
		e := flatten(ev)
		if e.typ != wakeType && a.handle(e) {
			present = true
		}
		if C.ng_poll(ev) == 0 {
			break
		}
	}
	drawn, closed, err := a.processRedraws()
	a.drawn = a.drawn || drawn
	return present, closed, err
}

// processRedraws applies queued redraw batches and paints at each flush.
func (a *App) processRedraws() (drawn, closed bool, err error) {
	a.mu.Lock()
	batches, ctl := a.batches, a.ctl
	a.batches, a.ctl = nil, nil
	closed = a.closed
	a.mu.Unlock()
	for _, f := range ctl {
		f()
	}
	if a.fatal != nil {
		return drawn, closed, a.fatal
	}
	for _, b := range batches {
		flush, err := a.scr.HandleRedraw(b)
		if err != nil {
			return drawn, closed, err
		}
		if flush {
			a.applyUIChanges()
			a.paintDirty()
			drawn = true
		}
	}
	return drawn, closed, nil
}

// applyUIChanges reacts to option_set and set_title after a flush.
func (a *App) applyUIChanges() {
	changed := a.scr.TakeOptionChanges()
	if slices.Contains(changed, "guifont") || slices.Contains(changed, "guifontwide") ||
		slices.Contains(changed, "linespace") {
		gf, gfw := a.scr.OptionString("guifont"), a.scr.OptionString("guifontwide")
		if err := a.loadFonts(gf, gfw, a.scr.OptionInt("linespace")); err != nil {
			a.errorMessage(err.Error())
		} else {
			// Like GVim: keep 'lines' and 'columns' and resize the window.
			a.resizeWindowToGrid(a.scr.Width, a.scr.Height)
		}
	}
	if a.scr.TakeColorsChanged() {
		a.scr.MarkAllDirty()
	}
	if a.scr.Title != a.title {
		a.title = a.scr.Title
		t := C.CString(a.title)
		C.SDL_SetWindowTitle(a.win, t)
		C.free(unsafe.Pointer(t))
	}
}

func (a *App) errorMessage(msg string) {
	fmt.Fprintln(os.Stderr, "ng:", msg)
	a.nv.Notify("nvim_echo", []any{[]any{"ng: " + msg, "ErrorMsg"}}, true, map[string]any{})
}

func (a *App) dpi() float64 {
	s := float64(C.SDL_GetWindowDisplayScale(a.win))
	if s <= 0 {
		s = 1
	}
	return 96 * s
}

// loadFonts replaces the font set. The atlas is emptied and every row is
// repainted.
func (a *App) loadFonts(guifont, guifontwide string, linespace int) error {
	dpi := a.dpi()
	key := fmt.Sprintf("%s\x00%s\x00%d\x00%f", guifont, guifontwide, linespace, dpi)
	if key == a.fontKey && a.fonts != nil {
		return nil
	}
	f, err := font.NewSet(guifont, guifontwide, dpi, linespace)
	if err != nil {
		return err
	}
	for _, w := range f.Warnings {
		a.errorMessage(w)
	}
	if a.fonts != nil {
		a.fonts.Close()
	}
	a.fonts, a.fontKey = f, key
	a.resetAtlas()
	a.scr.MarkAllDirty()
	return nil
}

func (a *App) resetAtlas() {
	clear(a.glyphs)
	a.packer.Reset()
}

// resizeWindowToGrid asks the window manager for a window that fits
// cols x rows, then fits the grid to whatever size the window really got
// (tiling window managers such as i3 decide the size themselves).
func (a *App) resizeWindowToGrid(cols, rows int) {
	d := float64(C.ng_pixel_density(a.win))
	w := int(math.Ceil(float64(cols*a.fonts.CellW) / d))
	h := int(math.Ceil(float64(rows*a.fonts.CellH) / d))
	C.SDL_SetWindowSize(a.win, C.int(w), C.int(h))
	C.SDL_SyncWindow(a.win)
	a.updatePixelSize()
}

// updatePixelSize reads the drawable size and resizes the grid to fit.
func (a *App) updatePixelSize() {
	var w, h C.int
	C.SDL_GetWindowSizeInPixels(a.win, &w, &h)
	a.pixW, a.pixH = int(w), int(h)
	a.gridCols = max(1, a.pixW/a.fonts.CellW)
	a.gridRows = max(1, a.pixH/a.fonts.CellH)
	if a.attached && (a.gridCols != a.scr.Width || a.gridRows != a.scr.Height) {
		a.nv.TryResize(a.gridCols, a.gridRows)
	}
	a.scr.MarkAllDirty()
}

// ensureScreenTexture (re)creates the texture that holds the painted grid.
// It reports whether the texture is new and must be painted completely.
func (a *App) ensureScreenTexture() bool {
	if a.screenTex != nil && a.texW == a.pixW && a.texH == a.pixH {
		return false
	}
	if a.screenTex != nil {
		C.SDL_DestroyTexture(a.screenTex)
	}
	a.screenTex = C.ng_create_texture(a.ren, C.SDL_TEXTUREACCESS_TARGET, C.int(max(1, a.pixW)), C.int(max(1, a.pixH)))
	a.texW, a.texH = a.pixW, a.pixH
	return true
}

// paintDirty repaints the changed rows into the screen texture.
func (a *App) paintDirty() {
	full := a.ensureScreenTexture()
	if a.screenTex == nil {
		return
	}
	if full {
		a.scr.MarkAllDirty()
	}
	rows := a.scr.TakeDirty()
	C.SDL_SetRenderTarget(a.ren, a.screenTex)
	if full || len(rows) == a.scr.Height {
		paint.Padding(a, a.scr, a.fonts, a.pixW, a.pixH)
	}
	for _, r := range rows {
		paint.Row(a, a.scr, a.fonts, r)
	}
	a.flushGlyphs()
	C.SDL_SetRenderTarget(a.ren, nil)
}

// present shows the screen texture and the cursor on top of it.
func (a *App) present() {
	if a.screenTex == nil {
		return
	}
	C.ng_blit(a.ren, a.screenTex, C.int(a.pixW), C.int(a.pixH))
	paint.Cursor(a, a.scr, a.fonts, a.focused)
	a.flushGlyphs()
	C.SDL_RenderPresent(a.ren)
}

// FillRect implements paint.Painter.
func (a *App) FillRect(x, y, w, h int, c screen.Color) {
	C.ng_fill(a.ren, C.int(x), C.int(y), C.int(w), C.int(h), C.Uint32(c))
}

// DrawGlyph implements paint.Painter. Glyph quads are batched and drawn
// with one SDL_RenderGeometry call per flushGlyphs.
func (a *App) DrawGlyph(g *font.Glyph, x, y int, c screen.Color) {
	r, ok := a.glyphs[g]
	if !ok {
		px, py, fits := a.packer.Alloc(g.W, g.H)
		if !fits {
			// Atlas full: draw what is queued, then start over.
			a.flushGlyphs()
			a.resetAtlas()
			if px, py, fits = a.packer.Alloc(g.W, g.H); !fits {
				return
			}
		}
		a.upload = atlas.AlphaToRGBA(g.Pix, g.W, g.H, g.Stride, a.upload)
		C.ng_update(a.atlasTex, C.int(px), C.int(py), C.int(g.W), C.int(g.H),
			unsafe.Pointer(&a.upload[0]), C.int(g.W*4))
		r = rect{px, py, g.W, g.H}
		a.glyphs[g] = r
	}
	cr, cg, cb := c.RGB()
	col := C.SDL_FColor{r: C.float(cr) / 255, g: C.float(cg) / 255, b: C.float(cb) / 255, a: 1}
	x0, y0 := C.float(x), C.float(y)
	x1, y1 := C.float(x+r.w), C.float(y+r.h)
	u0, v0 := C.float(r.x)/atlasSize, C.float(r.y)/atlasSize
	u1, v1 := C.float(r.x+r.w)/atlasSize, C.float(r.y+r.h)/atlasSize
	base := C.int(len(a.verts))
	a.verts = append(a.verts,
		C.SDL_Vertex{position: C.SDL_FPoint{x: x0, y: y0}, color: col, tex_coord: C.SDL_FPoint{x: u0, y: v0}},
		C.SDL_Vertex{position: C.SDL_FPoint{x: x1, y: y0}, color: col, tex_coord: C.SDL_FPoint{x: u1, y: v0}},
		C.SDL_Vertex{position: C.SDL_FPoint{x: x1, y: y1}, color: col, tex_coord: C.SDL_FPoint{x: u1, y: v1}},
		C.SDL_Vertex{position: C.SDL_FPoint{x: x0, y: y1}, color: col, tex_coord: C.SDL_FPoint{x: u0, y: v1}},
	)
	a.idx = append(a.idx, base, base+1, base+2, base, base+2, base+3)
}

func (a *App) flushGlyphs() {
	if len(a.idx) == 0 {
		return
	}
	C.SDL_RenderGeometry(a.ren, a.atlasTex, &a.verts[0], C.int(len(a.verts)), &a.idx[0], C.int(len(a.idx)))
	a.verts, a.idx = a.verts[:0], a.idx[:0]
}

// handle processes one SDL event and reports whether the window needs to
// be presented again.
func (a *App) handle(e event) bool {
	switch e.typ {
	case C.SDL_EVENT_QUIT, C.SDL_EVENT_WINDOW_CLOSE_REQUESTED:
		// Let Neovim decide: it asks about unsaved buffers, and ng exits
		// when the connection closes.
		a.nv.Command("confirm qall")
	case C.SDL_EVENT_WINDOW_PIXEL_SIZE_CHANGED:
		a.updatePixelSize()
		a.paintDirty()
		return true
	case C.SDL_EVENT_WINDOW_DISPLAY_SCALE_CHANGED:
		if err := a.loadFonts(a.scr.OptionString("guifont"), a.scr.OptionString("guifontwide"),
			a.scr.OptionInt("linespace")); err == nil {
			a.updatePixelSize()
			a.paintDirty()
		}
		return true
	case C.SDL_EVENT_WINDOW_EXPOSED:
		return true
	case C.SDL_EVENT_RENDER_TARGETS_RESET, C.SDL_EVENT_RENDER_DEVICE_RESET:
		if a.screenTex != nil {
			C.SDL_DestroyTexture(a.screenTex)
			a.screenTex = nil
		}
		a.resetAtlas()
		a.paintDirty()
		return true
	case C.SDL_EVENT_WINDOW_FOCUS_GAINED, C.SDL_EVENT_WINDOW_FOCUS_LOST:
		a.focused = e.typ == C.SDL_EVENT_WINDOW_FOCUS_GAINED
		if a.attached {
			a.nv.SetFocus(a.focused)
		}
		return true
	case C.SDL_EVENT_KEY_DOWN:
		a.keyDown(e)
	case C.SDL_EVENT_TEXT_INPUT:
		if a.suppressText {
			a.suppressText = false
			return false
		}
		a.nv.Input(input.Text(e.text))
	case C.SDL_EVENT_MOUSE_BUTTON_DOWN, C.SDL_EVENT_MOUSE_BUTTON_UP,
		C.SDL_EVENT_MOUSE_MOTION, C.SDL_EVENT_MOUSE_WHEEL:
		a.mouse(e)
	case C.SDL_EVENT_DROP_FILE:
		if e.drop != "" {
			a.nv.Notify("nvim_exec_lua", "vim.cmd.drop(vim.fn.fnameescape(...))", []any{e.drop})
		}
	}
	return false
}

func mods(m uint16) input.Mod {
	var r input.Mod
	if m&C.SDL_KMOD_SHIFT != 0 {
		r |= input.ModShift
	}
	if m&C.SDL_KMOD_CTRL != 0 {
		r |= input.ModCtrl
	}
	if m&C.SDL_KMOD_ALT != 0 {
		r |= input.ModAlt
	}
	if m&C.SDL_KMOD_GUI != 0 {
		r |= input.ModSuper
	}
	return r
}

func specialKey(k uint32) input.Key {
	switch k {
	case C.SDLK_ESCAPE:
		return input.KeyEscape
	case C.SDLK_RETURN, C.SDLK_KP_ENTER:
		return input.KeyEnter
	case C.SDLK_TAB:
		return input.KeyTab
	case C.SDLK_BACKSPACE:
		return input.KeyBackspace
	case C.SDLK_DELETE:
		return input.KeyDelete
	case C.SDLK_INSERT:
		return input.KeyInsert
	case C.SDLK_HOME:
		return input.KeyHome
	case C.SDLK_END:
		return input.KeyEnd
	case C.SDLK_PAGEUP:
		return input.KeyPageUp
	case C.SDLK_PAGEDOWN:
		return input.KeyPageDown
	case C.SDLK_UP:
		return input.KeyUp
	case C.SDLK_DOWN:
		return input.KeyDown
	case C.SDLK_LEFT:
		return input.KeyLeft
	case C.SDLK_RIGHT:
		return input.KeyRight
	case C.SDLK_SPACE:
		return input.KeySpace
	}
	if k >= C.SDLK_F1 && k <= C.SDLK_F12 {
		return input.KeyF1 + input.Key(k-C.SDLK_F1)
	}
	return input.KeyNone
}

func (a *App) keyDown(e event) {
	a.suppressText = false
	m := mods(e.mod)
	if k := specialKey(e.key); k != input.KeyNone {
		if s := input.SpecialKey(k, m); s != "" {
			a.nv.Input(s)
			a.suppressText = m&(input.ModCtrl|input.ModAlt|input.ModSuper) != 0
		}
		return
	}
	// Printable key with Ctrl/Alt/Super: SDL may or may not also send a
	// text event for it, so the next text event is dropped.
	if e.key < C.SDLK_SCANCODE_MASK {
		if s := input.ChordKey(rune(e.key), m); s != "" {
			a.nv.Input(s)
			a.suppressText = true
		}
	}
}

func (a *App) cellAt(x, y float32) (row, col int) {
	d := float32(C.ng_pixel_density(a.win))
	col = int(x*d) / a.fonts.CellW
	row = int(y*d) / a.fonts.CellH
	return min(max(row, 0), a.scr.Height-1), min(max(col, 0), a.scr.Width-1)
}

func (a *App) mouse(e event) {
	if !a.scr.MouseEnabled {
		return
	}
	mod := input.MouseModifier(mods(uint16(C.SDL_GetModState())))
	switch e.typ {
	case C.SDL_EVENT_MOUSE_BUTTON_DOWN, C.SDL_EVENT_MOUSE_BUTTON_UP:
		var name string
		switch e.button {
		case C.SDL_BUTTON_LEFT:
			name = "left"
		case C.SDL_BUTTON_MIDDLE:
			name = "middle"
		case C.SDL_BUTTON_RIGHT:
			name = "right"
		default:
			return
		}
		row, col := a.cellAt(e.x, e.y)
		a.mouseRow, a.mouseCol = row, col
		if e.typ == C.SDL_EVENT_MOUSE_BUTTON_DOWN {
			a.mouseButton = name
			a.nv.InputMouse(name, "press", mod, row, col)
		} else {
			a.mouseButton = ""
			a.nv.InputMouse(name, "release", mod, row, col)
		}
	case C.SDL_EVENT_MOUSE_MOTION:
		row, col := a.cellAt(e.x, e.y)
		moved := row != a.mouseRow || col != a.mouseCol
		a.mouseRow, a.mouseCol = row, col
		if a.mouseButton != "" && moved {
			a.nv.InputMouse(a.mouseButton, "drag", mod, row, col)
		}
	case C.SDL_EVENT_MOUSE_WHEEL:
		row, col := a.mouseRow, a.mouseCol
		switch {
		case e.y > 0:
			a.nv.InputMouse("wheel", "up", mod, row, col)
		case e.y < 0:
			a.nv.InputMouse("wheel", "down", mod, row, col)
		}
		switch {
		case e.x > 0:
			a.nv.InputMouse("wheel", "right", mod, row, col)
		case e.x < 0:
			a.nv.InputMouse("wheel", "left", mod, row, col)
		}
	}
}

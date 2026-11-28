package gui

/*
#include "sdl.h"
*/
import "C"

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/Shougo/ng/internal/atlas"
	"github.com/Shougo/ng/internal/font"
	"github.com/Shougo/ng/internal/paint"
	"github.com/Shougo/ng/internal/screen"
)

const atlasSize = 2048

type rect struct{ x, y, w, h int }

// loadOptionFonts loads the fonts that 'guifont', 'guifontwide' and
// 'linespace' ask for.
func (a *App) loadOptionFonts() error {
	return a.loadFonts(a.scr.OptionString("guifont"), a.scr.OptionString("guifontwide"),
		a.scr.OptionInt("linespace"))
}

// loadFonts replaces the font set. The atlas is emptied and every row is
// repainted.
func (a *App) loadFonts(guifont, guifontwide string, linespace int) error {
	dpi := a.dpi()
	key := fmt.Sprintf("%s\x00%s\x00%d\x00%f", guifont, guifontwide, linespace, dpi)
	if key == a.fontKey && a.fonts != nil {
		a.tracef(time.Now(), "font    unchanged guifont=%q guifontwide=%q dpi=%.1f", guifont, guifontwide, dpi)
		return nil
	}
	f, err := font.NewSet(guifont, guifontwide, dpi, linespace)
	if err != nil {
		a.tracef(time.Now(), "font    guifont=%q guifontwide=%q dpi=%.1f: %v", guifont, guifontwide, dpi, err)
		return err
	}
	a.tracef(time.Now(), "font    guifont=%q guifontwide=%q dpi=%.1f -> %s warnings=%q",
		guifont, guifontwide, dpi, f.Describe(), f.Warnings)
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
	if !a.shown || a.fonts == nil {
		return
	}
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
	if a.screenTex == nil || !a.shown {
		return
	}
	if !a.presented {
		a.presented = true
		a.mark("first frame presented")
	}
	C.ng_blit(a.ren, a.screenTex, C.int(a.pixW), C.int(a.pixH))
	a.dropImages()
	paint.Images(a, a.scr, a.fonts)
	if !a.busyHidesCursor(time.Now()) {
		paint.Cursor(a, a.scr, a.fonts, a.focused)
	}
	area, candidateX := paint.CursorRect(a.scr, a.fonts), 0
	if a.preedit.Text != "" {
		area, candidateX = paint.DrawPreedit(a, a.scr, a.fonts, a.preedit)
		candidateX -= area.X
	}
	a.flushGlyphs()
	a.setIMEArea(area, candidateX)
	C.SDL_RenderPresent(a.ren)
	if a.trace != nil {
		if l, ok := a.latency.Present(time.Now()); ok {
			a.tracef(time.Now(), "latency %.2f ms for %s (queue %.2f, nvim %.2f, frame %.2f)",
				msf(l.Total()), l.Key, msf(l.Queue), msf(l.Nvim), msf(l.Frame))
		}
		m := a.scr.CurrentMode()
		a.tracef(time.Now(), "present cursor=(%d,%d) mode=%s busy=%v",
			a.scr.CursorRow, a.scr.CursorCol, m.Name, a.scr.Busy)
	}
}

// busyDelay is how long Neovim must stay busy before ng hides the
// cursor. Neovim sends busy_start and busy_stop around short pieces of
// work too, for example for every key typed on the command line with ui2.
// A terminal hides the cursor for those as well, but it shows the screen
// only at its refresh, so a busy period of a few milliseconds never
// appears; ng presents frames in between and the cursor blinked away.
const busyDelay = 100 * time.Millisecond

// busyHidesCursor reports whether the cursor is hidden at now because
// Neovim has been busy for at least busyDelay.
func (a *App) busyHidesCursor(now time.Time) bool {
	return a.scr.Busy && !a.busySince.IsZero() && now.Sub(a.busySince) >= busyDelay
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
		if g.RGBA != nil {
			a.upload = append(a.upload[:0], g.RGBA...)
		} else {
			a.upload = atlas.AlphaToRGBA(g.Pix, g.W, g.H, g.Stride, a.upload)
		}
		C.ng_update(a.atlasTex, C.int(px), C.int(py), C.int(g.W), C.int(g.H),
			unsafe.Pointer(&a.upload[0]), C.int(g.W*4))
		r = rect{px, py, g.W, g.H}
		a.glyphs[g] = r
	}
	cr, cg, cb := c.RGB()
	col := C.SDL_FColor{r: C.float(cr) / 255, g: C.float(cg) / 255, b: C.float(cb) / 255, a: 1}
	if g.RGBA != nil {
		// A color glyph (emoji) keeps its own colors.
		col = C.SDL_FColor{r: 1, g: 1, b: 1, a: 1}
	}
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

package gui

/*
#include "sdl.h"
*/
import "C"

import (
	"time"

	"github.com/Shougo/ng/internal/input"
	"github.com/Shougo/ng/internal/paint"
)

// imeInsertOnly is g:ng_ime_insert_only, true unless set to 0 or false.
func (a *App) imeInsertOnly() bool {
	switch v := a.vars["ng_ime_insert_only"].(type) {
	case int64:
		return v != 0
	case bool:
		return v
	}
	return true
}

// updateTextInput turns SDL text input, and with it the input method, on
// in modes where text is typed and off elsewhere (see input.IMEWanted).
func (a *App) updateTextInput() {
	if want := input.IMEWanted(a.scr.ModeName, a.imeInsertOnly()); want != a.textInput {
		a.setTextInput(want)
	}
}

func (a *App) setTextInput(on bool) {
	if on {
		C.SDL_StartTextInput(a.win)
	} else {
		C.SDL_StopTextInput(a.win)
		a.preedit = paint.Preedit{}
	}
	a.textInput = on
	a.imeArea = [5]int{} // send the input area again
	if a.trace != nil {
		a.tracef(time.Now(), "ime     text input %v (mode %s)", on, a.scr.ModeName)
	}
}

// setIMEArea tells the input method where the cursor (or the text being
// composed) is, so that its candidate window appears there. Only changes
// are sent.
func (a *App) setIMEArea(r paint.Rect, cursorX int) {
	d := float64(C.ng_pixel_density(a.win))
	pt := func(v int) int { return int(float64(v) / d) }
	v := [5]int{pt(r.X), pt(r.Y), max(1, pt(r.W)), max(1, pt(r.H)), pt(cursorX)}
	if v == a.imeArea {
		return
	}
	a.imeArea = v
	C.ng_set_text_input_area(a.win, C.int(v[0]), C.int(v[1]), C.int(v[2]), C.int(v[3]), C.int(v[4]))
}

// leavingTextMode turns the input method off as soon as a key that leaves
// Insert mode is pressed, before Neovim reports the mode change: otherwise
// a key typed right after <Esc> could still be taken by the input method.
// If the mode does not change (Terminal mode, an <Esc> mapping), the next
// flush turns it on again.
func (a *App) leavingTextMode() {
	if a.textInput && a.imeInsertOnly() {
		a.setTextInput(false)
	}
}

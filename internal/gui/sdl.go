package gui

/*
#cgo pkg-config: sdl3
#include <stdlib.h>
#include "sdl.h"
*/
import "C"

import (
	"errors"
	"runtime"
	"unsafe"
)

func init() {
	// SDL must be driven from the thread that initialized it.
	runtime.LockOSThread()
}

func sdlError(what string) error {
	return errors.New(what + ": " + C.GoString(C.SDL_GetError()))
}

// event is the Go copy of ng_event.
type event struct {
	typ    uint32
	key    uint32
	mod    uint16
	repeat bool
	text   string
	x, y   float32
	button int
	state  uint32
	drop   string
	// SDL_EVENT_TEXT_EDITING
	editStart, editLength int
}

func flatten(e *C.SDL_Event) event {
	var o C.ng_event
	C.ng_flatten(e, &o)
	ev := event{
		typ:        uint32(o._type),
		key:        uint32(o.key),
		mod:        uint16(o.mod),
		repeat:     o.repeat != 0,
		editStart:  int(o.edit_start),
		editLength: int(o.edit_length),
		x:          float32(o.x),
		y:          float32(o.y),
		button:     int(o.button),
		state:      uint32(o.state),
	}
	if ev.typ == C.SDL_EVENT_TEXT_INPUT || ev.typ == C.SDL_EVENT_TEXT_EDITING {
		if p := C.ng_event_text(e); p != nil {
			ev.text = C.GoString(p)
			C.SDL_free(unsafe.Pointer(p))
		}
	}
	if ev.typ == C.SDL_EVENT_DROP_FILE || ev.typ == C.SDL_EVENT_DROP_TEXT {
		if p := C.ng_drop_data(e); p != nil {
			ev.drop = C.GoString(p)
			C.SDL_free(unsafe.Pointer(p))
		}
	}
	return ev
}

// wake interrupts waitEvent from another goroutine.
func wake() { C.ng_wake() }

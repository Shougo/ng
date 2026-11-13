package gui

// #include "sdl.h"
// #include <stdlib.h>
import "C"

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
)

// iconPNG is assets/icons/hicolor/128x128/apps/ng.png. Under Wayland the
// compositor takes the icon from ng.desktop instead (the app ID is "ng").
//
//go:embed icon.png
var iconPNG []byte

// setIcon sets the window icon. Failing is harmless: the window manager
// shows its default icon.
func (a *App) setIcon() {
	img, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		return
	}
	b := img.Bounds()
	rgba := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	pix := C.CBytes(rgba.Pix)
	defer C.free(pix)
	C.ng_set_icon(a.win, C.int(b.Dx()), C.int(b.Dy()), pix)
}

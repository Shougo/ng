package gui

// #include "sdl.h"
// #include <stdlib.h>
import "C"

import (
	"image"
	"image/draw"
)

// imageTexture is the texture of an image placed with vim.ui.img.
type imageTexture struct {
	img image.Image // the image the texture was made from
	tex *C.SDL_Texture
}

// DrawImage implements paint.ImageDrawer. Textures are made on first use
// and kept until the image is deleted or replaced (see dropImages).
func (a *App) DrawImage(id uint32, img image.Image, src, dst image.Rectangle) {
	it, ok := a.imageTex[id]
	if !ok || it.img != img {
		if ok {
			C.SDL_DestroyTexture(it.tex)
		}
		b := img.Bounds()
		rgba := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
		pix := C.CBytes(rgba.Pix)
		tex := C.ng_image_texture(a.ren, C.int(b.Dx()), C.int(b.Dy()), pix)
		C.free(pix)
		if tex == nil {
			delete(a.imageTex, id)
			return
		}
		it = imageTexture{img, tex}
		a.imageTex[id] = it
	}
	b := img.Bounds()
	C.ng_draw_image(a.ren, it.tex,
		C.int(src.Min.X-b.Min.X), C.int(src.Min.Y-b.Min.Y), C.int(src.Dx()), C.int(src.Dy()),
		C.int(dst.Min.X), C.int(dst.Min.Y), C.int(dst.Dx()), C.int(dst.Dy()))
}

// dropImages frees the textures of images that were deleted or replaced.
func (a *App) dropImages() {
	for id, it := range a.imageTex {
		if a.scr.Graphics.Image(id) != it.img {
			C.SDL_DestroyTexture(it.tex)
			delete(a.imageTex, id)
		}
	}
}

package paint

import (
	"image"
	"image/color"

	"github.com/Shougo/ng/internal/font"
	"github.com/Shougo/ng/internal/screen"
)

// ImageDrawer draws a region of an image scaled into a rectangle of the
// window. id identifies the image (it may be cached by id); src is in the
// image's pixels, dst in the window's.
type ImageDrawer interface {
	DrawImage(id uint32, img image.Image, src, dst image.Rectangle)
}

// Images draws the images placed on the grid (screen.Graphics, from
// vim.ui.img), in stacking order, over the text: placements with a
// negative z, meant to be below the text, are also drawn over it for now.
// Images are clipped to the grid.
//
// A placement's size follows the kitty graphics protocol: with both
// columns and rows it fills those cells; with one of them the other
// follows the image's aspect ratio; with neither it is drawn at its own
// size in pixels.
func Images(d ImageDrawer, s *screen.Screen, f *font.Set) {
	grid := image.Rect(0, 0, s.Width*f.CellW, s.Height*f.CellH)
	for _, p := range s.Graphics.Placements() {
		img := s.Graphics.Image(p.Image)
		if img == nil {
			continue
		}
		b := img.Bounds()
		if b.Empty() {
			continue
		}
		w, h := b.Dx(), b.Dy()
		switch {
		case p.Cols > 0 && p.Rows > 0:
			w, h = p.Cols*f.CellW, p.Rows*f.CellH
		case p.Cols > 0:
			w = p.Cols * f.CellW
			h = w * b.Dy() / b.Dx()
		case p.Rows > 0:
			h = p.Rows * f.CellH
			w = h * b.Dx() / b.Dy()
		}
		if w <= 0 || h <= 0 {
			continue
		}
		dst := image.Rect(p.Col*f.CellW, p.Row*f.CellH, p.Col*f.CellW+w, p.Row*f.CellH+h)
		vis := dst.Intersect(grid)
		if vis.Empty() {
			continue
		}
		// The part of the image that lands in the visible part of dst.
		src := image.Rect(
			b.Min.X+(vis.Min.X-dst.Min.X)*b.Dx()/w,
			b.Min.Y+(vis.Min.Y-dst.Min.Y)*b.Dy()/h,
			b.Min.X+(vis.Max.X-dst.Min.X)*b.Dx()/w,
			b.Min.Y+(vis.Max.Y-dst.Min.Y)*b.Dy()/h,
		)
		if src.Empty() {
			continue
		}
		d.DrawImage(p.Image, img, src, vis)
	}
}

// DrawImage implements ImageDrawer with nearest-neighbour sampling and
// alpha blending.
func (p *ImagePainter) DrawImage(_ uint32, img image.Image, src, dst image.Rectangle) {
	dst = dst.Intersect(p.Img.Bounds())
	for y := dst.Min.Y; y < dst.Max.Y; y++ {
		sy := src.Min.Y + (y-dst.Min.Y)*src.Dy()/max(1, dst.Dy())
		for x := dst.Min.X; x < dst.Max.X; x++ {
			sx := src.Min.X + (x-dst.Min.X)*src.Dx()/max(1, dst.Dx())
			c := color.NRGBAModel.Convert(img.At(sx, sy)).(color.NRGBA)
			if c.A == 0 {
				continue
			}
			d := p.Img.RGBAAt(x, y)
			a := uint32(c.A)
			mix := func(s, d uint8) uint8 { return uint8((uint32(s)*a + uint32(d)*(255-a)) / 255) }
			p.Img.SetRGBA(x, y, color.RGBA{mix(c.R, d.R), mix(c.G, d.G), mix(c.B, d.B), 0xff})
		}
	}
}

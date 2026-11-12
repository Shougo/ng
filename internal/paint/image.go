package paint

import (
	"image"
	"image/color"

	"github.com/Shougo/ng/internal/font"
	"github.com/Shougo/ng/internal/screen"
)

// ImagePainter paints into an RGBA image. It is used by tests and can
// produce screenshots without a display.
type ImagePainter struct {
	Img *image.RGBA
}

// NewImagePainter returns a painter for a w x h image.
func NewImagePainter(w, h int) *ImagePainter {
	return &ImagePainter{Img: image.NewRGBA(image.Rect(0, 0, w, h))}
}

func rgba(c screen.Color) color.RGBA {
	r, g, b := c.RGB()
	return color.RGBA{r, g, b, 0xff}
}

// FillRect implements Painter.
func (p *ImagePainter) FillRect(x, y, w, h int, c screen.Color) {
	col := rgba(c)
	r := image.Rect(x, y, x+w, y+h).Intersect(p.Img.Bounds())
	for yy := r.Min.Y; yy < r.Max.Y; yy++ {
		for xx := r.Min.X; xx < r.Max.X; xx++ {
			p.Img.SetRGBA(xx, yy, col)
		}
	}
}

// DrawGlyph implements Painter.
func (p *ImagePainter) DrawGlyph(g *font.Glyph, x, y int, c screen.Color) {
	fr, fg, fb := c.RGB()
	b := p.Img.Bounds()
	for gy := 0; gy < g.H; gy++ {
		for gx := 0; gx < g.W; gx++ {
			px, py := x+gx, y+gy
			if px < b.Min.X || py < b.Min.Y || px >= b.Max.X || py >= b.Max.Y {
				continue
			}
			a := uint32(g.Pix[gy*g.Stride+gx])
			if a == 0 {
				continue
			}
			d := p.Img.RGBAAt(px, py)
			mix := func(s, d uint8) uint8 { return uint8((uint32(s)*a + uint32(d)*(255-a)) / 255) }
			p.Img.SetRGBA(px, py, color.RGBA{mix(fr, d.R), mix(fg, d.G), mix(fb, d.B), 0xff})
		}
	}
}

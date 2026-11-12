// Package atlas allocates space for glyphs in a texture atlas.
package atlas

// Packer is a shelf allocator for the glyph atlas: glyphs are placed left
// to right in rows ("shelves") as tall as the tallest glyph in them.
type Packer struct {
	w, h       int
	x, y, rowH int
}

const atlasPad = 1 // empty pixels between glyphs

// NewPacker returns an allocator for a w x h atlas.
func NewPacker(w, h int) *Packer { return &Packer{w: w, h: h} }

// Alloc reserves a w x h area. It returns false when the atlas is full.
func (p *Packer) Alloc(w, h int) (x, y int, ok bool) {
	if w+atlasPad > p.w || h+atlasPad > p.h {
		return 0, 0, false
	}
	if p.x+w+atlasPad > p.w {
		p.x, p.y, p.rowH = 0, p.y+p.rowH, 0
	}
	if p.y+h+atlasPad > p.h {
		return 0, 0, false
	}
	x, y = p.x, p.y
	p.x += w + atlasPad
	p.rowH = max(p.rowH, h+atlasPad)
	return x, y, true
}

// Reset frees the whole atlas.
func (p *Packer) Reset() { p.x, p.y, p.rowH = 0, 0, 0 }

// AlphaToRGBA expands an 8-bit coverage bitmap to white RGBA pixels whose
// alpha is the coverage, so that the vertex color tints the glyph.
func AlphaToRGBA(pix []byte, w, h, stride int, dst []byte) []byte {
	n := w * h * 4
	if cap(dst) < n {
		dst = make([]byte, n)
	}
	dst = dst[:n]
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := (y*w + x) * 4
			dst[o], dst[o+1], dst[o+2] = 0xff, 0xff, 0xff
			dst[o+3] = pix[y*stride+x]
		}
	}
	return dst
}

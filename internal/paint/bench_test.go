package paint

import (
	"testing"

	"github.com/Shougo/ng/internal/screen"
)

// fullScreen returns redraw params that fill a cols x rows screen with
// mixed ASCII and Japanese text in several highlights.
func fullScreen(cols, rows int) []any {
	params := []any{}
	for r := 0; r < rows; r++ {
		cells := []any{}
		col := 0
		for col < cols {
			switch {
			case col%20 < 10 && col+2 <= cols:
				cells = append(cells, []any{"漢", int64(col % 3)}, []any{""})
				col += 2
			default:
				cells = append(cells, []any{string(rune('a' + col%26)), int64(col % 3)})
				col++
			}
		}
		params = append(params, []any{"grid_line", []any{int64(1), int64(r), int64(0), cells, false}})
	}
	return append(params, []any{"flush"})
}

// BenchmarkFullRepaint measures applying a full-screen update for a
// 200x60 grid and painting every row (the worst case: e.g. <C-f>).
// The image painter is slower than the GPU, so this is an upper bound for
// the CPU side of a frame.
func BenchmarkFullRepaint(b *testing.B) {
	f, err := loadFontB()
	if err != nil {
		b.Skip(err)
	}
	defer f.Close()
	s := screen.New(200, 60)
	ev := fullScreen(200, 60)
	p := NewImagePainter(200*f.CellW, 60*f.CellH)
	rec := &recorder{}
	b.Run("screen-only", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			s.HandleRedraw(ev)
			s.TakeDirty()
		}
	})
	b.Run("ops", func(b *testing.B) {
		// Building draw operations without rasterizing to an image: the
		// work the SDL path does on the CPU before the GPU blits glyphs.
		for i := 0; i < b.N; i++ {
			s.HandleRedraw(ev)
			for _, r := range s.TakeDirty() {
				rec.rects = rec.rects[:0]
				Row(rec, s, f, r)
			}
		}
	})
	b.Run("image", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			s.HandleRedraw(ev)
			for _, r := range s.TakeDirty() {
				Row(p, s, f, r)
			}
		}
	})
}

package game

import (
	"image"
	"image/color"
)

// The original renderer submits fixed sprite parts in HUD/actor order.
// Limit the native painter to the same per-row part and tile-width budget.
// Transparent and horizontally off-screen parts still consume that budget.
type spriteLines struct{ Count, Pixels [Height]int }
type spriteSpan struct {
	drawTarget
	Left  int
	Width [Height]int
}
type drawTarget interface {
	image.Image
	Set(int, int, color.Color)
}

func (s *spriteSpan) Set(x, y int, c color.Color) {
	if y >= 0 && y < Height && x >= s.Left && x < s.Left+s.Width[y] {
		s.drawTarget.Set(x, y, c)
	}
}
func (b *spriteLines) reserve(left, top, width, height int) [Height]int {
	var rows [Height]int
	for y := max(0, top); y < min(Height, top+height); y++ {
		if b.Count[y] < 20 {
			rows[y] = max(0, min(width, 320-b.Pixels[y])) &^ 7
		}
		b.Count[y]++
		b.Pixels[y] += width
	}
	return rows
}

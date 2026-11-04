package game

import (
	"image"
	"image/draw"
)

// drawOver is draw.Draw(dst, r, src, sp, draw.Over) with a fast path for the
// paletted planes the converted stages use. The standard library has no
// specialised Paletted→RGBA path and goes through RGBA64At per pixel, which
// made it the largest cost of a frame. The result is bit-identical: the
// blend below is the same premultiplied formula image/draw uses.
func drawOver(dst *image.RGBA, r image.Rectangle, src image.Image, sp image.Point) {
	p, ok := src.(*image.Paletted)
	if !ok || len(p.Palette) == 0 || len(p.Palette) > 256 {
		draw.Draw(dst, r, src, sp, draw.Over)
		return
	}
	// Clip exactly as image/draw does.
	orig := r.Min
	r = r.Intersect(dst.Rect)
	r = r.Intersect(p.Rect.Add(orig.Sub(sp)))
	if r.Empty() {
		return
	}
	sp = sp.Add(r.Min.Sub(orig))

	var lut [256][4]uint32
	var valid [256]bool
	for i, c := range p.Palette {
		cr, cg, cb, ca := c.RGBA()
		lut[i] = [4]uint32{cr, cg, cb, ca}
		valid[i] = true
	}
	const m = 1<<16 - 1
	w := r.Dx()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		sy := sp.Y + y - r.Min.Y
		si := p.PixOffset(sp.X, sy)
		srow := p.Pix[si : si+w]
		di := dst.PixOffset(r.Min.X, y)
		drow := dst.Pix[di : di+4*w : di+4*w]
		for x, idx := range srow {
			if !valid[idx] {
				// Out-of-palette index: keep the standard library's behaviour.
				draw.Draw(dst, image.Rect(r.Min.X+x, y, r.Min.X+x+1, y+1), src, image.Pt(sp.X+x, sy), draw.Over)
				continue
			}
			s := &lut[idx]
			if s[3] == 0 {
				continue
			}
			d := drow[x*4 : x*4+4 : x*4+4]
			if s[3] == m {
				d[0], d[1], d[2], d[3] = uint8(s[0]>>8), uint8(s[1]>>8), uint8(s[2]>>8), uint8(s[3]>>8)
				continue
			}
			a := (m - s[3]) * 0x101
			d[0] = uint8((uint32(d[0])*a/m + s[0]) >> 8)
			d[1] = uint8((uint32(d[1])*a/m + s[1]) >> 8)
			d[2] = uint8((uint32(d[2])*a/m + s[2]) >> 8)
			d[3] = uint8((uint32(d[3])*a/m + s[3]) >> 8)
		}
	}
}

// atlasOpacity reports, per palette index of the sprite atlas, whether that
// colour is visible. The atlas never changes, so it is computed once.
func (g *Game) atlasOpacity(atlas *image.Paletted) *[256]bool {
	if g.atlasAlpha == nil || g.atlasAlphaOf != atlas {
		t := new([256]bool)
		for i, c := range atlas.Palette {
			if i < 256 {
				_, _, _, a := c.RGBA()
				t[i] = a != 0
			}
		}
		g.atlasAlpha, g.atlasAlphaOf = t, atlas
	}
	return g.atlasAlpha
}

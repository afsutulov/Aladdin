package game

import (
	"aladdin/internal/data"
	"image"
	"image/color"
)

type CreditRoll struct {
	Ops                                                 []data.CreditOp
	Op, Row, Scroll, Offset, Step, Half, FontBank, Tick int
	Tiles                                               [32][40]int
	Palette                                             [64][3]uint8
	Heading                                             [16][3]uint8
	SkipText, LastApple                                 bool
	Done                                                bool
}

func newCreditRoll(m data.MenuData) *CreditRoll {
	c := &CreditRoll{Ops: m.Credits, Row: 28, FontBank: 0x4000, Palette: m.CreditPalette, Heading: m.CreditHeading}
	c.line()
	c.Tick = 1
	return c
}
func (c *CreditRoll) line() {
	for c.Op < len(c.Ops) {
		op := c.Ops[c.Op]
		if c.SkipText && op.Kind == "line" {
			return
		}
		c.Op++
		if op.Kind == "fade" {
			for i := 0; i < 16; i++ {
				for ch := 0; ch < 3; ch++ {
					target := uint8(0)
					if op.Target != 0 {
						target = c.Heading[i][ch]
					}
					v := int(c.Palette[i][ch])
					if v < int(target) {
						v += 34
					} else if v > int(target) {
						v -= 34
					}
					c.Palette[i][ch] = uint8(v)
				}
			}
			continue
		}
		if !c.SkipText {
			for x := 0; x < 40; x++ {
				c.Tiles[c.Row][x] = c.FontBank
			}
		}
		for _, g := range op.Glyphs {
			if g.X < 0 || g.X >= 40 {
				continue
			}
			tile := g.Tile
			if tile&0x10000 != 0 {
				tile = tile&63 | c.FontBank
			}
			c.Tiles[(c.Row+g.Y)&31][g.X] = tile
		}
		if op.EndBank >= 0 {
			c.FontBank = op.EndBank
		}
		return
	}
	c.Done = true
}

// Two video boundaries precede each one-pixel scroll write (1DEECA).
func (c *CreditRoll) Update(in Input) {
	if c.Done {
		return
	}
	if c.LastApple {
		c.SkipText = true
	}
	if c.Half == 0 {
		c.Half = 1
		return
	}
	if in.Enter {
		c.Done = true
		return
	}
	c.LastApple = in.Apple
	c.Half = 0
	c.Scroll = c.Offset
	c.Offset = (c.Offset + 1) & 255
	c.Step++
	if c.Step == 16 {
		c.Step = 0
		c.Row = (c.Row + 2) & 31
		c.line()
	}
	if !c.Done {
		c.Tick = (c.Tick + 1) & 255
	}
}
func (g *Game) renderCredits() *image.RGBA {
	a := g.Attract
	c := a.Credits
	pal := c.Palette
	g.drawCreditText(c, pal, false)
	if a.Phase == "credits_in" {
		g.attractPaletteFade(a.Age, true)
	} else if a.Phase == "credits_out" {
		g.attractPaletteFade(a.Age, false)
	}
	return g.Canvas
}

func (g *Game) drawCreditText(c *CreditRoll, pal [64][3]uint8, transparent bool) {
	font, e := g.Project.Image("menu-font.png")
	if e != nil {
		g.Err = e
		return
	}
	f := font.(*image.Paletted)
	for y := 0; y < Height; y++ {
		sy := (y + c.Scroll) & 255
		for x := 0; x < Width; x++ {
			tile := c.Tiles[sy/8][x/8]
			n := tile & 63
			index := f.ColorIndexAt(n%16*8+x%8, n/16*8+sy%8)
			rgb := pal[0]
			if index != 0 {
				rgb = pal[(tile>>13&3)*16+int(index)]
			}
			if index != 0 || !transparent {
				g.Canvas.SetRGBA(x, y, color.RGBA{rgb[0], rgb[1], rgb[2], 255})
			}
		}
	}
}

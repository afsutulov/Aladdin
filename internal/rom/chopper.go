package rom

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// Normal-intensity Mega Drive digital RGB levels (the original 3-bit value
// occupies bits 1..3 of a 4-bit channel). Shadow/highlight are separate modes.
var ColorLevels = [8]uint8{0, 34, 68, 102, 136, 170, 204, 238}

type Part struct{ X, Y, W, H, Address int }
type Frame struct {
	Address int
	Box     [4]int
	Parts   []Part
}
type ROM struct{ Bytes []byte }

func (r ROM) U16(a int) int { return int(binary.BigEndian.Uint16(r.Bytes[a:])) }
func (r ROM) U32(a int) int { return int(binary.BigEndian.Uint32(r.Bytes[a:])) }
func (r ROM) Frame(a int) (f Frame, err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("frame outside ROM at %x", a)
		}
	}()
	if a < 0 || a+18 > len(r.Bytes) {
		return f, fmt.Errorf("invalid frame pointer")
	}
	n := r.U16(a) + 1
	if n > 128 {
		return f, fmt.Errorf("invalid part count")
	}
	f.Address = a
	for i := 0; i < 4; i++ {
		f.Box[i] = int(r.Bytes[a+2+i])
	}
	for i := 0; i < n; i++ {
		o := a + 6 + i*12
		info := r.U16(o)
		p := Part{X: int(r.Bytes[o+2]), Y: int(r.Bytes[o+3]), W: int(r.Bytes[info+8]), H: int(r.Bytes[info+9])}
		p.Address = (r.U16(o+4)-0x9500)<<1 | (r.U16(o+6)-0x9600)<<9 | (r.U16(o+8)-0x9700)<<17
		if p.W < 8 || p.H < 8 || p.W > 128 || p.H > 128 || p.W%8 != 0 || p.H%8 != 0 || p.Address < 0 || p.Address+p.W*p.H/2 > len(r.Bytes) {
			return f, fmt.Errorf("invalid tile part")
		}
		f.Parts = append(f.Parts, p)
	}
	return f, nil
}
func (r ROM) Frames() []Frame {
	var fs []Frame
	for a := 0x6ae; a+4 < len(r.Bytes); a += 4 {
		f, e := r.Frame(r.U32(a))
		if e != nil {
			break
		}
		fs = append(fs, f)
	}
	return fs
}

// MansPalette is the game's default character palette, as RGB triples.
func MansPalette() color.Palette {
	values := []int{0x707, 0x763, 0x751, 0x642, 0x531, 0x420, 0x310, 0x200, 0x401, 0x347, 0x236, 0x202, 0x444, 0x222, 0x777, 0}
	levels := ColorLevels
	p := make(color.Palette, 16)
	for i, v := range values {
		a := uint8(255)
		if i == 0 {
			a = 0
		}
		p[i] = color.NRGBA{levels[v>>8&7], levels[v>>4&7], levels[v&7], a}
	}
	return p
}
func (r ROM) RenderFrame(f Frame) *image.Paletted {
	im := image.NewPaletted(image.Rect(0, 0, 256, 256), MansPalette())
	for _, p := range f.Parts {
		for x := 0; x < p.W; x++ {
			for y := 0; y < p.H; y++ {
				o := p.Address + ((x/8)*(p.H/8)+y/8)*32 + (y%8)*4 + (x%8)/2
				v := r.Bytes[o]
				if x%2 == 0 {
					v >>= 4
				}
				v &= 15
				if v != 0 && image.Pt(p.X+x, p.Y+y).In(im.Rect) {
					im.SetColorIndex(p.X+x, p.Y+y, v)
				}
			}
		}
	}
	return im
}

// RenderPart decodes independent artwork without merged-canvas clipping.
func (r ROM) RenderPart(p Part) *image.Paletted {
	im := image.NewPaletted(image.Rect(0, 0, p.W, p.H), MansPalette())
	for x := 0; x < p.W; x++ {
		for y := 0; y < p.H; y++ {
			o := p.Address + ((x/8)*(p.H/8)+y/8)*32 + (y%8)*4 + (x%8)/2
			v := r.Bytes[o]
			if x%2 == 0 {
				v >>= 4
			}
			im.SetColorIndex(x, y, v&15)
		}
	}
	return im
}

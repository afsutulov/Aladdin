package rom

import (
	"aladdin/internal/data"
	"fmt"
)

func (r ROM) menuData(s *data.ScriptSet) error {
	m := &s.Menu
	m.Text = map[string][]data.MenuGlyph{}
	m.EndBanks = map[string]int{}
	for bank, a := range []int{0x151f34, 0x151f14, 0x151734, 0x1517d4} {
		for i := 0; i < 16; i++ {
			v := r.U16(a + 2*i)
			m.Palette[bank*16+i] = [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
		}
	}
	entries := []struct {
		name          string
		address, x, y int
	}{{"main", 0x14ed6e, 14, 18}, {"options", 0x14edad, 10, 18}, {"difficulty_label", 0x14ed9f, 10, 16}, {"on_music", 0x14ebf5, 21, 18}, {"off_music", 0x14ebf9, 21, 18}, {"on_fx", 0x14ebf5, 25, 20}, {"off_fx", 0x14ebf9, 25, 20}, {"tracking0", 0x14ed87, 21, 24}, {"tracking1", 0x14ed8f, 21, 24}, {"tracking2", 0x14ed97, 21, 24}, {"sound_top", 0x151570, 0, 0}, {"sound_bottom", 0x15156c, 12, 25}}
	for i := 0; i < 3; i++ {
		entries = append(entries, struct {
			name          string
			address, x, y int
		}{fmt.Sprintf("difficulty%d", i), r.U32(0x45ec + 4*i), 23, 16})
	}
	for _, e := range entries {
		g, bank, err := r.menuText(e.address, e.x, e.y)
		if err != nil {
			return fmt.Errorf("menu %s: %w", e.name, err)
		}
		m.Text[e.name] = g
		m.EndBanks[e.name] = bank
	}
	for pc := 0x14ee4a; r.Bytes[pc] != 255; pc += 16 {
		if pc > 0x15156c {
			return fmt.Errorf("unterminated sound menu")
		}
		end := pc + 1
		for end < pc+16 && r.Bytes[end] != 0 {
			end++
		}
		m.Sounds = append(m.Sounds, data.MenuSound{ID: int(r.Bytes[pc]), Label: string(r.Bytes[pc+1 : end])})
	}
	for i := 0; i < 5; i++ {
		entry := 0x5136 + 6*i
		address := r.U32(entry)
		d := data.AttractDemo{Stage: int(r.Bytes[entry+4])}
		for r.Bytes[address] != 0 {
			d.Inputs = append(d.Inputs, r.Bytes[address])
			address++
			if len(d.Inputs) > 4000 {
				return fmt.Errorf("unterminated attract input")
			}
		}
		m.Demos = append(m.Demos, d)
	}
	for i := 0; i < 64; i++ {
		v := r.U16(0x1525a2 + 2*i)
		m.CreditPalette[i] = [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
	}
	for i := 0; i < 16; i++ {
		v := r.U16(0x1523ac + 2*i)
		m.CreditHeading[i] = [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
	}
	for pc := 0x150596; pc < 0x151568; {
		v := r.Bytes[pc]
		if v == 0xfd {
			break
		}
		if v == 0xfe || v == 0xff {
			target := 0
			if v == 0xfe {
				target = 1
			}
			m.Credits = append(m.Credits, data.CreditOp{Kind: "fade", Target: target})
			pc++
			continue
		}
		if v == 7 {
			m.Credits = append(m.Credits, data.CreditOp{Kind: "blank", EndBank: -1})
			pc++
			continue
		}
		glyphs, bank, next, e := r.decodeMenuText(pc, 0, 0)
		if e != nil {
			return e
		}
		m.Credits = append(m.Credits, data.CreditOp{Kind: "line", Glyphs: glyphs, EndBank: bank})
		pc = next
	}
	return nil
}

// menuText converts the text data language into glyph positions once, offline.
// These are font/tile attributes and layout commands, never CPU instructions.
func (r ROM) menuText(pc, x, y int) ([]data.MenuGlyph, int, error) {
	g, b, _, e := r.decodeMenuText(pc, x, y)
	return g, b, e
}
func (r ROM) decodeMenuText(pc, x, y int) ([]data.MenuGlyph, int, int, error) {
	bank := -1
	var out []data.MenuGlyph
	emit := func(c int) {
		attr := bank
		if attr < 0 {
			attr = 0x10000
		}
		out = append(out, data.MenuGlyph{X: x, Y: y, Tile: attr | (c - 32)})
		x++
	}
	for budget := 0; budget < 4096; budget++ {
		v := int(r.Bytes[pc])
		pc++
		switch {
		case v == 0:
			return out, bank, pc, nil
		case v >= 32:
			emit(v)
		case v == 1:
			x += int(int8(r.Bytes[pc]))
			pc++
		case v == 2:
			y += int(int8(r.Bytes[pc]))
			pc++
		case v == 3 || v == 4:
			n, c := int(r.Bytes[pc]), int(r.Bytes[pc+1])
			pc += 2
			for i := 0; i < n; i++ {
				old := x
				emit(c)
				if v == 4 {
					x = old
					y++
				}
			}
		case v == 5:
			w, h, c := int(r.Bytes[pc]), int(r.Bytes[pc+1]), int(r.Bytes[pc+2])
			pc += 3
			for j := 0; j < h; j++ {
				old := x
				for i := 0; i < w; i++ {
					emit(c)
				}
				x = old
				y++
			}
		case v == 7:
			x = 0
			y++
		case v >= 8 && v <= 11:
			bank = (v - 8) * 0x2000
		default:
			return nil, bank, pc, fmt.Errorf("unconverted text command%d at%x", v, pc-1)
		}
	}
	return nil, bank, pc, fmt.Errorf("unterminated menu text")
}

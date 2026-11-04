package rom

import (
	"aladdin/internal/data"
	"fmt"
)

func (r ROM) storyData(s *data.ScriptSet) error {
	d := &s.Story
	d.Pages = map[int]data.StoryPage{}
	d.Layouts = map[int]data.StoryLayout{}
	d.Plans = map[int]data.StoryPlan{}
	d.Palettes = map[int][16][3]uint8{}
	palette := func(a int) {
		if _, ok := d.Palettes[a]; ok {
			return
		}
		var p [16][3]uint8
		for i := range p {
			v := r.U16(a + i*2)
			p[i] = [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
		}
		d.Palettes[a] = p
	}
	for _, v := range []data.StoryLayout{{Source: 0x1df04a, Background: 0x155d76, Palette: 0x152014, Characters: 0x15b6b2}, {Source: 0x1df078, Background: 0x156be0, Palette: 0x1520f4, Characters: 0x15b6b2}, {Source: 0x1df0a6, Background: 0x15581a, Palette: 0x151f34, Characters: 0x15f036}, {Source: 0x1df0d4, Background: 0x156f0c, Palette: 0x152134, Characters: 0x15b6b2}, {Source: 0x1df102, Palette: 0x151ff4, Characters: 0x15b6b2}, {Source: 0x1df120, Palette: 0x1517d4, Characters: 0x15b6b2}} {
		d.Layouts[v.Source] = v
		palette(v.Palette)
	}
	for _, a := range []int{0x151fd4, 0x1515f4, 0x151f34} {
		palette(a)
	}
	entries := []struct{ page, text, fg, p0, p1, x, y int }{
		{0x1df36c, 0x14f7b2, 0x156a6c, 0x1520d4, 0x1521b4, 21, 8},
		{0x1df392, 0x14f83e, 0x156898, 0x1520b4, 0x1521b4, 2, 10},
		{0x1df3b8, 0x14f911, 0x156498, 0x152074, 0x1521b4, 17, 5},
		{0x1df3de, 0x14fa42, 0x156498, 0x152074, 0x1521b4, 19, 5},
		{0x1df404, 0x14faf3, 0x156126, 0x152054, 0x1521d4, 19, 3},
		{0x1df42a, 0x14fc7b, 0x156498, 0x152074, 0x1521b4, 18, 8},
		{0x1df450, 0x14fcf8, 0x156498, 0x152074, 0x1521b4, 17, 7},
		{0x1df476, 0x14fe37, 0x155f92, 0x152034, 0x1521d4, 21, 10},
		{0x1df49c, 0x14fecf, 0, 0x1515f4, 0x1515f4, 4, 5},
		{0x1df4c2, 0x14ff3e, 0, 0x151f34, 0x151f34, 5, 12},
		{0x1df588, 0x150035, 0x15668e, 0x152094, 0x1521d4, 18, 7},
		{0x1df5ae, 0x1500c1, 0, 0x1515f4, 0x1515f4, 3, 6},
		{0x1df5d4, 0x1501f8, 0, 0x1515f4, 0x1515f4, 3, 12},
		{0x1df5fa, 0x15026a, 0, 0x1515f4, 0x1515f4, 3, 11},
		{0x1df620, 0x1502dc, 0, 0x1515f4, 0x1515f4, 3, 11},
		{0x1df646, 0x15034c, 0, 0x1515f4, 0x1515f4, 3, 11},
		{0x1df66c, 0x1503be, 0, 0x1515f4, 0x1515f4, 3, 11},
		{0x1df692, 0x15047e, 0, 0x1515f4, 0x1515f4, 3, 6},
	}
	for _, v := range entries {
		ops, e := r.storyText(v.text, v.x, v.y)
		if e != nil {
			return e
		}
		d.Pages[v.page] = data.StoryPage{Source: v.page, Text: v.text, Foreground: v.fg, Palette: [2]int{v.p0, v.p1}, Ops: ops, Scroll: v.page == 0x1df4c2}
		// Observed 1DCC46 resource transfers, independently recorded before conversion.
		waits := map[int]int{0x155f92: 1, 0x156126: 2, 0x156498: 1, 0x156898: 1}
		page := d.Pages[v.page]
		page.ReloadWait = waits[v.fg]
		if v.fg == 0x156126 {
			page.ReloadRows = []int{22}
			page.ReloadRaster = []int{160}
			page.ReloadRasterColumns = []int{18} // Tile-map upload overtakes the video beam within this text row.
		} // RNC tile upload clears upper rows before the next video's lower text.
		if v.fg == 0x156498 {
			page.ReloadGlyphDelay = 1
		}
		if v.page == 0x1df4c2 {
			page.Ops[4].TransferWait = 1
		} // 90-glyph rewrite crosses a video boundary after clear.
		d.Pages[v.page] = page

		palette(v.p0)
		palette(v.p1)
	}
	d.Plans[0] = data.StoryPlan{Music: 85, Steps: []data.StoryStep{{Layout: 0x1df078, Page: 0x1df3de}}}
	d.Plans[1] = data.StoryPlan{Music: 84, Steps: []data.StoryStep{{Layout: 0x1df0d4, Page: 0x1df5ae}, {Layout: 0x1df0d4, Page: 0x1df36c, CheckSkip: true}, {Layout: 0x1df120, Page: 0x1df392, CheckSkip: true}}}
	d.Plans[3] = data.StoryPlan{Music: 85, Steps: []data.StoryStep{{Layout: 0x1df0d4, Page: 0x1df5d4}, {Layout: 0x1df04a, Page: 0x1df3b8, CheckSkip: true}}}
	d.Plans[4] = data.StoryPlan{Music: 86, Steps: []data.StoryStep{{Layout: 0x1df0d4, Page: 0x1df620}, {Layout: 0x1df0d4, Page: 0x1df404, CheckSkip: true}, {Layout: 0x1df102, Page: 0x1df66c, CheckSkip: true}}}
	d.Plans[5] = data.StoryPlan{Music: 85, Steps: []data.StoryStep{{Layout: 0x1df102, Page: 0x1df42a, CheckSkip: true}, {Layout: 0x1df102, Page: 0x1df450, CheckSkip: true}}}
	d.Plans[7] = data.StoryPlan{Music: 85, Steps: []data.StoryStep{{Layout: 0x1df04a, Page: 0x1df476, CheckSkip: true}}}
	d.Plans[9] = data.StoryPlan{Music: 85, Steps: []data.StoryStep{{Layout: 0x1df0d4, Page: 0x1df692}}}
	d.Plans[10] = data.StoryPlan{Music: 86, Steps: []data.StoryStep{{Layout: 0x1df0a6, Page: 0x1df4c2, CheckSkip: true}}}
	d.Plans[11] = data.StoryPlan{Music: 85, Steps: []data.StoryStep{{Layout: 0x1df0d4, Page: 0x1df646}, {Layout: 0x1df102, Page: 0x1df588, CheckSkip: true}}}
	// Black resource-transfer intervals between pages. Counts belong to these
	// layout/portrait jobs, not a global replay frame or captured actor state.
	transferFrames := map[int][]int{1: {0, 61, 59}, 3: {0, 61}, 4: {0, 61, 58}, 5: {0, 59}, 11: {0, 59}}
	for stage, counts := range transferFrames {
		plan := d.Plans[stage]
		for i, n := range counts {
			plan.Steps[i].LoadFrames = n
			plan.Steps[i].SkipBeforeLoad = i > 0 && stage != 3
			if stage == 3 && i > 0 {
				plan.Steps[i].SkippedLoadFrames = 58
			}
		}
		d.Plans[stage] = plan
	}

	return nil
}

func (r ROM) storyText(pc, x, y int) ([]data.StoryOp, error) {
	bank := 0
	var out []data.StoryOp
	var glyphs []data.MenuGlyph
	flush := func() {
		if len(glyphs) > 0 {
			out = append(out, data.StoryOp{Kind: "glyphs", Glyphs: glyphs})
			glyphs = nil
		}
	}
	emit := func(c int) { glyphs = append(glyphs, data.MenuGlyph{X: x, Y: y, Tile: bank | 0x8000 | (c - 32)}); x++ }
	for budget := 0; budget < 8192; budget++ {
		if pc >= len(r.Bytes) {
			return nil, fmt.Errorf("story text outside ROM")
		}
		v := int(r.Bytes[pc])
		pc++
		switch {
		case v == 0:
			flush()
			return out, nil
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
			if n == 0 {
				n = 256
			}
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
		case v == 6:
			flush()
			n := int(r.Bytes[pc])
			pc++
			if n == 0 {
				n = 256
			}
			out = append(out, data.StoryOp{Kind: "wait", Count: n})
		case v == 7:
			x = 0
			y++
		case v >= 8 && v <= 11:
			bank = (v - 8) * 0x2000
		case v == 12:
			pc += 3
		case v == 13 || v == 14:
			flush()
			kind := "clear"
			if v == 14 {
				kind = "fade-clear"
			}
			out = append(out, data.StoryOp{Kind: kind})
		case v == 15:
			flush()
			out = append(out, data.StoryOp{Kind: "spawn", Template: r.U32(pc), X: r.U16(pc + 4), Y: r.U16(pc + 6)})
			pc += 8
		default:
			return nil, fmt.Errorf("unknown story command %d at %x", v, pc-1)
		}
	}
	return nil, fmt.Errorf("unterminated story text")
}

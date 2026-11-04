package rom

import "aladdin/internal/data"

// preludeData converts original graphics/palettes and source load-job timings.
// No CPU instructions or reference runner are consumed by the desktop game.
func (r ROM) preludeData(s *data.ScriptSet) {
	d := &s.Prelude
	d.Maps = map[int]int{0x157134: 0x15a110, 0x158ef9: 0x168906, 0x1595c5: 0x168906, 0x159744: 0x168906}
	color := func(a int) [3]uint8 {
		v := r.U16(a)
		return [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
	}
	for i := 0; i < 16; i++ {
		d.ScarabPalette[32+i] = color(0x152274 + i*2)
		d.ScarabPalette[48+i] = color(0x1517d4 + i*2)
		d.ScarabFlash[i] = color(0x152294 + i*2)
	}
	for i := 0; i < 32; i++ {
		d.ScarabCycle[i] = color(0x1522b4 + i*2)
	}
	for i := 0; i < 64; i++ {
		d.CardPalette[i] = color(0x15240c + i*2)
	}
	// The two sprite-art warm-up boundaries follow the map/character load.
	d.ScarabLoad = 24
	d.ScarabWarm = 2
	d.CardLoad = 54
	d.CardMapLoad = 1
	d.StoryLoad = 60
	e := &s.Ending
	e.Maps = map[int]int{0x156d88: 0x15b6b2, 0x158a2f: 0x164aa0}
	for i := 0; i < 16; i++ {
		e.FlightPalette[16+i] = color(0x152114 + i*2)
	}
	e.FlightPalette[0] = [3]uint8{0, 0, 34}
	for i := range e.KissPalette {
		e.KissPalette[i] = color(0x152522 + i*2)
	}
	// The kiss job includes its separate constructor/artwork boundary.
	e.FlightLoad, e.KissLoad = 58, 57
	t := &s.LevelTitle
	t.Maps = map[int]int{0x157ac0: 0x15f036, 0x157850: 0x15f036}
	for i := range t.Palette {
		t.Palette[i] = color(0x1521f4 + i*2)
	}
	for i := range t.Wave {
		t.Wave[i] = int(int16(r.U16(0x2380 + i*2)))
	}
	// Measured map B, map A and character transfer jobs at 1DBADC..1DBB06.
	t.LoadFrames = 62
}

package game

import "aladdin/internal/data"

// palaceCarpetMarker reconstructs the paired constructors at 1E12A2 onward.
// Failure to allocate the companion intentionally leaves the first actor and
// consumed map marker in place, as in the original fixed-pool constructors.
func (w *NativeWorld) palaceCarpetMarker(m data.Marker) (bool, error) {
	motion := map[int]int{0xe0: 0x146af6, 0xe1: 0x146bf4, 0xe2: 0x146bf4, 0xe4: 0x147068, 0xe8: 0x14754e, 0xe9: 0x14761a}[m.Type]
	if motion == 0 {
		return false, nil
	}
	for i := 1; i <= 24; i++ {
		if o := w.Objects[i]; o != nil && o.Record[0] == 0x65 {
			return true, nil
		}
	}
	pool := 1
	if m.Type == 0xe4 {
		pool = 0
	}
	first, err := w.spawnMarker(m, markerRecipe{template: 0x1e2e36, pool: pool, tiles: -1})
	if err != nil || len(first) == 0 {
		return true, err
	}
	w.set("flag_f607", 0)
	second, err := w.spawnMarker(m, markerRecipe{template: 0x1e2e5e, pool: 1, tiles: -1, motion: motion})
	if err != nil || len(second) == 0 {
		return true, err
	}
	first[0].Partner, second[0].Partner = second[0], first[0]
	x, y := second[0].Position()
	if m.Type != 0xe0 {
		x -= 32
	}
	w.set("carpet_target_x", x&65535)
	w.set("carpet_target_y", (y+64)&65535)
	return true, nil
}

// palaceScroll ports 1D5406. Its caller supplies the completed drift phase;
// additions and logical shifts retain their original unsigned word width.
func palaceScroll(camera, phase int) [28]int {
	n, p := uint16(-camera), uint16(phase)
	a := (n + p + p) >> 2 & 511
	t := n + p + p
	b := ((t >> 1) + t) >> 2 & 511
	c := ((n >> 1) + n) >> 2 & 511
	d := n >> 1 & 511
	return [28]int{0, 0, 0, int(a), int(a), int(b), int(b), int(c), int(c), int(c), int(c), int(c), int(c), int(c), int(c), int(c), int(c), int(c), int(c), int(c), int(c), int(c), int(d), int(d), int(d), int(d), int(d), int(d)}
}

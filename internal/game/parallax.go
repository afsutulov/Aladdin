package game

// caveScroll preserves the half-speed rear plane at 1D589E.
func caveScroll(x int) int { return int(-((uint16(x) + 16) >> 1) & 511) }

// dungeonScroll is 1D592C. The room occupies the rear plane in this stage;
// the front plane wraps at 512 by 256 and moves at three halves speed.
func dungeonScroll(x, y int) (int, int) {
	px, py := uint16(x), uint16(y)
	return int(-((px+px+px)>>1 + 16) & 511), int(((py+py+py)>>1 + 64) & 255)
}

// agrabahScroll reconstructs the 28 row-scroll values at 1D50F8. Arithmetic
// before a logical shift wraps at the original 16-bit width.
func agrabahScroll(camera, phase int) [28]int {
	n, p := uint16(-camera), uint16(phase)
	t := (n + p) >> 1
	t += p
	a := int(t >> 1 & 511)
	b := int((n + p) >> 2 & 511)
	c := int((n + p) >> 3 & 511)
	d := (n >> 1) + n
	return [28]int{a, a, b, 0, b, c, c, 0, 0, 0, 0, 0, int(n >> 3 & 511), int(n >> 3 & 511), int(n >> 2 & 511), int(d >> 2 & 511), int(d >> 2 & 511), int(n >> 1 & 511), int(n >> 1 & 511), int(n >> 1 & 511), int(n & 511), int(n & 511), int(n & 511), int(n & 511), int(n & 511), int(n & 511), int(n & 511), int(n & 511)}
}

// desertScroll converts 1D5284. Each row group retains its own original
// unsigned-word additions and logical shifts, including wrap at 65536.
func desertScroll(camera, phase int) [28]int {
	n, p := uint16(-camera), uint16(phase)
	t := n + p + p
	a := ((t >> 1) + t) >> 1 & 511
	b := ((t >> 1) + t) >> 2 & 511
	c := t >> 2 & 511
	d := (n + p) >> 2 & 511
	e := ((n >> 1) + n) >> 2 & 511
	f := n >> 1 & 511
	return [28]int{0, 0, 0, 0, int(a), int(a), int(b), int(b), int(c), int(d), int(e), int(e), int(e), int(e), int(e), int(e), int(e), int(f), int(f), int(f), int(f), int(f), int(f), int(f), int(f), int(f), int(f), int(f)}
}

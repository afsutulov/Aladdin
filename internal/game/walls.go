package game

// playerWalls reconstructs the original left/right/overhead tile sensors
// (1D7D60..1D7EF0). The collision-class byte is independent of floor contour.
func (w *NativeWorld) playerWalls() {
	w.set("ceiling", 0)
	if w.Stage.Index == 8 || w.Objects[0].Frame < 0 {
		return
	}
	x, y := w.PlayerPosition()
	physicalY := y - 0x110
	if physicalY < 0 || physicalY >= w.Stage.Height-0x30 {
		return
	}
	row, col := physicalY>>4, x>>4
	columns := w.Stage.Width / 16
	solid := func(r, c int) bool {
		// The source advances a room pointer across row boundaries. Preserve
		// this indexing rather than clamping the column to its current row.
		i := r*columns + c
		if i < 0 || i >= len(w.Stage.Room) {
			return false
		}
		f := int(w.Stage.Room[i])/2 + 2
		return f < len(w.Stage.Floor) && w.Stage.Floor[f] > 0xdf
	}
	for _, side := range []struct {
		offset, outward int
		wall, near, far string
	}{
		{0, -1, "wall_left", "flag_f5e8", "flag_f5e9"},
		{2, 1, "wall_right", "flag_f5eb", "flag_f5ec"},
	} {
		w.set(side.wall, 0)
		w.set(side.near, 0)
		w.set(side.far, 0)
		c := col + side.offset
		if solid(row, c) || solid(row+1, c) {
			w.set(side.wall, 255)
			continue
		}
		if solid(row+1, c+side.outward) {
			w.set(side.near, 255)
		}
		if solid(row+1, c+2*side.outward) {
			w.set(side.far, 255)
		}
		if solid(row+2, c) || (!w.flag("grounded") && !w.flag("flag_f653") && solid(row+3, c)) {
			w.set(side.wall, 255)
		}
	}
	if solid(row, col+1) {
		w.set("ceiling", 255)
	}
}

// blockedWalk is the native wall-contact stance at 1D477C..1D4808.
func (w *NativeWorld) blockedWalk() {
	w.set("momentum_x", 0)
	w.set("run_speed", 0)
	w.set("walking", 0)
	if w.flag("jumping") {
		return
	}
	if w.get("flag_f5e5") == 0xb0 || w.flag("on_object") {
		w.animation("idle")
		w.set("look_target_y", 0x188)
		w.set("crouching", 0)
		w.set("looking_up", 0)
		return
	}
	if !w.flag("grounded") || w.flag("stopping") {
		return
	}
	w.animation("stop")
	w.set("stopping", 255)
	w.set("crouching", 0)
	w.set("looking_up", 0)
	w.set("landing_combo", 0)
}

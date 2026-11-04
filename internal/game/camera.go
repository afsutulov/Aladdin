package game

import "fmt"

// camera reconstructs the table-driven camera at 1D4F30..1D5098. The
// one-pixel fallback runs on alternate video frames in normal camera mode.
func (w *NativeWorld) advanceCameraTiles() {
	if w.Stage.Index == 8 {
		return
	}
	// The original tile streaming window advances by one tile on the next
	// video frame, with a signed fine offset in [-15, 15]. Its boundary is
	// also used by the camera, so rounding the pixel position is incorrect.
	for _, p := range [][2]string{{"camera_x", "camera_tile_x"}, {"camera_y", "camera_tile_y"}} {
		coarse := w.get(p[1])
		fine := w.get(p[0]) - coarse
		if fine >= 16 {
			coarse += 16
		} else if fine <= -16 {
			coarse -= 16
		}
		w.set(p[1], coarse)
	}
}
func (w *NativeWorld) camera() error {
	if n := w.get("camera_delay"); n != 0 {
		w.set("camera_delay", n-1)
		return nil
	}
	if w.flag("flag_f6a3") {
		return nil
	}
	x, y := w.PlayerPosition()
	for axis, a := range []struct {
		position, target, dimension, margin int
		field                               string
		table                               []uint8
	}{
		{x, w.get("look_target_x"), w.Stage.Width, 0x161, "camera_x", w.Scripts.CameraX},
		{y, w.get("look_target_y"), w.Stage.Height, 0xf1, "camera_y", w.Scripts.CameraY},
	} {
		old := w.get(a.field)
		screen := uint16(a.position - old)
		delta := screen - uint16(a.target)
		negative := int16(delta) < 0
		if axis == 0 {
			negative = screen < uint16(a.target)
		}
		if delta == 0 {
			continue
		}
		if negative {
			delta = -delta
		}
		distance := int(int16(delta))
		rate := 0
		if axis == 0 && distance < 0 && distance >= -len(w.Scripts.CameraXPrefix) {
			rate = int(w.Scripts.CameraXPrefix[len(w.Scripts.CameraXPrefix)+distance])
		} else if distance < 0 || distance >= len(a.table) {
			return fmt.Errorf("camera %d distance %d outside original rate table", axis, distance)
		} else {
			rate = int(a.table[distance])
		}
		if rate == 0 {
			if axis == 1 && w.flag("on_object") && w.get("standing_object") == 0x59 {
				continue
			}
			switch w.get("camera_rate_mode") {
			case 0:
				rate = w.get("tick") & 1
			case 1:
				rate = 0
			default:
				rate = 1
			}
		}
		if negative {
			coarse := w.get("camera_tile_x")
			if axis == 1 {
				coarse = w.get("camera_tile_y")
			}
			if coarse >= 17 {
				w.set(a.field, old-rate)
			}
		} else if old+rate < a.dimension-a.margin {
			w.set(a.field, old+rate)
		}
	}
	return nil
}

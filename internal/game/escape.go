package game

import (
	"aladdin/internal/choreography"
	"aladdin/internal/data"
)

// escapeBehavior ports 1E05DC..1E08D8. Allocation failure in a periodic
// region returns before the shared boulder cooldown, as in the source.
func (w *NativeWorld) escapeBehavior() {
	x, y := w.PlayerPosition()
	if !w.flag("exit_countdown") && uint16(x) >= 0x27c5 {
		w.set("exit_countdown", 255)
	}
	if w.get("tick")&127 == 0 {
		px, py, region := 0, 0, 0
		if uint16(x) >= 0x1730 && uint16(x) < 0x1920 && uint16(y) >= 0x100 && uint16(y) < 0x230 {
			px, py, region = 0x18f0, 0x100, 1
		} else if uint16(x) >= 0x6a0 && uint16(x) < 0x920 && uint16(y) >= 0x100 && uint16(y) < 0x250 {
			py, region = 0x110, 2
		} else if uint16(x) >= 0x9d0 && uint16(x) < 0xb20 && uint16(y) >= 0x100 && uint16(y) < 0x250 {
			px, py, region = 0xae0, 0x110, 3
		}
		if region != 0 {
			slot := w.markerSlot(3)
			if slot < 0 {
				return
			}
			if region == 2 {
				px = 0x820
				if byte(w.Random()) >= 128 {
					px = 0x860
				}
			}
			o := choreography.FromTemplate(w.Scripts.Templates[0x1e393a])
			o.SetWord(26, 0x400)
			o.SetWord(2, px)
			o.SetWord(4, py)
			w.Objects[slot] = o
		}
	}
	if cooldown := w.get("flag_f635"); cooldown != 0 {
		w.set("flag_f635", (cooldown-1)&255)
		return
	}
	for _, g := range []struct{ left, right, x, y int }{
		{0x360, 0x370, 0x310, 0x110}, {0xe20, 0xe30, 0xdc0, 0x110}, {0x1270, 0x1280, 0x1230, 0x160},
		{0x1550, 0x1560, 0x1530, 0x180}, {0x1cd0, 0x1ce0, 0x1d80, 0x100}, {0x2260, 0x2270, 0x21f0, 0x100},
	} {
		if uint16(x) < uint16(g.left) || uint16(x) >= uint16(g.right) {
			continue
		}
		slot := w.markerSlot(3)
		if slot < 0 {
			return
		}
		o := choreography.FromTemplate(w.Scripts.Templates[0x1e394e])
		o.Record[9] = 255
		o.SetWord(2, g.x)
		o.SetWord(4, g.y)
		w.Objects[slot] = o
		w.set("flag_f635", 60)
		return
	}
}

func (w *NativeWorld) escapeMarker(m data.Marker) (bool, error) {
	if m.Type != 0xfb {
		return false, nil
	}
	x, _ := w.PlayerPosition()
	if uint16(x) < 0x2646 {
		return true, nil
	}
	spawned, err := w.spawnMarker(m, markerRecipe{template: 0x1e2e36, pool: 1, tiles: -1, motion: 0x146aa4})
	if len(spawned) > 0 {
		w.set("flag_f607", 0)
	}
	return true, err
}

func escapeJitter(tick int) int {
	if tick&255 < 32 {
		return tick & 3
	}
	return 0
}

// escapeScroll is 1D56D6. Upper and lower row groups use different speeds
// and a complementary horizontal quake phase during the first 32 ticks.
func escapeScroll(camera, tick int) [28]int {
	x := uint16(camera)
	if tick&255 < 32 {
		x += uint16(^tick & 3)
	}
	n := -((x + 16) >> 1)
	var rows [28]int
	for i := range rows {
		rows[i] = int(n & 511)
		if i < 21 {
			rows[i] = int(n >> 1 & 511)
		}
	}
	return rows
}

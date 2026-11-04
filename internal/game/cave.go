package game

import (
	"aladdin/internal/choreography"
	"aladdin/internal/data"
)

// advanceScreenShake follows the stage scroll callback. Its terminating zero
// leaves the previous sprite offset intact.
func (w *NativeWorld) advanceScreenShake() {
	if (w.Stage.Index == 5 || w.Stage.Index == 6 || w.Stage.Index == 7) && w.screenShake() != 0 {
		w.set("sprite_shake", int(uint16(w.screenShake())))
		w.set("screen_shake_cursor", w.get("screen_shake_cursor")+2)
	}
}

func (w *NativeWorld) screenShake() int {
	i := (w.get("screen_shake_cursor") - 0x7680) / 2
	if i < 0 || i >= len(w.Scripts.ScreenShake) {
		return 0
	}
	return w.Scripts.ScreenShake[i]
}

// caveMarker handles constructors whose allocation and random-call order
// cannot be described by a single marker recipe.
func (w *NativeWorld) caveMarker(m data.Marker) (bool, error) {
	switch m.Type {
	case 0xd7: // 1E116A: one or two independently scattered decorations.
		spawn := func(second bool) (bool, error) {
			v, err := w.spawnMarker(m, markerRecipe{template: 0x1e3476, pool: 2, tiles: -1})
			if err != nil || len(v) == 0 {
				return false, err
			}
			o := v[0]
			if second {
				o.SetWord(4, int(o.Word(4))+8)
				o.Record[9] = 255
			}
			o.SetWord(2, int(o.Word(2))+int(w.Random()&7)-3)
			r := w.Random()
			if r&1 != 0 {
				o.Animation = 0x14bf90
			}
			if r&2 != 0 {
				o.Record[53] = 255
			}
			return true, nil
		}
		if byte(w.Random()) < 200 {
			ok, err := spawn(false)
			if !ok || err != nil {
				return true, err
			}
			if byte(w.Random()) >= 200 {
				return true, nil
			}
		}
		_, err := spawn(true)
		return true, err
	case 0xf9: // 1E1508: the summoned carpet and its hidden companion.
		if !w.flag("flag_f634") {
			return true, nil
		}
		first, err := w.spawnMarker(m, markerRecipe{template: 0x1e2e36, pool: 1, tiles: -1})
		if err != nil || len(first) == 0 {
			return true, err
		}
		w.set("flag_f607", 0)
		second, err := w.spawnMarker(m, markerRecipe{template: 0x1e2e5e, pool: 1, tiles: -1, motion: 0x14742e})
		if err != nil || len(second) == 0 {
			return true, err
		}
		first[0].Partner = second[0]
		second[0].Partner = first[0]
		x, y := second[0].Position()
		w.set("carpet_target_x", (x-32)&65535)
		w.set("carpet_target_y", (y+64)&65535)
		return true, nil
	}
	return false, nil
}

// caveFallingRocks ports the two periodic spawn regions at 1E0494.
func (w *NativeWorld) caveFallingRocks() {
	if w.get("tick")&63 != 0 {
		return
	}
	x, y := w.PlayerPosition()
	region := 0
	if uint16(x) >= 0x20 && uint16(x) < 0x1a0 && uint16(y) >= 0x110 && uint16(y) < 0x320 {
		region = 1
	} else if uint16(x) >= 0xb80 && uint16(x) < 0xc78 && uint16(y) >= 0x218 && uint16(y) < 0x3b0 {
		region = 2
	}
	if region == 0 {
		return
	}
	slot := w.markerSlot(3)
	if slot < 0 {
		return
	}
	o := choreography.FromTemplate(w.Scripts.Templates[0x1e3912])
	o.SetWord(26, 0x400)
	n := w.Random() & 3
	positions := [3][2]int{{0x10, 0x140}, {0x60, 0x118}, {0x1a8, 0x110}}
	if region == 2 {
		positions = [3][2]int{{0xc70, 0x178}, {0xbe8, 0x190}, {0xbc8, 0x198}}
	}
	index := 0
	if n >= 2 {
		index = int(n) - 1
	}
	o.SetWord(2, positions[index][0])
	o.SetWord(4, positions[index][1])
	w.Objects[slot] = o
}

// breakCaveStatue converts the progress latches and room-cell deletion at
// 1D91AA/1D6A4A/1D6A80. A private room copy keeps resets and other games intact.
func (w *NativeWorld) breakCaveStatue(kind byte) {
	flag := "flag_f630"
	if kind == 0x1f {
		flag = "flag_f631"
	} else if kind == 0x20 {
		flag = "flag_f632"
	}
	w.set(flag, 255)
	if kind == 0x1e {
		return
	}
	if w.RemovedCells == nil {
		stage := *w.Stage
		stage.Room = append([]uint16(nil), stage.Room...)
		w.Stage = &stage
		w.RemovedCells = map[int]bool{}
	}
	a, b := byte(0x4f), byte(0xfe)
	if kind == 0x20 {
		a, b = 0x4e, 0xfd
	}
	for i, tile := range w.Stage.Room {
		f := int(tile)/2 + 2
		if f < len(w.Stage.Floor) && (w.Stage.Floor[f] == a || w.Stage.Floor[f] == b) {
			w.Stage.Room[i] = 0
			w.RemovedCells[i] = true
		}
	}
}

// caveGuardianContact is 1D9212. The guardian counts accepted sword
// contacts separately from ordinary enemy HP and unlocks the carpet at ten.
func (w *NativeWorld) caveGuardianContact(o *choreography.Object) error {
	if !w.flag("sword_contact") {
		w.damagePlayer()
		return nil
	}
	if o.Record[0] == 0x1d {
		if !w.flag("invulnerability") {
			w.Sound(0x21, false)
			w.damagePlayer()
		}
		return nil
	}
	x, _ := o.Position()
	px, _ := w.PlayerPosition()
	if !((!w.flag("facing_left") && uint16(px) < uint16(x)) || (w.flag("facing_left") && uint16(px) >= uint16(x))) {
		return nil
	}
	w.set("flag_f648", (w.get("flag_f648")+1)&255)
	if w.get("flag_f648") >= 10 {
		o.Record[0] = 0x8c
		// This direct replacement intentionally leaves the controller's
		// link bit and slot reference intact (unlike choreography deletion).
		if err := w.replaceActor(o, 0x1e334a); err != nil {
			return err
		}
		w.set("flag_f634", 255)
		w.set("pending_score", w.get("pending_score")+int(o.Record[8]))
		w.Sound(0x23, false)
		return nil
	}
	o.Record[0] = 0x8c
	o.Animation = 0x14c6c4
	o.AnimationDelay = 0
	o.Motion = 0
	o.MotionDelay = 0
	w.Sound(8, false)
	label, err := w.ResumePlayer(w.Objects[0])
	if err != nil {
		return err
	}
	w.sourceAnimation(label)
	w.set("run_speed", 0)
	w.set("walking", 0)
	w.set("sword_contact", 0)
	return nil
}

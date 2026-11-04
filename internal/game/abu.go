package game

import "aladdin/internal/choreography"

func abuStage(index int) bool { return index == 2 || index == 6 || index == 13 }

// prepareAbu follows 1E0CA8/1E0DB8/1E0CFE. These rooms bypass ordinary
// map-loading sweeps and initialize the same bonus player as a restart.
func (w *NativeWorld) prepareAbu() error {
	w.set("flag_f6a3", 255)
	w.set("flag_f6a4", 255)
	w.set("tick", 255)
	w.set("camera_x", w.Checkpoint.CameraX)
	w.set("camera_tile_x", w.Checkpoint.CameraX)
	w.set("grounded", 255)
	w.set("jump_hold", 255)
	w.sourceAnimation(0x14947c)
	w.set("flag_f62e", 0)
	if events := w.Scripts.AbuSchedules[w.Stage.Index]; len(events) > 0 {
		w.set("bonus_event_cursor", events[0].Source)
	}
	if w.Stage.Index == 6 {
		if slot := w.markerSlot(1); slot >= 0 {
			o := choreography.FromTemplate(w.Scripts.Templates[0x1e3ade])
			o.SetWord(2, 160)
			o.SetWord(4, 448)
			o.Record[9] = 255
			w.Objects[slot] = o
		}
	}
	if err := w.showMessage(0x10); err != nil {
		return err
	}
	w.syncPlayer()
	c := choreography.Choreographer{Scripts: w.Scripts}
	for i := range w.Objects {
		if o := w.Objects[i]; o != nil && o.Alive() {
			if err := c.Animation(w, o); err != nil {
				return err
			}
		}
	}
	w.updateHUDWave()
	return nil
}

// abuBehavior ports the byte clock at 1E0C62. Consuming an event does not
// depend on successful actor allocation, and the counter wraps as a byte.
func (w *NativeWorld) abuBehavior() {
	defer func() {
		if w.Stage.Index == 6 {
			w.set("fall_ticks", 0)
		}
	}()
	for _, e := range w.Scripts.AbuSchedules[w.Stage.Index] {
		if e.Source != w.get("bonus_event_cursor") {
			continue
		}
		n := (w.get("flag_f62e") + 1) & 255
		w.set("flag_f62e", n)
		if n <= e.Delay {
			return
		}
		w.set("bonus_event_cursor", e.Source+6)
		w.set("flag_f62e", 0)
		w.abuEvent(e.Kind, e.X, e.Y)
		return
	}
}
func (w *NativeWorld) abuEvent(kind, x, y int) {
	spawn := func(pool, template int) *choreography.Object {
		slot := w.markerSlot(pool)
		if slot < 0 {
			return nil
		}
		o := w.actorTemplate(template, w.Objects[slot])
		o.SetWord(2, x)
		o.SetWord(4, y)
		w.Objects[slot] = o
		return o
	}
	face := func(o *choreography.Object) {
		if o != nil && uint16(x) >= 141 {
			o.Record[9] = 255
		}
	}
	finish := func() {
		w.set("action_locked", 255)
		w.sourceAnimation(0x14d970)
		w.set("exit_countdown", 200)
		w.set("flag_faa2", 255)
	}
	switch kind {
	case 0xe0:
	case 0xe1:
		if o := spawn(3, 0x1e32e6); o != nil {
			o.SetWord(2, x-12)
			if p := spawn(3, 0x1e32e6); p != nil {
				p.SetWord(2, x+12)
			}
		}
	case 0xe2:
		w.set("action_locked", 255)
	case 0xe3:
		if w.flag("grounded") && !w.flag("jumping") && !w.flag("exit_countdown") {
			finish()
		}
	case 0xe4, 0xe5:
		if o := spawn(0, 0x1e3a66); o != nil && kind == 0xe4 {
			o.Record[9] = 255
		}
	case 0xe6:
		if o := spawn(3, 0x1e307a); o != nil {
			w.PaletteBank2 = 0x1519d4
			face(o)
		}
	case 0xe7:
		w.Sound(0x5d, false)
	case 0xe8:
		if o := spawn(0, 0x1e394e); o != nil {
			o.SetWord(30, 0x4000)
			w.PaletteBank2 = 0x151a54
			face(o)
		}
	case 0xe9:
		face(spawn(3, 0x1e3912))
	case 0xea:
		finish()
	case 0xeb:
		if o := spawn(3, 0x1e31e2); o != nil {
			o.Motion = 0x148486
			face(o)
		}
	case 0xec:
		face(spawn(3, 0x1e321e))
	case 0xed, 0xee:
		counter := "gems_ascii"
		if kind == 0xee {
			counter = "apples_ascii"
		}
		if w.get(counter) == 0x3939 {
			return
		}
		if o := spawn(3, 0x1e2f9e); o != nil {
			o.Record[6] = 1
			o.Motion = 0x14842e
			if kind == 0xed {
				o.Record[0] = 0x40
				o.Record[41] = 1
				o.Animation = 0x14a532
			} else {
				o.Record[0] = 0x46
				o.Record[41] = 0
				o.Animation = 0x14a56c
			}
		}
	case 0xef:
		spawn(3, 0x1e3106)
	case 0xf0:
		if w.get("lives") != 9 {
			if o := spawn(3, 0x1e2fb2); o != nil {
				o.Motion = 0x1483f0
			}
		}
	case 0xf1:
		face(spawn(3, 0x1e31e2))
	}
}
func abuAnimation(label int) int {
	switch label {
	case 0x1494aa, 0x1494bc:
		return 0x14947c
	case 0x14973a:
		return 0x149708
	case 0x149074:
		return 0x1491c4
	case 0x1498ea, 0x149888:
		return 0x14982a
	case 0x1496ac:
		return 0x14969c
	case 0x1497ba:
		return 0x149692
	}
	return label
}
func (w *NativeWorld) abuAnimation(label int) int {
	if w.flag("flag_f6a3") {
		return abuAnimation(label)
	}
	return label
}

// abuWater preserves the one-shot latch and allocation order at 1DFBB8.
func (w *NativeWorld) abuWater() {
	if w.flag("flag_f6a6") {
		return
	}
	w.set("flag_f6a6", 255)
	if !w.flag("exit_countdown") {
		w.set("action_locked", 255)
		w.set("exit_countdown", 50)
		if !w.flag("sword_contact") {
			w.set("sword_cooldown", 1)
		}
	}
	if slot := w.markerSlot(0); slot >= 0 {
		if o := w.Objects[0]; o != nil {
			w.Delete(o, false)
		}
		x, y := w.PlayerPosition()
		o := choreography.FromTemplate(w.Scripts.Templates[0x1e3a8e])
		o.SetWord(2, x)
		o.SetWord(4, y)
		w.Objects[slot] = o
	}
}
func (w *NativeWorld) beginAbuIntro() error {
	w.stopAudio()
	w.requestMusic(18)
	w.clearLifeActors()
	w.set("invulnerability", 0)
	w.set("flag_f695", 0)
	w.set("tick", 255)
	o := choreography.FromTemplate(w.Scripts.Templates[0x1e3b06])
	o.SetWord(2, 64)
	o.SetWord(4, 440)
	o.Record[9] = 0
	w.Objects[2] = o
	if err := w.lifeActors(); err != nil {
		return err
	}
	w.Flow.Phase, w.Flow.Age = FlowBonusIntro, 0
	w.cacheCompleteActors()
	return nil
}

package game

import "aladdin/internal/choreography"

// initializeIago ports the fixed actor slots in the initializer 1E0ECC.
func (w *NativeWorld) initializeIago() {
	for _, e := range []struct{ slot, template, x, y int }{{2, 0x1e31a6, 0x184, 0x156}, {3, 0x1e31ce, 0x176, 0x1b8}, {4, 0x1e31ba, 0x176, 0x1b8}} {
		o := choreography.FromTemplate(w.Scripts.Templates[e.template])
		o.SetWord(2, e.x)
		o.SetWord(4, e.y)
		w.Objects[e.slot] = o
	}
}

// iagoBehavior is 1E0B38. Its comparison splits at byte 127, not 128.
func (w *NativeWorld) iagoBehavior() {
	tick := w.get("tick")
	if tick == 13 {
		if slot := w.markerSlot(3); slot >= 0 {
			o := choreography.FromTemplate(w.Scripts.Templates[0x1e31f6])
			o.SetWord(2, 0x2b1)
			o.SetWord(4, 0x22a)
			w.Objects[slot] = o
		}
		return
	}
	if tick&127 != 0 {
		return
	}
	x, _ := w.PlayerPosition()
	low := byte(w.Random()) < 127
	o := choreography.FromTemplate(w.Scripts.Templates[0x1e3a3e])
	o.Animation = 0x14e0f8
	if low {
		o.Animation = 0x14e118
	}
	if uint16(x) < 0x182 {
		o.Record[9] = 255
		o.Animation = 0x14e194
		if low {
			o.Animation = 0x14e1c8
		}
	}
	o.SetWord(2, 0x184)
	o.SetWord(4, 0x166)
	w.Objects[1] = o
}

// iagoProjectile is 1D6796, including the zero-before-decrement HP rule.
func (w *NativeWorld) iagoProjectile(p, o *choreography.Object) error {
	w.Sound(0x2d, false)
	o.Record[0] = 0x8c
	o.Animation, o.AnimationDelay = 0x14cb42, 0
	w.Delete(p, false)
	if err := w.explodeProjectile(p); err != nil {
		return err
	}
	if o.Record[1] != 0 {
		o.Record[1]--
		if o.Record[1] == 7 || o.Record[1] == 3 || o.Record[1] == 1 {
			o.Animation = 0x14cb64
		}
		return nil
	}
	w.set("pending_score", w.get("pending_score")+int(o.Record[8]))
	w.set("exit_countdown", 32)
	w.Delete(o, true)
	if err := w.replaceActor(o, 0x1e2ed6); err != nil {
		return err
	}
	w.Sound(3, true)
	return nil
}

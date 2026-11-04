package game

import "aladdin/internal/choreography"

// jafarPalaceBehavior is 1E0AD8, including the frame-zero rectangular gate.
func (w *NativeWorld) jafarPalaceBehavior() {
	x, y := w.PlayerPosition()
	if w.get("tick") != 0 || uint16(x) < 0x10 || uint16(x) >= 0x470 || uint16(y) < 0x130 || uint16(y) >= 0x1d0 {
		return
	}
	if slot := w.markerSlot(3); slot >= 0 {
		o := choreography.FromTemplate(w.Scripts.Templates[0x1e39b2])
		o.Record[9] = 255
		o.SetWord(2, 0x4c0)
		o.SetWord(4, 0x1b0)
		w.Objects[slot] = o
	}
}

// palaceBossContact ports 1D9900. Swords repel Aladdin; these bosses take
// damage through their separate projectile path rather than sword HP rules.
func (w *NativeWorld) palaceBossContact(o *choreography.Object) {
	if o.Record[60]&128 != 0 {
		return
	}
	if !w.flag("sword_contact") {
		if o.Record[0] == 0x16 {
			o.Animation, o.AnimationDelay = 0x14d08c, 0
		}
		w.damagePlayer()
		return
	}
	x, _ := o.Position()
	px, _ := w.PlayerPosition()
	vx := 0xfc00
	if uint16(x) < uint16(px) {
		vx = 0x600
	}
	w.set("momentum_x", vx)
	w.set("momentum_y", 0xfc00)
	w.Sound(0x31, false)
	w.sourceAnimation(0x1491fe)
	w.set("look_target_x", 160)
	w.set("look_target_y", 352)
	w.set("jumping", 255)
	w.set("jump_released", 0)
	w.set("walking", 0)
}

// palaceBossProjectile is 1D6BB8. The source tests zero BEFORE decrementing:
// initial HP ten/twenty therefore requires eleven/twenty-one accepted hits.
func (w *NativeWorld) palaceBossProjectile(p, o *choreography.Object) error {
	if o.Record[60]&128 != 0 {
		return nil
	}
	w.Delete(p, false)
	w.Sound(3, true)
	w.Sound(0x6c, false)
	spawn := func(template int, from *choreography.Object) {
		if slot := w.markerSlot(0); slot >= 0 {
			v := choreography.FromTemplate(w.Scripts.Templates[template])
			x, y := from.Position()
			v.SetWord(2, x)
			v.SetWord(4, y)
			v.Record[6], v.Record[7] = from.Record[6], from.Record[7]
			w.Objects[slot] = v
		}
	}
	spawn(0x1e335e, p)
	if o.Record[1] != 0 {
		o.Record[1]--
		return nil
	}
	w.set("pending_score", w.get("pending_score")+int(o.Record[8]))
	w.Sound(0x42, false)
	if o.Record[0] == 0x15 {
		w.Delete(o, false)
		if err := w.replaceActor(o, 0x1e334a); err != nil {
			return err
		}
		w.set("exit_countdown", 20)
		return nil
	}
	w.set("flag_f640", 255)
	for i := 1; i <= 24; i++ {
		if v := w.Objects[i]; v != nil && (v.Record[0] == 0x95 || v.Record[0] == 0x53) {
			w.Delete(v, false)
		}
	}
	w.Delete(o, false)
	if err := w.replaceActor(o, 0x1e3566); err != nil {
		return err
	}
	w.PaletteBank2 = 0x151d14
	spawn(0x1e334a, o)
	return nil
}

// palaceInteriorScroll is the single-row/slow-vertical background at 1D5658.
func palaceInteriorScroll(x, y int) (int, int) {
	return int(-((uint16(x) + 16) >> 1) & 511), int(uint16(y) >> 5 & 255)
}

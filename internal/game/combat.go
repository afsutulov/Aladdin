package game

import "aladdin/internal/choreography"

// damagePlayer reconstructs the ordinary contact response at 1D8C60.
func (w *NativeWorld) damagePlayer() {
	if w.flag("action_locked") || w.flag("death_pending") || w.flag("exit_countdown") || w.flag("invulnerability") {
		return
	}
	air := w.flag("jumping") || !w.flag("grounded") || w.flag("climbing") || w.flag("hanging") || w.flag("on_object") || w.flag("previous_standing_object")
	if w.flag("flag_f6a3") {
		w.sourceAnimation(0x149f40)
		w.set("action_locked", 255)
		w.set("exit_countdown", 50)
		if !w.flag("sword_contact") {
			w.set("sword_cooldown", 1)
		}
		return
	}
	if !air && !w.flag("walking") {
		if w.flag("sword_cooldown") || w.flag("apple_cooldown") {
			air = true
		} else {
			w.sourceAnimation(0x149f5c)
		}
	}
	w.set("run_speed", 0)
	w.set("walking", 0)
	w.Sound(0x31, false)
	// The source invokes the health routine 1/2/3 times by difficulty; its
	// invulnerability gate prevents subsequent decrements in the same tick.
	for i := 0; i <= w.get("difficulty"); i++ {
		if w.flag("exit_countdown") || w.flag("death_pending") || w.flag("invulnerability") {
			continue
		}
		if !w.flag("health") {
			w.set("death_pending", 10)
			continue
		}
		w.set("invulnerability", 40)
		if !w.flag("invincible") {
			w.set("health", w.get("health")-1)
		}
	}
}

func (w *NativeWorld) guardHitSpark(o *choreography.Object) error {
	if o.Record[7]&1 == 0 {
		w.Sound(8, false)
		return nil
	}
	w.Sound(0x79, false)
	x, y := o.Position()
	px, py := w.PlayerPosition()
	x += (px - x) >> 2
	y += (py - 24 - y) >> 2
	index := w.markerSlot(0)
	if index < 0 {
		index = w.markerSlot(3)
	}
	if index < 0 {
		return nil
	}
	spark := w.actorTemplate(0x1e3372, w.Objects[index])
	spark.SetWord(2, x)
	spark.SetWord(4, y)
	w.Objects[index] = spark
	return nil
}

func (w *NativeWorld) guardContact(o *choreography.Object) error {
	x, _ := o.Position()
	px, _ := w.PlayerPosition()
	front := (!w.flag("facing_left") && px < x) || (w.flag("facing_left") && px >= x)
	if o.Record[60]&128 != 0 && o.Record[60]&32 == 0 {
		return nil
	}
	if !front || !w.flag("sword_contact") {
		if o.Record[60]&32 != 0 {
			w.damagePlayer()
		}
		return nil
	}
	if o.Record[60]&32 != 0 {
		w.parryTarget = o
		w.ParryFlash = true
		return nil
	}
	switch o.Record[0] {
	case 0x22:
		o.Animation = 0x14b048
	case 0x25:
		o.Animation = 0x14b096
	case 0x23:
		o.Animation = 0x14b46a
	case 0x26:
		o.Animation = 0x14b49a
	}
	o.Record[0] = 0x8c
	o.AnimationDelay = 0
	o.Motion = 0
	o.MotionDelay = 0
	if e := w.guardHitSpark(o); e != nil {
		return e
	}
	if w.get("difficulty") != 0 && o.Record[1] != 0 {
		o.Record[1]--
		w.Sound(0x41, false)
		w.set("walking", 0)
		w.set("run_speed", 0)
		w.set("sword_step", 0)
		label, e := w.ResumePlayer(w.Objects[0])
		if e != nil {
			return e
		}
		w.sourceAnimation(label)
		w.set("sword_contact", 0)
		o.Record[60] &^= 32
		return nil
	}
	w.set("pending_score", w.get("pending_score")+int(o.Record[8]))
	return w.replaceActor(o, 0x1e2ed6)
}

// finishParry resumes the same logical tick after its original VBlank wait.
func (w *NativeWorld) finishParry(o *choreography.Object) {
	if w.flag("grounded") && !w.flag("hanging") {
		w.set("momentum_x", 0)
		w.set("action_cooldown", (w.get("action_cooldown")+4)&255)
		if w.get("sword_contact") == 1 {
			w.sourceAnimation(0x14a0a4)
		} else {
			w.sourceAnimation(0x14a18e)
		}
	}
	w.set("sword_contact", 0)
	w.set("walking", 0)
	o.Record[60] &^= 32
	o.AnimationDelay = 0
	switch o.Record[0] {
	case 0x22:
		o.Animation = 0x14b080
	case 0x25:
		o.Animation = 0x14b0ce
	case 0x23:
		o.Animation = 0x14b518
	case 0x26:
		o.Animation = 0x14b4ca
	}
}

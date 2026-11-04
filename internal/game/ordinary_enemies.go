package game

import "aladdin/internal/choreography"

// ordinaryEnemyContact is the non-parrying melee path at 1D93CA.
// Its hit-point rules differ from the large guards' two-contact reaction.
func (w *NativeWorld) ordinaryEnemyContact(o *choreography.Object, hurt bool) error {
	x, _ := o.Position()
	px, _ := w.PlayerPosition()
	front := (!w.flag("facing_left") && px < x) || (w.flag("facing_left") && px >= x)
	if w.flag("sword_contact") && front && w.lastSwordTarget != o {
		kind := o.Record[0]
		w.lastSwordTarget = o
		if e := w.guardHitSpark(o); e != nil {
			return e
		}
		dead := o.Record[1] == 0
		if !dead && kind != 0x17 {
			o.Record[1]--
			dead = o.Record[1] == 0
		}
		if dead {
			w.set("pending_score", w.get("pending_score")+int(o.Record[8]))
			if kind == 0x17 {
				return w.finishLampEnemy(o)
			}
			if kind == 0x14 || kind == 0x15 {
				w.set("exit_countdown", 32)
			}
			w.Delete(o, true)
			return w.replaceActor(o, 0x1e2ed6)
		}
		o.Record[1]--
		w.set("sword_step", 0)
		w.set("run_speed", 0)
		w.set("walking", 0)
		label, e := w.ResumePlayer(w.Objects[0])
		if e != nil {
			return e
		}
		w.sourceAnimation(label)
		if kind == 0x17 {
			o.Record[0] = 0x8c
			o.Animation = 0x14c4f0
			o.AnimationDelay = 0
			o.Motion = 0
			o.MotionDelay = 0
			w.Sound(0x6a, false)
		}
	}
	if hurt && !w.flag("sword_contact") {
		w.damagePlayer()
	}
	return nil
}

// knifeContact ports 1D9630. A sword redirects the original actor record
// into the seven-slot projectile pool, then creates the impact glint.
func (w *NativeWorld) knifeContact(o *choreography.Object) error {
	if !w.flag("sword_contact") {
		w.Delete(o, false)
		w.damagePlayer()
		return nil
	}
	slot := -1
	for i := 25; i < 32; i++ {
		if w.Objects[i] == nil || !w.Objects[i].Alive() {
			slot = i
			break
		}
	}
	if slot >= 0 {
		o.Record[0] = 0x87
		o.Record[6] = 64
		o.SetWord(26, 0xf600)
		o.Record[9] ^= 255
		o.Motion = 0x147d1e
		copyActor := *o
		w.Objects[slot] = &copyActor
		o.Record[0] = 0
	}
	w.Sound(0x21, false)
	index := w.markerSlot(0)
	if index >= 0 {
		spark := w.actorTemplate(0x1e3516, w.Objects[index])
		x, y := o.Position()
		spark.SetWord(2, x)
		spark.SetWord(4, y)
		w.Objects[index] = spark
	}
	return nil
}

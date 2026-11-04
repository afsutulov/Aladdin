package game

import "aladdin/internal/choreography"

// springboardContact is 1DA5A8. Its horizontal response replaces only the
// velocity's high byte, preserving the original fractional low byte.
func (w *NativeWorld) springboardContact(o *choreography.Object) {
	if int16(w.get("momentum_y")) < 0 || w.flag("action_locked") {
		return
	}
	x, y := o.Position()
	px, py := w.PlayerPosition()
	d := py - y
	if d < 0 {
		d = -d
	}
	if d >= 6 {
		return
	}
	w.set("player_y", y)
	w.set("momentum_y", 0xf800)
	w.set("look_target_x", 160)
	w.sourceAnimation(0x1491fe)
	w.set("jumping", 255)
	w.set("jump_released", 0)
	o.Record[0] = 0x8c
	o.Animation = 0x14a70c
	o.AnimationDelay = 0
	dx := px - x
	v := int(byte(dx))
	if dx < 0 {
		v = -int(byte(-dx))
	}
	if v < 0 {
		v = -((-v) >> 3)
	} else {
		v >>= 3
	}
	w.set("momentum_x", int(uint16(uint16(byte(v))<<8|uint16(w.get("momentum_x")&255))))
}

// lampContact moves the collected actor into the projectile pool. Its native
// choreography emits the subsequent effects and homing projectiles.
func (w *NativeWorld) lampContact(o *choreography.Object) {
	for slot := 25; slot < 32; slot++ {
		if w.Objects[slot] != nil && w.Objects[slot].Alive() {
			continue
		}
		o.Record[0] = 0x8a
		o.Animation = 0x14d7ae
		o.AnimationDelay = 0
		clone := *o
		w.Objects[slot] = &clone
		o.Record[0] = 0
		return
	}
}

func (w *NativeWorld) solidActorContact(o *choreography.Object) {
	x, _ := o.Position()
	px, _ := w.PlayerPosition()
	right := px < x
	if right && !w.flag("input_right") || !right && !w.flag("input_left") {
		return
	}
	w.set("flag_f610", 4)
	w.set("stopping", 255)
	side, wall := "flag_f612", "wall_right"
	if !right {
		side, wall = "flag_f611", "wall_left"
	}
	w.set(side, 255)
	if w.get("momentum_y")>>8 != 0 || w.flag("jumping") {
		w.set(wall, 255)
	}
}

// actorWalk is the original short stop and slow-walk response at
// 1D481E..1D491C, distinct from the fixed-wall stance.
func (w *NativeWorld) actorWalk(left bool) (int, bool) {
	if w.flag("jumping") || !w.flag("grounded") {
		return 0, false
	}
	flag, walking, target := "flag_f612", 3, 112
	if left {
		flag, walking, target = "flag_f611", 253, 208
	}
	if w.flag(flag) {
		if w.get("walking") != walking {
			w.sourceAnimation(0x1496ce)
			w.set("walking", walking)
			if left {
				w.set("look_target_x", 112)
				w.set("camera_delay", 7)
			}
		}
		return 0, true
	}
	if w.flag("flag_f610") {
		if w.get("walking") != walking {
			w.set("run_speed", 1)
			w.set("look_target_x", target)
			w.set("camera_delay", 7)
			w.sourceAnimation(0x1496ce)
			w.set("walking", walking)
		}
		dx := w.get("run_speed")
		if left {
			dx = -dx
		}
		return dx, true
	}
	return 0, false
}

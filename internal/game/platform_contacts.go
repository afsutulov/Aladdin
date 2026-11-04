package game

import "aladdin/internal/choreography"

// platformContact translates the ordinary one-way platform handlers. They
// adjust the contact position before the shared landing path at 1D6320.
func (w *NativeWorld) platformContact(o *choreography.Object) bool {
	if w.flag("jumping") && !w.flag("jump_released") || o.Record[6]&16 == 0 {
		return false
	}
	x, y := o.Position()
	px, py := w.PlayerPosition()
	kind := o.Record[0]
	offset, limit := 0, 0
	carry := false
	switch kind {
	case 0x7b, 0x7c:
		offset, limit = 11, 10
	case 0x7d, 0x7e:
		offset, limit, carry = 11, 6, true
	case 0x5f:
		// 1D9DF6 extends the tolerance by this frame's downward travel.
		offset, limit = -2, 12
		if uint16(py) >= uint16(y+2) && uint16(py) >= uint16(w.get("previous_player_y")) {
			limit += int(uint16(py - w.get("previous_player_y")))
		}
	case 0x5c, 0x5d, 0x5e:
		offset, limit = 18, 6
	case 0x65:
		w.set("player_x", x)
		w.set("player_y", y)
		if w.flag("walking") {
			w.sourceAnimation(0x149a70)
			w.set("walking", 0)
		}
		w.set("run_speed", 0)
		w.set("look_target_x", 160)
		w.set("look_target_y", 376)
		return true
	case 0x66:
		w.set("player_y", y)
		return true
	case 0x56, 0x57:
		offset, limit = -32, 10
	case 0x59, 0x5a:
		offset, limit, carry = 19, 12, true
	case 0x69, 0x6a:
		limit = 10
	case 0x6b:
		offset, limit, carry = 18, 6, true
	case 0x6e, 0x6f:
		offset, limit = 11, 12
	case 0x70, 0x71, 0x72, 0x73:
		offset, limit = 8, 12
	default:
		return false
	}
	target := y - offset
	distance := int(int16(py - target))
	if distance < 0 {
		distance = -distance
	}
	if distance >= limit {
		return false
	}
	w.set("player_y", target)
	if carry {
		dx := int(int8(o.Record[28]))
		if kind != 0x6b || dx <= 0 || !w.flag("wall_right") {
			w.set("player_x", px+dx)
		}
	}
	switch kind {
	case 0x56, 0x57:
		w.set("player_x", x)
		w.set("hanging", 255)
		w.set("climbing", 255)
		w.set("look_target_x", 144)
		if kind == 0x56 {
			o.Record[0] = 0x57
			o.Animation, o.AnimationDelay = 0x14ca30, 0
		}
	case 0x7d:
		if uint16(px) >= uint16(x+8) {
			o.Record[0] = 0x7e
			o.Animation = 0x14bf5c
		} else if uint16(px) < uint16(x-8) {
			o.Record[0] = 0x7e
			o.Animation = 0x14bf2c
		}
	case 0x7b:
		if uint16(px) >= uint16(x-16) && uint16(px) < uint16(x+16) && !w.flag("sword_contact") && int16(w.get("momentum_y")) > 0 {
			w.set("walking", 0)
			w.set("run_speed", 0)
			o.Record[0] = 0x7c
			o.Motion = 0x147e34
			o.MotionDelay = 0
			if slot := w.markerSlot(3); slot >= 0 {
				v := choreography.FromTemplate(w.Scripts.Templates[0x1e3552])
				v.SetWord(2, x)
				v.SetWord(4, y)
				w.Objects[slot] = v
			}
		}
	case 0x5c:
		o.Record[0] = 0x5d
		o.Animation = 0x14a6b2
	case 0x69:
		o.Record[0] = 0x6a
		o.Motion = 0x148af2
		w.Sound(0x45, false)
	case 0x6e:
		o.Record[0] = 0x6f
		o.Animation = 0x14ad7a
		o.AnimationDelay = 0
	case 0x70, 0x71:
		o.Record[0] = 0x72
		o.Animation = 0x14bdcc
		if kind == 0x71 {
			o.Animation = 0x14be0c
		}
		o.AnimationDelay = 0
		if o.Record[52] != 0 {
			w.Activated[int(uint16(o.Word(50)))] = false
		}
	}
	return true
}

func (w *NativeWorld) landOnObject(o *choreography.Object) {
	if w.flag("jumping") {
		if !w.flag("jump_released") {
			return
		}
		w.set("jumping", 0)
	}
	if w.flag("action_locked") {
		return
	}
	if w.get("momentum_y") != 0 {
		w.set("flag_f623", 0)
		animation := 0x1497e4
		if o.Record[0] >= 0x56 && o.Record[0] < 0x58 {
			animation = 0x148f00
		} else if w.flag("flag_f6a3") {
			animation = 0x14969c
		} else if w.get("run_speed") == 0 {
			animation = 0x1496ac
			if w.get("fall_ticks") >= 40 {
				animation = 0x149152
			}
		}
		w.sourceAnimation(animation)
		w.set("momentum_y", 0)
		w.set("walking", 0)
	}
	w.set("fall_ticks", 0)
	w.set("grounded", 255)
	w.set("standing_object", int(o.Record[0]))
	w.set("on_object", 255)
	w.syncPlayer()
}

// steerCarpet is the spring-to-target motion used by the carpet and its
// companion at 1E0038/1E00C4. Velocity remains a signed fractional word.
func (w *NativeWorld) steerCarpet(o *choreography.Object, stepX int, face bool) {
	x, y := o.Position()
	x += stepX
	y -= 2
	o.SetWord(2, x)
	o.SetWord(4, y)
	for _, axis := range []struct{ position, target, member, limit int }{
		{x, w.get("carpet_target_x"), 24, 0x68}, {y, w.get("carpet_target_y"), 26, 0x7c},
	} {
		d := int(uint16(axis.position)) - int(uint16(axis.target))
		a := d
		if a < 0 {
			a = -a
		}
		if a > axis.limit {
			a = axis.limit
		}
		if d > 0 {
			a = -a
		}
		o.SetWord(axis.member, int(o.Word(axis.member))+a)
	}
	if face && (w.get("standing_object") == 0x65 || w.get("standing_object") == 0x67) {
		o.Record[9] = 0
		if int8(o.Record[28]) < 0 {
			o.Record[9] = 255
		}
	}
}

// movingPoleContact is the hanging platform path at 1DA35A.
func (w *NativeWorld) movingPoleContact(o *choreography.Object) bool {
	if w.flag("action_locked") || int16(w.get("momentum_y")) < 0 || w.flag("jumping") && !w.flag("jump_released") || o.Record[6]&16 == 0 {
		return false
	}
	x, y := o.Position()
	px, _ := w.PlayerPosition()
	distance := int(int16(px - x))
	if distance < 0 {
		distance = -distance
	}
	if distance >= 12 {
		return false
	}
	w.set("player_x", x)
	w.set("player_y", y+16)
	if !w.flag("flag_f625") {
		w.sourceAnimation(0x148f00)
		w.set("look_target_x", 160)
		w.set("look_target_y", 344)
		w.set("run_speed", 0)
		w.set("walking", 0)
		w.set("momentum_x", 0)
		w.set("momentum_y", 0)
	}
	w.set("hanging", 255)
	w.set("climbing", 255)
	w.set("flag_f625", 4)
	return true
}

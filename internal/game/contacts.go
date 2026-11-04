package game

import (
	"aladdin/internal/choreography"
	"image"
)

func (w *NativeWorld) actorBox(o *choreography.Object, flip bool) (image.Rectangle, bool) {
	if o == nil || !o.Alive() || o.Frame < 0 || o.Frame >= len(w.Frames) {
		return image.Rectangle{}, false
	}
	b := w.Frames[o.Frame].Box
	x, y := o.Position()
	left, right := b[0], b[2]
	if flip && o.Record[9] != 0 {
		left = int(byte(-right))
		right = int(byte(-b[0]))
	}
	return image.Rect(x+left, y+b[1], x+right, y+b[3]), true
}

func (w *NativeWorld) incrementASCII(name string) {
	v := w.get(name)
	if v == 0x3939 {
		return
	}
	v++
	if v&255 == 0x3a {
		v = (v+0x100)&^255 | 0x30
	}
	w.set(name, v)
}

// The original contact edges include equality at right/bottom only.
func (w *NativeWorld) playerContacts() error {
	return w.playerContactsFrom(1, true)
}
func (w *NativeWorld) playerContactsFrom(start int, reset bool) error {
	if reset {
		w.set("previous_standing_object", w.get("standing_object"))
		w.set("standing_object", 0)
		w.set("on_object", 0)
		w.set("flag_f611", 0)
		w.set("flag_f612", 0)
		for _, key := range []string{"invulnerability", "flag_f610"} {
			if w.get(key) > 0 {
				w.set(key, w.get(key)-1)
			}
		}
	}
	player, ok := w.actorBox(w.Objects[0], true)
	if !ok {
		return nil
	}
	playerY := int(w.Objects[0].Word(4))
	for i := start; i <= 24; i++ {
		o := w.Objects[i]
		if o == nil || o.Record[0] >= 0x87 {
			continue
		}
		box, ok := w.actorBox(o, true)
		if !ok {
			continue
		}
		// Horizontal bounds are cached by the original loop, while vertical
		// bounds follow a preceding platform contact in the same pass.
		current := player.Add(image.Pt(0, int(w.Objects[0].Word(4))-playerY))
		if current.Max.X < box.Min.X || current.Max.Y < box.Min.Y || current.Min.X >= box.Max.X || current.Min.Y >= box.Max.Y {
			continue
		}
		switch o.Record[0] {
		case 0x53: // 1D98A8: the boss's pull impulse.
			for slot := 1; slot <= 24; slot++ {
				if boss := w.Objects[slot]; boss != nil && boss.Record[0] == 0x16 {
					w.set("look_target_x", 160)
					w.Objects[0].AnimationDelay = 0
					x, _ := boss.Position()
					px, _ := w.PlayerPosition()
					dx := 0x100
					if uint16(x) < uint16(px) {
						dx = -dx
					}
					w.set("momentum_x", (w.get("momentum_x")+dx)&65535)
					break
				}
			}
			o.Record[0] = 0x8c
			o.Animation, o.AnimationDelay = 0x14c22c, 0
			o.SetWord(24, 0)
			o.SetWord(26, 0)
		case 0x15, 0x16:
			w.palaceBossContact(o)
		case 0x2d: // 1D9BFA: collect the floating Aladdin image.
			w.set("flag_f63f", 255)
			w.Sound(0x5d, false)
			o.Record[0], o.Record[6] = 0x8c, 1
			o.Animation, o.AnimationDelay = 0x1491cc, 0
			o.Motion, o.MotionDelay = 0x148b3a, 0
			if slot := w.markerSlot(0); slot >= 0 {
				v := w.actorTemplate(0x1e334a, w.Objects[slot])
				x, y := o.Position()
				v.SetWord(2, x)
				v.SetWord(4, y)
				w.Objects[slot] = v
			}
		case 0x54, 0x55:
			w.lampHandContact(o)
		case 0x67, 0x68:
			if !w.flag("jumping") || w.flag("jump_released") {
				x, y := w.PlayerPosition()
				o.SetWord(2, x)
				o.SetWord(4, y)
				w.set("look_target_x", 160)
				w.set("look_target_y", 376)
				w.set("grounded", 255)
				w.set("fall_ticks", 0)
				if o.Record[0] != 0x68 {
					w.sourceAnimation(0x149a8a)
					o.Record[0] = 0x68
				}
				w.set("on_object", 255)
			}
		case 0x5b:
			w.Sound(0x65, false)
			w.set("death_pending", 255)
		case 0x48:
			w.set("flag_f62c", (w.get("flag_f62c")+1)&255)
			if w.get("flag_f62c") >= 4 {
				w.set("flag_f62c", 0)
				if w.get("apples_ascii") != 0x3939 {
					w.incrementASCII("apples_ascii")
					w.Sound(0x0c, false)
				}
			}
			w.set("pending_score", w.get("pending_score")+15)
			w.Delete(o, true)
			if err := w.collectActor(o, 0x1e30ca); err != nil {
				return err
			}

		case 0x84:
			if !w.flag("flag_f6a3") {
				w.set("death_pending", 1)
			} else {
				w.sourceAnimation(0x149f40)
				w.set("action_locked", 255)
				if !w.flag("sword_contact") {
					w.set("sword_cooldown", 1)
				}
			}
		case 0x1c, 0x1d:
			if e := w.caveGuardianContact(o); e != nil {
				return e
			}
		case 0x1e, 0x1f, 0x20:
			if w.flag("sword_contact") {
				w.breakCaveStatue(o.Record[0])
				w.Delete(o, true)
				if e := w.replaceActor(o, 0x1e2ed6); e != nil {
					return e
				}
			}
		case 0x0f:
			if w.flag("sword_contact") {
				w.Delete(o, false)
				if e := w.replaceActor(o, 0x1e334a); e != nil {
					return e
				}
			}
		case 0x7f, 0x81:
			x, _ := o.Position()
			if !w.flag("sword_contact") || uint16(current.Min.X) >= uint16(x-8) && uint16(current.Min.X) < uint16(x+8) {
				w.damagePlayer()
			}
		case 0x35:
			o.Record[0] = 0x8c
			o.Animation = 0x14b722
			o.AnimationDelay = 0
			if !w.flag("sword_contact") {
				w.damagePlayer()
			} else {
				w.Sound(0x21, false)
				if slot := w.markerSlot(0); slot >= 0 {
					v := w.actorTemplate(0x1e3516, w.Objects[slot])
					x, y := o.Position()
					v.SetWord(2, x)
					v.SetWord(4, y)
					w.Objects[slot] = v
				}
			}
		case 0x34, 0x82:
			w.damagePlayer()
		case 0x4d, 0x4e, 0x4f, 0x50, 0x51, 0x52:
			flag := map[byte]string{0x4d: "flag_f649", 0x4e: "flag_f64a", 0x4f: "flag_f64b", 0x50: "flag_f64c", 0x51: "flag_f639", 0x52: "flag_f64d"}[o.Record[0]]
			w.set(flag, 255)
			w.Sound(0x67, false)
			w.set("pending_score", w.get("pending_score")+15)
			w.Delete(o, true)
			if e := w.collectActor(o, 0x1e30ca); e != nil {
				return e
			}
		case 0x27:
			if w.flag("sword_contact") {
				if e := w.breakScenery(o); e != nil {
					return e
				}
			}
		case 0x44, 0x45:
			name := "scarab_first"
			if o.Record[0] == 0x45 {
				name = "scarab_second"
			}
			w.set(name, 255)
			o.Record[0] = 0x8c
			o.Motion = 0x148b72
			o.AnimationDelay = 0
			w.set("look_target_x", 112)
			w.set("look_target_y", 392)
			w.Sound(0x64, false)
		case 0x3d, 0x42:
			w.set("exit_countdown", 32)
			w.Delete(o, true)
			if e := w.collectActor(o, 0x1e30ca); e != nil {
				return e
			}
			w.Sound(0x64, false)
			w.set("pending_score", w.get("pending_score")+100)
		case 0x43:
			w.Delete(o, true)
			if e := w.collectActor(o, 0x1e30ca); e != nil {
				return e
			}
			w.Sound(0x64, false)
			w.set("pending_score", w.get("pending_score")+100)
			w.set("flag_f6ab", 255)
		case 3:
			if !w.flag("sword_contact") {
				if e := w.ordinaryEnemyContact(o, true); e != nil {
					return e
				}
			} else {
				o.Record[0] = 0x8c
				o.Animation = 0x14a770
				if w.flag("flag_f6a3") {
					o.Animation = 0x14a7f6
				}
				o.AnimationDelay = 0
				w.Objects[0].Record[52] = 0
			}
		case 0x38, 0x80:
			if !w.flag("action_locked") && !w.flag("sword_contact") && !w.flag("invulnerability") {
				w.set("walking", 0)
				w.damagePlayer()
			}
		case 9:
			w.solidActorContact(o)
		case 0x19:
			if e := w.ordinaryEnemyContact(o, false); e != nil {
				return e
			}
			if !w.flag("sword_contact") {
				for n := 0; n < 3; n++ {
					_, _ = w.Callback("consume_apple", w.Objects[0])
				}
				o.Record[0] = 0x18
				w.sourceAnimation(0x149f5c)
			}
		case 0x85:
			if e := w.merchantContact(o); e != nil {
				return e
			}
		case 1:
			w.springboardContact(o)
		case 0x3c:
			w.lampContact(o)
		case 0x4c:
			if w.get("lives") == 9 {
				continue
			}
			w.set("lives", w.get("lives")+1)
			w.Sound(0x66, false)
			w.set("pending_score", w.get("pending_score")+15)
			w.Delete(o, true)
			if e := w.collectActor(o, 0x1e30ca); e != nil {
				return e
			}
		case 0x47:
			w.set("flag_f6a8", 255)
			w.Sound(0x69, false)
			w.set("pending_score", w.get("pending_score")+25)
			w.Delete(o, true)
			if e := w.collectActor(o, 0x1e335e); e != nil {
				return e
			}
		case 0x49:
			w.checkpointContact(o)
		case 6, 0x13:
			if e := w.ordinaryEnemyContact(o, false); e != nil {
				return e
			}
		case 0x60, 5, 7, 0xb, 0x0c, 0x12, 0x14, 0x17, 0x1a, 0x1b, 0x21, 0x24, 0x2e, 0x83:
			if e := w.ordinaryEnemyContact(o, true); e != nil {
				return e
			}
		case 0x30, 0x31, 0x32, 0x33, 0x37:
			if e := w.knifeContact(o); e != nil {
				return e
			}
		case 0x6c, 0x6d:
			w.camelContact(o)
		case 0x75, 0x76, 0x77, 0x78, 0x79, 0x7a:
			if w.movingPoleContact(o) {
				w.landOnObject(o)
			}
		case 0x56, 0x57, 0x59, 0x5a, 0x5c, 0x5d, 0x5e, 0x5f, 0x65, 0x66, 0x69, 0x6a, 0x6b, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x7b, 0x7c, 0x7d, 0x7e:
			if w.platformContact(o) {
				w.landOnObject(o)
			}
		case 0x22, 0x23, 0x25, 0x26:
			if e := w.guardContact(o); e != nil {
				return e
			}
			if w.parryTarget != nil {
				w.resumeContact = i + 1
				return nil
			}
		case 0x41:
			if w.flag("sword_contact") {
				continue
			}
			fallthrough
		case 0x40:
			if w.get("gems_ascii") == 0x3939 && o.Record[52] != 0 {
				w.Activated[int(uint16(o.Word(50)))] = false
			}
			w.incrementASCII("gems_ascii")
			w.Sound(0x0d, false)
			w.set("pending_score", w.get("pending_score")+15)
			if e := w.collectActor(o, 0x1e30ca); e != nil {
				return e
			}
		case 0x46:
			if w.get("apples_ascii") == 0x3939 && o.Record[52] != 0 {
				w.Activated[int(uint16(o.Word(50)))] = false
			}
			w.incrementASCII("apples_ascii")
			w.Sound(0x0b, false)
			if e := w.collectActor(o, 0x1e30ca); e != nil {
				return e
			}
		case 0x3a:
			w.set("genie_tokens", (w.get("genie_tokens")+1)&255)
			w.Sound(0x69, false)
			w.set("pending_score", w.get("pending_score")+25)
			if e := w.collectActor(o, 0x1e335e); e != nil {
				return e
			}
		case 0x4a:
			w.set("health", min(w.get("max_health"), w.get("health")+3-w.get("difficulty")))
			w.Sound(0x62, false)
			w.set("pending_score", w.get("pending_score")+15)
			if e := w.collectActor(o, 0x1e30ca); e != nil {
				return e
			}
		}
	}
	return nil
}

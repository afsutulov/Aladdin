package game

import "aladdin/internal/choreography"

func (w *NativeWorld) explodeProjectile(o *choreography.Object) error {
	template := 0x1e2ec2
	knife := o.Record[0] == 0x31
	if knife {
		template = 0x1e3516
	}
	if e := w.replaceActor(o, template); e != nil {
		return e
	}
	if w.get("tick")&2 != 0 {
		o.Record[9] = 255
	}
	if knife {
		w.Sound(0x21, false)
	} else {
		w.Sound(3, true)
		w.Sound(0x22, false)
	}
	return nil
}

// projectileContacts ports the ordinary guard cases in the dispatch at 218A.
// Original projectile tests deliberately use unreflected frame boxes.
func (w *NativeWorld) projectileContacts() error {
	for slot := 25; slot < 32; slot++ {
		projectile := w.Objects[slot]
		if projectile == nil || !projectile.Alive() || projectile.Record[0] >= 0x8b {
			continue
		}
		box, ok := w.actorBox(projectile, false)
		if !ok {
			continue
		}
		for target := 1; target <= 24; target++ {
			o := w.Objects[target]
			if o == nil || !o.Alive() || o.Record[0] >= 0x38 {
				continue
			}
			other, ok := w.actorBox(o, false)
			if !ok {
				continue
			}
			if box.Max.X < other.Min.X || box.Max.Y < other.Min.Y || box.Min.X >= other.Max.X || box.Min.Y >= other.Max.Y {
				continue
			}
			if o.Record[0] == 4 {
				if err := w.iagoProjectile(projectile, o); err != nil {
					return err
				}
				continue
			}
			if o.Record[0] == 0x15 || o.Record[0] == 0x16 {
				if err := w.palaceBossProjectile(projectile, o); err != nil {
					return err
				}
				continue
			}
			if o.Record[0] == 0x1c || o.Record[0] == 0x1d {
				projectile.Record[0] = 0x8c
				if projectile.Record[6]&64 == 0 {
					projectile.Record[6] |= 64
					projectile.Record[9] ^= 255
				}
				continue
			}
			if o.Record[0] == 0x10 || o.Record[0] == 0x1e {
				px, _ := w.PlayerPosition()
				x, _ := projectile.Position()
				if o.Record[0] == 0x10 || (uint16(px) < uint16(x)) == (o.Record[9] == 0) {
					if projectile.Record[6]&64 == 0 {
						projectile.Record[6] |= 64
						projectile.Record[9] ^= 255
					}
					continue
				}
			}
			if o.Record[0] >= 0x1e && o.Record[0] <= 0x20 && o.Record[1] == 0 {
				w.breakCaveStatue(o.Record[0])
			}
			if o.Record[0] == 0x35 {
				// 1D6D52 clears the type before selecting the impact, even for a knife.
				w.Delete(projectile, false)
				if e := w.explodeProjectile(projectile); e != nil {
					return e
				}
				continue
			}
			if o.Record[0] == 5 {
				o.Animation, o.AnimationDelay = 0x14d208, 0
				o.Motion, o.MotionDelay = 0x14813e, 0
			}
			if o.Record[0] == 0x27 {
				if e := w.explodeProjectile(projectile); e != nil {
					return e
				}
				if e := w.breakScenery(o); e != nil {
					return e
				}
				continue
			}
			if o.Record[0] == 0x2f {
				// 1D6D18 replaces this projectile without releasing its old
				// artwork reservation; preserve that source capacity loss.
				w.retainArtwork(projectile)
				if e := w.explodeProjectile(projectile); e != nil {
					return e
				}
				w.set("flag_f622", 255)
				o.Record[0], o.Record[6] = 0x8c, 64
				o.Animation = 0x14bd76
				if slot := w.markerSlot(0); slot >= 0 {
					v := choreography.FromTemplate(w.Scripts.Templates[0x1e335e])
					x, y := o.Position()
					v.SetWord(2, x)
					v.SetWord(4, y)
					w.Objects[slot] = v
				}
				continue
			}
			if o.Record[0] == 0x14 || o.Record[0] == 0x17 {
				kind := o.Record[0]
				if e := w.explodeProjectile(projectile); e != nil {
					return e
				}
				if o.Record[1] == 0 {
					if kind == 0x17 {
						w.set("pending_score", w.get("pending_score")+int(o.Record[8]))
						if e := w.finishLampEnemy(o); e != nil {
							return e
						}
					} else if e := w.killProjectileTarget(o); e != nil {
						return e
					}
				} else {
					o.Record[1]--
					o.Animation = 0x14b5c0
					o.AnimationDelay = 0
					if kind == 0x17 {
						o.Animation = 0x14c4f0
						o.Motion, o.MotionDelay = 0, 0
						w.Sound(0x6b, false)
					}
				}
				continue
			}
			if o.Record[0] == 9 || o.Record[0] == 0x18 {
				if e := w.explodeProjectile(projectile); e != nil {
					return e
				}
				continue
			}
			if o.Record[0] == 0x31 || o.Record[0] == 0x37 {
				kind := projectile.Record[0]
				if kind == 0x88 {
					projectile.Record[0] = 0x8c
					projectile.Animation = 0x14a4c8
					projectile.AnimationDelay = 0
					projectile.Motion = 0
				}
				if kind == 0x88 || kind == 0x8a {
					o.Record[0] = 0x8c
					o.Record[6] = 64
				}
				continue
			}
			if o.Record[0] == 0x21 || o.Record[0] == 0x24 {
				if projectile.Record[0] != 0x8a {
					if e := w.explodeProjectile(projectile); e != nil {
						return e
					}
					if byte(w.Random()) >= 200 {
						o.Animation = 0x14af56
						o.AnimationDelay = 0
						o.Motion = 0
						o.MotionDelay = 0
						continue
					}
				}
				if e := w.killProjectileTarget(o); e != nil {
					return e
				}
				continue
			}
			if o.Record[0] == 3 || o.Record[0] == 5 || o.Record[0] == 6 || o.Record[0] == 7 || o.Record[0] == 0xb || o.Record[0] == 0xc || o.Record[0] == 0xf || o.Record[0] == 0x12 || o.Record[0] == 0x13 || o.Record[0] == 0x1a || o.Record[0] == 0x1b || o.Record[0] >= 0x1e && o.Record[0] <= 0x20 || o.Record[0] == 0x2e {
				if o.Record[0] == 3 {
					o.Animation = 0x14a770
					if w.flag("flag_f6a3") {
						o.Animation = 0x14a7f6
					}
					o.AnimationDelay = 0
					o.Record[52] = 0
				}
				lethal := projectile.Record[0] == 0x8a
				if e := w.explodeProjectile(projectile); e != nil {
					return e
				}
				if !lethal && o.Record[1] != 0 {
					o.Record[1]--
					continue
				}
				if e := w.killProjectileTarget(o); e != nil {
					return e
				}
				continue
			}
			animation := 0
			switch o.Record[0] {
			case 0x22:
				animation = 0x14b048
			case 0x23:
				animation = 0x14b46a
			case 0x25:
				animation = 0x14b096
			case 0x26:
				animation = 0x14b49a
			default:
				continue
			}
			if o.Record[60]&128 != 0 {
				continue
			}
			o.Record[0] = 0x8c
			o.Animation = animation
			o.AnimationDelay = 0
			o.Motion = 0
			o.MotionDelay = 0
			lethal := projectile.Record[0] == 0x8a
			if e := w.explodeProjectile(projectile); e != nil {
				return e
			}
			if !lethal && o.Record[1] != 0 {
				o.Record[1]--
				continue
			}
			if e := w.killProjectileTarget(o); e != nil {
				return e
			}
		}
	}
	return nil
}

func (w *NativeWorld) killProjectileTarget(o *choreography.Object) error {
	w.set("pending_score", w.get("pending_score")+int(o.Record[8]))
	if o.Record[0] == 0x14 || o.Record[0] == 0x15 || o.Record[0] == 0x17 {
		w.set("exit_countdown", 32)
	}
	w.Delete(o, true)
	if e := w.replaceActor(o, 0x1e2ed6); e != nil {
		return e
	}
	w.Sound(3, true)
	return nil
}

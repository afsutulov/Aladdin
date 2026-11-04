package game

import "aladdin/internal/choreography"

// lampHandContact ports the two launchers at 1DA472/1DA4F6. A launched
// hand becomes scenery, so a sustained overlap cannot launch a second time.
func (w *NativeWorld) lampHandContact(o *choreography.Object) {
	if int16(w.get("momentum_y")) < 0 || w.flag("action_locked") {
		return
	}
	x, _ := o.Position()
	px, _ := w.PlayerPosition()
	if o.Record[0] == 0x55 && (uint16(px) > uint16(x+24) || uint16(px) <= uint16(x-24)) {
		return
	}
	w.sourceAnimation(0x1491fe)
	w.set("jumping", 255)
	w.set("jump_released", 0)
	if o.Record[0] == 0x55 {
		w.set("momentum_y", 0xf800)
		w.Sound(0x4b, false)
		o.Animation = 0x14c9c8
	} else {
		vx := 0x700
		if o.Record[9] != 0 {
			vx = -vx
		}
		w.set("momentum_x", vx&65535)
		w.set("momentum_y", 0xf900)
		o.Animation = 0x14c9a4
		if w.Random()&1 != 0 {
			w.Sound(0x47, false)
		} else {
			w.Sound(0x46, false)
		}
	}
	o.Record[0] = 0x8c
	o.AnimationDelay = 0
}

// cacheVisibleFrame retains the completed presentation while the next game
// step is built. The source submits its sprite list after the video boundary;
// visible actors, camera and HUD therefore belong to the preceding step.
func (w *NativeWorld) cacheVisibleFrame() {
	v := *w
	v.VisibleFrame = nil
	v.Variables = make(map[string]uint32, len(w.Variables))
	for name, value := range w.Variables {
		v.Variables[name] = value
	}
	for i, o := range w.Objects {
		if o != nil {
			actor := *o
			v.Objects[i] = &actor
		}
	}
	w.VisibleFrame = &v
}

// prepareScene advances the loading sweeps at 1D4D0A, including marker
// streaming, actor retirement, motion and artwork allocation. Loading the
// preconverted room itself needs no console decompression or video hardware.
func (w *NativeWorld) prepareScene() error { return w.prepareSceneEntry(255, 255) }
func (w *NativeWorld) prepareSceneEntry(tick, grounded int) error {
	w.set("tick", tick)
	w.set("grounded", grounded)
	w.set("jump_hold", 255)
	if w.Stage.Index == 9 {
		w.set("flag_f5c0", 1)
	}
	w.set("camera_x", w.Checkpoint.CameraX)
	w.set("camera_tile_x", w.Checkpoint.CameraX)
	w.LastActivationX, w.LastActivationY = w.Checkpoint.CameraX, w.Checkpoint.CameraY
	w.set("flag_f6a5", 255)
	c := choreography.Choreographer{Scripts: w.Scripts}
	draw := func() error {
		if err := w.moveActors(); err != nil {
			return err
		}
		if w.get("tick")&1 != 0 {
			for i := 0; i < len(w.Objects); i++ {
				o := w.Objects[i]
				if o != nil && o.Alive() {
					if err := c.Animation(w, o); err != nil {
						return err
					}
				}
			}
		}
		w.updateHUDWave()
		if w.Stage.Index == 10 {
			w.set("background_phase", (w.get("background_phase")+1)&65535)
		} else if w.Stage.Index == 0 {
			w.set("background_phase", (w.get("background_phase")-1)&65535)
		}
		w.advanceCameraTiles()
		return w.activateMarkers()
	}
	for i := 0; i < 23; i++ {
		if w.get("camera_x")+16 < w.Stage.Width-0x161 {
			w.set("camera_x", w.get("camera_x")+16)
			if err := draw(); err != nil {
				return err
			}
		}
	}
	for i := 0; i < 23; i++ {
		w.set("tick", (w.get("tick")+1)&255)
		if w.get("camera_tile_x") >= 17 {
			w.set("camera_x", w.get("camera_x")-16)
			if err := draw(); err != nil {
				return err
			}
		}
	}
	w.set("flag_f6a5", 0)
	w.set("tick", 255)
	w.sourceAnimation(0x1494aa)
	w.set("grounded", 255)
	w.set("fall_ticks", 0)
	w.set("momentum_y", 0)
	for i := 0; i < len(w.Objects); i++ {
		o := w.Objects[i]
		if o != nil && o.Alive() {
			if err := c.Animation(w, o); err != nil {
				return err
			}
		}
	}
	w.updateHUDWave()
	return nil
}

// lampBehavior is the Inside the Lamp callback at 1E09C4. The periodic
// counter resets even outside its spawn region or when the actor pool is full.
func (w *NativeWorld) lampBehavior() {
	x, y := w.PlayerPosition()
	clock := (w.get("flag_f63b") + 1) & 255
	w.set("flag_f63b", clock)
	if clock >= 0x50 {
		w.set("flag_f63b", 0)
		if uint16(y) >= 0x19b && uint16(x) >= 0x6b0 && uint16(x) < 0x9c8 {
			if slot := w.markerSlot(2); slot >= 0 {
				o := choreography.FromTemplate(w.Scripts.Templates[0x1e36a6])
				o.Record[0], o.Record[9], o.Record[60] = 0x56, 255, 2
				o.Animation, o.Motion = 0x14ca1e, 0x147742
				o.SetWord(2, 0xa22)
				o.SetWord(4, 0x220+int(w.Random()&0x20))
				w.Objects[slot] = o
			}
		}
	}
	if !w.flag("exit_countdown") && uint16(x) >= 0x1b9e {
		w.set("exit_countdown", 1)
	}
	if uint16(x) >= 0x4d3 && uint16(x) < 0x6ac && w.get("tick")&127 == 0x49 {
		if slot := w.markerSlot(3); slot >= 0 {
			o := choreography.FromTemplate(w.Scripts.Templates[0x1e384a])
			o.SetWord(2, 0x5a0)
			o.SetWord(4, 0x100)
			w.Objects[slot] = o
		}
	}
	w.set("flag_f62b", (w.get("flag_f62b")+1)&127)
}

// lampScroll is the two-speed rear-plane row scroll at 1D57F4. The logical
// right shift after negation preserves the source unsigned-word arithmetic.
func lampScroll(camera int) [28]int {
	n := -((uint16(camera) + 16) >> 1)
	var rows [28]int
	for i := range rows {
		rows[i] = int(n & 511)
		if i < 16 {
			rows[i] = int(n >> 1 & 511)
		}
	}
	return rows
}

package game

import "aladdin/internal/choreography"

// breakScenery is the shared sword/apple path at 1D667A. It opens a gem
// and creates a separate puff, subject to the original fixed actor pools.
func (w *NativeWorld) breakScenery(o *choreography.Object) error {
	o.Record[0] = 0x8c
	o.Animation = 0x14ac98
	o.AnimationDelay = 0
	x, y := o.Position()
	if slot := w.markerSlot(3); slot >= 0 {
		v := w.actorTemplate(0x1e2f9e, w.Objects[slot])
		v.Record[0] = 0x41
		v.Animation = 0x14a532
		v.SetWord(2, x)
		v.SetWord(4, y)
		w.Objects[slot] = v
	}
	if slot := w.markerSlot(0); slot >= 0 {
		v := w.actorTemplate(0x1e334a, w.Objects[slot])
		v.SetWord(2, x)
		v.SetWord(4, y)
		w.Objects[slot] = v
	}
	return nil
}

// releaseScenery converts the two global decoration-release callbacks. The
// first available decoration is selected from slot 24 down to slot one.
func (w *NativeWorld) releaseScenery(motion int) {
	for i := 24; i >= 1; i-- {
		o := w.Objects[i]
		if o == nil || o.Record[0] != 0x91 {
			continue
		}
		o.Record[24] = byte(int(w.Random()&7) - 3)
		o.Record[0] = 0x8c
		o.Motion = motion
		o.Animation = 0x14bf9c
		o.Record[6] |= 64
		if o.Record[52] != 0 {
			w.Activated[int(uint16(o.Word(50)))] = false
		}
		return
	}
}

// finishLampEnemy is 1D99A6. The source leaves a puff in the defeated
// actor's slot and starts its Genie scene in the descending actor pool.
func (w *NativeWorld) finishLampEnemy(o *choreography.Object) error {
	x, y := o.Position()
	w.Delete(o, true)
	if e := w.replaceActor(o, 0x1e334a); e != nil {
		return e
	}
	if slot := w.markerSlot(2); slot >= 0 {
		v := w.actorTemplate(0x1e3b56, w.Objects[slot])
		v.SetWord(2, x)
		v.SetWord(4, y-32)
		v.Record[9] = 255
		w.Objects[slot] = v
		w.set("flag_f647", 8)
		w.Sound(0x14, false)
	}
	return nil
}

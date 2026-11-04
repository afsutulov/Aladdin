package game

import (
	"aladdin/internal/choreography"
	"fmt"
)

// showMessage is the sprite-text path at 1DCA76. The converted message holds
// original glyph choreography and spacing, rather than a replacement font.
func (w *NativeWorld) showMessage(id int) error {
	return w.buildMessage(id, false)
}
func (w *NativeWorld) buildMessage(id int, storage bool) error {
	m, ok := w.Scripts.Messages[id]
	if !ok {
		return fmt.Errorf("message %x still requires conversion", id)
	}
	w.set("message_id", id)
	for i := 1; i < 32; i++ {
		if o := w.Objects[i]; o != nil && o.Record[0] == 0x8d {
			w.Delete(o, false)
		}
	}
	for _, g := range m.Glyphs {
		slot := -1
		for i := 1; i < 32; i++ {
			if w.Objects[i] == nil || !w.Objects[i].Alive() {
				slot = i
				break
			}
		}
		if slot < 0 {
			break
		}
		o := choreography.FromTemplate(w.Scripts.Templates[0x1e2f26])
		if storage {
			o = w.actorTemplate(0x1e2f26, w.Objects[slot])
		}
		o.Animation = g.Animation
		o.Motion = g.Motion
		o.SetWord(2, g.X)
		o.SetWord(4, g.Y)
		w.Objects[slot] = o
	}
	return nil
}
func (w *NativeWorld) decrementASCII(name string) {
	v := w.get(name)
	if v == 0x3030 {
		return
	}
	lo, hi := byte(v), byte(v>>8)
	lo--
	if lo < '0' {
		lo = '9'
		hi--
	}
	w.set(name, int(hi)<<8|int(lo))
}

// merchantContact is 1DA640: Up buys a life for five gems on the left,
// or a continue for ten gems on the right. Releasing Up clears its latch.
func (w *NativeWorld) merchantContact(o *choreography.Object) error {
	targets := func() { w.set("look_target_x", 160); w.set("look_target_y", 384) }
	if w.flag("action_locked") {
		targets()
		return nil
	}
	if !w.flag("input_up") {
		w.set("flag_f636", 0)
		targets()
		return nil
	}
	if w.flag("flag_f636") {
		targets()
		return nil
	}
	x, _ := o.Position()
	px, _ := w.PlayerPosition()
	price := 10
	if px < x {
		price = 5
	}
	need := 0x3130
	if price == 5 {
		need = 0x3035
	}
	if w.get("gems_ascii") < need {
		for _, a := range w.Objects {
			if a != nil && a.Record[0] == 0x8d {
				return nil
			}
		}
		w.set("flag_f636", 255)
		return w.showMessage(0x12)
	}
	w.set("flag_f636", 255)
	if price == 5 && w.get("lives") == 9 {
		return w.showMessage(0x17)
	}
	for i := 0; i < price; i++ {
		w.decrementASCII("gems_ascii")
	}
	w.Sound(0x48, false)
	if price == 5 {
		w.set("lives", min(9, w.get("lives")+1))
		w.Sound(0x66, false)
	} else {
		w.set("continues", (w.get("continues")+1)&255)
	}
	if e := w.showMessage(0x15); e != nil {
		return e
	}
	targets()
	return nil
}

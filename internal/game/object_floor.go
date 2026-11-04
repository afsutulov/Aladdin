package game

import "aladdin/internal/choreography"

// objectFloor is the ordinary actor terrain path at 1D8366..1D856C.
// A descending actor probes three contour rows, then snaps to the first
// surface. Behaviour bit 4 selects the original bounce response.
func (w *NativeWorld) objectFloor(o *choreography.Object) error {
	if o.Record[6]&1 == 0 || o.Word(26) < 0 || o.Frame < 0 {
		return nil
	}
	o.Record[61] = 0
	x, y := o.Position()
	disableFloor := func() { o.Record[6] &^= 1; o.Record[6] |= 64 }
	if y > w.Stage.Height+0xc8 {
		disableFloor()
		return nil
	}
	row, col := (y-0xf0)>>4, (x+16)>>4
	columns := w.Stage.Width / 16
	probeY := y - 16
	for n := 0; n < 3; n++ {
		i := (row+n)*columns + col
		if i < 0 || i >= len(w.Stage.Room) {
			disableFloor()
			return nil
		}
		f := int(w.Stage.Room[i]) / 2
		if f+2 >= len(w.Stage.Floor) {
			disableFloor()
			return nil
		}
		contour := int(w.Stage.Floor[f])
		w.set("flag_f62f", contour)
		p := contour*16 + (x & 15)
		height := 0
		if p < len(w.Stage.Contours) {
			height = int(w.Stage.Contours[p] & 63)
		}
		if height != 0 {
			o.Record[61] = w.Stage.Floor[f+2]
			if o.Record[60]&16 != 0 {
				o.Record[9] = 0
				if contour < len(w.Scripts.BounceFacing) && w.Scripts.BounceFacing[contour]&128 != 0 {
					o.Record[9] = 255
				}
				o.SetWord(26, -int(uint16(o.Word(26))>>1))
				if o.Record[0] != 0x37 {
					w.Sound(0x50, true)
					w.Sound(0x50, false)
				}
			} else {
				o.SetWord(26, 0)
			}
			o.SetWord(4, (probeY&^15)+(height-1))
			o.Record[7] |= 16
			return nil
		}
		probeY += 16
	}
	if o.Record[6]&128 == 0 {
		o.SetWord(26, int(o.Word(26))+0x78)
	}
	return nil
}

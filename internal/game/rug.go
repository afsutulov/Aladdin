package game

import (
	"aladdin/internal/choreography"
	"fmt"
	"image"
)

// initializeRugRide reconstructs the stage-specific setup at 1E0E3A.
func (w *NativeWorld) initializeRugRide() {
	w.set("rug_event_cursor", 0x2b59)
	w.set("rug_distance", 0)
	w.set("rug_speed", 6)
	w.set("rug_acceleration_clock", 0)
	w.set("rug_palette_cursor", 0)
	w.set("rug_lava_scroll", 320)
	w.set("flag_f6a4", 255)
	w.set("tick", 255)
	w.set("grounded", 255)
	w.set("jump_hold", 255)
	w.set("camera_x", w.Checkpoint.CameraX)
	w.set("camera_tile_x", w.Checkpoint.CameraX)
	w.set("camera_tile_y", w.Checkpoint.CameraY)
	w.sourceAnimation(0x149a8a)
	o := choreography.FromTemplate(w.Scripts.Templates[0x1e349e])
	x, y := w.PlayerPosition()
	o.SetWord(2, x)
	o.SetWord(4, y)
	w.Objects[1] = o
	w.PaletteOverrides = nil
}

// prepareRugRide advances the source's two 23-draw loading sweeps. The
// carpet keeps its motion/animation phase; its position is reset on entry.
func (w *NativeWorld) prepareRugRide() error {
	for i := 0; i < 46; i++ {
		if i >= 23 {
			w.set("tick", (w.get("tick")+1)&255)
		}
		if err := w.lifeActors(); err != nil {
			return err
		}
		w.updateHUDWave()
	}
	w.set("tick", 255)
	x, y := w.PlayerPosition()
	w.Objects[1].SetWord(2, x)
	w.Objects[1].SetWord(4, y)
	w.sourceAnimation(0x149a70)
	c := choreography.Choreographer{Scripts: w.Scripts}
	if err := c.Animation(w, w.Objects[0]); err != nil {
		return err
	}
	if e := c.Animation(w, w.Objects[1]); e != nil {
		return e
	}
	w.updateHUDWave()
	return nil
}

// rugControls is the automatic flight and vertical input path at 1D42B4.
func (w *NativeWorld) rugControls(in Input) {
	x, y := w.PlayerPosition()
	speed := w.get("rug_speed")
	x = (x + speed) & 65535
	if o := w.Objects[1]; o != nil {
		o.SetWord(2, int(o.Word(2))+speed)
	}
	w.set("player_x", x)
	w.set("camera_x", (x-120)&65535)
	if in.Up {
		if uint16(y-w.get("camera_y")) >= 0x112 {
			y -= 4
		}
	} else if in.Down && uint16(y-w.get("camera_y")) < 0x187 {
		y += 4
	}
	w.set("player_y", y)
}

// rugBehavior ports 1E08DA. Schedule entries are converted stage data;
// actor construction, timing, flight and palette changes are native Go.
func (w *NativeWorld) rugBehavior() error {
	if o := w.Objects[1]; o != nil {
		w.set("player_x", int(uint16(o.Word(2))))
		o.SetWord(4, w.get("player_y"))
	}
	clock := w.get("rug_acceleration_clock") + 1
	if clock >= 0x136 {
		clock = 0
		w.set("rug_speed", (w.get("rug_speed")+1)&65535)
	}
	w.set("rug_acceleration_clock", clock)
	d := w.Stage.RugRide
	if d == nil {
		return fmt.Errorf("missing Rug Ride schedule")
	}
	for n := 0; n < 2; n++ {
		i := w.get("rug_palette_cursor") / 6
		if i >= len(d.LavaCycle) {
			i = 0
		}
		p := d.LavaCycle[i]
		if w.PaletteOverrides == nil {
			w.PaletteOverrides = map[int][3]uint8{}
		}
		w.PaletteOverrides[p.Index] = p.RGB
		w.set("rug_palette_cursor", ((i+1)%len(d.LavaCycle))*6)
	}
	if w.get("tick")&63 == 0 {
		w.set("rug_lava_scroll", (w.get("rug_lava_scroll")+1)&65535)
	}
	distance := w.get("rug_distance") + w.get("rug_speed")
	if distance < 192 {
		w.set("rug_distance", distance)
		return nil
	}
	w.set("rug_distance", (distance-192)&65535)
	cursor := w.get("rug_event_cursor")
	i := (cursor - 0x2b59) / 2
	if i < 0 || i >= len(d.Events) {
		return fmt.Errorf("Rug Ride schedule cursor %x outside converted data", cursor)
	}
	w.set("rug_event_cursor", cursor+2)
	if err := w.rugEvent(d.Events[i].Kind, d.Events[i].Value); err != nil {
		return err
	}
	random := w.Random()
	if byte(random) < 0x46 {
		if slot := w.markerSlot(3); slot >= 0 {
			o := choreography.FromTemplate(w.Scripts.Templates[0x1e35b6])
			o.SetWord(2, w.get("camera_x")+320)
			o.SetWord(4, int(random&15)+0x1c4)
			w.Objects[slot] = o
		}
	}
	return nil
}

func (w *NativeWorld) rugEvent(kind, value int) error {
	spawn := func(template, pool, x, y, animation, motion int) *choreography.Object {
		slot := w.markerSlot(pool)
		if slot < 0 {
			return nil
		}
		o := choreography.FromTemplate(w.Scripts.Templates[template])
		o.SetWord(2, x)
		o.SetWord(4, y)
		if animation != 0 {
			o.Animation = animation
		}
		if motion != 0 {
			o.Motion = motion
		}
		w.Objects[slot] = o
		return o
	}
	x, y := w.get("camera_x")+320, w.get("camera_y")+256+value
	switch kind {
	case 0xfc, 0xfd, 0xff: // 1E014C is deliberately inert.
	case 0xfe:
		spawn(0x1e38c2, 3, x, y, 0, 0)
	case 0xf5:
		if w.get("gems_ascii") != 0x3939 {
			if o := spawn(0x1e2f9e, 3, x, y, 0x14a532, 0); o != nil {
				o.Record[0] = 0x40
				o.Record[41] = 1
			}
		}
	case 0xf6:
		if o := spawn(0x1e2f9e, 3, x, y, 0x14a578, 0x148d50); o != nil {
			o.Record[0] = 0x3a
			o.Record[41] = 6
		}
	case 0xf7:
		if spawn(0x1e38d6, 3, x, y, 0, 0) != nil {
			spawn(0x1e38d6, 3, x+32, y, 0, 0)
		}
	case 0xf8:
		if w.get("lives") != 9 {
			spawn(0x1e2fb2, 3, x, y, 0, 0)
		}
	case 0xf9:
		if spawn(0x1e38ae, 0, 398, 356, 0x14d8fa, 0x148392) != nil {
			spawn(0x1e38ae, 0, 398, 376, 0x14d902, 0x14837c)
		}
	case 0xfa:
		spawn(0x1e38ae, 0, 398, 356, 0x14d8fa, 0x1483a6)
	case 0xfb:
		spawn(0x1e38ae, 0, 398, 356, 0x14d902, 0x14837c)
	case 0xf4:
		spawn(0x1e38ae, 0, 398, 356, 0x14d90a, 0)
	case 0xf3:
		w.set("flag_f62d", value)
	case 0xf2:
		w.set("exit_countdown", 1)
		w.set("flag_f51f", 255)
	default:
		return fmt.Errorf("Rug Ride event %x requires conversion", kind)
	}
	return nil
}

// rugScroll is 1D556C: two foreground speeds and four rear-plane bands.
func rugScroll(camera, lava int) (front, back [28]int) {
	x := uint16(camera)
	a := int((-((x >> 1) + 16)) & 511)
	b := int((-(x + 16)) & 511)
	for i := range front {
		front[i] = a
		if i >= 21 {
			front[i] = b
		}
		switch {
		case i < 3:
			back[i] = b
		case i < 20:
			back[i] = lava & 511
		case i < 24:
			back[i] = int((-(((x >> 1) + x) >> 1) - 16) & 511)
		default:
			back[i] = int((-(x << 1)) & 511)
		}
	}
	return
}
func (g *Game) drawRugScene(dst *image.RGBA) {
	front, back := rugScroll(g.CameraX, g.World.get("rug_lava_scroll"))
	// Attract presentation is latched after the pending tile phase advances.
	// The ordinary live-world renderer predicts the following presentation.
	phase := g.World.get("scene_tile_phase") / 14
	if !g.World.flag("attract_mode") {
		phase++
	}
	phase %= 8
	plane := func(name string, rows [28]int) {
		im, e := g.Project.Image(name)
		if e != nil {
			g.Err = e
			return
		}
		im = g.paletteImage(im)
		for row, offset := range rows {
			start := offset
			if start > 0 {
				start -= 512
			}
			for ; start < Width; start += 512 {
				drawOver(dst, image.Rect(start, row*8, start+512, row*8+8), im, image.Pt(0, row*8))
			}
		}
	}
	plane(fmt.Sprintf("stage08-bg-%d.png", phase), back)
	plane("stage08-low.png", front)
	g.drawActors(dst, false)
	plane(fmt.Sprintf("stage08-bg-%d-high.png", phase), back)
	plane("stage08-high.png", front)
	g.drawActors(dst, true)
	g.drawHUD(dst)
}

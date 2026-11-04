package game

import (
	"aladdin/internal/choreography"
	"image"
	"image/color"
	"image/draw"
)

func geniePhase(p FlowPhase) bool {
	return p == FlowGenieFadeIn || p == FlowGenie || p == FlowGenieFadeOut
}

// beginGenie ports 1DBF12. Only converted artwork/game data are consumed;
// all selection, timing, controls and inventory changes below are native Go.
func (w *NativeWorld) beginGenie() error {
	w.stopAudio()
	leaks := w.ArtworkLeaks
	w.clearLifeActors()
	w.ArtworkLeaks = leaks
	w.set("invulnerability", 0)
	w.PaletteBank2 = 0
	w.PaletteOverrides = nil
	for _, v := range []struct{ slot, template, x, y, animation, motion int }{
		{1, 0x1e3c1e, 294, 408, 0x14e7ca, 0},
		{5, 0x1e3c32, 376, 404, 0, 0x148d50},
		{6, 0x1e2f26, 392, 404, 0, 0x148d9e},
		{7, 0x1e2f26, 409, 404, 0, 0x148d9e},
	} {
		o := choreography.FromTemplate(w.Scripts.Templates[v.template])
		o.SetWord(2, v.x)
		o.SetWord(4, v.y)
		if v.animation != 0 {
			o.Animation = v.animation
		}
		if v.motion != 0 {
			o.Motion = v.motion
		}
		w.Objects[v.slot] = o
	}
	w.genieDigits()
	w.set("tick", 255)
	if err := w.lifeActors(); err != nil {
		return err
	}
	w.updateHUDWave()
	w.Flow.Phase, w.Flow.Age = FlowGenieFadeIn, 0
	return nil
}

// genieDigits is 1DC308. Updating an animation pointer does not reset its delay.
// Tens disappearing clears the actor byte without releasing its artwork.
func (w *NativeWorld) genieDigits() {
	n := int(byte(w.get("genie_tokens")))
	x := 392
	tens := w.Objects[6]
	ones := w.Objects[7]
	if tens != nil {
		if n/10 == 0 {
			if tens.Alive() {
				w.retainArtwork(tens)
			}
			tens.Record[0] = 0
		} else {
			tens.Animation = w.Scripts.GenieDigits[n/10]
			tens.SetWord(2, x)
			tens.Record[0] = 0x8c
			x += 17
		}
	}
	if ones != nil {
		ones.Animation = w.Scripts.GenieDigits[n%10]
		ones.SetWord(2, x)
	}
}

// chooseGenie preserves the weighted sixteen-entry table and retries on the
// same kind. Each retry consumes a normal source RNG step (1DBEE6).
func (w *NativeWorld) chooseGenie() {
	o := w.Objects[1]
	for {
		c := w.Scripts.GenieChoices[int(w.Random()&0x78)/8]
		if byte(c.Kind) == o.Record[0] {
			continue
		}
		o.Record[0] = byte(c.Kind)
		o.Animation = c.Animation
		o.AnimationDelay = 0
		return
	}
}

func (w *NativeWorld) genieInput(in Input) {
	// Source polls A/B/C, but not Start, and requires release after a claim.
	if !in.Jump && !in.Sword && !in.Apple {
		w.set("flag_f643", 0)
		return
	}
	if !w.flag("flag_f641") {
		return
	}
	w.Sound(0x5a, false)
	w.set("flag_f643", 255)
	w.set("flag_f641", 0)
	w.set("genie_tokens", (w.get("genie_tokens")-1)&255)
	if w.get("flag_f645") != 1 {
		w.set("flag_f645", (w.get("flag_f645")-1)&255)
	}
	w.genieDigits()
	w.set("flag_f644", 60)
	switch w.Objects[1].Record[0] {
	case 1:
		w.Sound(0x5b, false)
		w.set("lives", min(w.get("lives")+1, 9))
		w.Sound(0x66, false)
	case 2:
		w.Sound(0x5b, false)
		if w.get("gems_ascii") != 0x3939 {
			w.incrementASCII("gems_ascii")
		}
	case 3:
		w.Sound(0x5b, false)
		for i := 0; i < 5; i++ {
			if w.get("apples_ascii") != 0x3939 {
				w.incrementASCII("apples_ascii")
			}
		}
	case 4:
		w.Sound(0x0e, false)
		w.set("genie_tokens", 0)
		w.genieDigits()
	}
}

func (w *NativeWorld) genieTick(in Input) error {
	w.cacheVisibleFrame()
	// The palette pulse is written after the submitted frame. Keep its old
	// values independent of the map that the next tick mutates.
	if w.PaletteOverrides != nil {
		p := map[int][3]uint8{}
		for i, c := range w.PaletteOverrides {
			p[i] = c
		}
		w.VisibleFrame.PaletteOverrides = p
	}
	if w.Flow.Age != 0 {
		w.genieInput(in)
	}
	w.Flow.Age++
	w.set("tick", (w.get("tick")+1)&255)
	if !w.flag("flag_f643") {
		delay := (w.get("flag_f644") - 1) & 255
		w.set("flag_f644", delay)
		if delay == 0 {
			w.chooseGenie()
			w.Sound(0x5b, false)
			if !w.flag("genie_tokens") {
				w.Flow.Phase, w.Flow.Age = FlowGenieFadeOut, 0
				return nil
			}
			w.Sound(0x51, false)
			w.set("flag_f644", w.get("flag_f645")>>2)
			w.set("flag_f641", 255)
		}
	}
	if err := w.lifeActors(); err != nil {
		return err
	}
	w.updateHUDWave()
	w.geniePulse()
	return nil
}
func (w *NativeWorld) geniePulse() {
	row := -1
	switch w.get("tick") & 63 {
	case 10:
		row = 0
	case 30:
		row = 1
	}
	if row < 0 {
		return
	}
	if w.PaletteOverrides == nil {
		w.PaletteOverrides = map[int][3]uint8{}
	}
	for _, c := range w.Scripts.GeniePulse[row] {
		w.PaletteOverrides[c.Index] = c.RGB
	}
}
func (w *NativeWorld) finishGenie() error {
	w.selectNextStage()
	if abuStage(w.Flow.NextStage) {
		return w.beginAbuIntro()
	}
	w.clearLifeActors()
	w.Flow.Phase, w.Flow.Age = FlowLoad, 0
	return nil
}

func (g *Game) drawGenie() {
	p := g.renderPalette()
	bg := p[0]
	draw.Draw(g.Canvas, g.Canvas.Rect, &image.Uniform{C: color.RGBA{bg[0], bg[1], bg[2], 255}}, image.Point{}, draw.Src)
	for _, layer := range []struct {
		name string
		high bool
	}{{"genie-low.png", false}, {"genie-high.png", true}} {
		im, err := g.Project.Image(layer.name)
		if err != nil {
			g.Err = err
			return
		}
		drawOver(g.Canvas, g.Canvas.Rect, g.paletteImage(im), image.Point{})
		g.drawActors(g.Canvas, layer.high)
	}
	g.drawHUD(g.Canvas)
}

// StartGenie opens the bonus directly for scene selection/capture. Normal play
// enters through the original completion route and uses its collected tokens.
func (g *Game) StartGenie(tokens byte) {
	if tokens == 0 {
		return
	}
	if g.World == nil {
		g.Start(1)
		if g.Err != nil {
			return
		}
	}
	g.World.set("genie_tokens", int(tokens))
	g.Mode = 1
	g.Err = g.World.beginGenie()
}

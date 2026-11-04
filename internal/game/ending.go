package game

import (
	"aladdin/internal/choreography"
	"image"
	"image/color"
)

// EndingSession ports the terminal route at 1DF734. Its resource jobs, actor
// loops, independent video waits, palette changes and credit roll run in Go.
type EndingSession struct {
	World   *NativeWorld
	Scene   *StorySession
	Credits *CreditRoll
	Phase   string
	Age     int
	Music   bool
}

func (g *Game) beginEnding() {
	w := g.World
	w.clearLifeActors()
	w.Life.Phase = LifeClear
	w.set("camera_x", 0)
	w.set("camera_y", 0)
	w.stopAudio()
	if g.Menu == nil || g.Menu.Music {
		w.requestMusic(26)
	}
	s := &StorySession{World: w, RasterLine: -1, Loading: w.Scripts.Ending.FlightLoad}
	g.Ending = &EndingSession{World: w, Scene: s, Phase: "flight-load", Music: g.Menu == nil || g.Menu.Music}
}
func (e *EndingSession) phase(name string) { e.Phase = name; e.Age = 0 }
func (e *EndingSession) actor(template, x, y, animation int) {
	o := choreography.FromTemplate(e.World.Scripts.Templates[template])
	o.SetWord(2, x)
	o.SetWord(4, y)
	if animation != 0 {
		o.Animation = animation
		o.Motion = 0
	}
	e.World.Objects[1] = o
}
func (e *EndingSession) tick() error {
	e.Scene.DisplayActors = e.Scene.PendingActors
	e.World.set("tick", (e.World.get("tick")+1)&255)
	if err := e.World.lifeActors(); err != nil {
		return err
	}
	e.Scene.cacheActors()
	return nil
}
func endingPalette(p *[64][3]uint8, target [64][3]uint8) {
	for i := range p {
		for ch := 0; ch < 3; ch++ {
			v, t := int(p[i][ch]), int(target[i][ch])
			if v < t {
				v = min(t, v+34)
			} else if v > t {
				v = max(t, v-34)
			}
			p[i][ch] = uint8(v)
		}
	}
}
func (e *EndingSession) Update(in Input) error {
	s, w := e.Scene, e.World
	s.DisplayActors = s.PendingActors
	w.SoundEvents = nil
	e.Age++
	switch e.Phase {
	case "flight-load":
		if e.Age == w.Scripts.Ending.FlightLoad {
			s.Loading = 0
			s.Layout.Background = 0x156d88
			s.Palette = w.Scripts.Ending.FlightPalette
			e.actor(0x1e3b2e, 128, 368, 0)
			e.phase("flight")
			e.Age = 1
			return e.tick()
		}
	case "flight":
		if e.Age <= 321 {
			return e.tick()
		}
		e.phase("flight-out")
		e.Age = 1
	case "flight-out":
		endingPalette(&s.Palette, [64][3]uint8{})
		if e.Age == 16 {
			e.phase("kiss-load")
			s.Loading = w.Scripts.Ending.KissLoad
		}
	case "kiss-load":
		if e.Age == w.Scripts.Ending.KissLoad {
			s.Loading = 0
			s.Layout.Background = 0x158a2f
			e.actor(0x1e3b1a, 384, 376, 0x14e5d2)
			w.set("invulnerability", 0)
			w.set("tick", 254)
			e.phase("kiss-prime")
			return e.tick()
		}
	case "kiss-prime":
		e.phase("kiss-in")
		e.Age = 1
		s.DisplayActors = s.PendingActors
	case "kiss-in":
		endingPalette(&s.Palette, w.Scripts.Ending.KissPalette)
		if e.Age == 16 {
			e.phase("kiss")
		}
	case "kiss":
		s.DisplayActors = s.PendingActors
		// 1DF86C waits once before 1DD0F2's separate video wait.
		if e.Age&1 == 0 {
			if err := e.tick(); err != nil {
				return err
			}
		}
		if e.Age == 162 {
			e.phase("hold")
		}
	case "hold":
		if e.Age == 121 {
			e.phase("credits-in")
		}
	case "credits-in":
		if e.Age > 1 {
			endingPalette(&s.Palette, w.Scripts.Menu.CreditPalette)
		}
		if e.Age == 16 {
			e.Credits = newCreditRoll(w.Scripts.Menu)
			e.Credits.Tick = (w.get("tick") + 1) & 255
			e.phase("credits")
		}
	case "credits":
		if e.Age == 1 {
			w.set("tick", e.Credits.Tick)
			return nil
		}
		// Gameplay disables the interrupt text-skip latch (1DDAB0), unlike
		// attract credits. Only Start is sampled by 1DEEFE here.
		e.Credits.Update(Input{Enter: in.Enter})
		s.Palette = e.Credits.Palette
		w.set("tick", e.Credits.Tick)
		if e.Credits.Done {
			e.phase("out")
			e.Age = 1
		}
	case "return-wait":
		if e.Age == 16 {
			e.phase("done")
		}
	case "out":
		endingPalette(&s.Palette, [64][3]uint8{})
		if e.Age == 16 {
			e.phase("return-wait")
		}
	}
	return nil
}
func (g *Game) updateEnding(in Input) {
	if err := g.Ending.Update(in); err != nil {
		g.Err = err
		return
	}
	if g.Ending.Phase == "done" {
		g.OpenTitle()
	}
}
func (g *Game) renderEnding() *image.RGBA {
	e := g.Ending
	g.renderScene(e.Scene)
	// The direct first palette write crosses a scanline. The optional
	// sound-driver request shifts that boundary by six scanlines.
	if e.Phase == "flight" && e.Age == 1 {
		line, lastColor := 87, 27
		if !e.Music {
			line, lastColor = 81, 20
		}
		for y := 0; y <= line; y++ {
			for x := 0; x < Width; x++ {
				keep := false
				if y == line {
					for _, name := range []string{"story-156d88-low.png", "story-156d88-high.png"} {
						im, err := g.Project.Image(name)
						if err != nil {
							g.Err = err
							return g.Canvas
						}
						n := im.(*image.Paletted).ColorIndexAt(x, y)
						if n != 0 && int(n) <= lastColor {
							keep = true
						}
					}
				}
				if !keep {
					g.Canvas.SetRGBA(x, y, color.RGBA{A: 255})
				}
			}
		}
	}
	if e.Credits != nil && (e.Phase == "credits" || e.Phase == "out") {
		g.drawCreditText(e.Credits, e.Scene.Palette, true)
	}
	return g.Canvas
}

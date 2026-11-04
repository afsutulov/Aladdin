package game

import (
	"aladdin/internal/choreography"
	"aladdin/internal/data"
	"image"
	"image/draw"
)

// NewGamePrelude ports 1DBCB8/1DC866 and their resource-job presentation.
// Only ordinary Go state, converted maps and native actor choreography run here.
type NewGamePrelude struct {
	World               *NativeWorld
	Scene               *StorySession
	Data                *data.PreludeData
	Outgoing            *image.RGBA
	Phase               string
	Age, Cycle          int
	PendingCycle, Flash bool
	Done                bool
}

func (g *Game) beginPrelude(outgoing *image.RGBA) {
	w := g.Menu.World
	// Start is accepted with the last unfaded title still displayed; the next
	// update enters the first outgoing-fade boundary.
	p := &NewGamePrelude{World: w, Data: &g.Project.Scripts.Prelude, Outgoing: outgoing, Phase: "title-out", Age: -1}
	p.Scene = &StorySession{World: w, RasterLine: -1}
	g.Prelude = p
	// The original keeps title music running through its outgoing fade.
	if g.soundOutputWorld == w {
		g.soundOutputMode = g.Mode
	}
}
func (p *NewGamePrelude) phase(name string) { p.Phase = name; p.Age = 0 }
func (p *NewGamePrelude) black()            { p.Scene.Palette = [64][3]uint8{} }
func (p *NewGamePrelude) scarab() error {
	w := p.World
	w.clearLifeActors()
	w.Life.Phase = LifeClear
	w.set("flag_f624", 0)
	w.PaletteBank2 = 0x152274
	w.set("tick", 255)
	for _, v := range []struct {
		slot, x, animation int
		flip               bool
	}{{20, 88, 0, false}, {19, 476, 0x14add8, true}} {
		o := choreography.FromTemplate(w.Scripts.Templates[0x1e3156])
		o.SetWord(2, v.x)
		o.SetWord(4, 496)
		if v.animation != 0 {
			o.Animation = v.animation
		}
		if v.flip {
			o.Record[9] = 255
		}
		w.Objects[v.slot] = o
	}
	p.Cycle = 32
	p.Flash = false
	p.PendingCycle = false
	p.Scene.Layout.Background = 0x157134
	p.Scene.Page.Foreground = 0
	if e := w.lifeActors(); e != nil {
		return e
	}
	p.Scene.cacheActors()
	p.phase("scarab-warm")
	return nil
}
func (p *NewGamePrelude) scarabTick() error {
	if p.PendingCycle {
		start := p.Cycle / 2
		copy(p.Scene.Palette[:16], p.Data.ScarabCycle[start:start+16])
		p.Cycle -= 2
		if p.Cycle == 0 {
			p.Cycle = 32
		}
	}
	p.World.set("tick", (p.World.get("tick")+1)&255)
	if e := p.World.lifeActors(); e != nil {
		return e
	}
	p.Scene.cacheActors()
	flash := p.World.flag("flag_f624")
	if flash != p.Flash {
		bank := p.Data.ScarabPalette[32:48]
		if flash {
			bank = p.Data.ScarabFlash[:]
		}
		copy(p.Scene.Palette[32:48], bank)
		if !flash {
			clear(p.Scene.Palette[:16])
		}
		p.Flash = flash
	}
	p.PendingCycle = flash
	return nil
}
func (p *NewGamePrelude) card() {
	p.World.stopAudio()
	p.World.clearLifeActors()
	p.Scene.PendingActors = [32]*choreography.Object{}
	p.Scene.DisplayActors = p.Scene.PendingActors
	p.black()
	p.Scene.Layout.Background = 0x158ef9
	p.Scene.Page.Foreground = 0x1595c5
	p.phase("card-load")
}
func (p *NewGamePrelude) cardFade(in bool) {
	for i, target := range p.Data.CardPalette {
		for channel, v := range target {
			if in {
				p.Scene.Palette[i][channel] = uint8(min(int(v), p.Age*34))
			} else {
				p.Scene.Palette[i][channel] = uint8(max(0, int(p.Scene.Palette[i][channel])-34))
			}
		}
	}
}
func preludeAction(in Input) bool { return in.Enter || in.Sword || in.Jump || in.Apple }
func (p *NewGamePrelude) Update(in Input) error {
	p.Scene.DisplayActors = p.Scene.PendingActors
	p.World.SoundEvents = nil
	switch p.Phase {
	case "title-out":
		p.Age++
		if p.Age == 16 {
			p.black()
			p.World.stopAudio()
			p.World.clearLifeActors()
			p.World.Life.Phase = LifeClear
			p.phase("scarab-load")
		}
	case "scarab-load":
		p.Age++
		if p.Age == p.Data.ScarabLoad {
			return p.scarab()
		}
	case "scarab-warm":
		p.Age++
		if p.Age == p.Data.ScarabWarm {
			p.Scene.Palette = p.Data.ScarabPalette
			p.World.Sound(19, false)
			p.phase("scarab")
			p.Age = 1
			return p.scarabTick()
		}
	case "scarab":
		if p.Age == 85 || preludeAction(in) {
			p.card()
			return nil
		}
		p.Age++
		return p.scarabTick()
	case "card-load":
		p.Age++
		if p.Age == p.Data.CardLoad {
			p.phase("card-in")
		}
	case "card-in":
		p.Age++
		if p.Age == 16 {
			p.Scene.Palette = p.Data.CardPalette
			p.phase("card-first")
		} else {
			p.cardFade(true)
		}
	case "card-first":
		p.World.set("tick", (p.World.get("tick")+1)&255)
		p.Age++
		if p.Age == 31 {
			for i := 0; i < 16; i++ {
				for c := 0; c < 3; c++ {
					p.Scene.Palette[i][c] = uint8(max(0, int(p.Scene.Palette[i][c])-34))
				}
			}
			p.phase("card-map")
		}
	case "card-map":
		p.Age++
		if p.Age == p.Data.CardMapLoad {
			p.Scene.Page.Foreground = 0x159744
			p.phase("card-second")
		}
	case "card-second":
		p.World.set("tick", (p.World.get("tick")+1)&255)
		p.Age++
		if p.Age == 61 {
			p.phase("card-wait")
		}
	case "card-wait":
		p.World.set("tick", (p.World.get("tick")+1)&255)
		p.Age++
		if p.Age == 901 || preludeAction(in) {
			p.phase("card-out")
		}
	case "card-out":
		p.Age++
		if p.Age == 16 {
			p.black()
			p.World.stopAudio()
			p.World.requestMusic(84)
			p.phase("story-load")
		} else {
			p.cardFade(false)
		}
	case "story-load":
		p.Age++
		if p.Age == p.Data.StoryLoad {
			p.Done = true
		}
	}
	return nil
}
func (g *Game) updatePrelude(in Input) {
	p := g.Prelude
	if e := p.Update(in); e != nil {
		g.Err = e
		return
	}
	if !p.Done {
		return
	}
	s, e := newStorySession(p.World, g.Stage().Index)
	if e != nil {
		g.Err = e
		return
	}
	// Preserve the already-playing music and the clock/seed advanced by the prelude.
	if s != nil {
		s.World = p.World
	}
	g.Story = s
	g.Prelude = nil
}
func (g *Game) renderPrelude() *image.RGBA {
	p := g.Prelude
	if p.Phase == "title-out" {
		draw.Draw(g.Canvas, g.Canvas.Rect, p.Outgoing, image.Point{}, draw.Src)
		g.attractPaletteFade(p.Age+1, false)
		return g.Canvas
	}
	old := g.Story
	g.Story = p.Scene
	defer func() { g.Story = old }()
	return g.renderStory()
}

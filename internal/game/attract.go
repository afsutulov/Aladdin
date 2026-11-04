package game

import (
	"fmt"
	"image/color"
)

type AttractSession struct {
	Phase              string
	Age, Index, Cursor int
	Buttons            byte
	Credits            *CreditRoll
	Display            *NativeWorld
}

func demoInput(b byte) Input {
	return Input{Up: b&1 == 0, Down: b&2 == 0, Left: b&4 == 0, Right: b&8 == 0, Sword: b&16 == 0, Jump: b&32 == 0}
}
func (g *Game) requestAttract() {
	g.Attract = &AttractSession{Phase: "menu_out", Index: g.NextDemo, Buttons: 255}
}
func (g *Game) startAttractDemo(index int) {
	if index < 0 || index >= len(g.Project.Scripts.Menu.Demos) {
		g.Err = fmt.Errorf("invalid attract demo%d", index)
		return
	}
	d := g.Project.Scripts.Menu.Demos[index]
	g.Start(d.Stage)
	if g.Err != nil {
		return
	}
	w := g.World
	if d.Stage == 0 {
		if e := w.showMessage(0x18); e != nil {
			g.Err = e
			return
		}
	}
	if d.Stage == 0 || d.Stage == 4 {
		// The book/prelude leaves tick193 and an airborne stance before
		// loading. Both affect scripts during the two loading sweeps.
		if e := w.prepareSceneEntry(193, 0); e != nil {
			g.Err = e
			return
		}
	}
	w.Seed = 12345678
	w.set("tick", 255)
	w.set("attract_mode", 1)
	w.set("difficulty", g.Menu.Difficulty)
	w.set("camera_rate_mode", g.Menu.Tracking)
	lives, apples := 3, 0x3130
	if g.Menu.Difficulty == 0 {
		lives, apples = 5, 0x3135
	} else if g.Menu.Difficulty == 2 {
		lives, apples = 2, 0x3035
	}
	w.set("lives", lives)
	w.set("apples_ascii", apples)
	g.NextDemo = index + 1
	g.Attract = &AttractSession{Phase: "demo", Index: index, Buttons: 255}
	if e := w.Update(Input{}); e != nil {
		g.Err = e
	}
	g.syncWorldPresentation()
}
func (g *Game) beginAttractExit() {
	a := g.Attract
	g.World.cacheVisibleFrame()
	a.Display = g.World.VisibleFrame
	g.World.clearLifeActors()
	a.Phase = "demo_out"
	a.Age = 0
}
func (g *Game) updateAttract(in Input) {
	a := g.Attract
	switch a.Phase {
	case "menu_out":
		a.Age++
		if a.Age == 16 {
			if a.Index < 5 {
				g.startAttractDemo(a.Index)
			} else {
				g.NextDemo = 0
				g.Menu.World.stopAudio()
				a.Credits = newCreditRoll(g.Project.Scripts.Menu)
				a.Phase = "credits_in"
				a.Age = 0
			}
		}
	case "credits_in":
		a.Age++
		if a.Age == 16 {
			a.Phase = "credits"
			a.Age = 0
		}
	case "credits":
		a.Credits.Update(in)
		if a.Credits.Done {
			a.Phase = "credits_out"
			a.Age = 0
		}
	case "credits_out":
		a.Age++
		if a.Age == 16 {
			g.startAttractDemo(0)
		}
	case "demo_out":
		a.Age++
		if a.Age == 16 {
			g.openTitleAfterAttract()
		}
	case "demo":
		w := g.World
		ordinary := w.Life.Phase == LifePlaying && w.Flow.Phase == FlowPlaying && w.parryTarget == nil
		if ordinary {
			bytes := g.Project.Scripts.Menu.Demos[a.Index].Inputs
			if a.Cursor == len(bytes) {
				g.beginAttractExit()
				return
			}
			a.Buttons = bytes[a.Cursor]
			a.Cursor++
			if in.Jump || in.Sword || in.Apple || in.Enter {
				g.beginAttractExit()
				return
			}
		}
		controls := in
		if ordinary || w.parryTarget != nil {
			controls = demoInput(a.Buttons)
		}
		if e := w.Update(controls); e != nil {
			g.Err = e
			return
		}
		if w.Flow.Phase == FlowLoad {
			g.advanceStage()
		}
		if w.Life.Phase == LifeGameOver {
			g.beginAttractExit()
			return
		}
		g.syncWorldPresentation()
	}
}
func (g *Game) attractPaletteFade(age int, in bool) {
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			c := g.Canvas.RGBAAt(x, y)
			f := func(v uint8) uint8 {
				if in {
					return uint8(min(int(v), age*34))
				}
				return uint8(max(0, int(v)-age*34))
			}
			g.Canvas.SetRGBA(x, y, color.RGBA{f(c.R), f(c.G), f(c.B), c.A})
		}
	}
}

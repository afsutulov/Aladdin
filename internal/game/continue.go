package game

import (
	"aladdin/internal/choreography"
	"image"
	"image/draw"
)

func continuePhase(p LifePhase) bool {
	return p == LifeContinue || p == LifeContinueDecision || p == LifeContinueFade
}
func (w *NativeWorld) cacheLifeActors() {
	for i, o := range w.Objects {
		w.Life.RenderActors[i] = nil
		if o != nil {
			v := *o
			w.Life.RenderActors[i] = &v
		}
	}
}

// beginContinue ports 1DB034; the original polls Left and Right, with Left
// winning simultaneous directions. The action buttons do not choose an answer.
func (w *NativeWorld) beginContinue() error {
	w.stopAudio()
	storage := w.Objects
	leaks := w.ArtworkLeaks
	w.clearLifeActors()
	w.ArtworkLeaks = leaks
	w.PaletteBank2 = 0
	w.PaletteOverrides = nil
	w.set("invulnerability", 0)
	w.Life = LifeCycle{Phase: LifeContinue}
	for _, v := range []struct{ slot, template, x, y, animation int }{{2, 0x1e3b06, 308, 440, 0x14947c}, {1, 0x1e3b1a, 288, 440, 0x14e5fc}} {
		o := w.actorTemplate(v.template, storage[v.slot])
		o.SetWord(2, v.x)
		o.SetWord(4, v.y)
		o.Animation = v.animation
		o.Motion = 0
		w.Objects[v.slot] = o
	}
	if e := w.showMessage(0x16); e != nil {
		return e
	}
	w.set("tick", 255)
	if e := w.lifeActors(); e != nil {
		return e
	}
	w.cacheLifeActors()
	w.set("tick", 0)
	w.Sound(0x0a, false)
	return w.lifeActors()
}
func (w *NativeWorld) updateContinue(in Input) error {
	if w.Life.Phase == LifeContinueDecision && w.Life.Age >= 121 {
		w.Life.Phase = LifeContinueFade
		w.Life.Age = 0
		return nil
	}
	if w.Life.Phase == LifeContinue || w.Life.Phase == LifeContinueDecision {
		w.cacheLifeActors()
	}
	switch w.Life.Phase {
	case LifeContinue:
		if in.Left || in.Right {
			w.Life.Accepted = !in.Left
			// The source constructor replaces animation and motion but retains
			// position and unused loop storage. Its old artwork remains reserved.
			for _, v := range []struct{ slot, template int }{{2, 0x1e3b06}, {1, 0x1e3b1a}} {
				old := w.Objects[v.slot]
				if old != nil {
					w.retainArtwork(old)
				}
				o := w.actorTemplate(v.template, old)
				if old != nil {
					x, y := old.Position()
					o.SetWord(2, x)
					o.SetWord(4, y)
				}
				if !w.Life.Accepted {
					o.Record[9] = 255
				} else {
					o.Record[9] = 0
				}
				w.Objects[v.slot] = o
			}
			if w.Life.Accepted {
				lives := 3
				if w.get("difficulty") == 0 {
					lives = 5
				} else if w.get("difficulty") > 1 {
					lives = 2
				}
				w.set("lives", lives)
				w.set("score", 0)
				apples := 0x3130
				if w.get("difficulty") == 0 {
					apples = 0x3135
				} else if w.get("difficulty") > 1 {
					apples = 0x3035
				}
				w.set("apples_ascii", apples)
				w.set("gems_ascii", 0x3030)
				w.set("genie_tokens", 0)
				w.set("continues", (w.get("continues")-1)&255)
			}
			w.Life.Phase = LifeContinueDecision
			w.Life.Age = 0
		}
		fallthrough
	case LifeContinueDecision:
		w.Life.Age++
		w.set("tick", (w.get("tick")+1)&255)
		if e := w.lifeActors(); e != nil {
			return e
		}
	case LifeContinueFade:
		w.Life.Age++
		if w.Life.Age == 16 {
			if w.Life.Accepted {
				w.Life.Phase = LifeContinueLoad
			} else {
				w.Life = LifeCycle{Phase: LifeGameOver}
			}
		}
	}
	return nil
}

func (g *Game) continueStage() {
	old := g.World
	g.Start(old.Stage.Index)
	if g.Err != nil {
		return
	}
	w := g.World
	for _, n := range []string{"lives", "continues", "score", "score_progress", "apples_ascii", "gems_ascii", "genie_tokens", "difficulty", "invincible", "camera_rate_mode"} {
		w.set(n, old.get(n))
	}
	w.Seed = old.Seed
	w.RouteCursor = old.RouteCursor
	apples := w.get("apples_ascii")
	if e := w.respawn(); e != nil {
		g.Err = e
		return
	}
	w.set("apples_ascii", apples)
	w.set("invulnerability", 0)
	w.Life.FreshStage = true
}
func (g *Game) drawContinue() {
	im, e := g.Project.Image("continue.png")
	if e != nil {
		g.Err = e
		return
	}
	draw.Draw(g.Canvas, g.Canvas.Rect, im, image.Point{}, draw.Src)
	g.drawActors(g.Canvas)
}

// actorTemplate preserves the fields that constructor 1D8A4A leaves untouched.
func (w *NativeWorld) actorTemplate(template int, old *choreography.Object) *choreography.Object {
	o := choreography.FromTemplate(w.Scripts.Templates[template])
	if old != nil {
		for _, r := range [][2]int{{2, 6}, {14, 19}, {36, 41}, {56, 60}} {
			copy(o.Record[r[0]:r[1]], old.Record[r[0]:r[1]])
		}
		o.MotionRepeat = old.MotionRepeat
		o.MotionLoop = old.MotionLoop
		o.AnimationRepeat = old.AnimationRepeat
		o.AnimationLoop = old.AnimationLoop
		o.Return = old.Return

	}
	return o
}

// PreviewLastLifeLoss enters the ordinary death path for development capture.
// Normal gameplay reaches it through damage or falling with no spare lives.
func (g *Game) PreviewLastLifeLoss(continues byte) {
	if g.World == nil {
		g.Start(1)
		if g.Err != nil {
			return
		}
	}
	w := g.World
	w.set("lives", 0)
	w.set("continues", int(continues))
	w.set("pending_score", 0)
	w.set("health", 0)
	w.set("invulnerability", 0)
	w.set("death_pending", 255)
	g.Mode = 1
}

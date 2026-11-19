package game

import (
	"image"
	"image/color"
	"image/draw"
)

func gameOverPhase(p LifePhase) bool { return p >= LifeGameOverFadeIn && p <= LifeGameOverFadeOut }
func (w *NativeWorld) beginGameOver() error {
	w.stopAudio()
	w.clearLifeScene()
	w.PaletteBank2 = 0
	w.PaletteOverrides = nil
	w.set("invulnerability", 0)
	w.Life = LifeCycle{Phase: LifeGameOverFadeIn}
	if e := w.buildMessage(0x0e, true); e != nil {
		return e
	}
	w.set("tick", 255)
	w.set("gameover_cycle_cursor", 28)
	if e := w.lifeActors(); e != nil {
		return e
	}
	w.cacheLifeActors()
	w.Sound(0x0e, false)
	return nil
}
func (w *NativeWorld) updateGameOver(in Input) error {
	press := in.Jump || in.Sword || in.Apple || in.Enter
	switch w.Life.Phase {
	case LifeGameOverFadeIn:
		w.Life.Age++
		if w.Life.Age == 16 {
			w.Life.Phase = LifeGameOverCycle
			w.Life.Age = 0
		}
	case LifeGameOverCycle:
		w.Life.Age++
		if w.Life.Age%4 == 1 {
			w.cacheLifeActors()
			w.set("tick", (w.get("tick")+1)&255)
			return w.lifeActors()
		}
		if w.Life.Age%4 == 0 {
			if w.PaletteOverrides == nil {
				w.PaletteOverrides = map[int][3]uint8{}
			}
			cursor := w.get("gameover_cycle_cursor") / 2
			for j := 0; j < 14; j++ {
				w.PaletteOverrides[34+j] = w.Scripts.GameOverCycle[cursor+j]
			}
			cursor--
			if cursor == 0 {
				cursor = 14
			}
			w.set("gameover_cycle_cursor", cursor*2)
			w.set("tick", (w.get("tick")+1)&255)
			if press || w.Life.Age >= 244 {
				w.Life.Accepted = !press
				w.Life.Phase = LifeGameOverBetweenFade
				w.Life.Age = 0
			}
		}
	case LifeGameOverBetweenFade:
		w.Life.Age++
		if w.Life.Age == 16 {
			if !w.Life.Accepted {
				w.Life = LifeCycle{Phase: LifeGameOver}
				return nil
			}
			return w.beginGameOverFinal()
		}
	case LifeGameOverFinal:
		if w.Life.Age >= 179 || press {
			w.Life.Phase = LifeGameOverFadeOut
			w.Life.Age = 0
			return nil
		}
		w.Life.Age++
		w.cacheLifeActors()
		w.set("tick", (w.get("tick")+1)&255)
		return w.lifeActors()
	case LifeGameOverFadeOut:
		w.Life.Age++
		if w.Life.Age == 16 {
			w.Life = LifeCycle{Phase: LifeGameOver}
		}
	}
	return nil
}
func (w *NativeWorld) beginGameOverFinal() error {
	w.stopAudio()
	storage := w.Objects
	leaks := w.ArtworkLeaks
	w.clearLifeActors()
	w.ArtworkLeaks = leaks
	w.PaletteOverrides = nil
	w.Life = LifeCycle{Phase: LifeGameOverFinal}
	o := w.actorTemplate(0x1e34da, storage[1])
	o.SetWord(2, 384)
	o.SetWord(4, 420)
	w.Objects[1] = o
	w.set("tick", 255)
	if e := w.lifeActors(); e != nil {
		return e
	}
	w.cacheLifeActors()
	w.set("tick", 0)
	return w.lifeActors()
}
func (g *Game) drawGameOver() {
	name := "gameover.png"
	if g.World.Life.Phase == LifeGameOverFinal || g.World.Life.Phase == LifeGameOverFadeOut {
		name = "gameover-final.png"
	}
	im, e := g.Project.Image(name)
	if e != nil {
		g.Err = e
		return
	}
	pal := g.renderPalette()
	c := pal[0]
	draw.Draw(g.Canvas, g.Canvas.Rect, &image.Uniform{C: color.RGBA{c[0], c[1], c[2], 255}}, image.Point{}, draw.Src)
	g.drawLifeImage(im)
	g.drawActors(g.Canvas)
}
func (g *Game) drawLifeImage(im image.Image) {
	p, ok := im.(*image.Paletted)
	if !ok {
		drawOver(g.Canvas, g.Canvas.Rect, im, image.Point{})
		return
	}
	pal := g.renderPalette()
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			i := p.ColorIndexAt(x, y)
			if i%16 != 0 {
				c := pal[i]
				g.Canvas.SetRGBA(x, y, color.RGBA{c[0], c[1], c[2], 255})
			}
		}
	}
}

// Clear live scene actors and release their graphics, retaining record storage
// left behind by 1D8958. The constructor reuses these non-live fields later.
func (w *NativeWorld) clearLifeScene() {
	old := w.Objects
	leaks := w.ArtworkLeaks
	w.clearLifeActors()
	w.ArtworkLeaks = leaks
	for i, o := range old {
		if o != nil {
			v := *o
			v.Record[0] = 0
			v.ArtworkValid = false
			v.ArtworkStart = 0
			v.ArtworkCount = 0
			w.Objects[i] = &v
		}
	}
}

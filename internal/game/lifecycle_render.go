package game

import (
	"image"
	"image/color"
)

func (g *Game) renderPalette() [64][3]uint8 {
	if g.Ending != nil {
		return g.Ending.Scene.Palette
	}
	if g.LevelTitle != nil {
		return g.LevelTitle.Scene.Palette
	}
	if g.Prelude != nil {
		return g.Prelude.Scene.Palette
	}
	if g.Story != nil {
		return g.Story.Palette
	}
	if g.Mode == 0 && g.Menu != nil {
		return g.Project.Scripts.Menu.Palette
	}
	if g.World != nil {
		if gameOverPhase(g.World.Life.Phase) {
			if g.World.Life.Phase == LifeGameOverFinal || g.World.Life.Phase == LifeGameOverFadeOut {
				return g.Project.Scripts.GameOverFinalPalette
			}
			p := g.Project.Scripts.GameOverPalette
			for i, c := range g.World.PaletteOverrides {
				p[i] = c
			}
			return p
		}
		if continuePhase(g.World.Life.Phase) {
			return g.Project.Scripts.ContinuePalette
		}
		if geniePhase(g.World.Flow.Phase) {
			p := g.Project.Scripts.GeniePalette
			for i, c := range g.World.PaletteOverrides {
				p[i] = c
			}
			return p
		}
		if g.World.Flow.Phase == FlowComplete || g.World.Flow.Phase == FlowCompleteFade || g.World.Flow.Phase == FlowBonusIntro || g.World.Flow.Phase == FlowBonusIntroFade {
			return g.Project.Scripts.LevelClearPalette
		}
		switch g.World.Life.Phase {
		case LifeClear, LifeDeathScene, LifeDeathFade, LifeHighScoreFade:
			return g.Project.Scripts.DeathPalette
		}
	}
	p := g.Stage().PaletteRGB
	if g.World != nil && g.World.PaletteBank2 != 0 {
		if bank, ok := g.Project.Scripts.Palettes[g.World.PaletteBank2]; ok {
			copy(p[32:48], bank[:])
		}
	}
	if g.World != nil {
		if geniePhase(g.World.Flow.Phase) {
			p := g.Project.Scripts.GeniePalette
			for i, c := range g.World.PaletteOverrides {
				p[i] = c
			}
			return p
		}
		for i, c := range g.World.PaletteOverrides {
			p[i] = c
		}
	}

	if g.World != nil && g.World.Stage.Index == 8 && g.World.Flow.Phase == FlowPlaying && g.World.Life.Phase == LifePlaying {
		// Sprite positions are displayed one ordinary step behind the palette
		// writes made before that frame's VBlank.
		d := g.World.Stage.RugRide
		cursor := g.World.get("rug_palette_cursor") / 6
		for n := 0; n < 2; n++ {
			c := d.LavaCycle[(cursor+n)%len(d.LavaCycle)]
			p[c.Index] = c.RGB
		}
	}
	return p
}

func (g *Game) paletteImage(im image.Image) image.Image {
	if g.World == nil || g.World.PaletteBank2 == 0 && len(g.World.PaletteOverrides) == 0 {
		return im
	}
	p, ok := im.(*image.Paletted)
	if !ok {
		return im
	}
	v := *p
	v.Palette = append(color.Palette(nil), p.Palette...)
	bank := g.renderPalette()
	for i := 0; i < len(v.Palette); i++ {
		_, _, _, alpha := p.Palette[i].RGBA()
		v.Palette[i] = color.NRGBA{bank[i][0], bank[i][1], bank[i][2], uint8(alpha >> 8)}
	}
	return &v
}

// actorTarget records the winning sprite's foreground priority at every
// opaque pixel. Earlier actor records win overlaps, regardless of that bit.
type actorTarget struct {
	*image.RGBA
	priority []bool
	high     bool
	lines    *spriteLines
}

func (d actorTarget) Set(x, y int, c color.Color) {
	if !image.Pt(x, y).In(d.Rect) {
		return
	}
	i := d.RGBA.PixOffset(x, y)
	if d.RGBA.Pix[i+3] != 0 {
		return
	}
	if rgba, ok := c.(color.RGBA); ok {
		d.RGBA.Pix[i], d.RGBA.Pix[i+1], d.RGBA.Pix[i+2], d.RGBA.Pix[i+3] = rgba.R, rgba.G, rgba.B, rgba.A
	} else {
		d.RGBA.Set(x, y, c)
	}
	d.priority[y*Width+x] = d.high
}
func (g *Game) prepareActors() {
	if g.actorLayer == nil {
		g.actorLayer = image.NewRGBA(image.Rect(0, 0, Width, Height))
		g.actorPriority = make([]bool, Width*Height)
	}
	clear(g.actorLayer.Pix)
	clear(g.actorPriority)
	if g.World == nil {
		return
	}
	actors := g.World.Objects
	if continuePhase(g.World.Life.Phase) || gameOverPhase(g.World.Life.Phase) {
		actors = g.World.Life.RenderActors
	}
	if g.World.Flow.Phase == FlowComplete || g.World.Flow.Phase == FlowCompleteFade || g.World.Flow.Phase == FlowBonusIntro || g.World.Flow.Phase == FlowBonusIntroFade {
		actors = g.World.Flow.RenderActors
	}
	lines := &spriteLines{}
	if g.World.Flow.Phase == FlowPlaying && g.World.Life.Phase < LifeClear {
		for _, s := range g.World.hudSprites() {
			lines.reserve(s.X, s.Y, s.Columns*8, s.Rows*8)
		}
	}
	for i := 0; i < len(actors); i++ {
		o := actors[i]
		if o == nil || !o.Alive() || o.Record[7]&32 != 0 {
			continue
		}
		if i == 0 && g.World.get("invulnerability")&1 != 0 {
			continue
		}
		x, y := o.Position()
		if o.Record[6]&8 != 0 {
			x -= 128
			y -= 256
		} else {
			x -= g.CameraX
			y -= 256 + g.CameraY
		}
		shakeX, shakeY := 0, 0
		if geniePhase(g.World.Flow.Phase) || continuePhase(g.World.Life.Phase) || gameOverPhase(g.World.Life.Phase) {
			shakeY = int(int16(g.World.get("sprite_shake")))
			shakeX = -int(int16(g.World.get("sprite_shake_x")))
		} else if (g.World.Stage.Index == 5 || g.World.Stage.Index == 6 || g.World.Stage.Index == 7) && g.World.Life.Phase < LifeClear && g.World.Flow.Phase < FlowComplete {
			shakeY = int(int16(g.World.get("sprite_shake")))
			if g.World.Stage.Index == 7 {
				shakeX = -g.World.get("sprite_shake_x")
			}
		}
		target := actorTarget{g.actorLayer, g.actorPriority, uint16(o.Word(30))&0x8000 != 0, lines}
		g.actorSprite(target, o.Frame, x, y, shakeX, shakeY, o.Record[9] != 0, o.Record[53] != 0, o.Record[6]&8 != 0, int(uint16(o.Word(30))>>13)&3)
	}
}
func (g *Game) drawActors(dst *image.RGBA, foreground ...bool) {
	if len(foreground) == 0 || !foreground[0] {
		g.prepareActors()
	}
	src := g.actorLayer
	for y := 0; y < Height; y++ {
		si, di := src.PixOffset(0, y), dst.PixOffset(0, y)
		srow, drow := src.Pix[si:si+4*Width], dst.Pix[di:di+4*Width]
		prio := g.actorPriority[y*Width : (y+1)*Width]
		for x := 0; x < Width; x++ {
			if srow[x*4+3] != 0 && (len(foreground) == 0 || prio[x] == foreground[0]) {
				copy(drow[x*4:x*4+4], srow[x*4:x*4+4])
			}
		}
	}
}

// Palette fades change each three-bit colour component by one level per
// video frame, then hold black for the rest of the sixteen-frame loop.
func (g *Game) applyLifeFade() {
	if g.World == nil {
		return
	}
	n := 0
	switch g.World.Life.Phase {
	case LifeFadeOut, LifeDeathFade, LifeContinueFade, LifeHighScoreFade, LifeGameOverBetweenFade, LifeGameOverFadeOut:
		n = g.World.Life.Age
	case LifeFadeIn:
		n = 16 - g.World.Life.Age
	}
	if g.World.Flow.Phase == FlowGenieFadeIn || g.World.Life.Phase == LifeGameOverFadeIn {
		age := g.World.Flow.Age
		if g.World.Life.Phase == LifeGameOverFadeIn {
			age = g.World.Life.Age
		}
		limit := age * 34
		for y := 0; y < Height; y++ {
			for x := 0; x < Width; x++ {
				c := g.Canvas.RGBAAt(x, y)
				g.Canvas.SetRGBA(x, y, color.RGBA{uint8(min(int(c.R), limit)), uint8(min(int(c.G), limit)), uint8(min(int(c.B), limit)), c.A})
			}
		}
		return
	}
	if g.World.Flow.Phase == FlowGenieFadeOut {
		n = g.World.Flow.Age
	}
	if g.World.Flow.Phase == FlowFadeOut || g.World.Flow.Phase == FlowCompleteFade || g.World.Flow.Phase == FlowBonusIntroFade {
		n = g.World.Flow.Age
	}
	if n == 0 {
		return
	}
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			c := g.Canvas.RGBAAt(x, y)
			lower := func(v uint8) uint8 { return uint8(max(0, int(v)-n*34)) }
			g.Canvas.SetRGBA(x, y, color.RGBA{lower(c.R), lower(c.G), lower(c.B), c.A})
		}
	}
}

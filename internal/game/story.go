package game

import (
	"aladdin/internal/choreography"
	"aladdin/internal/data"
	"fmt"
	"image"
	"image/color"
)

// StorySession runs declarative story text and the existing Go actor logic.
// Loading/card sequences surrounding these pages remain a separate conversion.
type StorySession struct {
	World                                                                       *NativeWorld
	Data                                                                        *data.StoryData
	Plan                                                                        data.StoryPlan
	Page                                                                        data.StoryPage
	Layout                                                                      data.StoryLayout
	Step, Op, Wait, Half, Hold, Fade, Scroll, Reload, Loading, Transfer         int
	Fading, Clearing, Blank, Skip, SampledSkip, Done, ClearPending, AfterReload bool
	TransferGlyphs, PendingGlyphs                                               []data.MenuGlyph
	RasterLine, RasterColumn                                                    int
	RasterTiles                                                                 [64]int
	RasterWritten                                                               [64]bool
	Palette                                                                     [64][3]uint8
	BackgroundX                                                                 [Height]int
	Tiles                                                                       [32][64]int
	Written                                                                     [32][64]bool
	DisplayActors, PendingActors                                                [32]*choreography.Object
}

func newStorySession(w *NativeWorld, stage int) (*StorySession, error) {
	d := &w.Scripts.Story
	p, ok := d.Plans[stage]
	if !ok || len(p.Steps) == 0 {
		return nil, nil
	}
	v := NewNativeWorld(w.Stage, w.Scripts, w.Frames)
	v.clearLifeActors()
	v.Life.Phase = LifeClear
	v.HUDWaveIndex, v.HUDWaveTile = w.HUDWaveIndex, w.HUDWaveTile
	v.Seed = w.Seed
	v.set("tick", w.get("tick"))
	v.requestMusic(p.Music)
	s := &StorySession{World: v, Data: d, Plan: p, Step: -1}
	if e := s.nextPage(); e != nil {
		return nil, e
	}
	return s, nil
}
func (s *StorySession) nextPage() error {
	s.Step++
	if s.Step >= len(s.Plan.Steps) || s.Skip && s.Plan.Steps[s.Step].SkipBeforeLoad {
		s.Done = true
		return nil
	}
	step := s.Plan.Steps[s.Step]
	var ok bool
	s.Page, ok = s.Data.Pages[step.Page]
	if !ok {
		return fmt.Errorf("missing story page %x", step.Page)
	}
	s.Layout, ok = s.Data.Layouts[step.Layout]
	if !ok {
		return fmt.Errorf("missing story layout %x", step.Layout)
	}
	if step.LoadFrames > 0 {
		s.Loading = step.LoadFrames
		if s.Skip && step.SkippedLoadFrames > 0 {
			s.Loading = step.SkippedLoadFrames
		}
		s.Palette = [64][3]uint8{}
		return nil
	}
	return s.activatePage()
}

func (s *StorySession) activatePage() error {
	if s.Skip && s.Plan.Steps[s.Step].CheckSkip {
		s.Done = true
		return nil
	}
	s.Op, s.Wait, s.Half, s.Hold, s.Fade, s.Scroll, s.Reload, s.Transfer = 0, 0, 0, 0, 0, 0, 0, 0
	s.RasterLine = -1
	s.ClearPending, s.AfterReload = false, false
	s.PendingGlyphs, s.TransferGlyphs = nil, nil
	s.Fading, s.Clearing, s.Blank = false, false, false
	s.Written = [32][64]bool{}
	s.Tiles = [32][64]int{}
	for bank, a := range []int{s.Page.Palette[0], s.Page.Palette[1], s.Layout.Palette, 0x151fd4} {
		p := s.Data.Palettes[a]
		copy(s.Palette[bank*16:], p[:])
	}
	s.Palette[0] = [3]uint8{}
	if e := s.consume(); e != nil {
		return e
	}
	// A page without a text wait starts its hold before its first visible boundary.
	if s.Wait == 0 && !s.Fading && s.Reload == 0 {
		s.Hold = 1
		return s.holdActors()
	}
	return nil
}
func (s *StorySession) consume() error {
	for s.Op < len(s.Page.Ops) && !s.Skip {
		op := s.Page.Ops[s.Op]
		s.Op++
		switch op.Kind {
		case "glyphs":
			if op.TransferWait > 0 {
				s.Transfer = op.TransferWait
				s.TransferGlyphs = op.Glyphs
				return nil
			}
			if s.AfterReload && s.Page.ReloadGlyphDelay > 0 {
				s.PendingGlyphs = op.Glyphs
			} else {
				s.writeGlyphs(op.Glyphs)
			}
			s.AfterReload = false
		case "wait":
			s.AfterReload = false
			s.Wait = op.Count
			s.Half = 1
			return nil
		case "clear":
			s.ClearPending = true
			s.Blank = false
			s.Reload = s.Page.ReloadWait
			if s.Reload == 0 {
				s.Written = [32][64]bool{}
				s.ClearPending = false
			}
			if s.Reload > 0 {
				return nil
			}
		case "fade-clear":
			s.Fading, s.Clearing = true, true
			s.Fade = -1
			return nil
		case "spawn":
			if e := s.World.Spawn(&choreography.Object{}, data.Instruction{Template: op.Template, OffsetX: op.X, OffsetY: op.Y}); e != nil {
				return e
			}
		default:
			return fmt.Errorf("unconverted story operation %q", op.Kind)
		}
	}
	if s.Skip {
		s.Fading = true
		s.Fade = 0
	}
	return nil
}
func (s *StorySession) writeGlyphs(glyphs []data.MenuGlyph) {
	for _, g := range glyphs {
		x, y := g.X&63, g.Y&31
		s.Tiles[y][x] = g.Tile
		s.Written[y][x] = true
	}
}

func (s *StorySession) cacheActors() {
	for i, o := range s.World.Objects {
		s.PendingActors[i] = nil
		if o != nil {
			v := *o
			s.PendingActors[i] = &v
		}
	}
}
func (s *StorySession) holdActors() error {
	// 1DF33E advances animation BEFORE motion; text waits use lifeActors' reverse order.
	c := choreography.Choreographer{Scripts: s.World.Scripts}
	if s.World.get("tick")&1 != 0 {
		for _, o := range s.World.Objects {
			if o != nil && o.Alive() {
				if e := c.Animation(s.World, o); e != nil {
					return e
				}
			}
		}
	}
	for _, o := range s.World.Objects {
		if o != nil && o.Alive() && o.Frame >= 0 {
			s.World.objectMomentum(o)
			if e := c.Motion(s.World, o); e != nil {
				return e
			}
		}
	}
	s.World.set("tick", (s.World.get("tick")+1)&255)
	s.cacheActors()
	return nil
}
func (s *StorySession) Update(in Input) error {
	if s.Done {
		return nil
	}
	s.RasterLine = -1
	if len(s.PendingGlyphs) > 0 {
		s.writeGlyphs(s.PendingGlyphs)
		s.PendingGlyphs = nil
	}
	s.DisplayActors = s.PendingActors
	s.World.SoundEvents = nil
	if s.SampledSkip {
		s.Skip = true
	}
	if s.Loading > 0 {
		s.Loading--
		if s.Loading == 0 {
			return s.activatePage()
		}
		return nil
	}
	if s.Fading {
		s.Fade++
		if s.Fade > 0 {
			for i := range s.Palette {
				for j := range s.Palette[i] {
					s.Palette[i][j] = uint8(max(0, int(s.Palette[i][j])-34))
				}
			}
		}
		if s.Fade < 16 {
			return nil
		}
		if s.Clearing {
			s.Fading, s.Clearing = false, false
			s.Blank = true
			s.Written = [32][64]bool{}
			p := s.Data.Palettes[0x151fd4]
			copy(s.Palette[48:], p[:])
			return s.consume()
		}
		s.World.clearLifeActors()
		s.PendingActors = [32]*choreography.Object{}
		s.DisplayActors = s.PendingActors
		return s.nextPage()
	}
	if s.Transfer > 0 {
		s.Transfer--
		if s.Transfer > 0 {
			return nil
		}
		s.writeGlyphs(s.TransferGlyphs)
		s.TransferGlyphs = nil
		return s.consume()
	}
	if s.Reload > 0 {
		rows := 32
		age := s.Page.ReloadWait - s.Reload
		if age < len(s.Page.ReloadRows) {
			rows = s.Page.ReloadRows[age]
		}
		if age < len(s.Page.ReloadRaster) {
			s.RasterLine = s.Page.ReloadRaster[age]
			s.RasterColumn = s.Page.ReloadRasterColumns[age]
			s.RasterTiles = s.Tiles[s.RasterLine/8]
			s.RasterWritten = s.Written[s.RasterLine/8]
		}
		for y := 0; y < rows; y++ {
			s.Written[y] = [64]bool{}
		}
		s.Reload--
		if s.Reload > 0 {
			return nil
		}
		s.ClearPending = false
		s.AfterReload = true
	}
	if s.Wait == 0 && s.Op < len(s.Page.Ops) && !s.Skip {
		if e := s.consume(); e != nil {
			return e
		}
		if s.Wait > 0 || s.Fading || s.Reload > 0 || s.Transfer > 0 {
			return nil
		}
	}
	if s.Wait > 0 {
		if s.Half == 0 {
			if s.Skip {
				return s.consume()
			}
			// 1DCBD2 samples the pad after the second wait, before the next first wait.
			s.SampledSkip = in.Enter || in.Apple || in.Sword || in.Jump
			s.Half = 1
			return nil
		}
		s.Half = 0
		s.Wait--
		s.World.set("tick", (s.World.get("tick")+1)&255)
		if e := s.World.lifeActors(); e != nil {
			return e
		}
		s.cacheActors()
		return nil
	}
	if s.Skip || s.Hold >= 300 {
		s.Fading = true
		s.Fade = 0
		return nil
	}
	if !s.Page.Scroll {
		s.SampledSkip = in.Enter || in.Apple || in.Sword || in.Jump
	}
	if s.Page.Scroll && s.Hold > 0 && s.World.get("tick")&2 != 0 {
		s.Scroll = min(32, s.Scroll+1)
	}
	s.Hold++
	if e := s.holdActors(); e != nil {
		return e
	}
	return nil
}
func (g *Game) beginStory() {
	var e error
	g.Story, e = newStorySession(g.World, g.Stage().Index)
	if e != nil {
		g.Err = e
	} else if g.Story == nil && g.Stage().Index != 12 {
		g.beginLevelTitle(g.World)
	}
}
func (g *Game) updateStory(in Input) {
	s := g.Story
	if e := s.Update(in); e != nil {
		g.Err = e
		return
	}
	if s.Done {
		g.World.Seed = s.World.Seed
		g.Story = nil
		g.beginLevelTitle(s.World)
	}
}
func (g *Game) renderStory() *image.RGBA {
	return g.renderScene(g.Story)
}
func (g *Game) renderScene(s *StorySession) *image.RGBA {
	old, cx, cy := g.World, g.CameraX, g.CameraY
	v := *s.World
	v.Objects = s.DisplayActors
	g.World = &v
	g.CameraX, g.CameraY = 0, 0
	defer func() { g.World, g.CameraX, g.CameraY = old, cx, cy }()
	backdrop := s.Palette[0]
	for i := 0; i < len(g.Canvas.Pix); i += 4 {
		g.Canvas.Pix[i] = backdrop[0]
		g.Canvas.Pix[i+1] = backdrop[1]
		g.Canvas.Pix[i+2] = backdrop[2]
		g.Canvas.Pix[i+3] = 255
	}
	if s.Loading > 0 {
		return g.Canvas
	}
	drawPlane := func(address int, high, foreground bool) {
		if address == 0 || foreground && s.Blank {
			return
		}
		suffix := "low"
		if high {
			suffix = "high"
		}
		im, e := g.Project.Image(fmt.Sprintf("story-%06x-%s.png", address, suffix))
		if e != nil {
			g.Err = e
			return
		}
		p, ok := im.(*image.Paletted)
		if !ok {
			g.Err = fmt.Errorf("story map lost indices")
			return
		}
		for y := 0; y < Height; y++ {
			yy := y
			if !foreground {
				yy = (yy + s.Scroll) & 255
			}
			for x := 0; x < Width; x++ {
				if foreground && (s.Written[yy/8][x/8] || yy == s.RasterLine && x/8 >= s.RasterColumn && s.RasterWritten[x/8]) {
					continue
				}
				xx := x
				if !foreground {
					xx = (x - s.BackgroundX[y]) & 511
				}
				index := p.ColorIndexAt(xx, yy)
				if index%16 != 0 {
					c := s.Palette[index]
					g.Canvas.SetRGBA(x, y, color.RGBA{c[0], c[1], c[2], 255})
				}
			}
		}
	}
	for _, high := range []bool{false, true} {
		drawPlane(s.Layout.Background, high, false)
		drawPlane(s.Page.Foreground, high, true)
		if high {
			im, e := g.Project.Image("menu-font.png")
			if e != nil {
				g.Err = e
				return g.Canvas
			}
			font, ok := im.(*image.Paletted)
			if !ok {
				g.Err = fmt.Errorf("story font lost indices")
				return g.Canvas
			}
			for y := 0; y < Height; y++ {
				yy := y
				for x := 0; x < Width; x++ {
					written := s.Written[yy/8][x/8]
					tile := s.Tiles[yy/8][x/8]
					if yy == s.RasterLine && x/8 >= s.RasterColumn && s.RasterWritten[x/8] {
						written = true
						tile = s.RasterTiles[x/8]
					}
					if !written {
						continue
					}
					n := tile & 63
					index := font.ColorIndexAt(n%16*8+x%8, n/16*8+yy%8)
					if index != 0 {
						c := s.Palette[(tile>>13&3)*16+int(index)]
						g.Canvas.SetRGBA(x, y, color.RGBA{c[0], c[1], c[2], 255})
					}
				}
			}
		}
		g.drawActors(g.Canvas, high)
	}
	return g.Canvas
}

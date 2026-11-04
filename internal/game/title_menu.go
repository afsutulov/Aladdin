package game

import (
	"aladdin/internal/choreography"
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

type TitleMenu struct {
	World                                                        *NativeWorld
	Kind                                                         string
	Selection, Timeout, ActionLatch, ReleaseLatch, VerticalLatch int
	Repeat, SoundCursor, FontBank                                int
	Music, Effects                                               bool
	Tracking, Difficulty                                         int
	DisplayActors                                                [32]*choreography.Object
	Glyphs                                                       [28][40]int
	Written                                                      [28][40]bool
	OptionValuesPending                                          bool
	DemoRequested                                                bool
}

func newTitleMenu(p *Project, tick int) (*TitleMenu, error) {
	return newTitleMenuIn(p, nil, tick)
}

// newTitleMenuIn builds the menu in an existing sound/clock world when one is
// supplied. A title theme that is already playing there (from the power-on
// intro) continues instead of restarting, as in the original.
func newTitleMenuIn(p *Project, w *NativeWorld, tick int) (*TitleMenu, error) {
	m := &TitleMenu{Kind: "main", Music: true, Effects: true, Difficulty: 1, Timeout: 1500, ReleaseLatch: 255, FontBank: 0x4000}
	if w == nil {
		w = NewNativeWorld(&p.Manifest.Stages[1], &p.Scripts, p.Manifest.Frames)
	}
	m.World = w
	if w.musicID != 26 {
		w.requestMusic(26)
	}
	w.clearLifeActors()
	o := choreography.FromTemplate(p.Scripts.Templates[0x1e302a])
	o.SetWord(2, 212)
	o.SetWord(4, 404)
	w.Objects[2] = o
	w.set("tick", tick)
	w.set("camera_x", 0)
	w.set("camera_y", 0)
	m.print("main")
	if e := m.mainFrame(); e != nil {
		return nil, e
	}
	return m, nil
}
func (m *TitleMenu) print(name string) {
	for _, g := range m.World.Scripts.Menu.Text[name] {
		tile := g.Tile
		if tile&0x10000 != 0 {
			tile = tile&63 | m.FontBank
		}
		if g.X >= 0 && g.X < 40 && g.Y >= 0 && g.Y < 28 {
			m.Glyphs[g.Y][g.X] = tile
			m.Written[g.Y][g.X] = true
		}
	}
	if bank, ok := m.World.Scripts.Menu.EndBanks[name]; ok && bank >= 0 {
		m.FontBank = bank
	}
}
func (m *TitleMenu) captureActors() {
	for i, o := range m.World.Objects {
		m.DisplayActors[i] = nil
		if o != nil {
			v := *o
			m.DisplayActors[i] = &v
		}
	}
}

// The menu advances animation before motion, as 1DE6C6/1DE844 do.
func (m *TitleMenu) actors() error {
	w := m.World
	c := choreography.Choreographer{Scripts: w.Scripts}
	if w.get("tick")&1 != 0 {
		for _, o := range w.Objects {
			if o != nil && o.Alive() {
				if e := c.Animation(w, o); e != nil {
					return e
				}
			}
		}
	}
	for _, o := range w.Objects {
		if o != nil && o.Alive() && o.Frame >= 0 {
			w.objectMomentum(o)
			if e := c.Motion(w, o); e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *TitleMenu) mainFrame() error {
	m.World.set("tick", (m.World.get("tick")+1)&255)
	m.Timeout--
	if m.Timeout <= 0 {
		m.Timeout = 0
		m.DemoRequested = true
		return nil
	}
	if e := m.actors(); e != nil {
		return e
	}
	return nil
}
func (m *TitleMenu) cursor(x, y int) {
	if o := m.World.Objects[2]; o != nil {
		o.SetWord(2, x)
		o.SetWord(4, y)
	}
}
func (m *TitleMenu) resetText() { m.Glyphs = [28][40]int{}; m.Written = [28][40]bool{} }
func (m *TitleMenu) openOptions() {
	m.Kind = "options"
	m.Selection = 0
	m.ActionLatch = 255
	m.resetText()
	m.print("options")
	m.print("difficulty_label")
	m.print(fmt.Sprintf("difficulty%d", m.Difficulty))
	m.audioLabels()
	m.print(fmt.Sprintf("tracking%d", m.Tracking))
	m.cursor(180, 388)
}
func (m *TitleMenu) audioLabels() {
	state := "off"
	if m.Music {
		state = "on"
	}
	m.print(state + "_music")
	state = "off"
	if m.Effects {
		state = "on"
	}
	m.print(state + "_fx")
}
func (m *TitleMenu) openMain() {
	m.Kind = "main"
	m.Selection = 0
	m.ActionLatch = 0
	m.ReleaseLatch = 255
	m.Timeout = 1500
	m.resetText()
	m.print("main")
	m.cursor(212, 404)
}
func (m *TitleMenu) soundLabels() {
	m.print("sound_top")
	sounds := m.World.Scripts.Menu.Sounds
	for row := 0; row < 9; row++ {
		n := m.SoundCursor + row - 4
		label := "              "
		if n >= 0 && n < len(sounds) {
			label = sounds[n].Label
		}
		for x, c := range []byte(label) {
			col := 12 + x
			if col < 40 {
				m.Glyphs[16+row][col] = m.FontBank | int(c) - 32
				m.Written[16+row][col] = true
			}
		}
	}
	m.print("sound_bottom")
}
func (m *TitleMenu) openSound() {
	m.Kind = "sound"
	m.Repeat = 0
	m.ActionLatch = 255
	m.resetText()
	m.soundLabels()
	m.cursor(204, 420)
}

// Update returns true only for the original main-menu Start choice.
func (m *TitleMenu) Update(in Input) (bool, error) {
	w := m.World
	m.OptionValuesPending = false
	w.SoundEvents = nil
	if m.Kind == "main" {
		m.captureActors()
		press := in.Jump || in.Sword || in.Apple || in.Enter
		if press && m.ReleaseLatch == 0 {
			if m.Selection == 0 {
				return true, nil
			}
			m.openOptions()
			w.set("tick", (w.get("tick")+1)&255)
			return false, nil
		}
		if !press {
			m.ReleaseLatch = 0
		}
		if in.Up || in.Down {
			if m.ActionLatch == 0 {
				m.Selection ^= 1
				m.cursor(212, 404+16*m.Selection)
				m.ActionLatch = 255
				m.Timeout = 1500
				w.Sound(4, false)
			}
		} else {
			m.ActionLatch = 0
		}
		return false, m.mainFrame()
	}
	if e := m.actors(); e != nil {
		return false, e
	}
	m.captureActors()
	if m.Kind == "sound" {
		if in.Down || in.Up {
			if m.Repeat != 0 {
				m.Repeat--
			} else {
				delta := 1
				if !in.Down {
					delta = -1
				}
				m.SoundCursor = clamp(m.SoundCursor+delta, 0, len(w.Scripts.Menu.Sounds)-1)
				m.soundLabels()
				m.Repeat = 7
			}
		} else if in.Apple {
			if m.ActionLatch == 0 {
				m.ActionLatch = 255
				w.Sound(w.Scripts.Menu.Sounds[m.SoundCursor].ID, false)
			}
		} else if in.Jump || in.Sword || in.Enter {
			if m.ActionLatch == 0 {
				selection := m.Selection
				if o := w.Objects[2]; o != nil {
					o.SetWord(4, 1)
				}
				m.captureActors()
				m.openOptions()
				m.Selection = selection
				m.cursor(180, 388+16*selection)
				m.OptionValuesPending = true
			}
		} else {
			m.ActionLatch = 0
		}
		w.set("tick", (w.get("tick")+1)&255)
		return false, nil
	}
	press := in.Jump || in.Sword || in.Apple || in.Enter
	if press && m.ActionLatch == 0 {
		m.ActionLatch = 255
		if in.Enter || m.Selection == 5 {
			m.openMain()
			return false, m.mainFrame()
		}
		switch m.Selection {
		case 0:
			m.Difficulty = (m.Difficulty + 1) % 3
			m.print("difficulty_label")
			m.print(fmt.Sprintf("difficulty%d", m.Difficulty))
		case 1:
			m.Music = !m.Music
			w.stopAudio()
			if m.Music {
				w.requestMusic(26)
			}
			m.audioLabels()
		case 2:
			m.Effects = !m.Effects
			m.audioLabels()
		case 3:
			m.openSound()
		case 4:
			m.Tracking = (m.Tracking + 1) % 3
			m.print(fmt.Sprintf("tracking%d", m.Tracking))
		}
		w.Sound(4, false)
		w.set("tick", (w.get("tick")+1)&255)
		return false, nil
	}
	if !press {
		m.ActionLatch = 0
	}
	if in.Up || in.Down {
		if m.VerticalLatch == 0 {
			delta := 1
			if in.Up {
				delta = -1
			}
			m.Selection = (m.Selection + delta + 6) % 6
			m.VerticalLatch = 255
			m.cursor(180, 388+16*m.Selection)
			w.Sound(4, false)
		}
	} else {
		m.VerticalLatch = 0
	}
	w.set("tick", (w.get("tick")+1)&255)
	return false, nil
}

func (g *Game) OpenTitle() {
	// The original never reseeds outside attract demonstrations: the seed
	// left by the previous scene carries into the menu and the next game.
	var seed uint32
	keep := g.World != nil
	if keep {
		seed = g.World.Seed
	}
	g.openTitle(nil, 138, keep, seed)
}

// openTitleAfterAttract is the return from an attract demonstration: the
// original menu then starts on frame counter 52 and keeps the random seed
// left by the demonstration.
func (g *Game) openTitleAfterAttract() {
	var seed uint32
	keep := g.World != nil
	if keep {
		seed = g.World.Seed
	}
	g.openTitle(nil, 51, keep, seed)
}

// openTitleFrom continues from the power-on intro with its clock, seed and
// already playing title theme.
func (g *Game) openTitleFrom(w *NativeWorld, tick int, seed uint32) {
	g.openTitle(w, tick, true, seed)
}

func (g *Game) openTitle(w *NativeWorld, tick int, keepSeed bool, seed uint32) {
	g.Intro = nil
	g.Ending = nil
	g.LevelEntry = nil
	g.LevelTitle = nil
	g.Prelude = nil
	g.Story = nil
	g.Attract = nil
	old := g.Menu
	m, e := newTitleMenuIn(g.Project, w, tick)
	if e != nil {
		g.Err = e
		return
	}
	if keepSeed {
		m.World.Seed = seed
	}
	if old != nil {
		m.Difficulty = old.Difficulty
		m.Music = old.Music
		m.Effects = old.Effects
		m.Tracking = old.Tracking
		m.SoundCursor = old.SoundCursor
	}
	g.Menu = m
	g.Mode = 0
}
func (g *Game) startFromTitle() {
	m := g.Menu
	outgoing := image.NewRGBA(g.Canvas.Rect)
	copy(outgoing.Pix, g.drawTitle().Pix)
	g.Start(1)
	if g.Err != nil {
		return
	}
	w := g.World
	w.set("difficulty", m.Difficulty)
	w.set("camera_rate_mode", m.Tracking)
	lives, apples := 3, 0x3130
	if m.Difficulty == 0 {
		lives, apples = 5, 0x3135
	} else if m.Difficulty == 2 {
		lives, apples = 2, 0x3035
	}
	w.set("lives", lives)
	w.set("apples_ascii", apples)
	g.beginPrelude(outgoing)
}
func (g *Game) drawTitle() *image.RGBA {
	m := g.Menu
	old, cx, cy := g.World, g.CameraX, g.CameraY
	v := *m.World
	v.Objects = m.DisplayActors
	g.World = &v
	g.CameraX, g.CameraY = 0, 0
	defer func() { g.World = old; g.CameraX, g.CameraY = cx, cy }()
	pal := g.Project.Scripts.Menu.Palette
	c := pal[0]
	draw.Draw(g.Canvas, g.Canvas.Rect, &image.Uniform{C: color.RGBA{c[0], c[1], c[2], 255}}, image.Point{}, draw.Src)
	for _, high := range []bool{false, true} {
		suffix := "low"
		if high {
			suffix = "high"
		}
		for _, plane := range []string{"b", "a"} {
			im, e := g.Project.Image("menu-" + plane + "-" + suffix + ".png")
			if e != nil {
				g.Err = e
				return g.Canvas
			}
			for y := 0; y < Height; y++ {
				for x := 0; x < Width; x++ {
					if plane == "a" && m.Written[y/8][x/8] {
						continue
					}
					c := color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
					if c.A != 0 {
						g.Canvas.Set(x, y, c)
					}
				}
			}
		}
		if !high {
			g.drawActors(g.Canvas, false)
		}
	}
	font, e := g.Project.Image("menu-font.png")
	if e != nil {
		g.Err = e
		return g.Canvas
	}
	f, ok := font.(*image.Paletted)
	if !ok {
		g.Err = fmt.Errorf("menu font lost indices")
		return g.Canvas
	}
	for y := 0; y < 28; y++ {
		for x := 0; x < 40; x++ {
			if !m.Written[y][x] {
				continue
			}
			// The source redraw after Sound Test crosses the video boundary:
			// option labels precede these value writes in the displayed frame.
			if m.OptionValuesPending && (y == 16 && x >= 10 && x < 32 || y == 18 && x >= 21 && x < 24 || y == 20 && x >= 25 && x < 28) {
				continue
			}
			tile := m.Glyphs[y][x]
			n := tile & 63
			bank := (tile >> 13 & 3) * 16
			for yy := 0; yy < 8; yy++ {
				for xx := 0; xx < 8; xx++ {
					index := f.ColorIndexAt(n%16*8+xx, n/16*8+yy)
					if index != 0 {
						c := pal[bank+int(index)]
						g.Canvas.SetRGBA(x*8+xx, y*8+yy, color.RGBA{c[0], c[1], c[2], 255})
					}
				}
			}
		}
	}
	g.drawActors(g.Canvas, true)
	return g.Canvas
}

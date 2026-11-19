// Package game is the native Go resource and movement harness. It deliberately
// does not pretend that reconstructed collision data is a completed game port.
package game

import (
	"aladdin/internal/data"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

const Width, Height = 320, 224

type Input struct{ Left, Right, Up, Down, Jump, Sword, Apple, Enter, Escape, Free, Debug, Reset, View, Space bool }
type Project struct {
	Manifest data.Manifest
	Scripts  data.ScriptSet
	Atlas    image.Image
	HUD      *image.Paletted
	Images   map[string]image.Image
	intro    *introData
}

func Load() (*Project, error) {
	p := &Project{Images: map[string]image.Image{}}
	b, e := data.Files.ReadFile("assets/manifest.json")
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(b, &p.Manifest); e != nil {
		return nil, e
	}
	b, e = data.Files.ReadFile("assets/scripts.json")
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(b, &p.Scripts); e != nil {
		return nil, e
	}
	p.Atlas, e = p.Image("sprites.png")
	if e != nil {
		return nil, e
	}
	hud, e := p.Image("hud.png")
	if e != nil {
		return nil, e
	}
	var ok bool
	p.HUD, ok = hud.(*image.Paletted)
	if !ok {
		return nil, fmt.Errorf("HUD atlas lost palette indices")
	}
	if len(p.Scripts.Ending.Maps) != 2 || p.Scripts.Ending.FlightLoad <= 0 || p.Scripts.Ending.KissLoad <= 0 || len(p.Manifest.Frames) != 1581 || len(p.Manifest.Stages) != 14 || len(p.Scripts.GenieChoices) != 16 || len(p.Scripts.GenieDigits) != 26 || len(p.Scripts.Messages[0x16].Glyphs) != 15 || len(p.Scripts.Messages[0x0e].Glyphs) != 16 || len(p.Scripts.Menu.Sounds) != 94 || len(p.Scripts.Menu.Text) < 15 || len(p.Scripts.Story.Pages) != 18 || len(p.Scripts.Story.Layouts) != 6 || len(p.Scripts.Story.Plans) != 9 || len(p.Scripts.Prelude.Maps) != 4 {
		return nil, fmt.Errorf("incomplete converted asset set")
	}
	return p, nil
}
func (p *Project) Image(name string) (image.Image, error) {
	if im := p.Images[name]; im != nil {
		return im, nil
	}
	f, e := data.Files.Open("assets/" + name)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	im, e := png.Decode(f)
	if e != nil {
		return nil, e
	}
	p.Images[name] = im
	return im, nil
}

type Player struct {
	X, Y, VX, VY int
	Facing       int
	Ground       bool
}
type Game struct {
	Ending                                            *EndingSession
	Intro                                             *PublisherIntro
	LevelEntry                                        *LevelEntry
	LevelTitle                                        *LevelTitle
	Prelude                                           *NewGamePrelude
	Story                                             *StorySession
	soundOutputWorld                                  *NativeWorld
	soundOutputKind                                   string
	soundOutputMode                                   int
	soundOutputSequence                               uint64
	soundOutputMusicSequence, soundOutputStopSequence uint64
	Attract                                           *AttractSession
	NextDemo                                          int
	Menu                                              *TitleMenu
	HighScore                                         int
	World                                             *NativeWorld
	Err                                               error
	Project                                           *Project
	Mode, Selection, StagePosition                    int
	Tick                                              int
	Player                                            Player
	CameraX, CameraY                                  int
	FreeCamera, Debug                                 bool
	FrameIndex                                        int
	AutoFrames                                        bool
	last                                              Input
	Canvas                                            *image.RGBA
	actorLayer                                        *image.RGBA
	actorPriority                                     []bool
	atlasAlpha                                        *[256]bool
	atlasAlphaOf                                      *image.Paletted
}

func New(p *Project) *Game {
	g := &Game{HighScore: 100000, Project: p, Canvas: image.NewRGBA(image.Rect(0, 0, Width, Height)), Selection: 1}
	g.Start(1)
	g.OpenTitle()
	return g
}

// NewFromPowerOn starts with the original power-on publisher intro.
func NewFromPowerOn(p *Project) *Game {
	g := New(p)
	g.OpenIntro()
	return g
}
func (g *Game) Stage() *data.Stage { return &g.Project.Manifest.Stages[g.StagePosition] }
func (g *Game) Start(index int)    { g.startStage(index, true) }

// startStage can allocate inventory/state without executing gameplay
// preparation before the original story/header transition.
func (g *Game) startStage(index int, prepare bool) {
	g.Ending = nil
	g.LevelEntry = nil
	g.LevelTitle = nil
	g.Prelude = nil
	g.Story = nil
	if g.World != nil {
		g.HighScore = g.World.get("high_score")
	}
	found := false
	for i, s := range g.Project.Manifest.Stages {
		if s.Index == index {
			g.StagePosition = i
			found = true
			break
		}
	}
	if !found {
		g.Err = fmt.Errorf("stage %d has not been converted", index)
		return
	}
	s := g.Stage()
	g.World = NewNativeWorld(s, &g.Project.Scripts, g.Project.Manifest.Frames)
	g.World.set("high_score", g.HighScore)
	g.World.requestMusic(s.Music)
	g.World.EnableStageActors()
	if prepare && abuStage(s.Index) {
		if err := g.World.prepareAbu(); err != nil {
			g.Err = err
			return
		}
	}
	if prepare && s.Index == 12 {
		g.World.initializeIago()
	}
	if prepare && s.Index >= 9 && s.Index <= 12 {
		if err := g.World.prepareScene(); err != nil {
			g.Err = err
			return
		}
	}
	if prepare && s.Index == 8 {
		if err := g.World.prepareRugRide(); err != nil {
			g.Err = err
			return
		}
	}
	g.Err = nil
	g.Player = Player{X: s.SpawnX * 256, Y: (s.SpawnY + 13) * 256, Facing: 1}
	g.CameraX = clamp(s.CameraX, 0, max(0, s.Width-Width))
	g.CameraY = clamp(s.CameraY, 0, max(0, s.Height-Height))
	g.FreeCamera = false
	g.Mode = 1
	// Decode before drawing, so file/decode failures are visible during loading.
	layers := []string{"low", "high", "bg"}
	if s.Index == 4 || s.Index == 8 {
		layers = append(layers, "bg-high")
	}
	for _, suffix := range layers {
		if _, e := g.Project.Image(fmt.Sprintf("stage%02d-%s.png", s.Index, suffix)); e != nil {
			panic(e)
		}
	}
}
func clamp(v, a, b int) int { return min(max(v, a), b) }
func (g *Game) Update(in Input) bool {
	g.Tick++
	old := g.last
	g.last = in
	if in.Escape && !old.Escape {
		if g.Mode == 0 {
			return true
		}
		g.OpenTitle()
		return false
	}
	if g.Intro != nil {
		g.updateIntro(in)
		return false
	}
	if g.Ending != nil {
		g.updateEnding(in)
		return false
	}
	if g.Prelude != nil {
		g.updatePrelude(in)
		return false
	}
	if g.LevelTitle != nil {
		g.updateLevelTitle(in)
		return false
	}
	if g.LevelEntry != nil {
		g.updateLevelEntry(in)
		return false
	}
	if g.Story != nil {
		g.updateStory(in)
		return false
	}
	if g.Attract != nil {
		g.updateAttract(in)
		return false
	}
	if g.Mode == 0 {
		if in.Free && !old.Free {
			g.Mode = 3
			return false
		}
		if in.View && !old.View {
			g.Mode = 2
			g.FrameIndex = 219
			return false
		}
		start, e := g.Menu.Update(in)
		if e != nil {
			g.Err = e
			return false
		}
		if start {
			g.startFromTitle()
		} else if g.Menu.DemoRequested {
			g.requestAttract()
		}
		return false
	}
	if g.Mode == 3 {
		if in.Up && !old.Up {
			g.Selection = (g.Selection + len(g.Project.Manifest.Stages) - 1) % len(g.Project.Manifest.Stages)
		}
		if in.Down && !old.Down {
			g.Selection = (g.Selection + 1) % len(g.Project.Manifest.Stages)
		}
		if in.Enter && !old.Enter {
			g.Start(g.Project.Manifest.Stages[g.Selection].Index)
		}
		if in.View && !old.View {
			g.Mode = 2
			g.FrameIndex = 219
		}
		return false
	}
	if in.View && !old.View {
		if g.Mode == 2 {
			g.Mode = 1
		} else {
			g.Mode = 2
		}
	}
	if g.Mode == 2 {
		if in.Left && !old.Left {
			g.FrameIndex = (g.FrameIndex + 1580) % 1581
		}
		if in.Right && !old.Right {
			g.FrameIndex = (g.FrameIndex + 1) % 1581
		}
		if in.Up && !old.Up {
			g.FrameIndex = (g.FrameIndex + 1581 - 20) % 1581
		}
		if in.Down && !old.Down {
			g.FrameIndex = (g.FrameIndex + 20) % 1581
		}
		if in.Space && !old.Space {
			g.AutoFrames = !g.AutoFrames
		}
		if g.AutoFrames && g.Tick%6 == 0 {
			g.FrameIndex = (g.FrameIndex + 1) % 1581
		}
		return false
	}
	if in.Free && !old.Free {
		g.FreeCamera = !g.FreeCamera
	}
	if in.Debug && !old.Debug {
		g.Debug = !g.Debug
	}
	if in.Reset && !old.Reset {
		g.Start(g.Stage().Index)
		return false
	}
	if g.FreeCamera {
		dx, dy := 0, 0
		if in.Left {
			dx -= 4
		}
		if in.Right {
			dx += 4
		}
		if in.Up {
			dy -= 4
		}
		if in.Down {
			dy += 4
		}
		g.CameraX = clamp(g.CameraX+dx, 0, max(0, g.Stage().Width-Width))
		g.CameraY = clamp(g.CameraY+dy, 0, max(0, g.Stage().Height-Height))
		return false
	}
	if err := g.World.Update(in); err != nil {
		g.Err = err
		return false
	}
	g.HighScore = g.World.get("high_score")
	if g.World.Flow.Phase == FlowLoad {
		g.advanceStage()
	}
	if g.World.Life.Phase == LifeContinueLoad {
		g.continueStage()
	}
	if g.World.Life.Phase == LifeGameOver {
		g.OpenTitle()
	}
	g.syncWorldPresentation()
	return false
}

func (g *Game) syncWorldPresentation() {
	x, y := g.World.PlayerPosition()
	p := &g.Player
	p.X = x * 256
	p.Y = (y - 256 + 13) * 256
	p.VX = int(int16(g.World.get("momentum_x")))
	p.VY = int(int16(g.World.get("momentum_y")))
	p.Ground = g.World.flag("grounded")
	p.Facing = 1
	if g.World.flag("facing_left") {
		p.Facing = -1
	}
	g.CameraX = g.World.get("camera_x")
	g.CameraY = g.World.get("camera_y")
}

// Floor samples contours for the debug overlay. NativeWorld handles the
// original player sensors, alternate contours and object interactions.
func Floor(s *data.Stage, x, from, to int) (int, bool) {
	if x < 0 || x >= s.Width {
		return 0, false
	}
	w := s.Width / 16
	best := math.MaxInt
	for row := max(0, from/16-1); row <= min(s.Height/16-1, to/16+1); row++ {
		i := row*w + x/16
		if i < 0 || i >= len(s.Room) {
			continue
		}
		o := int(s.Room[i]) / 2
		if o >= len(s.Floor) {
			continue
		}
		c := int(s.Floor[o])*16 + (x & 15)
		if c >= len(s.Contours) {
			continue
		}
		h := int(s.Contours[c] & 63)
		if h == 0 || h > 16 {
			continue
		}
		y := row*16 + h - 1
		if y >= from && y <= to {
			best = min(best, y)
		}
	}
	return best, best != math.MaxInt
}

func (g *Game) sprite(dst draw.Image, index, x, y, scale int, flip bool, banks ...int) {
	if index < 0 || index >= len(g.Project.Manifest.Frames) {
		return
	}
	f := g.Project.Manifest.Frames[index]
	palette := g.renderPalette()
	left := x + f.OffsetX*scale
	if flip {
		left = x - (f.OffsetX+f.W)*scale
	}
	top := y + f.OffsetY*scale
	flipY := len(banks) > 1 && banks[1] != 0
	if flipY {
		top = y - (f.OffsetY+f.H)*scale
	}
	g.blitSprite(dst, f.X, f.Y, f.W, f.H, left, top, scale, flip, flipY, palette, banks...)
}
func (g *Game) blitSprite(dst draw.Image, atlasX, atlasY, width, height, left, top, scale int, flip, flipY bool, palette [64][3]uint8, banks ...int) {
	if atlas, ok := g.Project.Atlas.(*image.Paletted); ok && len(banks) > 0 && image.Rect(atlasX, atlasY, atlasX+width, atlasY+height).In(atlas.Rect) {
		// Same result as the generic loop below, reading palette indices
		// directly instead of boxing a colour per atlas pixel.
		opaque := g.atlasOpacity(atlas)
		base := (banks[0] & 3) * 16
		for yy := 0; yy < height; yy++ {
			row := atlas.Pix[atlas.PixOffset(atlasX, atlasY+yy):][:width]
			for xx, idx := range row {
				if !opaque[idx] {
					continue
				}
				rgb := palette[base+int(idx)&15]
				c := color.RGBA{rgb[0], rgb[1], rgb[2], 255}
				sx, sy := xx, yy
				if flip {
					sx = width - 1 - xx
				}
				if flipY {
					sy = height - 1 - yy
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						dst.Set(left+sx*scale+dx, top+sy*scale+dy, c)
					}
				}
			}
		}
		return
	}
	for yy := 0; yy < height; yy++ {
		for xx := 0; xx < width; xx++ {
			c := g.Project.Atlas.At(atlasX+xx, atlasY+yy)
			_, _, _, a := c.RGBA()
			if a == 0 {
				continue
			}
			if len(banks) > 0 {
				if atlas, ok := g.Project.Atlas.(*image.Paletted); ok {
					i := int(atlas.ColorIndexAt(atlasX+xx, atlasY+yy)) & 15
					rgb := palette[(banks[0]&3)*16+i]
					c = color.RGBA{rgb[0], rgb[1], rgb[2], 255}
				}
			}
			sx, sy := xx, yy
			if flip {
				sx = width - 1 - xx
			}
			if flipY {
				sy = height - 1 - yy
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					dst.Set(left+sx*scale+dx, top+sy*scale+dy, c)
				}
			}
		}
	}
}

// actorSprite ports per-part culling at 1D60D0..1D619C. Facing offsets wrap
// as bytes before unsigned-word visibility tests; shake follows the gate.
func (g *Game) actorSprite(dst draw.Image, index, x, y, shakeX, shakeY int, flip, flipY, fixed bool, bank int) {
	if index < 0 || index >= len(g.Project.Manifest.Frames) {
		return
	}
	f := g.Project.Manifest.Frames[index]
	palette := g.renderPalette()
	for i := 0; i < len(f.Parts); i++ {
		p := f.Parts[i]
		px, py := p.OffsetX+128, p.OffsetY+128
		if flip {
			px = int(byte(-px - p.W))
		}
		if flipY && !fixed {
			py = int(byte(-py - p.H))
		}
		if uint16(x+px) >= 0x1bf || uint16(y+py) >= 0x15f {
			continue
		}
		left, top := x+px-128+shakeX, y+py-128+shakeY
		target := dst
		if d, ok := dst.(actorTarget); ok {
			target = &spriteSpan{d, left, d.lines.reserve(left, top, p.W, p.H)}
		}
		g.blitSprite(target, p.X, p.Y, p.W, p.H, left, top, 1, flip, flipY, palette, bank)
	}
}

func (g *Game) scene() {
	s := g.Stage()
	dst := g.Canvas
	if g.World != nil && (g.World.Life.Phase == LifeHighScore || g.World.Life.Phase == LifeHighScoreExit) {
		draw.Draw(dst, dst.Rect, image.Black, image.Point{}, draw.Src)
		return
	}
	if g.World != nil && gameOverPhase(g.World.Life.Phase) {
		g.drawGameOver()
		return
	}
	if g.World != nil && continuePhase(g.World.Life.Phase) {
		g.drawContinue()
		return
	}
	if g.World != nil {
		switch g.World.Flow.Phase {
		case FlowGenieFadeIn, FlowGenie, FlowGenieFadeOut:
			g.drawGenie()
			return
		case FlowComplete, FlowCompleteFade, FlowBonusIntro, FlowBonusIntroFade:
			name := "level-complete.png"
			if g.World.Flow.Phase == FlowBonusIntro || g.World.Flow.Phase == FlowBonusIntroFade {
				name = "abu-intro.png"
			} else if abuStage(g.World.Stage.Index) && !g.World.flag("flag_faa2") {
				name = "abu-failed.png"
			}
			im, e := g.Project.Image(name)
			if e != nil {
				g.Err = e
				return
			}
			draw.Draw(dst, dst.Rect, im, image.Point{}, draw.Src)
			g.drawActors(dst)
			return
		case FlowBetweenFade, FlowLoad:
			draw.Draw(dst, dst.Rect, image.Black, image.Point{}, draw.Src)
			return
		}
	}
	if g.World != nil && g.World.Life.Phase >= LifeClear && g.World.Life.Phase != LifeFadeIn {
		draw.Draw(dst, dst.Rect, image.Black, image.Point{}, draw.Src)
		if g.World.Life.Phase != LifeGameOver {
			g.drawActors(dst)
		}
		return
	}

	draw.Draw(dst, dst.Rect, &image.Uniform{C: color.RGBA{s.Backdrop[0], s.Backdrop[1], s.Backdrop[2], 255}}, image.Point{}, draw.Src)
	if s.Index == 8 && g.World != nil {
		g.drawRugScene(dst)
		return
	}
	bg, _ := g.Project.Image(fmt.Sprintf("stage%02d-bg.png", s.Index))
	bg = g.paletteImage(bg)
	interior := s.Index == 11 || s.Index == 12
	if interior {
		x, y := palaceInteriorScroll(g.CameraX, g.CameraY)
		for left := x - 512; left < Width; left += 512 {
			for top := -y; top < Height; top += 256 {
				drawOver(dst, image.Rect(left, top, left+512, top+256), bg, image.Point{})
			}
		}
	}
	var scroll [28]int
	if s.Index <= 2 && g.World != nil {
		phase := g.World.get("background_phase")
		if s.Index == 0 && g.World.flag("attract_mode") {
			phase = (phase - 1) & 65535
		}
		scroll = agrabahScroll(g.CameraX, phase)
		if s.Index == 2 {
			scroll = agrabahScroll(g.CameraX, (g.World.get("background_phase")-1)&65535)
			for i := range scroll {
				scroll[i] = scroll[0]
			}
		}
	} else if s.Index == 6 {
		for i := range scroll {
			scroll[i] = (-((int(uint16(g.CameraX)) + 16) >> 1)) & 511
		}
	} else if s.Index == 13 && g.World != nil {
		scroll = palaceScroll(g.CameraX, (g.World.get("background_phase")+1)&65535)
		for i := range scroll {
			scroll[i] = scroll[0]
		}
	} else if s.Index == 3 && g.World != nil {
		scroll = desertScroll(g.CameraX, g.World.get("background_phase"))
	} else if s.Index == 5 {
		for i := range scroll {
			scroll[i] = caveScroll(g.CameraX)
		}
	} else if s.Index == 7 && g.World != nil {
		scroll = escapeScroll(g.CameraX, g.World.get("tick"))
	} else if s.Index == 9 {
		scroll = lampScroll(g.CameraX)
	} else if s.Index == 10 && g.World != nil {
		// Clouds are written before the video boundary; actor/camera
		// presentation is latched from the preceding completed step.
		scroll = palaceScroll(g.CameraX, (g.World.get("background_phase")+1)&65535)
	} else {
		for i := range scroll {
			scroll[i] = (-g.CameraX / 4) & 511
		}
	}
	for row, offset := range scroll {
		if s.Index == 4 || interior {
			break // This stage's parallax plane sits in front of the room.
		}
		start := offset
		if start > 0 {
			start -= 512
		}
		for ; start < Width; start += 512 {
			drawOver(dst, image.Rect(start, row*8, start+512, row*8+8), bg, image.Pt(0, row*8))
		}
	}
	low, _ := g.Project.Image(fmt.Sprintf("stage%02d-low.png", s.Index))
	high, _ := g.Project.Image(fmt.Sprintf("stage%02d-high.png", s.Index))
	low, high = g.paletteImage(low), g.paletteImage(high)
	g.drawRoomPlane(dst, low)
	if s.Index == 4 {
		g.drawDungeonPlane(dst, bg)
	}
	g.drawActors(dst, false)
	g.drawRoomPlane(dst, high)
	if s.Index == 4 {
		front, _ := g.Project.Image("stage04-bg-high.png")
		g.drawDungeonPlane(dst, g.paletteImage(front))
	}
	g.drawActors(dst, true)
	g.drawHUD(dst)
	if g.World != nil && g.World.ParryFlash {
		// The reference palette write begins during the first scanlines.
		// Logical pause timing is validated; the exact write scanline can
		// still vary with the original renderer's workload.
		draw.Draw(dst, image.Rect(0, 14, Width, Height), &image.Uniform{C: color.RGBA{238, 238, 238, 255}}, image.Point{}, draw.Src)
	}
	if g.Debug {
		for x := 0; x < Width; x += 2 {
			for y := 0; y < Height; y++ {
				if _, ok := Floor(s, x+g.CameraX+16, y+g.CameraY+16, y+g.CameraY+16); ok {
					dst.Set(x, y, color.RGBA{0, 255, 160, 255})
				}
			}
		}
		for _, m := range s.Entities {
			x, y := m.X-g.CameraX-16, m.Y-g.CameraY-16
			rect(dst, x, y, 3, 3, color.RGBA{255, 80, 200, 255})
		}
	}
}

func (g *Game) sceneShake() int {
	if g.World == nil || (g.World.Stage.Index != 5 && g.World.Stage.Index != 6 && g.World.Stage.Index != 7) || g.World.Life.Phase >= LifeClear || g.World.Flow.Phase >= FlowComplete {
		return 0
	}
	return g.World.screenShake()
}

func (g *Game) drawRoomPlane(dst *image.RGBA, plane image.Image) {
	x, y := g.CameraX+16, g.CameraY+16+g.sceneShake()
	if g.World != nil && g.World.Stage.Index == 7 {
		x += escapeJitter(g.World.get("tick"))
	}
	if g.World == nil || len(g.World.RemovedCells) == 0 {
		drawOver(dst, dst.Rect, plane, image.Pt(x, y))
		return
	}
	columns := g.World.Stage.Width / 16
	for top := y &^ 15; top < y+Height; top += 16 {
		for left := x &^ 15; left < x+Width; left += 16 {
			if g.World.RemovedCells[(top/16)*columns+left/16] {
				continue
			}
			drawOver(dst, image.Rect(left-x, top-y, left-x+16, top-y+16), plane, image.Pt(left, top))
		}
	}
}

func (g *Game) drawDungeonPlane(dst *image.RGBA, plane image.Image) {
	x, y := dungeonScroll(g.CameraX, g.CameraY)
	for left := x - 512; left < Width; left += 512 {
		for top := -y; top < Height; top += 256 {
			drawOver(dst, image.Rect(left, top, left+512, top+256), plane, image.Point{})
		}
	}
}
func (g *Game) Render() *image.RGBA {
	if g.Intro != nil {
		return g.renderIntro()
	}
	if g.Ending != nil {
		return g.renderEnding()
	}
	if g.LevelEntry != nil {
		return g.renderLevelEntry()
	}
	if g.LevelTitle != nil {
		return g.renderScene(g.LevelTitle.Scene)
	}
	if g.Prelude != nil {
		return g.renderPrelude()
	}
	if g.Story != nil {
		return g.renderStory()
	}
	if g.Attract != nil && g.Attract.Credits != nil {
		return g.renderCredits()
	}
	if g.Attract != nil && g.Attract.Phase == "demo_out" && g.Attract.Display != nil {
		old := g.World
		g.World = g.Attract.Display
		defer func() { g.World = old }()
	}
	if g.Mode == 0 && g.Menu != nil {
		im := g.drawTitle()
		if g.Attract != nil && g.Attract.Phase == "menu_out" {
			g.attractPaletteFade(g.Attract.Age, false)
		}
		return im
	}
	logicWorld, cameraX, cameraY := g.World, g.CameraX, g.CameraY
	if g.World != nil && (g.World.BufferedPresentation || g.World.Stage.Index >= 9 && g.World.Stage.Index <= 12 || abuStage(g.World.Stage.Index) || g.World.flag("attract_mode") || g.World.Flow.Phase == FlowGenie || g.World.Flow.Phase == FlowGenieFadeOut) && g.World.VisibleFrame != nil && g.World.Life.Phase == LifePlaying && (g.World.Flow.Phase == FlowPlaying || g.World.Flow.Phase == FlowGenie || g.World.Flow.Phase == FlowGenieFadeOut) {
		g.World = g.World.VisibleFrame
		if logicWorld.BufferedPresentation {
			// 1D509A updates the scroll table after the preceding video wait;
			// the sprite list uploaded there still belongs to the prior step.
			v := *g.World
			v.Variables = make(map[string]uint32, len(g.World.Variables))
			for name, value := range g.World.Variables {
				v.Variables[name] = value
			}
			v.set("background_phase", logicWorld.get("background_phase"))
			g.World = &v
		}
		if logicWorld.Flow.Phase == FlowGenieFadeOut {
			g.World.Flow.Phase, g.World.Flow.Age = logicWorld.Flow.Phase, logicWorld.Flow.Age
		}
		g.CameraX, g.CameraY = g.World.get("camera_x"), g.World.get("camera_y")
		defer func() { g.World, g.CameraX, g.CameraY = logicWorld, cameraX, cameraY }()
	}
	g.scene()
	g.applyLifeFade()
	if g.Attract != nil && g.Attract.Phase == "demo_out" {
		g.attractPaletteFade(g.Attract.Age, false)
	}
	dst := g.Canvas
	if g.Mode == 3 {
		shade(dst, 0, 0, Width, Height, 150)
		Text(dst, 24, 15, "ALADDIN", color.RGBA{255, 208, 100, 255}, 3)
		Text(dst, 24, 43, "NATIVE PORT / WORK IN PROGRESS", color.RGBA{230, 220, 194, 255}, 1)
		Text(dst, 24, 61, "SCENE AND MOVEMENT PREVIEW", color.RGBA{194, 196, 214, 255}, 1)
		first := clamp(g.Selection-3, 0, max(0, len(g.Project.Manifest.Stages)-7))
		for n := first; n < min(first+7, len(g.Project.Manifest.Stages)); n++ {
			y := 81 + (n-first)*13
			c := color.RGBA{200, 200, 210, 255}
			if n == g.Selection {
				c = color.RGBA{255, 212, 100, 255}
				Text(dst, 15, y, ">", c, 1)
			}
			Text(dst, 25, y, g.Project.Manifest.Stages[n].Name, c, 1)
		}
		Text(dst, 24, 183, "ENTER: OPEN   F3: SPRITES", color.RGBA{235, 235, 240, 255}, 1)
		Text(dst, 24, 198, "MUSIC AND SOME GAMEPLAY UNFINISHED", color.RGBA{238, 163, 140, 255}, 1)
		return dst
	}
	if g.Mode == 2 {
		draw.Draw(dst, dst.Rect, &image.Uniform{C: color.RGBA{23, 27, 40, 255}}, image.Point{}, draw.Src)
		for yy := 40; yy < 190; yy += 8 {
			for xx := 48; xx < 272; xx += 8 {
				if (xx/8+yy/8)%2 == 0 {
					rect(dst, xx, yy, 8, 8, color.RGBA{33, 37, 51, 255})
				}
			}
		}
		g.sprite(dst, g.FrameIndex, 160, 127, 2, false)
		f := g.Project.Manifest.Frames[g.FrameIndex]
		Text(dst, 10, 12, fmt.Sprintf("CHOPPER FRAME %04d / 1581", g.FrameIndex), color.RGBA{255, 214, 130, 255}, 1)
		Text(dst, 10, 25, fmt.Sprintf("ROM ADDRESS %06X", f.Address), color.RGBA{180, 190, 210, 255}, 1)
		Text(dst, 10, 197, "ARROWS: SELECT  SPACE: AUTO", color.RGBA{230, 230, 240, 255}, 1)
		Text(dst, 10, 210, "F3: SCENE  ESC: MENU", color.RGBA{180, 190, 210, 255}, 1)
		return dst
	}
	if g.Debug {
		shade(dst, 0, 0, Width, 20, 180)
		Text(dst, 5, 5, "DEBUG - "+g.Stage().Name, color.RGBA{255, 235, 190, 255}, 1)
	}
	if g.FreeCamera {
		shade(dst, 0, 205, 320, 19, 180)
		Text(dst, 5, 211, "FREE CAMERA / F2 TO RETURN", color.RGBA{255, 235, 190, 255}, 1)
	}
	return dst
}
func rect(dst draw.Image, x, y, w, h int, c color.Color) {
	draw.Draw(dst, image.Rect(x, y, x+w, y+h), &image.Uniform{C: c}, image.Point{}, draw.Src)
}
func shade(dst draw.Image, x, y, w, h int, alpha uint8) {
	draw.Draw(dst, image.Rect(x, y, x+w, y+h), &image.Uniform{C: color.NRGBA{0, 0, 0, alpha}}, image.Point{}, draw.Over)
}

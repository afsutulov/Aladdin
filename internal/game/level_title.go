package game

import "aladdin/internal/data"

// LevelTitle is the native scene at 1DBA92..1DBC62. Resource jobs, video
// waits, actor updates and physical-button polling have separate boundaries.
type LevelTitle struct {
	World                *NativeWorld
	Scene                *StorySession
	Data                 *data.LevelTitleData
	Phase                string
	Age                  int
	Music, Effects, Done bool
}

func newLevelTitle(w *NativeWorld, music, effects bool) *LevelTitle {
	w.clearLifeActors()
	w.Life.Phase = LifeClear
	w.SoundEvents = nil
	if music && effects {
		w.Sound(23, false)
	}
	d := &w.Scripts.LevelTitle
	s := &StorySession{World: w, RasterLine: -1, Loading: d.LoadFrames}
	s.Layout.Background = 0x157ac0
	s.Page.Foreground = 0x157850
	return &LevelTitle{World: w, Scene: s, Data: d, Phase: "load", Music: music, Effects: effects}
}
func (g *Game) beginLevelTitle(w *NativeWorld) {
	if w == g.World {
		// A level with no preceding story still owns an independent cinematic
		// actor list. Clearing its lettering must not clear the gameplay actors.
		v := NewNativeWorld(w.Stage, w.Scripts, w.Frames)
		v.HUDWaveIndex, v.HUDWaveTile = w.HUDWaveIndex, w.HUDWaveTile
		v.Seed = w.Seed
		v.set("tick", w.get("tick"))
		w = v
	}
	music, effects := true, true
	if g.Menu != nil {
		music, effects = g.Menu.Music, g.Menu.Effects
	}
	g.LevelTitle = newLevelTitle(w, music, effects)
}
func (h *LevelTitle) phase(name string) { h.Phase = name; h.Age = 0 }
func (h *LevelTitle) wave() {
	value, index := 0, (h.World.get("tick")&62)/2
	for line := 0; line < 128; line++ {
		value = (value + h.Data.Wave[index]) & 511
		h.Scene.BackgroundX[line] = value
		index = (index + 1) & 31
	}
}
func (h *LevelTitle) fade(in bool) {
	for i, c := range h.Data.Palette {
		for channel, v := range c {
			if in {
				h.Scene.Palette[i][channel] = uint8(min(int(v), h.Age*34))
			} else {
				h.Scene.Palette[i][channel] = uint8(max(0, int(h.Scene.Palette[i][channel])-34))
			}
		}
	}
}
func (h *LevelTitle) Update(in Input) error {
	h.Scene.DisplayActors = h.Scene.PendingActors
	w := h.World
	w.SoundEvents = nil
	h.Age++
	switch h.Phase {
	case "load":
		if h.Age != h.Data.LoadFrames {
			break
		}
		h.Scene.Loading = 0
		if e := w.showMessage(w.Stage.Index); e != nil {
			return e
		}
		w.set("tick", 255)
		if e := w.lifeActors(); e != nil {
			return e
		}
		h.Scene.cacheActors()
		h.phase("warm")
	case "warm":
		h.wave()
		if !h.Music {
			h.phase("in")
			break
		}
		h.phase("music-wait")
		h.Age = 1
	case "music-wait":
		if h.Age == 6 {
			w.stopAudio()
			if h.Effects {
				w.Sound(24, false)
				w.Sound(10, false)
			}
			h.phase("in")
		}
	case "in":
		h.fade(true)
		if h.Age == 16 {
			h.Scene.Palette = h.Data.Palette
			h.phase("hold")
			return h.hold()
		}
	case "hold":
		// A physical button is polled when the preceding video wait returns.
		// The first hold update is mandatory, even with a button already held.
		if preludeAction(in) {
			h.wave()
			h.phase("out")
			h.Age = 1
			break
		}
		return h.hold()
	case "out":
		if h.Age > 1 {
			h.fade(false)
		}
		if h.Age == 17 {
			h.Done = true
		}
	}
	return nil
}
func (h *LevelTitle) hold() error {
	// The next wave table is written after the current video boundary.
	h.wave()
	h.World.set("tick", (h.World.get("tick")+1)&255)
	if e := h.World.lifeActors(); e != nil {
		return e
	}
	h.Scene.cacheActors()
	if h.Age == 449 {
		h.phase("out")
	}
	return nil
}
func (g *Game) updateLevelTitle(in Input) {
	h := g.LevelTitle
	if e := h.Update(in); e != nil {
		g.Err = e
		return
	}
	if h.Done {
		if _, ok := levelEntryProfiles[g.World.Stage.Index]; ok {
			g.LevelTitle = nil
			g.beginLevelEntry(h.World)
			return
		}
		g.World.Seed = h.World.Seed
		g.LevelTitle = nil
		if g.World.Life.Phase == LifePlaying {
			apples := g.World.get("apples_ascii")
			if e := g.World.respawn(); e != nil {
				g.Err = e
				return
			}
			g.World.set("apples_ascii", apples)
			g.World.set("invulnerability", 0)
		}
		g.World.requestMusic(g.World.Stage.Music)
		g.World.Life = LifeCycle{Phase: LifeFadeIn, FreshStage: true}
	}
}

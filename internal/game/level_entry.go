package game

import (
	"aladdin/internal/choreography"
	"fmt"
	"image"
	"image/color"
)

// LevelEntry runs converted resource jobs before the first ordinary update.
// Market, Rooftops and Desert have qualified resource/preparation profiles. Other
// stage loaders retain their existing initializer until converted.
type LevelEntry struct {
	World             *NativeWorld
	Job, Age          int
	Phase             string
	Music             bool
	PreparationVideos int
	SpriteVideos      int
	profile           entryProfile
}
type entryJob struct {
	name   string
	videos int
}

// Original map-object, character, background, room and row-table work jobs.
// These delays belong to resource/preparation work, not a replay timeline.
type entryProfile struct {
	jobs            [9]entryJob
	preparation     [2]int
	warmup          [2]int
	caption, scroll int
}

var levelEntryProfiles = map[int]entryProfile{
	0: {jobs: [9]entryJob{{"reset", 1}, {"objects", 9}, {"characters", 57}, {"background", 3}, {"room", 23}, {"rows", 1}, {"preparation", 0}, {"artwork", 6}, {"sprites", 1}}, preparation: [2]int{15, 25}, warmup: [2]int{2, 1}, caption: 0x18, scroll: -1},
	1: {jobs: [9]entryJob{{"reset", 1}, {"objects", 9}, {"characters", 57}, {"background", 3}, {"room", 23}, {"rows", 1}, {"preparation", 0}, {"artwork", 6}, {"sprites", 1}}, preparation: [2]int{9, 12}, scroll: -1},
	3: {jobs: [9]entryJob{{"reset", 1}, {"objects", 8}, {"characters", 52}, {"background", 3}, {"room", 17}, {"rows", 0}, {"preparation", 0}, {"artwork", 0}, {"sprites", 1}}, preparation: [2]int{13, 21}, caption: 0x11, scroll: 1},
}

func (g *Game) beginLevelEntry(clock *NativeWorld) {
	old := g.World
	profile, ok := levelEntryProfiles[old.Stage.Index]
	if !ok {
		g.Err = fmt.Errorf("stage%d has no converted entry profile", old.Stage.Index)
		return
	}
	w := NewNativeWorld(old.Stage, old.Scripts, old.Frames)
	for _, name := range []string{"lives", "continues", "apples_ascii", "gems_ascii", "score", "score_progress", "high_score", "difficulty", "camera_rate_mode", "invincible", "genie_tokens", "flag_f521"} {
		w.set(name, old.get(name))
	}
	w.RouteCursor = old.RouteCursor
	w.HUDWaveIndex, w.HUDWaveTile = old.HUDWaveIndex, old.HUDWaveTile
	w.EnableStageActors()
	w.Seed = clock.Seed
	w.set("tick", clock.get("tick"))
	w.set("player_x", 0)
	w.set("player_y", 0)
	w.Objects[0] = nil
	w.stopAudio()
	// Preserve the inventory world's identity across the cinematic transition.
	*old = *w
	music := g.Menu == nil || g.Menu.Music
	// The forward sweep has a fixed incoming clock. With its odd value,
	// additional animation/artwork work extends this stage-specific job.
	spriteVideos := profile.warmup[clock.get("tick")&1]
	if spriteVideos == 0 {
		spriteVideos = profile.jobs[8].videos
	}
	g.LevelEntry = &LevelEntry{World: old, Phase: "load", Music: music, PreparationVideos: profile.preparation[clock.get("tick")&1], SpriteVideos: spriteVideos, profile: profile}
}
func (g *Game) updateLevelEntry(in Input) {
	e := g.LevelEntry
	w := e.World
	w.SoundEvents = nil
	e.Age++
	if e.Phase == "fade" {
		if e.Age == 16 {
			g.LevelEntry = nil
			// The original initializer clears the physical input latches. Held
			// controls are first sampled after this mandatory initial video.
			if err := w.Update(Input{}); err != nil {
				g.Err = err
			}
			g.syncWorldPresentation()
		}
		return
	}
	for {
		videos := e.profile.jobs[e.Job].videos
		if e.Job == 6 {
			videos = e.PreparationVideos
		}
		if e.Job == 8 {
			videos = e.SpriteVideos
		}
		if e.Age < videos {
			return
		}
		switch e.Job {
		case 0:
			w.Objects[0] = choreography.FromTemplate(w.Scripts.Templates[0x1e2dfa])
		case 5:
			if e.profile.caption != 0 {
				if err := w.showMessage(e.profile.caption); err != nil {
					g.Err = err
					return
				}
			}
			if err := w.prepareInitialMap(w.get("tick")); err != nil {
				g.Err = err
				return
			}
		case 7:
			w.stopAudio()
			if e.Music {
				w.requestMusic(w.Stage.Music)
			}
			w.set("tick", 255)
			w.sourceAnimation(0x1494aa)
			w.set("grounded", 255)
			w.set("fall_ticks", 0)
			w.set("momentum_y", 0)
			c := choreography.Choreographer{Scripts: w.Scripts}
			for _, o := range w.Objects {
				if o != nil && o.Alive() {
					if err := c.Animation(w, o); err != nil {
						g.Err = err
						return
					}
				}
			}
			w.updateHUDWave()
		case 8:
			w.set("background_phase", (w.get("background_phase")+e.profile.scroll)&65535)
			w.BufferedPresentation, w.FirstStageUpdate = true, true
			e.Phase = "fade"
			g.syncWorldPresentation()
		}
		e.Job++
		e.Age = 0
		if e.Phase == "fade" {
			return
		}
		// Zero-video jobs complete in the same presentation boundary.
	}
}

func (g *Game) renderLevelEntry() *image.RGBA {
	e := g.LevelEntry
	if e.Phase != "fade" {
		for i := 0; i < len(g.Canvas.Pix); i += 4 {
			g.Canvas.Pix[i] = 0
			g.Canvas.Pix[i+1] = 0
			g.Canvas.Pix[i+2] = 0
			g.Canvas.Pix[i+3] = 255
		}
		return g.Canvas
	}
	g.scene()
	limit := e.Age * 34
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			c := g.Canvas.RGBAAt(x, y)
			g.Canvas.SetRGBA(x, y, color.RGBA{uint8(min(int(c.R), limit)), uint8(min(int(c.G), limit)), uint8(min(int(c.B), limit)), 255})
		}
	}
	return g.Canvas
}

// prepareInitialMap follows the fresh-stage sweeps at 1D4D0A. The first
// sweep precedes the first world-position sync; the reverse sweep syncs the
// player from the spawn offsets before each pass. No captured actor is used.
func (w *NativeWorld) prepareInitialMap(tick int) error {
	// Final Cut removes this marker AFTER preparation (1E2D60). Its
	// temporary actor still runs and consumes random values during loading.
	key := -1
	if w.Stage.Index == 1 {
		key = int(w.Stage.Room[0x54ba/2]) / 2
		delete(w.Activated, key)
	}
	w.set("tick", tick)
	w.set("grounded", 0)
	w.set("jump_hold", 255)
	w.set("player_x", 0)
	w.set("player_y", 0)
	w.syncPlayer()
	w.set("camera_x", w.Checkpoint.CameraX)
	w.set("camera_y", w.Checkpoint.CameraY)
	w.set("camera_tile_x", w.Checkpoint.CameraX)
	w.set("camera_tile_y", w.Checkpoint.CameraY)
	w.LastActivationX, w.LastActivationY = w.Checkpoint.CameraX, w.Checkpoint.CameraY
	w.set("flag_f6a5", 255)
	c := choreography.Choreographer{Scripts: w.Scripts}
	draw := func() error {
		if e := w.moveActors(); e != nil {
			return e
		}
		if w.get("tick")&1 != 0 {
			for i := 0; i < len(w.Objects); i++ {
				if o := w.Objects[i]; o != nil && o.Alive() {
					if e := c.Animation(w, o); e != nil {
						return e
					}
				}
			}
		}
		w.updateHUDWave()
		delta := -1
		if w.Stage.Index == 3 {
			delta = 1
		}
		w.set("background_phase", (w.get("background_phase")+delta)&65535)
		w.advanceCameraTiles()
		return w.activateMarkers()
	}
	for i := 0; i < 23; i++ {
		if w.get("camera_x")+16 < w.Stage.Width-0x161 {
			w.set("camera_x", w.get("camera_x")+16)
			if e := draw(); e != nil {
				return e
			}
		}
	}
	for i := 0; i < 23; i++ {
		w.set("player_x", w.Checkpoint.X)
		w.set("player_y", w.Checkpoint.Y)
		w.syncPlayer()
		w.set("tick", (w.get("tick")+1)&255)
		if w.get("camera_tile_x") >= 17 {
			w.set("camera_x", w.get("camera_x")-16)
			if e := draw(); e != nil {
				return e
			}
		}
	}
	w.set("flag_f6a5", 0)
	if key >= 0 {
		w.Activated[key] = true
	}
	return nil
}

package game

// SoundOutput separates native game commands from the desktop audio device.
// Music is controlled by game transitions, independently of effects and draws.
type SoundOutput struct {
	Reset, Enabled, MusicEnabled bool
	MusicChanged                 bool
	MusicID                      int
	Events                       []SoundEvent
}

func (w *NativeWorld) requestMusic(id int) {
	w.musicID = id
	w.musicSequence++
}
func (w *NativeWorld) stopAudio() {
	w.audioStopSequence++
	w.requestMusic(-1)
}

// Taking an output twice never repeats commands, including during suspended
// parry steps that retain the previous SoundEvents slice.
func (g *Game) TakeSoundOutput() SoundOutput {
	w, kind := g.World, ""
	if g.Mode == 0 && g.Menu != nil {
		w, kind = g.Menu.World, g.Menu.Kind
	}
	if g.Intro != nil {
		w, kind = g.Intro.World, "intro"
	}
	if g.Story != nil {
		w, kind = g.Story.World, "story"
	}
	if g.Prelude != nil {
		w, kind = g.Prelude.World, "prelude"
	}
	if g.LevelTitle != nil {
		w, kind = g.LevelTitle.World, "level-title"
	}
	if g.Ending != nil {
		w, kind = g.Ending.World, "ending"
	}
	soundTest := kind == "sound"
	out := SoundOutput{Enabled: g.Menu == nil || g.Menu.Effects || soundTest, MusicEnabled: g.Menu == nil || g.Menu.Music || soundTest, MusicID: -1}
	contextChanged := w != g.soundOutputWorld || g.Mode != g.soundOutputMode
	testChanged := soundTest != (g.soundOutputKind == "sound")
	if contextChanged {
		out.Reset = true
		g.soundOutputSequence, g.soundOutputMusicSequence, g.soundOutputStopSequence = 0, 0, 0
	}
	if testChanged {
		out.Reset = true
	}
	g.soundOutputWorld, g.soundOutputKind, g.soundOutputMode = w, kind, g.Mode
	if w == nil {
		return out
	}
	if w.audioStopSequence > g.soundOutputStopSequence {
		out.Reset = true
		g.soundOutputStopSequence = w.audioStopSequence
	}
	if contextChanged || testChanged || w.musicSequence > g.soundOutputMusicSequence {
		out.MusicChanged = true
		out.MusicID = w.musicID
		if soundTest {
			out.MusicID = -1
		} else if testChanged {
			out.MusicID = 26
		}
		g.soundOutputMusicSequence = w.musicSequence
	}
	for _, event := range w.SoundEvents {
		if event.sequence > g.soundOutputSequence {
			out.Events = append(out.Events, event)
			g.soundOutputSequence = event.sequence
		}
	}
	// The original Sound Test stops all sequences before each selected request.
	if soundTest && len(out.Events) != 0 {
		out.Reset = true
	}
	return out
}

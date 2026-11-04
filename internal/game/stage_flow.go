package game

import (
	"aladdin/internal/choreography"
	"aladdin/internal/data"
)

type FlowPhase uint8

const (
	FlowPlaying FlowPhase = iota
	FlowFadeOut
	FlowComplete
	FlowCompleteFade
	FlowBetweenFade
	FlowLoad
	FlowBonusIntro
	FlowBonusIntroFade
	FlowGenieFadeIn
	FlowGenie
	FlowGenieFadeOut
)

type StageFlow struct {
	Phase          FlowPhase
	Age, NextStage int
	GenieBonus     bool
	RenderActors   [32]*choreography.Object
}

func (w *NativeWorld) cacheCompleteActors() {
	for i, o := range w.Objects {
		w.Flow.RenderActors[i] = nil
		if o != nil {
			v := *o
			w.Flow.RenderActors[i] = &v
		}
	}
}

// advanceStage carries the source inventory across a new native stage. The
// story pages run after inventory transfer; book/loading scenes remain pending.
func (g *Game) advanceStage() {
	old := g.World
	if old.Flow.NextStage < 0 {
		g.beginEnding()
		return
	}
	g.Start(old.Flow.NextStage)
	if g.Err != nil {
		return
	}
	w := g.World
	for _, name := range []string{"apples_ascii", "gems_ascii", "lives", "continues", "score", "score_progress", "camera_rate_mode", "difficulty", "invincible", "flag_f521", "genie_tokens"} {
		w.set(name, old.get(name))
	}
	w.RouteCursor = old.RouteCursor
	w.Seed = old.Seed
	// New-level preparation shares the original player initializer with
	// respawn, but retains inventory and starts without damage protection.
	apples := w.get("apples_ascii")
	if abuStage(w.Stage.Index) {
		w.Life = LifeCycle{Phase: LifeFadeIn, FreshStage: true}
		g.beginStory()
		return
	}
	if e := w.respawn(); e != nil {
		g.Err = e
		return
	}
	w.set("apples_ascii", apples)
	w.set("invulnerability", 0)
	g.beginStory()
}

func initialRouteCursor(stage int, route []data.RouteStep) int {
	if stage == 1 {
		return 1
	}
	index := 0
	for i, s := range route {
		if s.Stage == stage && s.Mode != 255 {
			index = i + 1
			if stage != 0 {
				break
			}
		}
	}
	return index
}

// stageBehavior ports the converted stages' ordinary-frame callbacks.
func (w *NativeWorld) stageBehavior() error {
	if abuStage(w.Stage.Index) {
		w.abuBehavior()
	}
	if w.Stage.Index == 12 {
		w.iagoBehavior()
	}
	if w.Stage.Index == 11 {
		w.jafarPalaceBehavior()
	}
	if w.Stage.Index == 10 {
		x, y := w.PlayerPosition()
		if uint16(x) >= 0xc4a && uint16(y) >= 0x496 {
			w.set("exit_countdown", 1)
		}
	}
	if w.Stage.Index == 9 {
		w.lampBehavior()
	}
	if w.Stage.Index == 8 {
		return w.rugBehavior()
	}
	if w.Stage.Index == 7 {
		w.escapeBehavior()
	}
	if w.Stage.Index == 5 {
		w.caveFallingRocks()
	}
	if w.Stage.Index == 0 && w.get("flag_f647") > 0 {
		w.set("flag_f647", w.get("flag_f647")-1)
		if w.get("flag_f647") == 0 {
			w.PaletteBank2 = 0x1517b4
		}
	}
	if w.Stage.Index == 1 {
		x, y := w.PlayerPosition()
		if uint16(x) >= 0x1288 && uint16(y) < 0x1d6 {
			w.set("exit_countdown", 255)
		}
	}
	if w.Stage.Index == 4 {
		for _, event := range []struct {
			flag string
			id   int
		}{{"flag_f63d", 0x38}, {"flag_f64f", 0x45}} {
			if w.flag(event.flag) {
				w.Sound(event.id, false)
				w.set(event.flag, 0)
			}
		}
		x, y := w.PlayerPosition()
		if !w.flag("exit_countdown") && uint16(x) >= 0x8d9 && uint16(y) < 0x1ea {
			w.set("exit_countdown", 255)
		}
	}
	if w.Stage.Index == 0 || w.Stage.Index == 1 {
		if n := w.get("flag_f625"); n > 0 {
			w.set("flag_f625", n-1)
		}
	}
	return nil
}

func (w *NativeWorld) stageExitTrigger() bool {
	n := w.get("exit_countdown")
	if n == 0 {
		return false
	}
	if n != 255 {
		n--
		w.set("exit_countdown", n)
		if n != 0 {
			return false
		}
	}
	w.settleScore()
	w.stopAudio()
	w.Flow = StageFlow{Phase: FlowFadeOut, NextStage: -1, GenieBonus: w.get("genie_tokens") > 0}
	if w.Stage.Index == 10 {
		w.set("background_phase", (w.get("background_phase")+1)&65535)
	}
	return true
}
func (w *NativeWorld) beginLevelComplete() error {
	w.stopAudio()
	w.requestMusic(17)
	vx, vy, facing := w.get("momentum_x"), w.get("momentum_y"), w.get("facing_left")
	w.clearLifeActors()
	w.set("momentum_x", vx)
	w.set("momentum_y", vy)
	w.set("facing_left", facing)
	w.set("invulnerability", 0)
	w.set("flag_f695", 0)
	w.set("tick", 255)
	for _, v := range []struct{ slot, template, x, y int }{{1, 0x1e3b1a, 518, 440}, {2, 0x1e3b06, 458, 440}} {
		if abuStage(w.Stage.Index) && v.slot == 1 {
			continue
		}
		o := choreography.FromTemplate(w.Scripts.Templates[v.template])
		o.SetWord(2, v.x)
		o.SetWord(4, v.y)
		if v.slot == 1 {
			o.Animation = 0x14e7d2
		}
		w.Objects[v.slot] = o
	}
	if e := w.lifeActors(); e != nil {
		return e
	}
	w.Flow.Phase = FlowComplete
	w.Flow.Age = 0
	w.cacheCompleteActors()
	return nil
}
func (w *NativeWorld) updateStageFlow(in Input) error {
	w.SoundEvents = nil
	switch w.Flow.Phase {
	case FlowFadeOut:
		w.Flow.Age++
		if w.Flow.Age == 16 {
			// 1DB5A2/1DBF12 omit both interludes for this stage.
			// 1E1D12 carries its Abu token through the following boss.
			if w.Stage.Index == 10 {
				w.selectNextStage()
				w.clearLifeActors()
				w.Flow.Phase, w.Flow.Age = FlowLoad, 0
				return nil
			}
			return w.beginLevelComplete()
		}
	case FlowBonusIntro:
		if w.Flow.Age >= 300 || w.Flow.Age > 0 && (in.Jump || in.Sword || in.Apple || in.Enter) {
			w.Flow.Phase, w.Flow.Age = FlowBonusIntroFade, 0
			return nil
		}
		w.Flow.Age++
		w.cacheCompleteActors()
		w.set("tick", (w.get("tick")+1)&255)
		return w.lifeActors()
	case FlowBonusIntroFade:
		w.Flow.Age++
		if w.Flow.Age == 16 {
			w.clearLifeActors()
			w.Flow.Phase, w.Flow.Age = FlowLoad, 0
		}
	case FlowGenieFadeIn:
		w.Flow.Age++
		if w.Flow.Age == 16 {
			w.Flow.Phase, w.Flow.Age = FlowGenie, 0
			w.set("flag_f645", 40)
			w.set("flag_f644", 10)
			w.set("flag_f641", 0)
			w.set("flag_f643", 255)
			return w.genieTick(Input{})
		}
	case FlowGenie:
		return w.genieTick(in)
	case FlowGenieFadeOut:
		w.Flow.Age++
		if w.Flow.Age == 16 {
			return w.finishGenie()
		}
	case FlowComplete:
		if w.Flow.Age >= 300 || w.Flow.Age > 0 && (in.Jump || in.Sword || in.Apple || in.Enter) {
			w.Flow.Phase = FlowCompleteFade
			w.Flow.Age = 0
			return nil
		}
		w.Flow.Age++
		// The original submits this sprite list before advancing the scene
		// actors. Keep that display latch while native logic advances.
		w.cacheCompleteActors()
		w.set("tick", (w.get("tick")+1)&255)
		return w.lifeActors()
	case FlowCompleteFade:
		w.Flow.Age++
		if w.Flow.Age == 16 {
			w.Flow.Phase = FlowBetweenFade
			w.Flow.Age = 0
		}
	case FlowBetweenFade:
		w.Flow.Age++
		if w.Flow.Age == 16 {
			if w.Stage.Index != 10 && w.flag("genie_tokens") {
				return w.beginGenie()
			}
			return w.finishGenie()
		}
	}
	return nil
}

// selectNextStage converts the route words consumed at 1D338E. Mode one is
// the optional Abu stage; mode two is skipped after its original completion flag.
func (w *NativeWorld) selectNextStage() {
	switch w.Stage.Index {
	case 10:
		w.set("flag_f521", w.get("flag_f6a8"))
		w.set("flag_f6a8", 0)
	case 12:
		w.set("flag_f6a8", w.get("flag_f521"))
		w.set("flag_f521", 0)
	default:
		w.set("flag_f521", 0)
	}
	w.Flow.NextStage = -1
	for w.RouteCursor < len(w.Scripts.Route) {
		s := w.Scripts.Route[w.RouteCursor]
		w.RouteCursor++
		if s.Mode == 255 {
			return
		}
		if s.Mode == 1 && !w.flag("flag_f6a8") {
			continue
		}
		if s.Mode == 2 && w.flag("flag_f51f") {
			continue
		}
		w.Flow.NextStage = s.Stage
		return
	}
}

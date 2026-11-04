package game

import (
	"aladdin/internal/choreography"
	"strconv"
	"strings"
)

type LifePhase uint8

const (
	LifePlaying LifePhase = iota
	LifeFadeOut
	LifeClear
	LifeDeathScene
	LifeDeathFade
	LifePitWait
	LifeFadeIn
	LifeGameOver
	LifeContinue
	LifeContinueDecision
	LifeContinueFade
	LifeContinueLoad
	LifeGameOverFadeIn
	LifeGameOverCycle
	LifeGameOverBetweenFade
	LifeGameOverFinal
	LifeGameOverFadeOut
	LifeHighScoreFade
	LifeHighScore
	LifeHighScoreExit
)

type LifeCycle struct {
	Phase        LifePhase
	Age          int
	Pit          bool
	FreshStage   bool
	Accepted     bool
	RenderActors [32]*choreography.Object
}
type Checkpoint struct{ X, Y, CameraX, CameraY int }

// checkpointContact is 1D8DB4/1DACB4. Only the saved position uses the
// checkpoint actor's X; the player's current screen position is restored.
func (w *NativeWorld) checkpointContact(o *choreography.Object) {
	if !w.flag("grounded") {
		return
	}
	x, _ := o.Position()
	_, y := w.PlayerPosition()
	w.Checkpoint = Checkpoint{x, y, w.get("camera_x") &^ 15, w.get("camera_y") &^ 15}
	o.Record[0] = 0x92
	o.Record[52] = 255
	o.Animation = 0x14c1e8
	o.AnimationDelay = 0
	w.set("checkpoint_active", 255)
	w.Sound(0x63, false)
}

// deathTrigger follows 1D3414. The ten-tick request can be cancelled by
// invulnerability when it expires; falling below the map bypasses this gate.
func (w *NativeWorld) deathTrigger() bool {
	_, y := w.PlayerPosition()
	pit := y > w.Stage.Height+256
	if !pit {
		n := w.get("death_pending")
		if n == 0 {
			return false
		}
		if n != 255 {
			n--
			w.set("death_pending", n)
			if n != 0 {
				return false
			}
		}
		w.set("death_pending", 0)
		if w.flag("invulnerability") {
			return false
		}
		if o := w.Objects[0]; o != nil {
			o.Record[7] = 0
		}
		if w.Stage.Index == 8 {
			count := (w.get("flag_f520") + 1) & 255
			w.set("flag_f520", count)
			if count >= 3 {
				w.set("exit_countdown", 255)
			}
			return false
		}
	}
	w.stopAudio()
	w.Life = LifeCycle{Phase: LifeFadeOut, Pit: pit}
	return true
}

func (w *NativeWorld) clearLifeActors() {
	for _, o := range w.Objects {
		if o == nil || !o.Alive() {
			continue
		}
		// FF is a consumed checkpoint marker, not a constructor to restore.
		if o.Record[6]&32 != 0 && o.Record[52] != 0 && o.Record[52] != 255 {
			w.Activated[int(uint16(o.Word(50)))] = false
		}
	}
	w.Objects = [32]*choreography.Object{}
	w.VisibleFrame = nil
	w.ArtworkLeaks = [116]bool{}
	w.set("momentum_x", 0)
	w.set("momentum_y", 0)
	w.set("facing_left", 0)
	w.parryTarget = nil
	w.lastSwordTarget = nil
	w.ParryFlash = false
}
func (w *NativeWorld) settleScore() {
	w.set("new_high_score", 0)
	for w.get("pending_score") > 0 {
		w.addScorePoint()
	}
	if w.get("score") > w.get("high_score") {
		w.set("high_score", w.get("score"))
		w.set("new_high_score", 255)
	}
}
func (w *NativeWorld) lifeActors() error {
	c := choreography.Choreographer{Scripts: w.Scripts}
	for _, o := range w.Objects {
		if o == nil || !o.Alive() || o.Frame < 0 {
			continue
		}
		w.objectMomentum(o)
		if e := c.Motion(w, o); e != nil {
			return e
		}
	}
	if w.get("tick")&1 != 0 {
		for _, o := range w.Objects {
			if o != nil && o.Alive() {
				if e := c.Animation(w, o); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func (w *NativeWorld) updateLifeCycle(in Input) error {
	w.SoundEvents = nil
	switch w.Life.Phase {
	case LifeHighScoreFade, LifeHighScore, LifeHighScoreExit:
		return w.updateHighScore()
	case LifeGameOverFadeIn, LifeGameOverCycle, LifeGameOverBetweenFade, LifeGameOverFinal, LifeGameOverFadeOut:
		return w.updateGameOver(in)
	case LifeContinue, LifeContinueDecision, LifeContinueFade:
		return w.updateContinue(in)
	case LifeFadeOut:
		w.Life.Age++
		if w.Life.Age == 16 {
			vx, vy, facing := w.get("momentum_x"), w.get("momentum_y"), w.get("facing_left")
			w.clearLifeActors()
			if w.Life.Pit {
				// Pit death deletes actors without zeroing their record storage.
				w.set("momentum_x", vx)
				w.set("momentum_y", vy)
				w.set("facing_left", facing)
				w.settleScore()
				w.Life.Phase = LifePitWait
				w.Life.Age = 0
			} else {
				w.set("tick", 255)
				w.Life.Phase = LifeClear
				w.Life.Age = 0
			}
		}
	case LifeClear:
		for _, v := range []struct{ slot, template, x, y int }{{1, 0x1e2e22, 362, 394}, {2, 0x1e2e0e, 288, 376}} {
			o := choreography.FromTemplate(w.Scripts.Templates[v.template])
			o.SetWord(2, v.x)
			o.SetWord(4, v.y)
			w.Objects[v.slot] = o
		}
		w.settleScore()
		w.set("tick", 0)
		w.Life.Phase = LifeDeathScene
		w.Life.Age = 0
	case LifeDeathScene:
		// 1DD046 executes 260 scene updates and polls A/B/C/Start only
		// after the first 50. Polling follows the completed video frame.
		if w.Life.Age >= 259 || w.Life.Age >= 50 && (in.Jump || in.Sword || in.Apple || in.Enter) {
			return w.finishDeathScene()
		}
		w.Life.Age++
		w.set("tick", (w.get("tick")+1)&255)
		return w.lifeActors()
	case LifeDeathFade:
		w.Life.Age++
		if w.Life.Age == 16 {
			return w.respawn()
		}
	case LifePitWait:
		w.Life.Age++
		if w.Life.Age == 36 {
			return w.finishDeathScene()
		}
	case LifeFadeIn:
		w.Life.Age++
		if w.Life.Age == 16 {
			fresh := w.Life.FreshStage
			w.Life = LifeCycle{}
			if fresh {
				return nil
			}
			// Resume the remainder of the interrupted game tick; ordinary player
			// input begins on the next update, as in the source.
			if err := w.stageBehavior(); err != nil {
				return err
			}
			if w.stageExitTrigger() {
				return nil
			}
			w.updateScore()
			if w.get("tick")&1 != 0 {
				c := choreography.Choreographer{Scripts: w.Scripts}
				last := 0
				if w.Stage.Index == 12 || abuStage(w.Stage.Index) {
					last = len(w.Objects) - 1
				}
				for i := 0; i <= last; i++ {
					if o := w.Objects[i]; o != nil && o.Alive() {
						if e := c.Animation(w, o); e != nil {
							return e
						}
					}
				}
			}
			w.updateHUDWave()
		}
	}
	return nil
}
func (w *NativeWorld) finishDeathScene() error {
	if w.get("lives") == 0 {
		if w.flag("new_high_score") {
			w.set("new_high_score", 0)
			w.Life.Phase = LifeHighScoreFade
			w.Life.Age = 0
			return nil
		}
		if w.get("continues") != 0 {
			return w.beginContinue()
		}
		return w.beginGameOver()
	}
	if !w.flag("invincible") {
		w.set("lives", w.get("lives")-1)
	}
	w.Life.Phase = LifeDeathFade
	w.Life.Age = 0
	return nil
}

// respawn reconstructs gameplay state at the saved position. Native loading
// replaces the console's decompression and tile-streaming preparation; that
// loading interval is not yet reference-timed.
func (w *NativeWorld) respawn() error {
	w.requestMusic(w.Stage.Music)
	w.clearLifeActors()
	for name := range w.Variables {
		if strings.HasPrefix(name, "flag_") {
			a, e := strconv.ParseUint(strings.TrimPrefix(name, "flag_"), 16, 16)
			if e == nil && a >= 0xf524 && a <= 0xf659 {
				delete(w.Variables, name)
			}
		}
	}
	for _, name := range strings.Fields("carpet_target_x carpet_target_y background_phase rope_touch rope_top rail_touch rope_forced rope_cooldown floor_disabled previous_standing_object ceiling death_pending exit_countdown invulnerability input_left input_right input_up input_down run_speed jumping jump_hold jump_released grounded walking crouching looking_up action_locked on_object standing_object climbing hanging fall_ticks on_camel stopping sword_contact sword_combo apple_combo action_cooldown apple_step apple_window sword_step sword_window apple_cooldown landing_combo wall_left wall_right camera_delay") {
		w.set(name, 0)
	}
	w.Objects[0] = choreography.FromTemplate(w.Scripts.Templates[0x1e2dfa])
	w.set("sprite_shake_x", 0)
	if w.Stage.Index == 5 {
		w.set("flag_f633", 1)
	}
	if w.Stage.Index == 9 {
		w.set("flag_f5c0", 1)
	}
	w.set("player_x", w.Checkpoint.X)
	w.set("player_y", w.Checkpoint.Y)
	w.set("momentum_x", 0)
	w.set("momentum_y", 0)
	w.set("facing_left", 0)
	cameraX, preparationDraws := w.preparedCamera()
	// 1D4D0A's preparation sweeps and the resumed stage draw consume an
	// unfinished shake. Death fades leave that cursor frozen.
	for i := 0; i <= preparationDraws; i++ {
		w.advanceScreenShake()
	}
	w.set("camera_x", cameraX)
	if w.Stage.Index <= 2 {
		w.set("background_phase", (-preparationDraws-1)&65535)
	} else if w.Stage.Index == 3 || w.Stage.Index == 10 || w.Stage.Index == 13 {
		w.set("background_phase", (preparationDraws+1)&65535)
	}
	w.set("camera_y", w.Checkpoint.CameraY)
	w.set("camera_tile_x", w.get("camera_x")&^15)
	w.set("camera_tile_y", w.get("camera_y")&^15)
	w.set("health", 8)
	w.set("tick", 255)
	w.set("grounded", 255)
	w.set("jump_hold", 255)
	w.set("invulnerability", 40)
	apples := 0x3130
	if w.get("difficulty") == 0 {
		apples = 0x3135
	} else if w.get("difficulty") > 1 {
		apples = 0x3035
	}
	w.set("apples_ascii", apples)
	w.sourceAnimation(0x1494aa)
	if w.flag("checkpoint_active") {
		w.sourceAnimation(0x14e336)
	}
	w.syncPlayer()
	if w.Stage.Index == 8 {
		w.initializeRugRide()
		if err := w.prepareRugRide(); err != nil {
			return err
		}
	}
	if abuStage(w.Stage.Index) {
		if err := w.prepareAbu(); err != nil {
			return err
		}
		if err := w.moveActors(); err != nil {
			return err
		}
		w.Life = LifeCycle{Phase: LifeFadeIn}
		return nil
	}
	if w.Stage.Index == 12 {
		w.initializeIago()
		if err := w.prepareScene(); err != nil {
			return err
		}
		w.Life = LifeCycle{Phase: LifeFadeIn}
		return nil
	}
	w.LastActivationX = -1
	w.LastActivationY = -1
	if e := w.activateMarkers(); e != nil {
		return e
	}
	c := choreography.Choreographer{Scripts: w.Scripts}
	for _, o := range w.Objects {
		if o != nil && o.Alive() && w.Stage.Index != 8 {
			if e := c.Animation(w, o); e != nil {
				return e
			}
		}
	}
	w.Life = LifeCycle{Phase: LifeFadeIn}
	return nil
}

// preparedCamera follows the two 23-step map-preparation sweeps at 1D4D0A.
// Native image loading does not need their console CPU time, but their final
// camera and background phase remain part of the saved gameplay state.
func (w *NativeWorld) preparedCamera() (int, int) {
	if w.flag("flag_f6a3") {
		return w.Checkpoint.CameraX, 0
	}
	x, draws := w.Checkpoint.CameraX, 0
	for i := 0; i < 23; i++ {
		if x+16 < w.Stage.Width-0x161 {
			x += 16
			draws++
		}
	}
	for i := 0; i < 23; i++ {
		if x >= 17 {
			x -= 16
			draws++
		}
	}
	return x, draws
}

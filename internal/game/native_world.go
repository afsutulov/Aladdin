package game

import (
	"aladdin/internal/choreography"
	"aladdin/internal/data"
	"fmt"
)

// NativeWorld is the reconstructed game state. Variables are game properties,
// not console RAM, and every state transition below is ordinary Go code.
type NativeWorld struct {
	BufferedPresentation, FirstStageUpdate bool
	Scripts                                *data.ScriptSet
	Stage                                  *data.Stage
	Objects                                [32]*choreography.Object
	Variables                              map[string]uint32
	Seed                                   uint32
	SoundEvents                            []SoundEvent
	soundSequence                          uint64
	musicID                                int
	musicSequence, audioStopSequence       uint64
	VideoFrames                            uint64
	StageActors                            bool
	Activated                              map[int]bool
	LastActivationX, LastActivationY       int
	Frames                                 []data.Frame
	parryTarget                            *choreography.Object
	resumeContact                          int
	ParryFlash                             bool
	HUDWaveIndex, HUDWaveTile              int
	lastSwordTarget                        *choreography.Object
	Life                                   LifeCycle
	Checkpoint                             Checkpoint
	Flow                                   StageFlow
	RouteCursor                            int
	PaletteOverrides                       map[int][3]uint8
	PaletteBank2                           int
	ArtworkLeaks                           [116]bool
	RemovedCells                           map[int]bool
	VisibleFrame                           *NativeWorld
}
type SoundEvent struct {
	ID int
	// Overlay is the original stop-only command (1D70E4), not a second voice.
	Overlay  bool
	sequence uint64
}

func NewNativeWorld(s *data.Stage, scripts *data.ScriptSet, frames ...[]data.Frame) *NativeWorld {
	w := &NativeWorld{Scripts: scripts, Stage: s, musicID: -1, Variables: map[string]uint32{}, Activated: map[int]bool{}, Seed: 12345678}
	t := scripts.Templates[0x1e2dfa]
	w.Objects[0] = choreography.FromTemplate(t)
	w.set("player_x", s.SpawnX)
	w.set("player_y", s.SpawnY+256)
	w.set("health", 8)
	w.set("lives", 3)
	w.set("high_score", 100000)
	w.set("continues", 1)
	w.set("max_health", 8)
	w.set("difficulty", 1)
	w.set("screen_shake_cursor", 0x76a0)
	if s.Index == 5 {
		w.set("flag_f633", 1)
	}
	if s.Index == 9 {
		w.set("flag_f5c0", 1)
	}
	if len(frames) > 0 {
		w.Frames = frames[0]
	}
	w.set("apples_ascii", 0x3130)
	w.set("gems_ascii", 0x3030)
	w.set("look_target_x", s.SpawnX-s.CameraX)
	w.set("look_target_y", s.SpawnY+256-s.CameraY)
	w.set("camera_x", s.CameraX+16)
	w.set("camera_y", s.CameraY)
	w.set("camera_tile_x", (s.CameraX+16)&^15)
	w.set("camera_tile_y", s.CameraY&^15)
	w.Checkpoint = Checkpoint{X: s.SpawnX, Y: s.SpawnY + 256, CameraX: s.CameraX &^ 15, CameraY: s.CameraY &^ 15}
	w.RouteCursor = initialRouteCursor(s.Index, scripts.Route)
	if s.Index == 8 {
		w.initializeRugRide()
	}
	w.syncPlayer()
	return w
}
func (w *NativeWorld) get(s string) int {
	if s == "stage_index" {
		return w.Stage.Index
	}
	if o := w.Objects[0]; o != nil {
		switch s {
		case "momentum_x":
			return int(uint16(o.Word(24)))
		case "momentum_y":
			return int(uint16(o.Word(26)))
		case "facing_left":
			return int(o.Record[9])
		}
	}
	return int(w.Variables[s])
}
func (w *NativeWorld) set(s string, v int) {
	w.Variables[s] = uint32(v)
	if o := w.Objects[0]; o != nil {
		switch s {
		case "momentum_x":
			o.SetWord(24, v)
		case "momentum_y":
			o.SetWord(26, v)
		case "facing_left":
			o.Record[9] = byte(v)
		}
	}
}
func (w *NativeWorld) flag(s string) bool { return w.get(s) != 0 }
func (w *NativeWorld) PlayerPosition() (int, int) {
	return int(int16(w.get("player_x"))), int(int16(w.get("player_y")))
}
func (w *NativeWorld) syncPlayer() {
	o := w.Objects[0]
	if o == nil {
		return
	}
	x, y := w.PlayerPosition()
	o.SetWord(2, x)
	o.SetWord(4, y)
	o.SetWord(24, w.get("momentum_x"))
	o.SetWord(26, w.get("momentum_y"))
	o.Record[9] = byte(w.get("facing_left"))
}
func (w *NativeWorld) ReadVariable(name string, width int) (uint32, error) {
	// Some choreography tests all four adjacent direction bytes together.
	// Preserve their big-endian word/long aliases in the native properties.
	directions := []string{"input_right", "input_left", "input_up", "input_down"}
	for i, direction := range directions {
		if name == direction && width > 1 && i+width <= len(directions) {
			var v uint32
			for j := 0; j < width; j++ {
				v = v<<8 | uint32(w.get(directions[i+j])&255)
			}
			return v, nil
		}
	}
	v := uint32(w.get(name))
	if width == 1 && (name == "momentum_x" || name == "momentum_y" || name == "run_speed" || name == "player_x" || name == "player_y" || name == "apples_ascii" || name == "gems_ascii") {
		v >>= 8
	}
	if width == 1 {
		v &= 255
	} else if width == 2 {
		v &= 65535
	}
	return v, nil
}
func (w *NativeWorld) WriteVariable(name string, width int, v uint32) error {
	if width == 1 && (name == "momentum_x" || name == "momentum_y" || name == "run_speed" || name == "player_x" || name == "player_y" || name == "apples_ascii" || name == "gems_ascii") {
		v = (v&255)<<8 | uint32(w.get(name))&255
	} else if width == 1 {
		v &= 255
	} else if width == 2 {
		v &= 65535
	}
	w.set(name, int(v))
	return nil
}
func (w *NativeWorld) Random() uint16 {
	w.Seed = w.Seed*13 + 7
	return uint16(w.Seed) ^ uint16(w.Seed>>16)
}
func (w *NativeWorld) Sound(id int, overlay bool) {
	w.soundSequence++
	w.SoundEvents = append(w.SoundEvents, SoundEvent{ID: id, Overlay: overlay, sequence: w.soundSequence})
}
func (w *NativeWorld) Delete(o *choreography.Object, cascade bool) {
	o.Record[0] = 0
	if cascade && o.Partner != nil {
		o.Partner.Record[0] = 0
	}
	if o.Partner != nil {
		if !cascade {
			o.Partner.Record[60] &^= 4
		}
		o.Partner.Partner = nil
		o.Partner = nil
	}
}
func (w *NativeWorld) Spawn(parent *choreography.Object, n data.Instruction) error {
	_, ok := w.Scripts.Templates[n.Template]
	if !ok {
		return fmt.Errorf("unconverted object template %06x", n.Template)
	}
	index := -1
	switch n.Mode {
	case 0:
		for i := 3; i <= 22; i++ {
			if w.Objects[i] == nil || !w.Objects[i].Alive() {
				index = i
				break
			}
		}
	case 3:
		for i := 25; i < 32; i++ {
			if w.Objects[i] == nil || !w.Objects[i].Alive() {
				index = i
				break
			}
		}
	case 1, 5:
		for i := 1; i <= 24; i++ {
			if w.Objects[i] == nil || !w.Objects[i].Alive() {
				index = i
				break
			}
		}
	case 2, 6:
		for i := 20; i >= 1; i-- {
			if w.Objects[i] == nil || !w.Objects[i].Alive() {
				index = i
				break
			}
		}
	case 4:
		if w.Objects[0] == nil || !w.Objects[0].Alive() {
			index = 0
		}
	default:
		return fmt.Errorf("unknown original spawn pool %d", n.Mode)
	}
	if index < 0 {
		return nil
	} // Original fixed actor pools also drop this spawn.
	o := w.actorTemplate(n.Template, w.Objects[index])
	x, y := parent.Position()
	dx, dy := n.OffsetX, n.OffsetY
	if parent.Record[9] != 0 {
		dx = -dx
	}
	if parent.Record[53] != 0 {
		dy = -dy
	}
	o.SetWord(2, x+dx)
	o.SetWord(4, y+dy)
	o.Record[9] = parent.Record[9]
	o.Record[53] = parent.Record[53]
	if n.Animation != 0 {
		o.Animation = n.Animation
	}
	if n.Motion != 0 {
		o.Motion = n.Motion
	}
	if n.Mode == 5 || n.Mode == 6 {
		parent.Record[60] |= 4
		parent.Partner = o
		o.Partner = parent
	}
	w.Objects[index] = o
	return nil
}
func (w *NativeWorld) Callback(name string, o *choreography.Object) (int, error) {
	switch name {
	case "native_1dbe72":
		w.PaletteBank2 = 0x152294
		w.set("flag_f624", 255)
	case "native_1dbe88":
		w.PaletteBank2 = 0x152274
		w.set("flag_f624", 0)
	case "hide":
		o.Record[7] |= 32
	case "show":
		o.Record[7] &^= 32
	case "solid":
		o.Record[6] |= 16
	case "non_solid":
		o.Record[6] &^= 16
	case "set_attack_flag":
		o.Record[60] |= 32
	case "clear_attack_flag":
		o.Record[60] &^= 32
	case "native_1d7246":
		o.Record[60] &^= 16
	case "native_1d724e":
		o.Record[6] |= 64
	case "native_1d731c":
		o.Record[6] |= 1
	case "native_1d734a":
		o.Record[6] &^= 1
	case "native_1d7324":
		if e := w.replaceActor(o, 0x1e3516); e != nil {
			return 0, e
		}
		o.Record[0] = 0x8c
	case "native_1d74ec":
		w.releaseScenery(0x147de2)
	case "native_1d754c":
		w.releaseScenery(0x147dea)
	case "native_1d72bc":
		if o.Partner != nil {
			x, y := o.Partner.Position()
			o.SetWord(2, x)
			o.SetWord(4, y)
		}
	case "native_1d72d6":
		x, _ := o.Position()
		px, _ := w.PlayerPosition()
		if (x >= px) == w.flag("facing_left") {
			return 0x14dacc, nil
		}
	case "random_scatter":
		o.Record[24] = byte(int(w.Random()&15) - 7)
		o.Record[26] = byte(int(w.Random()&15) - 15)
	case "random_guard_voice":
		w.Sound(0x5e+int(w.Random()&3), false)
	case "native_1d73f6":
		id := 0x40
		if w.Random()&1 != 0 {
			id = 0x44
		}
		w.Sound(id, false)
	case "consume_apple":
		v := w.get("apples_ascii")
		if v != 0x3030 {
			ones := byte(v) - 1
			tens := byte(v >> 8)
			if ones < '0' {
				ones = '9'
				if tens != '0' {
					tens--
				}
			}
			w.set("apples_ascii", int(tens)<<8|int(ones))
		}
	case "parry_impulse_xy":
		w.set("momentum_y", int(uint16(w.get("momentum_y")-0x82)))
		fallthrough
	case "parry_impulse_x":
		v := int(int16(w.get("momentum_x")))
		if w.flag("facing_left") {
			v += 0xa0
		} else {
			v -= 0xa0
		}
		w.set("momentum_x", int(uint16(v)))
	case "attack_if_player_right", "attack_if_player_left":
		px, _ := w.PlayerPosition()
		x, _ := o.Position()
		set := x < px
		if name == "attack_if_player_left" {
			set = !set
		}
		if set {
			o.Record[60] |= 32
		} else {
			o.Record[60] &^= 32
		}
	case "idle_sword_combo":
		if w.flag("sword_window") && w.flag("sword_step") {
			i := w.get("sword_step") - 1
			if i < 6 {
				w.set("sword_combo", 255)
				w.set("action_cooldown", (w.get("action_cooldown")+4)&255)
				return w.Scripts.Entry[fmt.Sprintf("combo_sword_idle_%d", i)], nil
			}
		}
	case "moving_attack_combo":
		if w.flag("sword_window") && w.flag("sword_step") {
			i := w.get("sword_step") - 1
			if i < 6 {
				w.set("sword_combo", 255)
				return w.Scripts.Entry[fmt.Sprintf("combo_sword_moving_%d", i)], nil
			}
		}
		if w.flag("apple_window") && w.flag("apple_step") {
			i := w.get("apple_step") - 1
			if i < 6 {
				w.set("apple_combo", 255)
				return w.Scripts.Entry[fmt.Sprintf("combo_apple_moving_%d", i)], nil
			}
		}
	case "native_1e2b28":
		kind := byte(w.get("flag_f522"))
		count := 0
		for i := 1; i <= 24; i++ {
			if v := w.Objects[i]; v != nil && v.Record[0] == kind {
				count++
			}
		}
		w.set("flag_f522", count)
	case "native_1e25d8":
		w.set("flag_f636", 0)
	case "native_1e24d2":
		x, _ := o.Position()
		px, _ := w.PlayerPosition()
		if (uint16(x) >= uint16(px)) == (o.Record[9] == 0) {
			return 0x14d2f6, nil
		}
		return 0x14d2b2, nil
	case "native_1e0038":
		w.steerCarpet(o, 3, true)
	case "native_1e00c4":
		w.steerCarpet(o, 1, false)
	case "native_1e012e":
		x, y := o.Position()
		w.set("carpet_target_x", (x-32)&65535)
		w.set("carpet_target_y", (y+64)&65535)
	case "native_1dfa8e":
		w.set("screen_shake_cursor", 0x7680)
	case "native_1dfa9a":
		w.set("screen_shake_cursor", 0x7694)
	case "native_1dfaa6":
		w.set("screen_shake_cursor", 0x769c)
	case "native_1dfab2":
		old := w.get("flag_f633")
		for {
			n := int(w.Random() & 1)
			if n != old {
				w.set("flag_f633", n)
				break
			}
		}
	case "native_1d75ac", "native_1d75e8", "native_1d7624", "native_1d7660", "native_1d769c", "native_1d76d8":
		v := map[string]struct {
			kind   byte
			motion int
		}{
			"native_1d75ac": {0x75, 0x1478d4}, "native_1d75e8": {0x76, 0x147944},
			"native_1d7624": {0x77, 0x1479b4}, "native_1d7660": {0x78, 0x147a5e},
			"native_1d769c": {0x79, 0x147bb8}, "native_1d76d8": {0x7a, 0x147c28},
		}[name]
		kind, motion := v.kind, v.motion
		for i := 1; i <= 24; i++ {
			if v := w.Objects[i]; v != nil && v.Record[0] == kind {
				return 0, nil
			}
		}
		if slot := w.markerSlot(1); slot >= 0 {
			v := w.actorTemplate(0x1e2e9a, w.Objects[slot])
			v.Record[0], v.Motion = kind, motion
			x, y := o.Position()
			v.SetWord(2, x)
			v.SetWord(4, y)
			w.Objects[slot] = v
		}
	case "native_1d7714":
		if o.Partner != nil {
			w.Delete(o.Partner, false)
		}
	default:
		return 0, fmt.Errorf("native behaviour callback still requires porting: %s", name)
	}
	return 0, nil
}
func (w *NativeWorld) ResumePlayer(o *choreography.Object) (int, error) {
	w.set("action_locked", 0)
	if w.flag("flag_f6a3") {
		if !w.flag("grounded") {
			return 0x1491c4, nil
		}
		if w.get("run_speed") == 1 || w.get("run_speed") == 2 {
			return 0x149708, nil
		}
		w.set("walking", 0)
		return 0x14947c, nil
	}
	if w.flag("hanging") {
		return 0x148f00, nil
	}
	if w.flag("on_camel") {
		return 0x14e556, nil
	}
	if w.flag("rope_forced") {
		return 0x148db6, nil
	}
	if w.flag("climbing") {
		return w.Scripts.Entry[fmt.Sprintf("rope_pose_%d", (w.get("player_y")>>2)&15)], nil
	}
	if w.get("standing_object") == 0x65 {
		return 0x149a70, nil
	}
	if w.flag("grounded") {
		if w.flag("crouching") {
			return 0x149a58, nil
		}
		if w.flag("looking_up") {
			return 0x1499d2, nil
		}
		if w.flag("stopping") {
			return 0x1496ce, nil
		}
		if w.get("run_speed") == 1 || w.get("run_speed") == 2 {
			p, e := w.Callback("moving_attack_combo", o)
			if p != 0 || e != nil {
				return p, e
			}
			return 0x14973a, nil
		}
		p, e := w.Callback("idle_sword_combo", o)
		if p != 0 || e != nil {
			return p, e
		}
		return 0x1494bc, nil
	}
	return 0x149074, nil
}
func (w *NativeWorld) animation(name string) {
	o := w.Objects[0]
	o.Animation = w.abuAnimation(w.Scripts.Entry[name])
	o.AnimationDelay = 0
}
func (w *NativeWorld) sourceAnimation(label int) {
	w.Objects[0].Animation = w.abuAnimation(label)
	w.Objects[0].AnimationDelay = 0
}

// playerFloor follows the original three contour probes at 1D7F3A..1D8268.
// The actor's Y includes the original +256 bias; its contour probe is 32px
// below the visual origin. Treating Y as the character's feet was incorrect.
func (w *NativeWorld) playerFloor() {
	if w.Stage.Index == 8 || w.flag("on_object") {
		return
	}
	x, y := w.PlayerPosition()
	physicalY := y - 256
	if physicalY < 0 || physicalY >= w.Stage.Height-32 {
		w.noFloor()
		return
	}
	row := (physicalY >> 4) + 1
	col := (x + 16) >> 4
	nibble := x & 15
	probeY := y - 16
	height := 0
	for i := 0; i < 3; i++ {
		if row+i < 0 || row+i >= w.Stage.Height/16 || col < 0 || col >= w.Stage.Width/16 {
			// 1D7FA0/1D7FD8/1D800E take the gravity path before
			// clearing the previous support flag at the room boundary.
			w.noFloor()
			return
		}
		o := int(w.Stage.Room[(row+i)*(w.Stage.Width/16)+col]) / 2
		if o >= len(w.Stage.Floor) {
			break
		}
		// Surface 47 selects the other contour in each terrain record.
		// Ordinary actors still use the first contour, as in the source.
		f := int(w.Stage.Floor[o+(w.get("flag_f5c0")&1)])
		xx := nibble
		if i == 2 {
			xx = 2
		}
		p := f*16 + xx
		if p >= len(w.Stage.Contours) {
			break
		}
		height = int(w.Stage.Contours[p] & 63)
		if height != 0 {
			break
		}
		probeY += 16
	}
	if height == 0 {
		w.set("grounded", 0)
		w.noFloor()
		return
	}
	if w.flag("floor_disabled") {
		w.noFloor()
		return
	}
	w.set("grounded", height)
	target := probeY&^15 | (height - 1)
	if y < target {
		if w.flag("jumping") && !w.flag("jump_released") {
			w.set("grounded", 255)
			return
		}
		if target-y >= 8 {
			w.set("grounded", 0)
			w.noFloor()
			return
		}
		y = target
	} else if y > target {
		if y-target > 8 {
			w.set("grounded", 0)
			w.noFloor()
			return
		}
		y = target
	}
	w.set("player_y", y)
	if w.flag("jumping") && !w.flag("jump_released") {
		w.set("grounded", 0)
		return
	}
	if !w.flag("jumping") && w.flag("fall_ticks") {
		n := w.get("fall_ticks")
		w.set("fall_ticks", 0)
		w.set("landing_combo", 0)
		if n >= 0x28 {
			w.set("momentum_y", 0)
			w.set("look_target_x", 0xa0)
			w.set("look_target_y", 0x188)
			w.sourceAnimation(0x149152)
			w.set("walking", 0)
			w.set("run_speed", 0)
			return
		}
		w.land()
	} else if w.flag("jumping") {
		w.land()
	}
}
func (w *NativeWorld) land() {
	w.set("jumping", 0)
	w.set("momentum_y", 0)
	if !w.flag("input_left") && !w.flag("input_right") {
		w.set("walking", 0)
	}
	if w.flag("action_locked") {
		return
	}
	if !w.flag("walking") || w.get("run_speed") == 0 {
		w.animation("land")
	} else {
		w.sourceAnimation(0x1497ba)
	}
	w.set("action_locked", 0)
}
func (w *NativeWorld) noFloor() {
	if w.flag("jumping") && !w.flag("jump_released") {
		return
	}
	if w.flag("climbing") || w.flag("hanging") || w.flag("rope_forced") {
		return
	}
	v := uint16(w.get("momentum_y") + 0x78)
	if v < 0x800 {
		w.set("momentum_y", int(v))
	}
	if w.get("fall_ticks") < 255 {
		w.set("fall_ticks", w.get("fall_ticks")+1)
	}
	if w.get("fall_ticks") == 0x28 {
		w.Objects[0].Animation, w.Objects[0].AnimationDelay = 0x149074, 0
	}
}
func (w *NativeWorld) momentum() {
	if w.Objects[0] == nil || !w.Objects[0].Alive() {
		return
	}
	x, y := w.PlayerPosition()
	vx, vy := int(int16(w.get("momentum_x"))), int(int16(w.get("momentum_y")))
	if vx > 0 {
		if vx >= 0x28 && !w.flag("wall_right") {
			if x-w.get("camera_x") < 0x130 {
				x += vx >> 8
			}
			vx -= 0x28
		} else {
			vx = 0
		}
	} else if vx < 0 {
		if -vx >= 0x28 && !w.flag("wall_left") && x-w.get("camera_x") >= 0x14 {
			x += vx >> 8
			vx += 0x28
		} else {
			vx = 0
		}
	}
	if vy > 0 {
		if vy >= 0x3c {
			y += vy >> 8
			vy -= 0x3c
		} else {
			vy = 0
			w.set("jump_released", 255)
		}
	} else if vy < 0 {
		if -vy >= 0x3c {
			if y-w.get("camera_y") < 0x14 {
				vy += 0x3c
			} else if w.flag("ceiling") {
				w.set("jumping", 0)
				vy = 0
				w.set("jump_released", 255)
			} else {
				y += vy >> 8
				vy += 0x3c
			}
		} else {
			vy = 0
			w.set("jump_released", 255)
		}
	}
	w.set("player_x", x)
	w.set("player_y", y)
	w.set("momentum_x", int(uint16(vx)))
	w.set("momentum_y", int(uint16(vy)))
}
func (w *NativeWorld) sampleDirections(in Input) {
	for name, on := range map[string]bool{"input_right": in.Right, "input_left": in.Left, "input_up": in.Up, "input_down": in.Down} {
		v := 0
		if on {
			v = 255
		}
		w.set(name, v)
	}
}
func (w *NativeWorld) controls(in Input) {
	w.sampleDirections(in)
	if w.Stage.Index == 8 {
		if !w.flag("death_pending") && !w.flag("action_locked") {
			w.rugControls(in)
		}
		for _, key := range []string{"action_cooldown", "sword_window", "apple_window"} {
			if w.get(key) > 0 {
				w.set(key, w.get(key)-1)
			}
		}
		return
	}
	if !w.flag("action_locked") && !w.flag("rope_forced") && !w.flag("rope_cooldown") && !w.flag("action_cooldown") {
		x, y := w.PlayerPosition()
		if !w.flag("death_pending") && !w.flag("exit_countdown") {
			if !w.flag("wall_left") && !w.flag("wall_right") {
				w.set("stopping", 0)
			}
			carpet := w.flag("on_object") && w.get("standing_object") == 0x65
			stance := !(in.Right || in.Left) || w.flag("crouching") || w.flag("looking_up") || carpet
			if in.Right || in.Left {
				left := in.Left && !in.Right
				walking := 1
				if left {
					walking = 255
					w.set("facing_left", 255)
				} else {
					w.set("facing_left", 0)
				}
				blocked := (left && w.flag("wall_left")) || (!left && w.flag("wall_right"))
				if carpet {
					// The original keeps the rider at the carpet's origin.
				} else if blocked {
					w.blockedWalk()
				} else if !w.flag("crouching") && !w.flag("looking_up") && !w.flag("climbing") && ((left && x-w.get("camera_x") >= 0x14) || (!left && x-w.get("camera_x") < 0x130)) {
					dx, handled := w.actorWalk(left)
					if handled {
						x += dx
					} else {
						if w.flag("stopping") {
							w.set("stopping", 0)
						}
						if !w.flag("jumping") && w.flag("grounded") {
							if w.get("walking") != walking {
								w.set("run_speed", 3)
								w.animation("run")
								w.set("walking", walking)
								if left {
									w.set("look_target_x", 0xd0)
								} else {
									w.set("look_target_x", 0x70)
								}
								w.set("camera_delay", 7)
								_, _ = w.Callback("moving_attack_combo", w.Objects[0])
							}
						}
						v := w.get("run_speed")
						if v == 0 {
							v = 2
						}
						if left {
							x -= v
						} else {
							x += v
						}
					}
				}
			}
			if w.flag("climbing") {
				w.set("stopping", 0)
				w.set("walking", 0)
				w.set("run_speed", 0)
			}
			if stance && !w.flag("climbing") {
				if !w.flag("flag_f6a3") && in.Down && w.flag("grounded") && !w.flag("jumping") && w.get("momentum_y") == 0 {
					if !w.flag("crouching") {
						w.animation("crouch")
						if carpet {
							w.sourceAnimation(0x149adc)
						}
						w.set("crouching", 255)
						w.set("walking", 0)
						w.set("stopping", 0)
					}
				} else if w.flag("crouching") {
					w.set("crouching", 0)
					w.set("look_target_y", 0x178)
				}
				if !w.flag("flag_f6a3") && in.Up && w.flag("grounded") && !w.flag("jumping") && !carpet && w.get("momentum_y") == 0 {
					if !w.flag("looking_up") {
						w.animation("look_up")
						w.set("looking_up", 255)
						w.set("walking", 0)
						w.set("stopping", 0)
					}
				} else if w.flag("looking_up") {
					w.set("looking_up", 0)
					w.set("look_target_y", 0x178)
				}
				if w.flag("grounded") && !w.flag("jumping") && !w.flag("crouching") && !w.flag("looking_up") {
					// 1D46C6 skips the landing combo while supported by an
					// actor or while either attack-combo window is active.
					if w.flag("on_object") || w.flag("sword_window") && w.flag("sword_step") || w.flag("apple_window") && w.flag("apple_step") {
						w.set("landing_combo", 0)
					}
					if w.flag("landing_combo") {
						w.sourceAnimation(0x14ae64)
						w.set("landing_combo", 0)
						w.set("walking", 0)
						w.set("stopping", 0)
					} else {
						w.set("run_speed", 0)
						if w.flag("stopping") {
							w.sourceAnimation(0x1496c8)
							w.set("stopping", 0)
							w.set("look_target_y", 0x188)
						} else if w.flag("walking") && !w.flag("sword_combo") && !w.flag("apple_combo") {
							w.animation("idle")
							w.set("walking", 0)
							w.set("stopping", 0)
							w.set("look_target_y", 0x188)
						}
					}
				}
			}
			w.set("player_x", x)
			w.set("player_y", y)
		}
		if w.flag("jumping") {
			if w.get("jump_hold") != 10 {
				w.set("jump_hold", (w.get("jump_hold")+1)&255)
				if in.Jump {
					w.set("momentum_y", int(uint16(w.get("momentum_y")-0x6c)))
					w.set("stopping", 0)
				}
			}
		} else if w.flag("grounded") || w.flag("climbing") || w.flag("hanging") {
			if !in.Jump {
				w.set("jump_hold", 0)
			} else if !w.flag("jump_hold") {
				w.set("crouching", 0)
				w.set("looking_up", 0)
				w.set("jumping", 255)
				// 1E2A34 retains the lower-wall exemption through the next
				// sensor pass when this jump starts on solid support.
				if w.flag("grounded") {
					w.set("flag_f653", 255)
				}
				w.set("jump_released", 0)
				w.set("jump_hold", 1)
				w.set("look_target_x", 0xa0)
				w.set("look_target_y", 0x178)
				w.set("camera_delay", 7)
				w.set("momentum_y", 0xfe00)
				if w.flag("rail_touch") {
					w.set("momentum_y", 0x400)
					w.animation("fall")
				} else if w.flag("climbing") {
					w.animation("jump_climb")
				} else if w.flag("walking") {
					w.animation("jump_moving")
				} else {
					w.animation("jump_still")
				}
				w.Sound(2, false)
			}
		}
		if !w.flag("flag_f6a3") {
			w.throwControl(in)
		}
		w.swordControl(in)
	}
	for _, key := range []string{"action_cooldown", "sword_window", "apple_window"} {
		if w.get(key) > 0 {
			w.set(key, w.get(key)-1)
		}
	}
}
func (w *NativeWorld) throwControl(in Input) {
	if !in.Apple {
		if w.get("apple_cooldown") > 0 {
			w.set("apple_cooldown", w.get("apple_cooldown")-1)
		}
		return
	}
	if w.get("apple_cooldown") > 0 {
		if w.get("apple_cooldown") > 1 {
			w.set("apple_cooldown", w.get("apple_cooldown")-1)
		}
		return
	}
	w.set("apple_cooldown", 14)
	if w.get("apples_ascii") == 0x3030 {
		w.Sound(0x51, false)
		return
	}
	name := "apple_still"
	if w.flag("climbing") || w.flag("hanging") {
		name = "apple_climbing"
	} else if w.flag("crouching") {
		name = "apple_crouch"
	} else if w.flag("jumping") || !w.flag("grounded") {
		name = "apple_air"
	} else if w.flag("walking") {
		name = "apple_moving"
	}
	w.animation(name)
	if name == "apple_still" {
		w.set("apple_step", 1)
		w.set("apple_window", 8)
	}
	if name != "apple_climbing" {
		w.Sound(3, false)
	}
}
func (w *NativeWorld) swordControl(in Input) {
	if (w.flag("climbing") && (in.Up || in.Down)) || (w.flag("hanging") && (in.Right || in.Left)) {
		return
	}
	if !in.Sword {
		w.set("sword_cooldown", 0)
		return
	}
	if w.get("sword_cooldown") > 0 {
		if w.get("sword_cooldown") > 1 {
			w.set("sword_cooldown", w.get("sword_cooldown")-1)
		}
		return
	}
	if w.flag("sword_contact") || w.flag("action_cooldown") {
		return
	}
	if w.flag("flag_f6a3") {
		w.sourceAnimation(0x149f72)
		w.set("sword_cooldown", 10)
		w.Sound(1, false)
		return
	}
	name := "sword_still"
	w.set("sword_step", 0)
	if w.flag("climbing") || w.flag("hanging") {
		name = "sword_climbing"
	} else if w.flag("crouching") {
		name = "sword_crouch"
	} else if !w.flag("grounded") {
		name = "sword_air"
	} else if w.flag("walking") {
		name = "sword_moving"
	} else if in.Up {
		name = "sword_up"
	}
	w.animation(name)
	w.set("sword_cooldown", 10)
	if name == "sword_still" {
		w.set("sword_step", 1)
		w.set("sword_window", 8)
	}
	if name == "sword_crouch" || (name == "sword_climbing" && w.flag("climbing")) {
		w.Sound(1, false)
	}
}
func (w *NativeWorld) Update(in Input) error {
	w.VideoFrames++
	if w.Flow.Phase != FlowPlaying {
		return w.updateStageFlow(in)
	}
	if w.Life.Phase != LifePlaying {
		return w.updateLifeCycle(in)
	}
	if w.parryTarget != nil {
		w.finishParry(w.parryTarget)
		w.parryTarget = nil
		w.ParryFlash = false
		if e := w.playerContactsFrom(w.resumeContact, false); e != nil {
			return e
		}
		if w.parryTarget != nil {
			return nil
		}
		return w.finishFrame(in)
	}
	if w.Stage.Index == 8 {
		w.set("scene_tile_phase", (w.get("scene_tile_phase")+14)%112)
	}
	if w.BufferedPresentation || w.Stage.Index >= 9 && w.Stage.Index <= 12 || abuStage(w.Stage.Index) || w.flag("attract_mode") {
		w.cacheVisibleFrame()
	}
	w.set("tick", (w.get("tick")+1)&255)
	w.SoundEvents = nil
	if w.Stage.Index == 7 {
		oldTick := (w.get("tick") - 1) & 255
		w.set("sprite_shake_x", escapeJitter(oldTick))
		if oldTick == 0 {
			w.Sound(0x53, false)
		}
	}
	w.advanceScreenShake()
	w.advanceCameraTiles()
	if e := w.activateMarkers(); e != nil {
		return e
	}
	w.playerFloor()
	w.playerWalls()
	w.set("flag_f653", 0)
	w.climbRope()
	w.hangRail()
	w.syncPlayer()
	if e := w.moveActors(); e != nil {
		return e
	}
	for i := 1; i < len(w.Objects); i++ {
		if o := w.Objects[i]; o != nil && o.Alive() && o.Frame >= 0 {
			if e := w.objectImpact(o); e != nil {
				return e
			}
		}
	}
	if e := w.playerContacts(); e != nil {
		return e
	}
	if w.parryTarget != nil {
		return nil
	}
	return w.finishFrame(in)
}
func (w *NativeWorld) finishFrame(in Input) error {
	w.set("previous_player_y", w.get("player_y"))
	w.sampleDirections(in)
	w.playerSurface()
	// 1D4364 bypasses integration while a death request is pending.
	if !w.flag("death_pending") {
		w.momentum()
	}
	w.controls(in)
	if e := w.camera(); e != nil {
		return e
	}
	w.syncPlayer()
	if e := w.projectileContacts(); e != nil {
		return e
	}
	if w.deathTrigger() {
		return nil
	}
	if err := w.stageBehavior(); err != nil {
		return err
	}
	if w.stageExitTrigger() {
		return nil
	}
	w.updateScore()
	if w.get("tick")&1 != 0 {
		c := choreography.Choreographer{Scripts: w.Scripts}
		w.lastSwordTarget = nil
		w.set("sword_contact", 0)
		w.set("sword_combo", 0)
		w.set("apple_combo", 0)
		for i := 0; i < len(w.Objects); i++ {
			o := w.Objects[i]
			if o != nil && o.Alive() {
				if e := c.Animation(w, o); e != nil {
					return e
				}
			}
		}
	}
	w.updateHUDWave()
	if w.FirstStageUpdate {
		// 1D30D0 primes the first scroll table before the initial loop. Later
		// iterations write it at 1D509A when the preceding video wait returns.
		w.FirstStageUpdate = false
	} else if w.Stage.Index <= 2 {
		w.set("background_phase", (w.get("background_phase")-1)&65535)
	} else if w.Stage.Index == 3 || w.Stage.Index == 10 || w.Stage.Index == 13 {
		w.set("background_phase", (w.get("background_phase")+1)&65535)
	}
	return nil
}

// objectMomentum is the signed-high-byte integration and damping from
// 1D8626..1D86E6. Its fractional word is retained, but positions are integers.
func (w *NativeWorld) objectMomentum(o *choreography.Object) {
	x, y := o.Position()
	vx, vy := int(o.Word(24)), int(o.Word(26))
	constant := o.Record[60]&1 != 0
	if vx != 0 {
		if vx > 0 {
			if vx < 0x28 {
				vx = 0
			} else {
				x += vx >> 8
				if !constant {
					vx -= 0x28
				}
			}
		} else {
			if -vx < 0x28 {
				vx = 0
			} else {
				x += vx >> 8
				if !constant {
					vx += 0x28
				}
			}
		}
	}
	if o.Record[6]&64 != 0 {
		vy = int(int16(vy + 0x78))
	}
	if vy != 0 {
		if vy > 0 {
			if vy < 0x3c {
				vy = 0
			} else {
				y += vy >> 8
				if !constant {
					vy -= 0x3c
				}
			}
		} else {
			if -vy < 0x3c {
				vy = 0
			} else {
				y += vy >> 8
				if !constant {
					vy += 0x3c
				}
			}
		}
	}
	o.SetWord(2, x)
	o.SetWord(4, y)
	o.SetWord(24, vx)
	o.SetWord(26, vy)
}
func (w *NativeWorld) objectImpact(o *choreography.Object) error {
	o.Record[7] &^= 16
	if o.Record[60]&8 == 0 {
		return w.objectFloor(o)
	}
	x, y := o.Position()
	row, col := (y-240)/16, (x+16)/16
	if y > w.Stage.Height+224 || y < 240 || row < 0 || col < 0 || row >= w.Stage.Height/16 || col >= w.Stage.Width/16 {
		return nil
	}
	f := int(w.Stage.Room[row*(w.Stage.Width/16)+col]) / 2
	if f+2 >= len(w.Stage.Floor) {
		return fmt.Errorf("impact floor record outside imported data")
	}
	xx := x & 15
	hit := false
	for i := 0; i < 2; i++ {
		p := int(w.Stage.Floor[f+i])*16 + xx
		if p < len(w.Stage.Contours) && w.Stage.Contours[p]&63 != 0 {
			hit = true
			break
		}
	}
	if !hit && w.Stage.Floor[f+2] < 0xe0 {
		return nil
	}
	return w.explodeProjectile(o)
}

// moveActors advances ordinary movement and source offscreen retirement.
func (w *NativeWorld) moveActors() error {
	c := choreography.Choreographer{Scripts: w.Scripts}
	for i := 0; i < len(w.Objects); i++ {
		o := w.Objects[i]
		// A newly spawned actor enters movement only after animation has
		// supplied its first frame (the original loop's artwork-pointer gate).
		if o == nil || !o.Alive() || o.Frame < 0 {
			continue
		}
		beforeX, beforeY := o.Position()
		if i != 0 && o.Frame >= 0 && w.outsideActorWindow(o) {
			w.despawn(o)
			continue
		}
		if i != 0 && o.Frame >= 0 {
			w.objectMomentum(o)
		}
		if e := c.Motion(w, o); e != nil {
			return e
		}
		if i == 0 {
			x, y := o.Position()
			w.set("player_x", w.get("player_x")+x-beforeX)
			w.set("player_y", w.get("player_y")+y-beforeY)
		}
		afterX, afterY := o.Position()
		dx, dy := afterX-beforeX, afterY-beforeY
		o.Record[28], o.Record[29] = byte(dx), byte(dy)
		if o.Record[60]&4 != 0 && o.Partner != nil {
			p := o.Partner
			px, py := p.Position()
			p.SetWord(2, px+dx)
			p.SetWord(4, py+dy)
			p.Record[28], p.Record[29] = byte(dx), byte(dy)
		}
	}
	return nil
}

package game

import (
	"aladdin/internal/choreography"
	"fmt"
)

func (w *NativeWorld) ropePose(y int) {
	if w.flag("action_locked") {
		return
	}
	i := (y >> 2) & 15
	w.set("facing_left", 0)
	if i >= 8 {
		w.set("facing_left", 255)
	}
	w.sourceAnimation(w.Scripts.Entry[fmt.Sprintf("rope_pose_%d", i)])
}
func (w *NativeWorld) alignRope() {
	x, _ := w.PlayerPosition()
	w.set("player_x", (x&^15)|8)
	w.set("look_target_x", 0xa0)
	w.set("camera_delay", 0)
}

// climbRope ports 1D3E0A..1D3F8A. It uses the previous video frame's
// directional inputs and surface latches, before the current input poll.
func (w *NativeWorld) climbRope() {
	clear := func() { w.set("climbing", 0) }
	if !w.flag("rope_touch") {
		clear()
		return
	}
	_, y := w.PlayerPosition()
	vy := int16(w.get("momentum_y"))
	if vy <= 0 && w.flag("jumping") && !w.flag("jump_released") {
		clear()
		return
	}
	if vy > 0 || w.flag("jumping") {
		if w.flag("ceiling") {
			clear()
			return
		}
		w.ropePose(y)
		w.set("momentum_x", 0)
		w.set("momentum_y", 0)
		w.set("run_speed", 0)
		w.set("jumping", 0)
		w.set("fall_ticks", 0)
		w.set("climbing", 255)
		w.set("walking", 0)
		w.alignRope()
		return
	}
	if !w.flag("action_locked") {
		if w.flag("fall_ticks") {
			if w.get("tick")&1 != 0 {
				w.ropePose(y)
			}
			w.set("fall_ticks", 0)
		}
		if w.flag("input_up") {
			if !w.flag("ceiling") && !w.flag("rope_top") {
				w.set("player_y", y-1-(w.get("tick")&1))
				w.set("look_target_y", 0x190)
				w.set("camera_delay", 0)
				if w.get("tick")&1 != 0 {
					w.ropePose(y)
				}
				w.alignRope()
			}
		} else if w.flag("input_down") {
			if w.flag("grounded") {
				clear()
				return
			}
			w.set("player_y", y+2)
			w.set("look_target_y", 0x150)
			w.set("camera_delay", 0)
			if w.get("tick")&1 != 0 {
				w.ropePose(y)
			}
			w.alignRope()
		}
	}
	w.set("climbing", 255)
}

func (w *NativeWorld) railPose() {
	if w.flag("action_locked") {
		return
	}
	i := (w.get("rail_phase") & 255) >> 2
	w.sourceAnimation(w.Scripts.Entry[fmt.Sprintf("rail_pose_%d", i)])
	w.set("run_speed", 2)
}
func (w *NativeWorld) alignRail(y int) { w.set("player_y", (y&^15)|4); w.set("look_target_x", 0xa0) }

// hangRail ports 1D3F8C..1D412A, including the independent 40-step
// animation counter and its up/down camera targets.
func (w *NativeWorld) hangRail() {
	clear := func() { w.set("hanging", 0) }
	if !w.flag("rail_touch") || int16(w.get("momentum_y")) < 0 {
		clear()
		return
	}
	_, y := w.PlayerPosition()
	if w.flag("jumping") {
		if !w.flag("jump_released") {
			clear()
			return
		}
		w.railPose()
		w.set("momentum_x", 0)
		w.set("momentum_y", 0)
		w.set("run_speed", 0)
		w.set("jumping", 0)
		w.set("fall_ticks", 0)
		w.set("hanging", 255)
		w.set("walking", 0)
		w.alignRail(y)
		return
	}
	w.set("momentum_y", 0)
	if !w.flag("action_locked") {
		if w.flag("fall_ticks") {
			w.railPose()
			w.set("fall_ticks", 0)
		}
		if w.flag("input_left") || w.flag("input_right") {
			w.set("facing_left", 0)
			if w.flag("input_left") {
				w.set("facing_left", 255)
			}
			w.set("rail_phase", (w.get("rail_phase")+1)%40)
			w.railPose()
		}
	}
	w.alignRail(y)
	target := 0x150
	if w.flag("input_up") {
		target = 0x1b0
	} else if w.flag("input_down") {
		target = 0x130
	} else {
		w.set("camera_delay", 0)
	}
	if target != 0x150 && w.get("look_target_y") != target {
		w.set("camera_delay", 7)
	}
	w.set("look_target_y", target)
	w.set("hanging", 255)
}

// playerSurface ports the surface probe at 1DC66A and the converted rope,
// rail and floor-toggle constructors reached through its byte dispatch.
func (w *NativeWorld) playerSurface() {
	if w.Stage.Index == 8 || w.Objects[0].Frame < 0 {
		return
	}
	w.set("rope_touch", 0)
	w.set("rope_top", 0)
	w.set("rail_touch", 0)
	w.set("rope_forced", 0)
	w.set("on_camel", w.get("flag_f638"))
	w.set("flag_f638", 0)
	if n := w.get("rope_cooldown"); n > 0 {
		w.set("rope_cooldown", n-1)
		if n == 1 {
			w.Sound(0x27, true)
		}
	}
	x, y := w.PlayerPosition()
	yy := y - 240
	if yy < 0 || yy >= w.Stage.Height-32 {
		return
	}
	index := (yy>>4)*(w.Stage.Width/16) + ((x + 16) >> 4)
	if index < 0 || index >= len(w.Stage.Room) {
		return
	}
	offset := int(w.Stage.Room[index])/2 + 2
	if offset >= len(w.Stage.Floor) {
		return
	}
	kind := int(w.Stage.Floor[offset])
	w.set("flag_f5e5", kind)
	if kind != 0x47 {
		w.set("flag_f5e4", 0)
	}
	switch kind {
	case 0x20:
		w.set("death_pending", 255)
	case 0x40, 0x41:
		dx := 8
		if kind == 0x41 {
			dx = -8
		}
		w.set("player_x", x+dx)
		w.set("run_speed", 0)
	case 0x12:
		w.caveRailDebris()
	case 0x70:
		if w.flag("death_pending") {
			break
		}
		if w.flag("flag_f6a3") {
			w.abuWater()
			break
		}
		w.set("death_pending", 32)
		if slot := w.markerSlot(0); slot >= 0 {
			w.Objects[0].Record[0] = 0
			v := choreography.FromTemplate(w.Scripts.Templates[0x1e3a8e])
			v.SetWord(2, x)
			v.SetWord(4, y)
			w.Objects[slot] = v
		}
	case 0x0a:
		if w.Stage.Index == 11 && !w.flag("flag_f640") {
			break
		}
		found := false
		for i := 1; i <= 24; i++ {
			if o := w.Objects[i]; o != nil && o.Record[0] == 0x94 {
				found = true
				break
			}
		}
		if !found && w.flag("grounded") {
			if slot := w.markerSlot(3); slot >= 0 {
				o := choreography.FromTemplate(w.Scripts.Templates[0x1e3502])
				o.SetWord(2, x)
				o.SetWord(4, y)
				w.Objects[slot] = o
			}
		}
	case 0x10:
		w.set("flag_f606", 255)
	case 0x17:
		w.set("flag_f629", 255)
	case 0x18:
		w.set("flag_f628", 255)
	case 0x48:
		w.set("flag_f6a2", 0)
	case 0x2d:
		w.set("momentum_x", 0xfc00)
		w.set("momentum_y", 0x200)
		w.sourceAnimation(0x149074)
		w.Sound(0x27, false)
	case 0x28:
		if !w.flag("action_locked") && !w.flag("jumping") {
			w.set("momentum_x", 0)
			w.set("momentum_y", 0)
			w.set("walking", 0)
			w.set("run_speed", 0)
			w.set("fall_ticks", 0)
			if !w.flag("rope_cooldown") {
				w.sourceAnimation(0x148f00)
				w.set("player_y", (y&^15)+2)
				w.set("player_x", (x|31)-4)
				w.Sound(0x27, false)
			}
		}
		w.set("rope_cooldown", 4)
		w.set("player_x", w.get("player_x")-4)
		w.set("player_y", w.get("player_y")+2)
	case 0x30:
		w.set("momentum_y", (w.get("momentum_y")-0x7c)&65535)
		w.set("run_speed", 0)
		w.set("grounded", 255)
		w.alignRope()
		w.set("walking", 0)
		w.set("climbing", 255)
	case 1:
		w.set("flag_f5c0", 0)
	case 5:
		w.set("flag_f5c0", 1)
	case 0x1b:
		w.set("flag_f69f", 255)
	case 0x1d:
		w.set("flag_f6a1", 255)
	case 0x1f:
		w.set("flag_f6a2", 255)
	case 0x88:
		if w.get("tick")&3 != 0 {
			w.set("player_y", y+1)
			if int16(w.get("momentum_y")) >= 0 && (!w.flag("jumping") || w.flag("jump_released")) {
				w.set("on_camel", 255)
				w.set("on_object", 255)
				w.set("jumping", 0)
				w.set("grounded", 255)
				w.set("momentum_y", 0)
			}
		}
	case 9, 11, 12:
		if int16(w.get("momentum_y")) >= 0 {
			w.damagePlayer()
		}
	case 0x22, 0x23, 0x24:
		if !w.flag("flag_f652") {
			w.set("rope_touch", 255)
			if kind == 0x24 {
				w.set("rope_top", 255)
			}
		}
	case 0x25:
		if !w.flag("flag_f652") {
			w.set("rail_touch", 255)
			if w.Stage.Index == 5 {
				w.caveRailDebris()
			}
		}
	case 0x27:
		if !w.flag("flag_f652") {
			w.set("rope_top", 255)
			w.set("player_y", y-80)
			w.sourceAnimation(0x149b0a)
			w.set("action_locked", 255)
			w.set("walking", 0)
		}
	case 0x2b:
		// Forced pole descent at 1DFD76. Unlike free climbing this
		// continually reapplies its pose and accelerates the fall.
		if !w.flag("action_locked") && int16(w.get("momentum_y")) >= 0 && !w.flag("grounded") {
			w.set("jumping", 0)
			w.set("walking", 0)
			w.set("run_speed", 0)
			w.set("momentum_x", 0)
			w.set("fall_ticks", 0)
			w.set("player_x", (x&^15)|6)
			w.sourceAnimation(0x148db6)
			w.set("rope_forced", 1)
			if w.get("momentum_y")>>8 < 8 {
				w.set("momentum_y", w.get("momentum_y")+0x78)
			}
		}
	case 0x29:
		if !w.flag("jumping") {
			w.set("momentum_y", 0xfb00)
			w.set("momentum_x", 0xfc00)
			// This handler clears the delay without storing its computed
			// animation label; preserve that original behaviour.
			w.Objects[0].AnimationDelay = 0
			w.set("jumping", 255)
			w.set("jump_released", 0)
			w.set("walking", 0)
			w.Sound(0x3f, false)
		}
	case 0x47:
		if !w.flag("floor_disabled") && !w.flag("flag_f5e4") {
			w.set("flag_f5e4", 255)
			w.set("flag_f5c0", w.get("flag_f5c0")^1)
		}
	case 0x85:
		w.set("floor_disabled", 255)
	case 0x86:
		w.set("floor_disabled", 0)
	case 0x19:
		w.set("flag_f654", 255)
	}
}

func (w *NativeWorld) caveRailDebris() {
	if !w.flag("input_left") && !w.flag("input_right") {
		return
	}
	n := byte(w.Random())
	if n >= 40 {
		return
	}
	slot := w.markerSlot(3)
	if slot < 0 {
		return
	}
	o := choreography.FromTemplate(w.Scripts.Templates[0x1e3796])
	x, y := w.PlayerPosition()
	o.SetWord(2, x+int((n&7)-3))
	o.SetWord(4, y-42)
	if n < 27 {
		o.Animation = 0x14d03e
		if n < 13 {
			o.Animation = 0x14d02e
		}
	}
	w.Objects[slot] = o
}

// camelContact is the spring/spit trigger at 1DA418. The camel's converted
// animation performs the spit spawn and returns its kind to the idle state.
func (w *NativeWorld) camelContact(o *choreography.Object) {
	if int16(w.get("momentum_y")) <= 0 || o.Record[0] == 0x6d {
		return
	}
	o.Record[0] = 0x6d
	o.Animation = 0x14c244
	o.AnimationDelay = 0
	if w.flag("action_locked") {
		return
	}
	w.set("momentum_y", 0xfb00)
	w.sourceAnimation(0x1498f2)
	w.set("jumping", 255)
	w.set("jump_released", 0)
	w.set("walking", 0)
}

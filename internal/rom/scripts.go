package rom

import (
	"aladdin/internal/data"
	"fmt"
	"sort"
)

var variableNames = map[int]string{
	0xf65e: "bonus_event_cursor", 0xf67c: "scene_tile_phase", 0xf5a0: "rug_distance", 0xf5a2: "rug_speed", 0xf5a4: "rug_acceleration_clock", 0xf5a6: "rug_palette_cursor", 0xf5be: "rug_lava_scroll", 0xf65a: "rug_event_cursor",
	0xf5b0: "carpet_target_x", 0xf5b2: "carpet_target_y",
	0xf684: "checkpoint_active",
	0x7e36: "stage_index",
	0xf6a9: "scarab_first", 0xf6aa: "scarab_second",
	0x7e40: "high_score", 0xf613: "new_high_score",
	0x7e4f: "continues", 0xf68a: "message_id",
	0xf5f0: "rope_touch", 0xf5f1: "rope_top", 0xf5f8: "rail_touch", 0xf69a: "rail_phase",
	0xf5fd: "rope_forced", 0xf5fe: "rope_cooldown", 0xf5df: "floor_disabled", 0xf5f6: "previous_standing_object",
	0xf5ba: "background_phase",
	0x7e2a: "screen_shake_cursor",
	0xf59c: "sprite_shake",
	0xf59e: "sprite_shake_x",
	0xf5d4: "previous_player_y",
	0x7e06: "camera_x", 0x7e08: "camera_y", 0xf697: "camera_delay", 0x7e50: "camera_rate_mode",
	0xf5ed: "ceiling",
	0xf608: "death_pending", 0xf60b: "exit_countdown", 0xf614: "invulnerability", 0xf515: "max_health",
	0xf51d: "genie_tokens", 0xf67e: "pending_score", 0x7e30: "invincible",
	0x7e16: "camera_tile_x", 0x7e18: "camera_tile_y",
	0x7e12: "player_x", 0x7e14: "player_y", 0x7e6a: "momentum_x", 0x7e6c: "momentum_y",
	0x7e10: "look_target_y", 0x7e0e: "look_target_x", 0x7e31: "difficulty", 0x7e38: "tick", 0x7e5b: "facing_left",
	0xf5cc: "run_speed", 0xf5e0: "jumping", 0xf5e1: "jump_hold", 0xf5e2: "jump_released", 0xf5e3: "grounded",
	0xf598: "input_right", 0xf599: "input_left", 0xf59a: "input_up", 0xf59b: "input_down",
	0xf5ee: "walking", 0xf600: "crouching", 0xf601: "looking_up", 0xf609: "action_locked", 0xf5ef: "on_object", 0xf5f5: "standing_object",
	0xf5f2: "climbing", 0xf5f9: "hanging", 0xf60d: "fall_ticks", 0xf637: "on_camel", 0xf60f: "stopping",
	0xf5fa: "sword_contact", 0xf5fb: "sword_combo", 0xf5fc: "apple_combo", 0xf655: "action_cooldown",
	0xf656: "apple_step", 0xf657: "apple_window", 0xf658: "sword_step", 0xf659: "sword_window",
	0xf642: "apple_cooldown", 0xf519: "sword_cooldown", 0xf623: "landing_combo", 0xf5e7: "wall_left", 0xf5ea: "wall_right",
	0xf514: "health", 0xf4fa: "apples_ascii", 0xf4fc: "gems_ascii",
}

func variableName(a int) string {
	if s := variableNames[a]; s != "" {
		return s
	}
	return fmt.Sprintf("flag_%04x", a)
}

var instructionNames = []string{"jump", "flip", "stop", "write", "repeat", "loop", "random_jump", "offset", "bit_branch", "sound", "compare_branch", "spawn", "delete", "face_player", "return_player", "steer", "add", "callback", "call", "near_x", "near_y"}

func callbackName(a int) string {
	if n := map[int]string{0x1d725e: "hide", 0x1d7266: "show", 0x1d726e: "solid", 0x1d7276: "non_solid", 0x1d72fc: "set_attack_flag", 0x1d7304: "clear_attack_flag", 0x1d744e: "random_scatter", 0x1d7352: "random_guard_voice", 0x1e2cba: "idle_sword_combo", 0x1e2c66: "moving_attack_combo", 0x1e2812: "parry_impulse_xy", 0x1e281e: "parry_impulse_x", 0x1dab84: "consume_apple", 0x1e26d0: "attack_if_player_right", 0x1e26ec: "attack_if_player_left"}[a]; n != "" {
		return n
	}
	return fmt.Sprintf("native_%06x", a)
}

// ImportScripts decodes the game's two small data languages into a declarative
// IR. The 68000 code that implements the original languages is not imported.
func (r ROM) ImportScripts() (result data.ScriptSet, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("script conversion: %v", v)
		}
	}()
	result = data.ScriptSet{Animation: map[int]data.Instruction{}, Motion: map[int]data.Instruction{}, Templates: map[int]data.ObjectTemplate{}, Entry: map[string]int{
		"idle": 0x1494bc, "run": 0x14973a, "stop": 0x1496ce, "fall": 0x149074, "land": 0x1496ac,
		"jump_still": 0x1498ea, "jump_moving": 0x149888, "jump_climb": 0x1497f2,
		"crouch": 0x149a0c, "look_up": 0x149970,
		"sword_still": 0x149fa8, "sword_moving": 0x14a1a6, "sword_air": 0x14a2b8, "sword_crouch": 0x14a0c0,
		"apple_still": 0x149b14, "apple_moving": 0x149df4, "apple_air": 0x149eac, "apple_crouch": 0x149caa,
		"apple_climbing": 0x149d4a, "sword_climbing": 0x14a450,
		"sword_up": 0x14a36a,
	}}
	result.AbuSchedules = map[int][]data.AbuEvent{}
	for stage, start := range map[int]int{2: 0x2440, 6: 0x2a02, 13: 0x2844} {
		for pc := start; r.Bytes[pc] != 0; pc += 6 {
			if pc >= 0x2b59 {
				return result, fmt.Errorf("unterminated Abu schedule")
			}
			result.AbuSchedules[stage] = append(result.AbuSchedules[stage], data.AbuEvent{Source: pc, Delay: int(r.Bytes[pc]), Kind: int(r.Bytes[pc+1]), X: r.U16(pc + 2), Y: r.U16(pc + 4)})
		}
	}
	for i := 0; i < 64; i++ {
		v := r.U16(0x151f54 + i*2)
		result.GeniePalette[i] = [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
	}
	for i := 0; i < 16; i++ {
		a := 0x76a2 + i*8
		result.GenieChoices = append(result.GenieChoices, data.GenieChoice{Kind: r.U32(a) & 255, Animation: r.U32(a + 4)})
	}
	// Tokens are bytes; preserve the source table reads for all possible tens.
	for i := 0; i < 26; i++ {
		result.GenieDigits = append(result.GenieDigits, r.U32(0x508a+i*4))
	}
	for row, words := range [2][5]int{{0xc6c, 0xeae, 0x006, 0x24e, 0xc8e}, {0xe66, 0xe86, 0x000, 0x002, 0x202}} {
		for j, v := range words {
			result.GeniePulse[row][j] = data.PaletteChange{Index: [5]int{7, 8, 21, 22, 23}[j], RGB: [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}}
		}
	}
	result.CameraX = append([]uint8(nil), r.Bytes[0x302a:0x317c]...)
	// Horizontal subtraction uses unsigned carry before a signed-word table
	// index. A player just left of the camera reads the preceding data window.
	result.CameraXPrefix = append([]uint8(nil), r.Bytes[0x202a:0x302a]...)
	for a := 0x7680; a <= 0x76a0; a += 2 {
		result.ScreenShake = append(result.ScreenShake, int(int16(r.U16(a))))
	}
	for i := 0; i < 16; i++ {
		v := r.U16(0x1517d4 + i*2)
		result.DeathPalette[48+i] = [3]uint8{ColorLevels[(v>>1)&7], ColorLevels[(v>>5)&7], ColorLevels[(v>>9)&7]}
	}
	for i := 0; i < 64; i++ {
		v := r.U16(0x1522f4 + i*2)
		result.GameOverPalette[i] = [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
	}
	result.GameOverFinalPalette = result.DeathPalette
	for i := 0; i < 16; i++ {
		v := r.U16(0x1523ac + i*2)
		result.GameOverFinalPalette[i] = [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
	}
	for i := 0; i < 28; i++ {
		v := r.U16(0x152374 + i*2)
		result.GameOverCycle[i] = [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
	}
	result.ContinuePalette = result.DeathPalette
	for i := 0; i < 32; i++ {
		v := r.U16(0x1523cc + i*2)
		result.ContinuePalette[i] = [3]uint8{ColorLevels[v>>1&7], ColorLevels[v>>5&7], ColorLevels[v>>9&7]}
	}
	result.LevelClearPalette = result.DeathPalette
	result.Palettes = map[int][16][3]uint8{}
	for _, a := range []int{0x1517b4, 0x151874, 0x1519d4, 0x151a14, 0x151c14, 0x151c54, 0x151d14, 0x151cf4, 0x151d34, 0x151a54} {
		var p [16][3]uint8
		for i := range p {
			v := r.U16(a + i*2)
			p[i] = [3]uint8{ColorLevels[(v>>1)&7], ColorLevels[(v>>5)&7], ColorLevels[(v>>9)&7]}
		}
		result.Palettes[a] = p
	}
	for i := 0; i < 16; i++ {
		v := r.U16(0x15248c + i*2)
		result.LevelClearPalette[i] = [3]uint8{ColorLevels[(v>>1)&7], ColorLevels[(v>>5)&7], ColorLevels[(v>>9)&7]}
	}
	for pc := 0x4698; ; pc += 4 {
		step := data.RouteStep{Mode: r.U16(pc), Stage: r.U16(pc + 2)}
		result.Route = append(result.Route, step)
		if step.Mode == 255 {
			break
		}
	}
	result.CameraY = append([]uint8(nil), r.Bytes[0x317c:0x3250]...)
	result.BounceFacing = append([]uint8(nil), r.Bytes[0x7580:0x7680]...)
	for a := 0x2f7e; r.U16(a) != 0; a += 2 {
		result.HUDWave = append(result.HUDWave, r.U16(a)&2047)
	}
	var animationRoots, motionRoots []int
	for _, v := range result.GenieChoices {
		animationRoots = append(animationRoots, v.Animation)
	}
	for _, v := range result.GenieDigits {
		if v != 0 {
			animationRoots = append(animationRoots, v)
		}
	}
	r.preludeData(&result)
	if e := r.storyData(&result); e != nil {
		return result, e
	}
	if e := r.menuData(&result); e != nil {
		return result, e
	}
	result.Messages = map[int]data.Message{}
	for _, id := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 0x0e, 0x10, 0x11, 0x12, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a} {
		entry := 0x14f46a + id*12
		pc, x, y := r.U32(entry), r.U16(entry+4), r.U16(entry+6)
		m := data.Message{Callback: callbackName(r.U32(entry + 8))}
		motion := 0x146a78
		for budget := 0; budget < 1024; budget++ {
			v := int(r.Bytes[pc])
			pc++
			if v == 0 {
				break
			}
			switch {
			case v == 1:
				x += int(int8(r.Bytes[pc]))
				pc++
			case v == 2:
				y += int(int8(r.Bytes[pc]))
				pc++
			case v == 12:
				motion = int(r.Bytes[pc])<<16 | int(r.Bytes[pc+1])<<8 | int(r.Bytes[pc+2])
				pc += 3
			case v >= 32 && v <= 90:
				animation := r.U32(0x504a + (v-32)*4)
				if animation != 0 {
					m.Glyphs = append(m.Glyphs, data.MessageGlyph{X: x, Y: y, Animation: animation, Motion: motion})
					animationRoots = append(animationRoots, animation)
					motionRoots = append(motionRoots, motion)
				}
				x += 17
			default:
				return result, fmt.Errorf("message %x has unconverted text command %x", id, v)
			}
		}
		result.Messages[id] = m
	}
	for _, table := range []struct {
		name           string
		address, count int
	}{{"rope_pose", 0x148dc4, 16}, {"rail_pose", 0x148e74, 10}} {
		for i := 0; i < table.count; i++ {
			a := r.U32(table.address + i*4)
			result.Entry[fmt.Sprintf("%s_%d", table.name, i)] = a
			animationRoots = append(animationRoots, a)
		}
	}
	for _, table := range []struct {
		name    string
		address int
	}{
		{"combo_sword_moving", 0x1e2c0e}, {"combo_sword_idle", 0x1e2c26}, {"combo_apple_moving", 0x1e2c3e},
	} {
		for i := 0; i < 6; i++ {
			a := r.U32(table.address + i*4)
			result.Entry[fmt.Sprintf("%s_%d", table.name, i)] = a
			animationRoots = append(animationRoots, a)
		}
	}
	validAnimation := func(a int) bool {
		if a < 0x148db6 || a >= 0x14e90e {
			return false
		}
		v := r.U16(a)
		return v >= 0x6ae && v < 0x1f62 && (v-0x6ae)%4 == 0
	}
	// Object templates occupy fixed 20-byte records (19 bytes plus alignment).
	for a := 0x1e2dfa; a <= 0x1e3c5a; a += 20 {
		motion, animation := r.U32(a+6), r.U32(a+12)
		if (animation != 0 && !validAnimation(animation)) || (motion != 0 && (motion < 0x1469f0 || motion >= 0x148db6)) {
			continue
		}
		t := data.ObjectTemplate{Source: a, Type: int(r.Bytes[a]), HP: int(r.Bytes[a+1]), Flags: int(r.Bytes[a+2]), Flags2: int(r.Bytes[a+3]), Parameter: int(r.Bytes[a+4]), FlipX: r.Bytes[a+5] != 0, Motion: motion, Palette: r.U16(a + 10), Animation: animation, Tiles: int(r.Bytes[a+16]), FlipY: r.Bytes[a+17] != 0, Behaviour: int(r.Bytes[a+18])}
		result.Templates[a] = t
		animationRoots = append(animationRoots, animation)
		if motion != 0 {
			motionRoots = append(motionRoots, motion)
		}
	}
	for _, a := range result.Entry {
		animationRoots = append(animationRoots, a)
	}
	// References in native code include the remaining animation/movement entry
	// labels and the attack-selector tables. Only data pointers are retained.
	for a := 0x1d2f26; a < 0x1e2dfa; a += 2 {
		v := r.U32(a)
		if validAnimation(v) {
			animationRoots = append(animationRoots, v)
		}
		if v >= 0x1469f0 && v < 0x148db6 && v%2 == 0 {
			motionRoots = append(motionRoots, v)
		}
	}
	callbacks := map[string]bool{}
	var parse func([]int, bool) error
	parse = func(roots []int, motion bool) error {
		queue := append([]int(nil), roots...)
		nodes := result.Animation
		low, high := 0x148db6, 0x14e90e
		if motion {
			nodes = result.Motion
			low, high = 0x1469f0, 0x148db6
		}
		for len(queue) > 0 {
			a := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			if a == 0 {
				continue
			}
			if _, ok := nodes[a]; ok {
				continue
			}
			if a < low || a >= high {
				return fmt.Errorf("%s script target outside data: %06x", map[bool]string{true: "motion", false: "animation"}[motion], a)
			}
			n := data.Instruction{Source: a, Member: -1}
			op := int(r.Bytes[a])
			base := 0xea
			if motion {
				base = 0x80
			}
			if op < base || op >= base+21 {
				if motion {
					n.Kind = "move"
					n.DX = int(int8(r.Bytes[a]))
					n.DY = int(int8(r.Bytes[a+1]))
					n.Next = a + 2
				} else {
					if !validAnimation(a) {
						return fmt.Errorf("invalid frame reference at %06x", a)
					}
					n.Kind = "frame"
					n.Frame = (r.U16(a) - 0x6ae) / 4
					n.Next = a + 2
				}
			} else {
				k := op - base
				n.Kind = instructionNames[k]
				n.Mode = int(r.Bytes[a+1])
				size := 0
				switch n.Kind {
				case "jump", "random_jump", "near_x", "near_y", "callback":
					size = 6
				case "flip", "stop", "repeat", "loop", "sound", "delete", "face_player", "return_player":
					size = 2
				case "offset", "steer":
					size = 4
				case "bit_branch":
					size = 8
					n.Width = 1
					f := r.U16(a + 2)
					if n.Mode&128 != 0 {
						n.Member = f
					} else {
						n.Field = variableName(f)
					}
				case "write", "add", "compare_branch":
					n.Width = 4
					if n.Mode&7 == 1 {
						n.Width = 1
					} else if n.Mode&7 == 2 {
						n.Width = 2
					}
					size = 4 + max(n.Width, 2)
					f := r.U16(a + 2)
					local := n.Kind == "write" && n.Mode&16 != 0 || n.Kind == "add" && n.Mode&64 != 0 || n.Kind == "compare_branch" && n.Mode&128 != 0
					if local {
						n.Member = f
					} else {
						n.Field = variableName(f)
					}
					if n.Width == 4 {
						n.Value = uint32(r.U32(a + 4))
					} else {
						n.Value = uint32(r.U16(a + 4))
					}
					if n.Kind == "compare_branch" {
						size += 4
					}
				case "call":
					size = 6
					if n.Mode&128 != 0 {
						size = 2
					}
				case "spawn":
					size = 16
					n.Template = r.U32(a + 2)
					n.OffsetX = int(int8(r.Bytes[a+6]))
					n.OffsetY = int(int8(r.Bytes[a+7]))
					n.Animation = r.U32(a + 8)
					n.Motion = r.U32(a + 12)
				}
				n.Next = a + size
				switch n.Kind {
				case "jump", "random_jump", "near_x", "near_y":
					n.Target = r.U32(a + 2)
				case "bit_branch":
					n.Target = r.U32(a + 4)
				case "compare_branch":
					n.Target = r.U32(n.Next - 4)
				case "callback":
					n.Callback = callbackName(r.U32(a + 2))
					callbacks[n.Callback] = true
				case "call":
					if size == 6 {
						n.Target = r.U32(a + 2)
					}
				case "offset":
					n.Value = uint32(r.U16(a + 2))
				case "steer":
					n.OffsetX = int(r.Bytes[a+2])
					n.OffsetY = int(r.Bytes[a+3])
				}
			}
			nodes[a] = n
			// Scripts can replace either channel through actor fields. These
			// targets are data graph edges just like explicit jumps/spawns.
			if n.Kind == "write" && n.Member >= 0 && n.Width == 4 && n.Value != 0 {
				if n.Member == 10 {
					motionRoots = append(motionRoots, int(n.Value))
				}
				if n.Member == 32 {
					animationRoots = append(animationRoots, int(n.Value))
				}
			}
			if n.Target != 0 {
				queue = append(queue, n.Target)
			}
			if n.Kind == "spawn" {
				if n.Animation != 0 {
					animationRoots = append(animationRoots, n.Animation)
				}
				if n.Motion != 0 {
					motionRoots = append(motionRoots, n.Motion)
				}
			}
			if n.Kind != "jump" && n.Kind != "delete" && !(n.Kind == "stop" && ((n.Mode == 0) == motion)) && n.Kind != "return_player" && !(n.Kind == "call" && n.Mode&128 != 0) {
				queue = append(queue, n.Next)
			}
		}
		return nil
	}
	// A motion program can spawn animation programs, and conversely. Repeat
	// until discovery reaches a fixed point, without including native code.
	for {
		before := len(result.Animation) + len(result.Motion)
		if e := parse(animationRoots, false); e != nil {
			return result, e
		}
		if e := parse(motionRoots, true); e != nil {
			return result, e
		}
		if before == len(result.Animation)+len(result.Motion) {
			break
		}
	}
	for n := range callbacks {
		result.Callbacks = append(result.Callbacks, n)
	}
	sort.Strings(result.Callbacks)
	return result, nil
}

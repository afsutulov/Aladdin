// Package choreography implements the game's native animation/motion data
// languages in Go. It does not implement any console CPU or console hardware.
// Its input consists solely of the converted declarative data in scripts.json.
package choreography

import (
	"aladdin/internal/data"
	"encoding/binary"
	"fmt"
)

// Object preserves the original 66-byte actor record, not machine memory.
// Keeping the data layout makes its documented byte/word aliasing explicit.
type Object struct {
	Record                        [66]byte
	Frame                         int
	Animation, Motion             int
	AnimationDelay, MotionDelay   int
	AnimationRepeat, MotionRepeat int
	AnimationLoop, MotionLoop     int
	Return                        int
	Partner                       *Object
	Source                        int
	ArtworkStart, ArtworkCount    int
	ArtworkValid                  bool
}

func (o *Object) Read(member, width int) (uint32, error) {
	if member < 0 || member+width > len(o.Record) {
		return 0, fmt.Errorf("object member %x/%d out of bounds", member, width)
	}
	if width == 4 {
		switch member {
		case 10:
			return uint32(o.Motion), nil
		case 32:
			return uint32(o.Animation), nil
		}
	}
	switch width {
	case 1:
		return uint32(o.Record[member]), nil
	case 2:
		return uint32(binary.BigEndian.Uint16(o.Record[member:])), nil
	case 4:
		return binary.BigEndian.Uint32(o.Record[member:]), nil
	}
	return 0, fmt.Errorf("unsupported actor field width %d", width)
}
func (o *Object) Write(member, width int, v uint32) error {
	if member < 0 || member+width > len(o.Record) {
		return fmt.Errorf("object member %x/%d out of bounds", member, width)
	}
	switch width {
	case 1:
		o.Record[member] = byte(v)
	case 2:
		binary.BigEndian.PutUint16(o.Record[member:], uint16(v))
	case 4:
		binary.BigEndian.PutUint32(o.Record[member:], v)
	default:
		return fmt.Errorf("unsupported actor field width %d", width)
	}
	// Pointers stored in an actor are converted script labels, never CPU code.
	if member == 10 && width == 4 {
		o.Motion = int(v)
	}
	if member == 32 && width == 4 {
		o.Animation = int(v)
	}
	return nil
}
func (o *Object) Word(member int) int16     { v, _ := o.Read(member, 2); return int16(v) }
func (o *Object) SetWord(member int, v int) { _ = o.Write(member, 2, uint32(v)) }
func (o *Object) Position() (int, int)      { return int(o.Word(2)), int(o.Word(4)) }
func (o *Object) Alive() bool               { return o.Record[0] != 0 }
func FromTemplate(t data.ObjectTemplate) *Object {
	o := &Object{Source: t.Source, Frame: -1, Animation: t.Animation, Motion: t.Motion}
	o.Record[0] = byte(t.Type)
	o.Record[1] = byte(t.HP)
	o.Record[6] = byte(t.Flags)
	o.Record[7] = byte(t.Flags2)
	o.Record[8] = byte(t.Parameter)
	if t.FlipX {
		o.Record[9] = 255
	}
	if t.FlipY {
		o.Record[53] = 255
	}
	o.Record[41] = byte(t.Tiles)
	o.Record[60] = byte(t.Behaviour)
	o.SetWord(30, t.Palette)
	return o
}

type Host interface {
	ReadVariable(name string, width int) (uint32, error)
	WriteVariable(name string, width int, value uint32) error
	Random() uint16
	Sound(id int, overlay bool)
	Spawn(parent *Object, step data.Instruction) error
	Delete(object *Object, cascade bool)
	Callback(name string, object *Object) (replacement int, err error)
	ResumePlayer(object *Object) (int, error)
	PlayerPosition() (x, y int)
}
type Choreographer struct{ Scripts *data.ScriptSet }

func read(host Host, o *Object, n data.Instruction) (uint32, error) {
	if n.Member >= 0 {
		return o.Read(n.Member, n.Width)
	}
	return host.ReadVariable(n.Field, n.Width)
}
func write(host Host, o *Object, n data.Instruction, v uint32) error {
	if n.Member >= 0 {
		return o.Write(n.Member, n.Width, v)
	}
	return host.WriteVariable(n.Field, n.Width, v)
}
func mask(width int) uint32 {
	switch width {
	case 1:
		return 255
	case 2:
		return 65535
	}
	return ^uint32(0)
}
func reflect(v int, flipped bool) int {
	if flipped {
		return -v
	}
	return v
}

// Animation updates one animation tick, which the original scheduler invokes
// every other NTSC video frame. Movement has its own per-video-frame clock.
func (c Choreographer) Animation(host Host, o *Object) error {
	if h, ok := host.(interface{ ReserveArtwork(*Object) bool }); ok && !h.ReserveArtwork(o) {
		return nil
	}
	return c.step(host, o, false)
}
func (c Choreographer) Motion(host Host, o *Object) error { return c.step(host, o, true) }
func (c Choreographer) step(host Host, o *Object, motion bool) error {
	nodes, pc := c.Scripts.Animation, o.Animation
	delay, repeat, loop := &o.AnimationDelay, &o.AnimationRepeat, &o.AnimationLoop
	if motion {
		nodes, pc = c.Scripts.Motion, o.Motion
		delay, repeat, loop = &o.MotionDelay, &o.MotionRepeat, &o.MotionLoop
	}
	if pc == 0 || !o.Alive() {
		return nil
	}
	if motion && o.Record[6]&2 != 0 {
		return nil
	}
	first, ok := nodes[pc]
	if !ok {
		return fmt.Errorf("unconverted choreography entry %06x", pc)
	}
	if motion {
		if first.Kind != "move" {
			return fmt.Errorf("movement tick %06x has kind %s", pc, first.Kind)
		}
		x, y := o.Position()
		if o.Record[0] == 0x8b {
			x += first.DX
			y += first.DY
		} else {
			x += reflect(first.DX, o.Record[9] != 0)
			if !(first.DY >= 0 && o.Record[6]&1 != 0 && o.Record[7]&16 != 0) {
				y += reflect(first.DY, o.Record[53] != 0)
			}
		}
		o.SetWord(2, x)
		o.SetWord(4, y)
	} else {
		if first.Kind != "frame" {
			return fmt.Errorf("animation tick %06x has kind %s", pc, first.Kind)
		}
		o.Frame = first.Frame
	}
	// Like the original, the frame/move at the retained label is applied even
	// while its delay is counting down; the script cursor stays on that label.
	if *delay > 0 {
		*delay--
		return nil
	}
	pc = first.Next
	for budget := 0; budget < 1024; budget++ {
		n, ok := nodes[pc]
		if !ok {
			return fmt.Errorf("unconverted choreography command %06x", pc)
		}
		if n.Kind == "frame" || n.Kind == "move" {
			if motion {
				o.Motion = pc
			} else {
				o.Animation = pc
			}
			return nil
		}
		pc = n.Next
		switch n.Kind {
		case "jump":
			pc = n.Target
		case "flip":
			member := 9
			if n.Mode != 0 {
				member = 53
			}
			o.Record[member] ^= 255
		case "stop":
			if n.Mode != 0 {
				o.Animation = 0
				if !motion {
					return nil
				}
			} else {
				o.Motion = 0
				if motion {
					return nil
				}
			}
		case "write":
			if e := write(host, o, n, n.Value); e != nil {
				return e
			}
		case "repeat":
			if n.Mode&128 != 0 {
				*delay = n.Mode & 127
				if motion {
					o.Motion = pc
				} else {
					o.Animation = pc
				}
				return nil
			}
			*repeat = n.Mode
			*loop = pc
		case "loop":
			if *repeat > 0 {
				*repeat--
				pc = *loop
			}
		case "random_jump":
			if int(host.Random()&255) < n.Mode {
				pc = n.Target
			}
		case "offset":
			member := 2
			flip := o.Record[9] != 0
			if n.Mode != 0 {
				member = 4
				flip = o.Record[53] != 0
			}
			o.SetWord(member, int(o.Word(member))+reflect(int(int16(n.Value)), flip))
		case "bit_branch":
			v, e := read(host, o, n)
			if e != nil {
				return e
			}
			set := v&(1<<(n.Mode&7)) != 0
			if set == (n.Mode&64 != 0) {
				pc = n.Target
			}
		case "sound":
			host.Sound(n.Mode&127, n.Mode&128 != 0)
		case "compare_branch":
			v, e := read(host, o, n)
			if e != nil {
				return e
			}
			v &= mask(n.Width)
			want := n.Value & mask(n.Width)
			take := false
			switch n.Mode & 0x70 {
			case 0x10:
				take = want == v
			case 0x20:
				take = want != v
			case 0x30:
				take = want >= v
			default:
				// The source tests equality/inequality/greater-or-equal
				// explicitly; all other selector bits use unsigned less-than.
				take = want < v
			}
			if take {
				pc = n.Target
			}
		case "spawn":
			if e := host.Spawn(o, n); e != nil {
				return e
			}
		case "delete":
			host.Delete(o, n.Mode != 0)
			return nil
		case "face_player":
			px, _ := host.PlayerPosition()
			x, _ := o.Position()
			o.Record[9] = 0
			if px < x {
				o.Record[9] = 255
			}
		case "return_player":
			p, e := host.ResumePlayer(o)
			if e != nil {
				return e
			}
			pc = p
		case "steer":
			px, py := host.PlayerPosition()
			x, y := o.Position()
			acc := int8(n.Mode)
			ay, ax := acc, acc
			if uint16(y+int(int8(-byte(n.OffsetX)))) >= uint16(py) {
				ay = -ay
			}
			if uint16(x+int(int8(-byte(n.OffsetY)))) >= uint16(px) {
				ax = -ax
			}
			o.Record[26] += byte(ay)
			o.Record[24] += byte(ax)
		case "add":
			v, e := read(host, o, n)
			if e != nil {
				return e
			}
			if n.Mode&128 != 0 {
				v -= n.Value
			} else {
				v += n.Value
			}
			if e = write(host, o, n, v&mask(n.Width)); e != nil {
				return e
			}
		case "callback":
			p, e := host.Callback(n.Callback, o)
			if e != nil {
				return e
			}
			if p != 0 {
				pc = p
			}
		case "call":
			if n.Mode&128 != 0 {
				pc = o.Return
			} else {
				o.Return = pc
				pc = n.Target
			}
		case "near_x", "near_y":
			px, py := host.PlayerPosition()
			x, y := o.Position()
			distance, limit := px-x, n.Mode
			if n.Kind == "near_y" {
				distance = py - y
			} else if limit == 255 {
				limit = 320
			}
			if distance < 0 {
				distance = -distance
			}
			if distance <= limit {
				pc = n.Target
			}
		default:
			return fmt.Errorf("unsupported choreography operation %q", n.Kind)
		}
	}
	return fmt.Errorf("choreography cycle without a frame/movement tick")
}

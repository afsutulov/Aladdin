package main

import (
	"aladdin/internal/game"

	"github.com/hajimehoshi/ebiten/v2"
)

// stickThreshold is how far an analog stick must be pushed before it counts as
// a D-pad direction. Half travel ignores drift on worn sticks while still
// making diagonals easy to hold.
const stickThreshold = 0.5

// padState is one controller sampled in the standard (Xbox/PlayStation)
// layout. Buttons are named by position, so the same code covers XInput pads,
// DualSense/DualShock and any other controller Ebitengine recognises.
type padState struct {
	Up, Down, Left, Right                 bool    // D-pad
	StickX, StickY                        float64 // left stick, -1..1, +Y is down
	FaceDown, FaceRight, FaceLeft, FaceUp bool    // face buttons: A/Cross, B/Circle, X/Square, Y/Triangle
	Start, Back                           bool    // Menu/Options, View/Share (Create)
}

// mapPad merges a controller into the keyboard input. The Genesis pad has
// A (apple), B (sword), C (jump) and Start; they are placed where modern games
// put jump and attack:
//
//	A / Cross       jump      (Genesis C)
//	X / Square      sword     (Genesis B)
//	B / Circle      apple     (Genesis A)
//	Y / Triangle    apple     (second apple button)
//	Menu / Options  Start     (Enter)
//	View / Share    Esc       (back to the title menu)
//	D-pad and left stick move.
func mapPad(in *game.Input, p padState) {
	in.Up = in.Up || p.Up || p.StickY <= -stickThreshold
	in.Down = in.Down || p.Down || p.StickY >= stickThreshold
	in.Left = in.Left || p.Left || p.StickX <= -stickThreshold
	in.Right = in.Right || p.Right || p.StickX >= stickThreshold
	in.Jump = in.Jump || p.FaceDown
	in.Sword = in.Sword || p.FaceLeft
	in.Apple = in.Apple || p.FaceRight || p.FaceUp
	in.Enter = in.Enter || p.Start
	in.Escape = in.Escape || p.Back
}

var gamepadIDs []ebiten.GamepadID

// applyGamepads merges every connected controller into in. Ebitengine handles
// the platform side: XInput and DirectInput on Windows, GameController/IOKit
// on macOS and evdev on Linux, with the SDL controller database supplying the
// standard layout for DualSense, DualShock, Xbox, Switch Pro and similar pads.
func applyGamepads(in *game.Input) {
	gamepadIDs = ebiten.AppendGamepadIDs(gamepadIDs[:0])
	for _, id := range gamepadIDs {
		mapPad(in, readPad(id))
	}
	applyXInput(func(p padState) { mapPad(in, p) })
}

func readPad(id ebiten.GamepadID) padState {
	if ebiten.IsStandardGamepadLayoutAvailable(id) {
		b := func(x ebiten.StandardGamepadButton) bool { return ebiten.IsStandardGamepadButtonPressed(id, x) }
		return padState{
			Up: b(ebiten.StandardGamepadButtonLeftTop), Down: b(ebiten.StandardGamepadButtonLeftBottom),
			Left: b(ebiten.StandardGamepadButtonLeftLeft), Right: b(ebiten.StandardGamepadButtonLeftRight),
			StickX:   ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal),
			StickY:   ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical),
			FaceDown: b(ebiten.StandardGamepadButtonRightBottom), FaceRight: b(ebiten.StandardGamepadButtonRightRight),
			FaceLeft: b(ebiten.StandardGamepadButtonRightLeft), FaceUp: b(ebiten.StandardGamepadButtonRightTop),
			Start: b(ebiten.StandardGamepadButtonCenterRight), Back: b(ebiten.StandardGamepadButtonCenterLeft),
		}
	}
	// A controller missing from the database: assume the common XInput order
	// (A, B, X, Y, LB, RB, Back, Start) and the first two axes as the stick.
	b := func(n int) bool {
		return n < ebiten.GamepadButtonCount(id) && ebiten.IsGamepadButtonPressed(id, ebiten.GamepadButton(n))
	}
	axis := func(n int) float64 {
		if n < ebiten.GamepadAxisCount(id) {
			return ebiten.GamepadAxisValue(id, ebiten.GamepadAxisType(n))
		}
		return 0
	}
	return padState{
		StickX: axis(0), StickY: axis(1),
		FaceDown: b(0), FaceRight: b(1), FaceLeft: b(2), FaceUp: b(3),
		Back: b(6), Start: b(7),
	}
}

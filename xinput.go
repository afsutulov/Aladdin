package main

// XInput state layout and button bits, shared with the Windows poller so the
// mapping can be tested on every platform.

type xinputGamepad struct {
	Buttons      uint16
	LeftTrigger  uint8
	RightTrigger uint8
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

type xinputState struct {
	PacketNumber uint32
	Gamepad      xinputGamepad
}

const (
	xinputDpadUp    = 0x0001
	xinputDpadDown  = 0x0002
	xinputDpadLeft  = 0x0004
	xinputDpadRight = 0x0008
	xinputStart     = 0x0010
	xinputBack      = 0x0020
	xinputA         = 0x1000
	xinputB         = 0x2000
	xinputX         = 0x4000
	xinputY         = 0x8000
)

func xinputPad(g xinputGamepad) padState {
	b := func(mask uint16) bool { return g.Buttons&mask != 0 }
	return padState{
		Up: b(xinputDpadUp), Down: b(xinputDpadDown), Left: b(xinputDpadLeft), Right: b(xinputDpadRight),
		// XInput's Y axis points up; padState's points down.
		StickX: float64(g.ThumbLX) / 32767, StickY: -float64(g.ThumbLY) / 32767,
		FaceDown: b(xinputA), FaceRight: b(xinputB), FaceLeft: b(xinputX), FaceUp: b(xinputY),
		Start: b(xinputStart), Back: b(xinputBack),
	}
}

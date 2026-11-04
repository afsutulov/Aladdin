package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Ebitengine looks for XInput controllers only at start-up and when Windows
// reports a device change. A wired Xbox pad often finishes initialising after
// that notification, so Ebitengine never sees it. Polling XInput directly
// catches these pads; its state is merged with OR, so a pad that Ebitengine
// also reports is harmless.

const (
	xinputSlots = 4
	// XInputGetState on an empty slot can take a noticeable fraction of a
	// frame, so empty slots are probed about twice a second.
	xinputRetryTicks = 30
)

var (
	xinputGetState = loadXInputGetState()
	xinputRetry    [xinputSlots]int
)

func loadXInputGetState() *windows.LazyProc {
	for _, name := range []string{"xinput1_4.dll", "xinput1_3.dll", "xinput9_1_0.dll"} {
		dll := windows.NewLazySystemDLL(name)
		if dll.Load() != nil {
			continue
		}
		if p := dll.NewProc("XInputGetState"); p.Find() == nil {
			return p
		}
	}
	return nil
}

func applyXInput(apply func(padState)) {
	if xinputGetState == nil {
		return
	}
	for slot := range xinputRetry {
		if xinputRetry[slot] > 0 {
			xinputRetry[slot]--
			continue
		}
		var s xinputState
		if r, _, _ := xinputGetState.Call(uintptr(slot), uintptr(unsafe.Pointer(&s))); r != 0 {
			xinputRetry[slot] = xinputRetryTicks
			continue
		}
		apply(xinputPad(s.Gamepad))
	}
}

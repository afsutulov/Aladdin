package main

import (
	"aladdin/internal/game"
	"testing"
)

func TestMapPad(t *testing.T) {
	cases := []struct {
		name string
		pad  padState
		want game.Input
	}{
		{"idle", padState{StickX: 0.3, StickY: -0.2}, game.Input{}},
		{"dpad", padState{Up: true, Left: true}, game.Input{Up: true, Left: true}},
		{"stick diagonal", padState{StickX: 0.7, StickY: 0.6}, game.Input{Right: true, Down: true}},
		{"jump", padState{FaceDown: true}, game.Input{Jump: true}},
		{"sword", padState{FaceLeft: true}, game.Input{Sword: true}},
		{"apple right", padState{FaceRight: true}, game.Input{Apple: true}},
		{"apple top", padState{FaceUp: true}, game.Input{Apple: true}},
		{"start", padState{Start: true}, game.Input{Enter: true}},
		{"back", padState{Back: true}, game.Input{Escape: true}},
	}
	for _, c := range cases {
		var in game.Input
		mapPad(&in, c.pad)
		if in != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, in, c.want)
		}
	}
}

func TestMapPadKeepsKeyboard(t *testing.T) {
	in := game.Input{Left: true, Sword: true, Space: true}
	mapPad(&in, padState{})
	if !in.Left || !in.Sword || !in.Space {
		t.Fatalf("keyboard input lost: %+v", in)
	}
}

func TestXInputPad(t *testing.T) {
	p := xinputPad(xinputGamepad{Buttons: xinputA | xinputX | xinputStart | xinputDpadLeft, ThumbLY: 32767})
	var in game.Input
	mapPad(&in, p)
	want := game.Input{Jump: true, Sword: true, Enter: true, Left: true, Up: true}
	if in != want {
		t.Fatalf("got %+v, want %+v", in, want)
	}
}

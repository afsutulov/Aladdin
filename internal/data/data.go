package data

import "embed"

// Converted data only: no ROM image, CPU program, executable or emulator.
//
//go:embed assets/*
var Files embed.FS

type Frame struct {
	X, Y, W, H       int
	OffsetX, OffsetY int
	Address          int
	Box              [4]int
	Parts            []SpritePart
}

// SpritePart retains independently drawable bounds and indexed atlas artwork.
type SpritePart struct{ X, Y, W, H, OffsetX, OffsetY int }
type Stage struct {
	Index            int
	Name             string
	Music            int
	Width, Height    int
	CameraX, CameraY int
	SpawnX, SpawnY   int
	Backdrop         [3]uint8
	PaletteRGB       [64][3]uint8
	// Room entries are byte offsets in the block table; floor records use half that offset.
	Room     []uint16
	Floor    []uint8
	Contours []uint8
	RugRide  *RugRideData `json:",omitempty"`
	Entities []Marker
}
type Marker struct{ X, Y, Type int }
type Manifest struct {
	SourceSHA256 string
	RNCBlocks    int
	Frames       []Frame
	Stages       []Stage
}

// RugRideData holds the original event schedule and decoded colour cycle.
// Its scenery consists of two wrapping tile planes, with no collision room.
type RugRideData struct {
	Events    []RugEvent
	LavaCycle []PaletteChange
}
type RugEvent struct{ Kind, Value int }
type PaletteChange struct {
	Index int
	RGB   [3]uint8
}

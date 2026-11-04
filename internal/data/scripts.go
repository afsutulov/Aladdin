package data

// Instruction is converted game choreography, not a CPU instruction. Source
// addresses are labels used to audit the conversion against the supplied ROM.
// No ROM image or machine instructions are consumed by the runtime.
type Instruction struct {
	Source, Next                int
	Kind                        string
	Mode, Frame, DX, DY         int
	Field                       string
	Member, Width               int
	Value                       uint32
	Target                      int
	Template, Animation, Motion int
	OffsetX, OffsetY            int
	Callback                    string
}

type ObjectTemplate struct {
	Source                             int
	Type, HP, Flags, Flags2, Parameter int
	FlipX, FlipY                       bool
	Animation, Motion                  int
	Palette, Tiles, Behaviour          int
}

type AbuEvent struct{ Source, Delay, Kind, X, Y int }

type GenieChoice struct{ Kind, Animation int }

type MenuGlyph struct{ X, Y, Tile int }
type MenuSound struct {
	ID    int
	Label string
}
type AttractDemo struct {
	Stage  int
	Inputs []byte
}
type CreditOp struct {
	Kind            string
	Glyphs          []MenuGlyph
	EndBank, Target int
}
type MenuData struct {
	Demos         []AttractDemo
	Credits       []CreditOp
	CreditPalette [64][3]uint8
	CreditHeading [16][3]uint8
	Palette       [64][3]uint8
	Text          map[string][]MenuGlyph
	Sounds        []MenuSound
	EndBanks      map[string]int
}

// StoryOp is offline-converted tile text, never processor instructions.
type StoryOp struct {
	Kind                  string
	TransferWait          int
	Glyphs                []MenuGlyph
	Count, Template, X, Y int
}
type StoryPage struct {
	Source, Text, Foreground int
	ReloadWait               int
	ReloadRows               []int
	ReloadRaster             []int
	ReloadRasterColumns      []int
	ReloadGlyphDelay         int
	Palette                  [2]int
	Ops                      []StoryOp
	Scroll                   bool
}
type StoryLayout struct{ Source, Background, Palette, Characters int }
type StoryStep struct {
	Layout, Page      int
	LoadFrames        int
	SkippedLoadFrames int
	SkipBeforeLoad    bool
	CheckSkip         bool
}
type StoryPlan struct {
	Music int
	Steps []StoryStep
}
type StoryData struct {
	Pages    map[int]StoryPage
	Layouts  map[int]StoryLayout
	Plans    map[int]StoryPlan
	Palettes map[int][16][3]uint8
}
type PreludeData struct {
	Maps                                                     map[int]int
	ScarabPalette                                            [64][3]uint8
	ScarabFlash                                              [16][3]uint8
	ScarabCycle                                              [32][3]uint8
	CardPalette                                              [64][3]uint8
	ScarabLoad, ScarabWarm, CardLoad, CardMapLoad, StoryLoad int
}

// EndingData holds converted artwork and source resource-job presentation waits.
type EndingData struct {
	Maps                       map[int]int
	FlightPalette, KissPalette [64][3]uint8
	FlightLoad, KissLoad       int
}
type ScriptSet struct {
	Ending               EndingData
	LevelTitle           LevelTitleData
	Prelude              PreludeData
	Story                StoryData
	Menu                 MenuData
	GameOverPalette      [64][3]uint8
	GameOverFinalPalette [64][3]uint8
	GameOverCycle        [28][3]uint8
	ContinuePalette      [64][3]uint8
	GeniePalette         [64][3]uint8
	GenieChoices         []GenieChoice
	GenieDigits          []int
	GeniePulse           [2][5]PaletteChange
	AbuSchedules         map[int][]AbuEvent
	Animation            map[int]Instruction
	Motion               map[int]Instruction
	Templates            map[int]ObjectTemplate
	Entry                map[string]int
	Callbacks            []string
	CameraX, CameraY     []uint8
	CameraXPrefix        []uint8
	BounceFacing         []uint8
	HUDWave              []int
	ScreenShake          []int
	DeathPalette         [64][3]uint8
	Messages             map[int]Message
	LevelClearPalette    [64][3]uint8
	Route                []RouteStep
	Palettes             map[int][16][3]uint8
}

// LevelTitleData contains converted art, colors and the source wave increments.
type LevelTitleData struct {
	Maps       map[int]int
	Palette    [64][3]uint8
	Wave       [32]int
	LoadFrames int
}

type RouteStep struct{ Mode, Stage int }

// Message glyphs use the game's existing Chopper artwork and choreography.
type MessageGlyph struct{ X, Y, Animation, Motion int }
type Message struct {
	Glyphs   []MessageGlyph
	Callback string
}

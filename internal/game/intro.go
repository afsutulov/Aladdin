package game

import (
	"aladdin/internal/data"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

// PublisherIntro is the Disney Software / Virgin Games co-production intro
// that the original shows at power-on, before the title menu: the logo fades
// in, the Genie flies across it scattering sparkles, and the title theme
// starts on video boundary 69.
//
// The scene has no game logic beyond its timeline: its image is converted
// per video boundary (palette, plane, sprite cels and the sprite list),
// together with the original frame counter (FF7E38) and random seed (FF7DFA)
// that the intro advances. The seed and counter are carried into the title
// menu, so later gameplay randomness continues from the original values.
//
// Original rules reproduced here:
//   - A, B, C or Start held on any boundary from 90 to 369 ends the logo at
//     once; the screen stays black for 64 boundaries, then the title appears.
//   - Without input the last logo boundary is 375, followed by 73 black ones.
//   - Directions never skip. The title theme keeps playing into the menu.
type PublisherIntro struct {
	World *NativeWorld
	Frame int // next intro boundary
	Shown int // boundary whose image is displayed; -1 while black
	Gap   int // remaining black boundaries before the title
	Tick  int
	Seed  uint32
}

type introData struct {
	MusicFrame int `json:"music_frame"`
	SkipFirst  int `json:"skip_first"`
	SkipLast   int `json:"skip_last"`
	NaturalGap int `json:"natural_gap"`
	SkipGap    int `json:"skip_gap"`
	Palettes   [][64][3]uint8
	Cels       [][5]int // atlas x, y, width, height, palette line
	Frames     []introFrame
	plane      *image.Paletted
	sprites    *image.Paletted
}

// introFrame is [palette, tick, seed, [cel, x, y]...].
type introFrame struct {
	Palette, Tick int
	Seed          uint32
	Sprites       [][3]int
}

func (f *introFrame) UnmarshalJSON(b []byte) error {
	var raw [4]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	var flat []int
	for i, dst := range []any{&f.Palette, &f.Tick, &f.Seed, &flat} {
		if err := json.Unmarshal(raw[i], dst); err != nil {
			return err
		}
	}
	if len(flat)%3 != 0 {
		return fmt.Errorf("intro sprite list")
	}
	for i := 0; i < len(flat); i += 3 {
		f.Sprites = append(f.Sprites, [3]int{flat[i], flat[i+1], flat[i+2]})
	}
	return nil
}

func (p *Project) introData() (*introData, error) {
	if p.intro != nil {
		return p.intro, nil
	}
	b, err := data.Files.ReadFile("assets/intro.json")
	if err != nil {
		return nil, err
	}
	d := &introData{}
	if err = json.Unmarshal(b, d); err != nil {
		return nil, err
	}
	for name, dst := range map[string]**image.Paletted{"intro-plane.png": &d.plane, "intro-sprites.png": &d.sprites} {
		im, err := p.Image(name)
		if err != nil {
			return nil, err
		}
		pi, ok := im.(*image.Paletted)
		if !ok {
			return nil, fmt.Errorf("%s lost palette indices", name)
		}
		*dst = pi
	}
	if len(d.Frames) == 0 || d.SkipLast >= len(d.Frames) || d.MusicFrame >= len(d.Frames) {
		return nil, fmt.Errorf("incomplete intro data")
	}
	for _, f := range d.Frames {
		if f.Palette < 0 || f.Palette >= len(d.Palettes) {
			return nil, fmt.Errorf("intro palette")
		}
		for _, s := range f.Sprites {
			if s[0] < 0 || s[0] >= len(d.Cels) {
				return nil, fmt.Errorf("intro cel")
			}
		}
	}
	p.intro = d
	return d, nil
}

// OpenIntro starts the program the way the original powers on.
func (g *Game) OpenIntro() {
	d, err := g.Project.introData()
	if err != nil {
		g.Err = err
		return
	}
	if len(d.Frames) == 0 {
		g.Err = fmt.Errorf("incomplete intro data")
		return
	}
	w := NewNativeWorld(&g.Project.Manifest.Stages[1], &g.Project.Scripts, g.Project.Manifest.Frames)
	w.Seed = 0 // the original clears work RAM at power-on
	g.Intro = &PublisherIntro{World: w, Shown: -1}
	g.Menu = nil
	g.Mode = 0
}

func (g *Game) updateIntro(in Input) {
	d, err := g.Project.introData()
	if err != nil {
		g.Err = err
		return
	}
	it := g.Intro
	if it.Gap > 0 {
		it.Shown = -1
		it.Gap--
		if it.Gap == 0 {
			g.Intro = nil
			g.openTitleFrom(it.World, it.Tick, it.Seed)
		}
		return
	}
	f := it.Frame
	if f >= d.SkipFirst && f <= d.SkipLast && (in.Jump || in.Sword || in.Apple || in.Enter) {
		// The skip is read before this boundary's work: the counter and seed
		// keep the previous boundary's values.
		it.Shown, it.Gap = -1, d.SkipGap
		return
	}
	fr := d.Frames[f]
	it.Shown, it.Tick, it.Seed = f, fr.Tick, fr.Seed
	if f == d.MusicFrame {
		it.World.requestMusic(26)
	}
	it.Frame++
	if it.Frame == len(d.Frames) {
		it.Gap = d.NaturalGap + 1 // this boundary still shows the last logo image
	}
}

func (g *Game) renderIntro() *image.RGBA {
	d, err := g.Project.introData()
	it := g.Intro
	if err != nil || it.Shown < 0 {
		draw.Draw(g.Canvas, g.Canvas.Rect, image.Black, image.Point{}, draw.Src)
		return g.Canvas
	}
	fr := &d.Frames[it.Shown]
	pal := d.Palettes[fr.Palette]
	// Colour indices: backdrop 0, plane B, then sprites with the display's
	// per-line limits (20 sprites, 320 sprite pixels; earlier sprites win).
	var idx [Height][Width]uint8
	for y := 0; y < Height; y++ {
		row := d.plane.Pix[y*d.plane.Stride:]
		for x := 0; x < Width; x++ {
			idx[y][x] = row[x]
		}
	}
	type span struct{ cel, x, y, width int }
	var drawn [20]span
	for y := 0; y < Height; y++ {
		n, budget := 0, Width
		for _, s := range fr.Sprites {
			c := d.Cels[s[0]]
			if y < s[2] || y >= s[2]+c[3] {
				continue
			}
			if n == len(drawn) {
				break
			}
			take := min(c[2], budget)
			budget -= take
			drawn[n] = span{s[0], s[1], s[2], take}
			n++
			if budget == 0 {
				break
			}
		}
		for i := n - 1; i >= 0; i-- {
			sp := drawn[i]
			c := d.Cels[sp.cel]
			row := d.sprites.Pix[(c[1]+y-sp.y)*d.sprites.Stride+c[0]:]
			for x := 0; x < sp.width; x++ {
				if X := sp.x + x; X >= 0 && X < Width && row[x] != 0 {
					idx[y][X] = row[x] + uint8(c[4]*16)
				}
			}
		}
	}
	var lut [64]color.RGBA
	for i, c := range pal {
		lut[i] = color.RGBA{c[0], c[1], c[2], 255}
	}
	pix := g.Canvas.Pix
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			c := lut[idx[y][x]&63]
			o := y*g.Canvas.Stride + x*4
			pix[o], pix[o+1], pix[o+2], pix[o+3] = c.R, c.G, c.B, 255
		}
	}
	return g.Canvas
}

package main

import (
	"aladdin/internal/data"
	"aladdin/internal/game"
	"aladdin/internal/sound"
	"aladdin/internal/timing"
	"bytes"
	_ "embed"
	"flag"
	"image"
	"image/png"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

//go:embed icon.png
var icon []byte

type desktop struct {
	clock     timing.Clock
	dev       bool // development hotkeys (F1/F2/F3/R); off for normal play
	game      *game.Game
	mixer     *sound.Mixer
	player    *audio.Player
	stopAfter int
	capture   string
	ticks     int
	// fresh is set when a game step ran since the last Draw. The software
	// renderer is only worth running once per step: on 120/144/165 Hz
	// monitors Ebitengine calls Draw two or three times per Update, and the
	// screen keeps its previous contents (SetScreenClearedEveryFrame(false)).
	fresh bool
}

func (d *desktop) Update() error {
	pressed := func(keys ...ebiten.Key) bool {
		for _, k := range keys {
			if ebiten.IsKeyPressed(k) {
				return true
			}
		}
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	in := game.Input{Left: pressed(ebiten.KeyArrowLeft, ebiten.KeyA), Right: pressed(ebiten.KeyArrowRight, ebiten.KeyD), Up: pressed(ebiten.KeyArrowUp, ebiten.KeyW), Down: pressed(ebiten.KeyArrowDown, ebiten.KeyS), Jump: pressed(ebiten.KeySpace, ebiten.KeyZ), Sword: pressed(ebiten.KeyX), Apple: pressed(ebiten.KeyC), Enter: pressed(ebiten.KeyEnter), Escape: pressed(ebiten.KeyEscape), Space: pressed(ebiten.KeySpace)}
	if d.dev {
		// The original has no such controls; they would let a player reset a
		// level by accident, so they exist only in development launches.
		in.Free, in.Debug, in.Reset, in.View = pressed(ebiten.KeyF2), pressed(ebiten.KeyF1), pressed(ebiten.KeyR), pressed(ebiten.KeyF3)
	}
	if !d.clock.Advance() {
		return nil
	}
	d.fresh = true
	if d.game.Update(in) {
		return ebiten.Termination
	}
	if d.game.Err != nil {
		return d.game.Err
	}
	if d.mixer != nil {
		out := d.game.TakeSoundOutput()
		if out.Reset {
			d.mixer.StopAll()
		}
		d.mixer.SetEnabled(out.Enabled)
		d.mixer.SetMusicEnabled(out.MusicEnabled)
		if out.MusicChanged && out.MusicID >= 0 {
			if err := d.mixer.Command(out.MusicID, false); err != nil {
				return err
			}
		}
		for _, event := range out.Events {
			if err := d.mixer.Command(event.ID, event.Overlay); err != nil {
				return err
			}
		}
	}
	d.ticks++
	if d.stopAfter > 0 && d.ticks >= d.stopAfter {
		if d.capture != "" {
			f, e := os.Create(d.capture)
			if e != nil {
				return e
			}
			e = png.Encode(f, d.game.Render())
			f.Close()
			if e != nil {
				return e
			}
		}
		return ebiten.Termination
	}
	return nil
}
func (d *desktop) Draw(screen *ebiten.Image) {
	if !d.fresh {
		return
	}
	d.fresh = false
	screen.WritePixels(d.game.Render().Pix)
}
func (d *desktop) Layout(_, _ int) (int, int) { return game.Width, game.Height }
func main() {
	mute := flag.Bool("mute", false, "disable audio output")
	scenes := flag.Bool("scenes", false, "open the development scene selector")
	stage := flag.Int("stage", -1, "open a converted scene directly (0..13)")
	bonus := flag.Int("genie", 0, "open the Genie bonus with this many tokens (1..255)")
	lastLife := flag.Int("last-life", -1, "preview loss of the last life with 0..255 continues")
	stop := flag.Int("frames", 0, "exit after this many updates; 0 runs normally")
	capture := flag.String("capture", "", "save a native-renderer PNG when -frames finishes")
	dev := flag.Bool("dev", false, "enable development hotkeys (F1 contours, F2 scene selector/free camera, F3 sprite viewer, R reset)")
	flag.Parse()
	p, e := game.Load()
	if e != nil {
		log.Fatal(e)
	}
	development := *dev || *scenes || *stage >= 0 || *bonus != 0 || *lastLife >= 0
	var g *game.Game
	if development {
		g = game.New(p)
	} else {
		// Normal launch: the original power-on Disney/Virgin intro, then the title.
		g = game.NewFromPowerOn(p)
	}
	if *scenes {
		g.Mode = 3
	}
	if *stage >= 0 {
		if *stage > 13 {
			log.Fatal("scene is not converted")
		}
		g.Start(*stage)
	}
	if *bonus < 0 || *bonus > 255 {
		log.Fatal("Genie tokens must be between 1 and 255")
	}
	if *lastLife < -1 || *lastLife > 255 || *lastLife >= 0 && *bonus > 0 {
		log.Fatal("last-life must be 0..255 and cannot be combined with genie")
	}
	if *lastLife >= 0 {
		g.PreviewLastLifeLoss(byte(*lastLife))
	}
	if *bonus > 0 {
		g.StartGenie(byte(*bonus))
	}
	if im, e := png.Decode(bytes.NewReader(icon)); e == nil {
		ebiten.SetWindowIcon([]image.Image{im})
	}
	ebiten.SetTPS(timing.HostTPS)
	ebiten.SetWindowTitle("Aladdin")
	ebiten.SetWindowSize(960, 672)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenClearedEveryFrame(false)
	ebiten.SetScreenFilterEnabled(false)
	d := &desktop{game: g, stopAfter: *stop, capture: *capture, dev: development}
	if !*mute {
		files, err := fs.Sub(data.Files, "assets")
		if err != nil {
			log.Fatal(err)
		}
		d.mixer, err = sound.Load(files)
		if err != nil {
			log.Fatal(err)
		}
		context := audio.NewContext(sound.SampleRate)
		d.player, err = context.NewPlayer(d.mixer)
		if err != nil {
			log.Fatal(err)
		}
		d.player.SetBufferSize(time.Second / 60)
		defer d.player.Close()
		d.player.Play()
	}
	if e = ebiten.RunGame(d); e != nil {
		log.Fatal(e)
	}
}

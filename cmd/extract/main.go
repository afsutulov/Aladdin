// extract converts the user's exact Final Cut image into data consumed by the
// native renderer. Run once when rebuilding assets; the desktop never reads ROM.
package main

import (
	"aladdin/internal/data"
	"aladdin/internal/rom"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

var names = map[int]string{0: "Agrabah Rooftops", 1: "Agrabah Market", 2: "Abu in Agrabah", 3: "The Desert", 4: "Sultan's Dungeon", 5: "Cave of Wonders", 6: "Abu in the Cave", 7: "The Escape", 8: "Rug Ride", 9: "Inside the Lamp", 10: "Sultan's Palace", 11: "Jafar's Palace", 12: "Iago", 13: "Abu in the Palace"}

func savePNG(p string, im image.Image) {
	f, e := os.Create(p)
	if e != nil {
		log.Fatal(e)
	}
	defer f.Close()
	if e = png.Encode(f, im); e != nil {
		log.Fatal(e)
	}
}
func palette(r rom.ROM, a int) color.Palette {
	p := make(color.Palette, 64)
	lv := rom.ColorLevels
	for i := range p {
		v := r.U16(a + i*2)
		alpha := uint8(255)
		if i%16 == 0 {
			alpha = 0
		}
		p[i] = color.NRGBA{lv[v>>1&7], lv[v>>5&7], lv[v>>9&7], alpha}
	}
	return p
}
func tile(chars []byte, v int, p color.Palette) *image.Paletted {
	im := image.NewPaletted(image.Rect(0, 0, 8, 8), p)
	a := (v & 2047) * 32
	if a+32 > len(chars) {
		return im
	}
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			c := chars[a+y*4+x/2]
			if x%2 == 0 {
				c >>= 4
			}
			c &= 15
			if c == 0 {
				continue
			}
			xx, yy := x, y
			if v&0x800 != 0 {
				xx = 7 - x
			}
			if v&0x1000 != 0 {
				yy = 7 - y
			}
			im.SetColorIndex(xx, yy, uint8(v>>13&3)*16+c)
		}
	}
	return im
}
func copyTileIndices(dst *image.Paletted, x, y int, src *image.Paletted) {
	for yy := 0; yy < 8; yy++ {
		for xx := 0; xx < 8; xx++ {
			dst.SetColorIndex(x+xx, y+yy, src.ColorIndexAt(xx, yy))
		}
	}
}
func main() {
	in := flag.String("rom", "", "path to the supplied .gen file")
	out := flag.String("out", "internal/data/assets", "converted asset directory")
	flag.Parse()
	if *in == "" {
		flag.Usage()
		os.Exit(2)
	}
	b, e := os.ReadFile(*in)
	if e != nil {
		log.Fatal(e)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(b))
	if hash != "e18516722c0ff3466bf6f4d0c1092cc4bdab926c846db37876394ac300849b65" {
		log.Fatal("unsupported image: importer is qualified only for the supplied Final Cut SHA-256")
	}
	if e = os.MkdirAll(*out, 0755); e != nil {
		log.Fatal(e)
	}
	r := rom.ROM{Bytes: b}
	scripts, e := r.ImportScripts()
	if e != nil {
		log.Fatal(e)
	}
	scriptFile, e := os.Create(filepath.Join(*out, "scripts.json"))
	if e != nil {
		log.Fatal(e)
	}
	if e = json.NewEncoder(scriptFile).Encode(scripts); e != nil {
		log.Fatal(e)
	}
	scriptFile.Close()
	fmt.Printf("converted %d animation records, %d movement records and %d object templates\n", len(scripts.Animation), len(scripts.Motion), len(scripts.Templates))
	blocks := map[int][]byte{}
	for a := 0; a+18 < len(b); a++ {
		if string(b[a:a+4]) != "RNC\x01" {
			continue
		}
		p, e := rom.UnpackRNC1(b[a:])
		if e != nil {
			log.Fatalf("block %06x: %v", a, e)
		}
		blocks[a] = p
	}
	mp := make(color.Palette, 64)
	for i, c := range scripts.Menu.Palette {
		a := uint8(255)
		if i%16 == 0 {
			a = 0
		}
		mp[i] = color.NRGBA{c[0], c[1], c[2], a}
	}
	for _, planeInfo := range []struct {
		name    string
		address int
	}{{"a", 0x157c0e}, {"b", 0x15581a}} {
		for _, high := range []bool{false, true} {
			im := image.NewPaletted(image.Rect(0, 0, 320, 224), mp)
			plane := blocks[planeInfo.address]
			for y := 0; y < 28; y++ {
				for x := 0; x < 40; x++ {
					off := (y*64 + x) * 2
					v := int(plane[off])<<8 | int(plane[off+1])
					if (v&0x8000 != 0) != high {
						continue
					}
					t := tile(blocks[0x15f036], v, mp)
					for yy := 0; yy < 8; yy++ {
						for xx := 0; xx < 8; xx++ {
							i := t.ColorIndexAt(xx, yy)
							if i%16 != 0 {
								im.SetColorIndex(x*8+xx, y*8+yy, i)
							}
						}
					}
				}
			}
			priority := "low"
			if high {
				priority = "high"
			}
			savePNG(filepath.Join(*out, "menu-"+planeInfo.name+"-"+priority+".png"), im)
		}
	}
	// Story maps retain palette indices, tile priority and all 32 rows.
	storyMaps := map[int]int{}
	for address, characters := range scripts.Ending.Maps {
		storyMaps[address] = characters
	}
	for address, characters := range scripts.LevelTitle.Maps {
		storyMaps[address] = characters
	}
	for address, characters := range scripts.Prelude.Maps {
		storyMaps[address] = characters
	}
	for _, v := range scripts.Story.Layouts {
		if v.Background != 0 {
			storyMaps[v.Background] = v.Characters
		}
	}
	for _, v := range scripts.Story.Pages {
		if v.Foreground != 0 {
			storyMaps[v.Foreground] = 0x15b6b2
		}
	}
	for address, characters := range storyMaps {
		for _, high := range []bool{false, true} {
			im := image.NewPaletted(image.Rect(0, 0, 512, 256), mp)
			plane := blocks[address]
			if len(plane)%128 != 0 || len(plane) > 4096 {
				log.Fatalf("story map %x has %d bytes", address, len(plane))
			}
			for y := 0; y < len(plane)/128; y++ {
				for x := 0; x < 64; x++ {
					off := (y*64 + x) * 2
					v := int(plane[off])<<8 | int(plane[off+1])
					if (v&0x8000 != 0) != high {
						continue
					}
					t := tile(blocks[characters], v, mp)
					for yy := 0; yy < 8; yy++ {
						for xx := 0; xx < 8; xx++ {
							im.SetColorIndex(x*8+xx, y*8+yy, t.ColorIndexAt(xx, yy))
						}
					}
				}
			}
			priority := "low"
			if high {
				priority = "high"
			}
			savePNG(filepath.Join(*out, fmt.Sprintf("story-%06x-%s.png", address, priority)), im)
		}
	}
	font := image.NewPaletted(image.Rect(0, 0, 128, 32), rom.MansPalette())
	for i := 0; i < 64; i++ {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				v := b[0x152622+i*32+y*4+x/2]
				if x%2 == 0 {
					v >>= 4
				}
				font.SetColorIndex((i%16)*8+x, (i/16)*8+y, v&15)
			}
		}
	}
	savePNG(filepath.Join(*out, "menu-font.png"), font)
	// Original Level Complete plane. It uses 64 columns, as do the other
	// interlude tile maps. Sprite artwork is supplied by the existing atlas.
	clearPalette := make(color.Palette, 64)
	for i, rgb := range scripts.LevelClearPalette {
		a := uint8(255)
		if i%16 == 0 {
			a = 0
		}
		clearPalette[i] = color.NRGBA{rgb[0], rgb[1], rgb[2], a}
	}
	clearImage := image.NewRGBA(image.Rect(0, 0, 320, 224))
	bg := scripts.LevelClearPalette[0]
	draw.Draw(clearImage, clearImage.Rect, &image.Uniform{C: color.RGBA{bg[0], bg[1], bg[2], 255}}, image.Point{}, draw.Src)
	chars, plane := blocks[0x164aa0], blocks[0x159b21]
	for y := 0; y < 28; y++ {
		for x := 0; x < 40; x++ {
			off := (y*64 + x) * 2
			v := int(plane[off])<<8 | int(plane[off+1])
			draw.Draw(clearImage, image.Rect(x*8, y*8, x*8+8, y*8+8), tile(chars, v, clearPalette), image.Point{}, draw.Over)
		}
	}
	savePNG(filepath.Join(*out, "level-complete.png"), clearImage)
	for _, scene := range []struct {
		name         string
		chars, plane int
	}{{"abu-intro.png", 0x164aa0, 0x159f54}, {"abu-failed.png", 0x15a110, 0x159da6}} {
		im := image.NewRGBA(image.Rect(0, 0, 320, 224))
		draw.Draw(im, im.Rect, &image.Uniform{C: color.RGBA{bg[0], bg[1], bg[2], 255}}, image.Point{}, draw.Src)
		for y := 0; y < 28; y++ {
			for x := 0; x < 40; x++ {
				off := (y*64 + x) * 2
				plane := blocks[scene.plane]
				v := int(plane[off])<<8 | int(plane[off+1])
				draw.Draw(im, image.Rect(x*8, y*8, x*8+8, y*8+8), tile(blocks[scene.chars], v, clearPalette), image.Point{}, draw.Over)
			}
		}
		savePNG(filepath.Join(*out, scene.name), im)
	}
	for _, scene := range []struct {
		name    string
		planes  []int
		palette [64][3]uint8
	}{{"gameover.png", []int{0x158860, 0x158633}, scripts.GameOverPalette}, {"gameover-final.png", []int{0x158a2f}, scripts.GameOverFinalPalette}} {
		pal := make(color.Palette, 64)
		for i, c := range scene.palette {
			a := uint8(255)
			if i%16 == 0 {
				a = 0
			}
			pal[i] = color.NRGBA{c[0], c[1], c[2], a}
		}
		im := image.NewPaletted(image.Rect(0, 0, 320, 224), pal)
		for _, addr := range scene.planes {
			plane := blocks[addr]
			for y := 0; y < 28; y++ {
				for x := 0; x < 40; x++ {
					off := (y*64 + x) * 2
					v := int(plane[off])<<8 | int(plane[off+1])
					t := tile(blocks[0x164aa0], v, pal)
					for yy := 0; yy < 8; yy++ {
						for xx := 0; xx < 8; xx++ {
							index := t.ColorIndexAt(xx, yy)
							if index%16 != 0 {
								im.SetColorIndex(x*8+xx, y*8+yy, index)
							}
						}
					}
				}
			}
		}
		savePNG(filepath.Join(*out, scene.name), im)
	}
	cp := make(color.Palette, 64)
	for i, rgb := range scripts.ContinuePalette {
		a := uint8(255)
		if i%16 == 0 {
			a = 0
		}
		cp[i] = color.NRGBA{rgb[0], rgb[1], rgb[2], a}
	}
	ci := image.NewRGBA(image.Rect(0, 0, 320, 224))
	cback := scripts.ContinuePalette[0]
	draw.Draw(ci, ci.Rect, &image.Uniform{C: color.RGBA{cback[0], cback[1], cback[2], 255}}, image.Point{}, draw.Src)
	for y := 0; y < 28; y++ {
		for x := 0; x < 40; x++ {
			off := (y*64 + x) * 2
			plane := blocks[0x158b93]
			v := int(plane[off])<<8 | int(plane[off+1])
			draw.Draw(ci, image.Rect(x*8, y*8, x*8+8, y*8+8), tile(blocks[0x164aa0], v, cp), image.Point{}, draw.Over)
		}
	}
	savePNG(filepath.Join(*out, "continue.png"), ci)
	gp := make(color.Palette, 64)
	for i, rgb := range scripts.GeniePalette {
		a := uint8(255)
		if i%16 == 0 {
			a = 0
		}
		gp[i] = color.NRGBA{rgb[0], rgb[1], rgb[2], a}
	}
	// Compose both original planes at their actual priority, retaining indices.
	for _, priority := range []bool{false, true} {
		im := image.NewPaletted(image.Rect(0, 0, 320, 224), gp)
		for _, address := range []int{0x158126, 0x157e36} {
			plane := blocks[address]
			for y := 0; y < 28; y++ {
				for x := 0; x < 40; x++ {
					off := (y*64 + x) * 2
					v := int(plane[off])<<8 | int(plane[off+1])
					if (v&0x8000 != 0) != priority {
						continue
					}
					t := tile(blocks[0x162fbe], v, gp)
					for yy := 0; yy < 8; yy++ {
						for xx := 0; xx < 8; xx++ {
							index := t.ColorIndexAt(xx, yy)
							if index%16 != 0 {
								im.SetColorIndex(x*8+xx, y*8+yy, index)
							}
						}
					}
				}
			}
		}
		name := "genie-low.png"
		if priority {
			name = "genie-high.png"
		}
		savePNG(filepath.Join(*out, name), im)
	}
	m := data.Manifest{SourceSHA256: hash, RNCBlocks: len(blocks)}
	fs := r.Frames()
	// The HUD's original uncompressed tile banks, including score digits.
	hud := image.NewPaletted(image.Rect(0, 0, 128, 192), rom.MansPalette())
	for _, bank := range []struct{ tile, address, count int }{{0x680, 0x1453c0, 128}, {0x7c0, 0x146320, 40}} {
		for i := 0; i < bank.count; i++ {
			slot := bank.tile - 0x680 + i
			for yy := 0; yy < 8; yy++ {
				for xx := 0; xx < 8; xx++ {
					v := b[bank.address+i*32+yy*4+xx/2]
					if xx&1 == 0 {
						v >>= 4
					}
					hud.SetColorIndex((slot%16)*8+xx, (slot/16)*8+yy, v&15)
				}
			}
		}
	}
	savePNG(filepath.Join(*out, "hud.png"), hud)
	atlas := image.NewPaletted(image.Rect(0, 0, 2048, 8192), rom.MansPalette())
	x, y, rowH := 1, 1, 0
	for _, f := range fs {
		im := r.RenderFrame(f)
		minX, minY, maxX, maxY := 256, 256, 0, 0
		for yy := 0; yy < 256; yy++ {
			for xx := 0; xx < 256; xx++ {
				if im.ColorIndexAt(xx, yy) != 0 {
					minX = min(minX, xx)
					minY = min(minY, yy)
					maxX = max(maxX, xx+1)
					maxY = max(maxY, yy+1)
				}
			}
		}
		if maxX <= minX {
			minX, minY, maxX, maxY = 128, 128, 129, 129
		}
		w, h := maxX-minX, maxY-minY
		if x+w+1 > 2048 {
			x = 1
			y += rowH + 2
			rowH = 0
		}
		if y+h > 8192 {
			log.Fatal("sprite atlas overflow")
		}
		draw.Draw(atlas, image.Rect(x, y, x+w, y+h), im, image.Pt(minX, minY), draw.Src)
		m.Frames = append(m.Frames, data.Frame{X: x, Y: y, W: w, H: h, OffsetX: minX - 128, OffsetY: minY - 128, Address: f.Address, Box: f.Box})
		x += w + 2
		rowH = max(rowH, h)
	}
	// Keep independent parts for the original visibility gate and overlap order.
	y += rowH + 2
	x, rowH = 1, 0
	for i, f := range fs {
		for _, p := range f.Parts {
			if x+p.W+1 > 2048 {
				x = 1
				y += rowH + 2
				rowH = 0
			}
			if y+p.H > 8192 {
				log.Fatal("sprite part atlas overflow")
			}
			im := r.RenderPart(p)
			for yy := 0; yy < p.H; yy++ {
				for xx := 0; xx < p.W; xx++ {
					atlas.SetColorIndex(x+xx, y+yy, im.ColorIndexAt(xx, yy))
				}
			}
			m.Frames[i].Parts = append(m.Frames[i].Parts, data.SpritePart{X: x, Y: y, W: p.W, H: p.H, OffsetX: p.X - 128, OffsetY: p.Y - 128})
			x += p.W + 2
			rowH = max(rowH, p.H)
		}
	}
	savePNG(filepath.Join(*out, "sprites.png"), atlas.SubImage(image.Rect(0, 0, 2048, y+rowH+1)))
	for index := 0; index <= 13; index++ {
		h := 0x3250 + index*0x42
		floor, chars, room := blocks[r.U32(h+8)], blocks[r.U32(h+12)], blocks[r.U32(h+16)]
		w, hh := r.U16(h+48), r.U16(h+50)
		if index == 8 {
			// This stage scrolls two wrapping planes and spawns actors from
			// an event schedule. Its header's nominal width is a word value,
			// not the dimensions of an ordinary decompressed room.
			p := palette(r, r.U32(h+32))
			c := p[0].(color.NRGBA)
			s := data.Stage{Index: index, Name: names[index], Music: r.U16(h + 0x1a), Width: (w * 16) & 65535, Height: hh * 16,
				CameraX: r.U16(h), CameraY: r.U16(h + 2), SpawnX: r.U16(h) + r.U16(h+4), SpawnY: r.U16(h+2) + r.U16(h+6) - 256,
				Backdrop: [3]uint8{c.R, c.G, c.B}, RugRide: &data.RugRideData{}}
			for i, c := range p {
				rgb := c.(color.NRGBA)
				s.PaletteRGB[i] = [3]uint8{rgb.R, rgb.G, rgb.B}
			}
			for a := 0x2b59; a <= 0x2f7b; a += 2 {
				s.RugRide.Events = append(s.RugRide.Events, data.RugEvent{Kind: int(b[a]), Value: int(b[a+1])})
			}
			for a := 0x2fb8; a < 0x3018; a += 6 {
				v := r.U16(a + 4)
				s.RugRide.LavaCycle = append(s.RugRide.LavaCycle, data.PaletteChange{Index: (r.U16(a) & 127) / 2, RGB: [3]uint8{rom.ColorLevels[v>>1&7], rom.ColorLevels[v>>5&7], rom.ColorLevels[v>>9&7]}})
			}
			layers := []struct {
				name         string
				plane, chars []byte
			}{{"", blocks[0x153ec6], chars}, {"bg-", blocks[r.U32(h+28)], chars}}
			// The eight animation records replace alternating tile banks.
			// Decode the DMA source/destination/length at build time; only
			// complete PNG planes and a phase counter reach the runtime.
			for phase := 0; phase < 8; phase++ {
				animated := make([]byte, 1017*32)
				copy(animated, chars)
				for _, record := range []int{phase &^ 1, ((phase - 1) | 1) & 7} {
					a := r.U32(h+20) + record*14
					cmd := uint32(r.U32(a))
					dest := int((cmd>>16)&0x3fff | (cmd&3)<<14)
					count := int(b[a+5]) | int(b[a+7])<<8
					source := (int(b[a+9]) | int(b[a+11])<<8 | int(b[a+13]&127)<<16) * 2
					copy(animated[dest:dest+count*2], b[source:source+count*2])
				}
				layers = append(layers, struct {
					name         string
					plane, chars []byte
				}{fmt.Sprintf("bg%d-", phase), blocks[r.U32(h+28)], animated})
			}
			for _, layer := range layers {
				low, high := image.NewPaletted(image.Rect(0, 0, 512, 224), p), image.NewPaletted(image.Rect(0, 0, 512, 224), p)
				for yy := 0; yy < 28; yy++ {
					for xx := 0; xx < 64; xx++ {
						o := (yy*64 + xx) * 2
						v := int(layer.plane[o])<<8 | int(layer.plane[o+1])
						target := low
						if v&0x8000 != 0 {
							target = high
						}
						t := tile(layer.chars, v, p)
						// Preserve original palette indices even where two
						// initial colours coincide; lava cycles separate them.
						for row := 0; row < 8; row++ {
							copy(target.Pix[target.PixOffset(xx*8, yy*8+row):][:8], t.Pix[t.PixOffset(0, row):][:8])
						}
					}
				}
				if layer.name == "" {
					savePNG(filepath.Join(*out, "stage08-low.png"), low)
					savePNG(filepath.Join(*out, "stage08-high.png"), high)
				} else {
					name := "stage08-bg"
					if layer.name != "bg-" {
						name = fmt.Sprintf("stage08-bg-%s", layer.name[2:3])
					}
					savePNG(filepath.Join(*out, name+".png"), low)
					savePNG(filepath.Join(*out, name+"-high.png"), high)
				}
			}
			m.Stages = append(m.Stages, s)
			fmt.Printf("stage 08: Rug Ride, two wrapping planes, %d events\n", len(s.RugRide.Events))
			continue
		}
		if len(room) != w*hh*2 || len(chars) == 0 {
			fmt.Printf("stage %d: streamed layout not converted\n", index)
			continue
		}
		p, defs := palette(r, r.U32(h+32)), r.U32(h+36)
		bgColor := p[0].(color.NRGBA)
		s := data.Stage{Index: index, Name: names[index], Music: r.U16(h + 0x1a), Width: w * 16, Height: hh * 16, CameraX: r.U16(h), CameraY: r.U16(h + 2), SpawnX: r.U16(h) + r.U16(h+4), SpawnY: r.U16(h+2) + r.U16(h+6) - 256, Backdrop: [3]uint8{bgColor.R, bgColor.G, bgColor.B}, Floor: floor, Contours: append([]byte(nil), b[0x35ec:0x45ec]...)}
		for i, c := range p {
			rgb := c.(color.NRGBA)
			s.PaletteRGB[i] = [3]uint8{rgb.R, rgb.G, rgb.B}
		}
		low := image.NewPaletted(image.Rect(0, 0, s.Width, s.Height), p)
		high := image.NewPaletted(low.Rect, p)
		cache := map[int]*image.Paletted{}
		for yy := 0; yy < hh; yy++ {
			for xx := 0; xx < w; xx++ {
				o := (yy*w + xx) * 2
				v := int(room[o])<<8 | int(room[o+1])
				s.Room = append(s.Room, uint16(v))
				f := v / 2
				if f+3 < len(floor) && floor[f+3] != 0 {
					s.Entities = append(s.Entities, data.Marker{X: xx * 16, Y: yy * 16, Type: int(floor[f+3])})
				}
				for j := 0; j < 4; j++ {
					v := r.U16(defs + v + j*2)
					t := cache[v]
					if t == nil {
						t = tile(chars, v, p)
						cache[v] = t
					}
					target := low
					if v&0x8000 != 0 {
						target = high
					}
					dest := image.Pt(xx*16+(j%2)*8, yy*16+(j/2)*8)
					copyTileIndices(target, dest.X, dest.Y, t)
				}
			}
		}
		bgHeight := 224
		if index == 4 || index == 11 || index == 12 {
			bgHeight = 256 // Dungeon's front plane scrolls vertically as well.
		}
		bg := image.NewPaletted(image.Rect(0, 0, 512, bgHeight), p)
		bgHigh := image.NewPaletted(bg.Rect, p)
		if bgData := blocks[r.U32(h+28)]; len(bgData) >= 3584 {
			for yy := 0; yy < bgHeight/8 && (yy+1)*128 <= len(bgData); yy++ {
				for xx := 0; xx < 64; xx++ {
					o := (yy*64 + xx) * 2
					v := int(bgData[o])<<8 | int(bgData[o+1])
					target := bg
					if index == 4 && v&0x8000 != 0 {
						target = bgHigh
					}
					copyTileIndices(target, xx*8, yy*8, tile(chars, v, p))
				}
			}
		}
		savePNG(filepath.Join(*out, fmt.Sprintf("stage%02d-low.png", index)), low)
		savePNG(filepath.Join(*out, fmt.Sprintf("stage%02d-high.png", index)), high)
		savePNG(filepath.Join(*out, fmt.Sprintf("stage%02d-bg.png", index)), bg)
		if index == 4 {
			savePNG(filepath.Join(*out, "stage04-bg-high.png"), bgHigh)
		}
		m.Stages = append(m.Stages, s)
		fmt.Printf("stage %02d: %s, %dx%d, %d object markers\n", index, s.Name, s.Width, s.Height, len(s.Entities))
	}
	f, e := os.Create(filepath.Join(*out, "manifest.json"))
	if e != nil {
		log.Fatal(e)
	}
	if e = json.NewEncoder(f).Encode(m); e != nil {
		log.Fatal(e)
	}
	f.Close()
	fmt.Printf("verified %d RNC blocks; converted %d sprite frames and %d scene layouts\n", len(blocks), len(fs), len(m.Stages))
}

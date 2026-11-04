package game

import (
	"image"
	"image/color"
)

type hudSprite struct{ Tile, X, Y, Columns, Rows int }

// hudSprites reconstructs the original HUD sprite list (1D5E44..1D6086).
func (w *NativeWorld) hudSprites() []hudSprite {
	s := []hudSprite{{0x6ea, 16, 184, 3, 3}, {0x6e0 + w.get("lives"), 42, 200, 1, 1}}
	appendNumber := func(value, x int) {
		if value>>8 != '0' {
			s = append(s, hudSprite{0x6e0 + (value >> 8) - '0', x, 200, 1, 1})
			x += 8
		}
		s = append(s, hudSprite{0x6e0 + (value & 255) - '0', x, 200, 1, 1})
	}
	apples := w.get("apples_ascii")
	if apples != 0x3030 {
		if apples >= 0x3036 || w.get("tick")&15 >= 5 {
			s = append(s, hudSprite{0x6f3, 270, 192, 2, 2})
		}
		appendNumber(apples, 288)
	}
	if gems := w.get("gems_ascii"); gems != 0x3030 {
		s = append(s, hudSprite{0x6f7, 234, 192, 2, 2})
		appendNumber(gems, 252)
	}
	health := w.get("health")
	if health >= 3 || w.get("tick")&15 >= 5 {
		s = append(s, hudSprite{0x680, 18, 20, 4, 3}, hudSprite{0x68c, 50, 20, 2, 2})
		if health > 0 {
			s = append(s, hudSprite{0x6a0 - health*2, 66, 12, 1, 2})
			wave := w.HUDWaveTile
			if wave == 0 {
				wave = 0x6ae
			}
			wave -= (health - 1) * 2
			for i := 0; i < health; i++ {
				s = append(s, hudSprite{wave + i*2, 74 + i*8, 12, 1, 2})
			}
		}
	}
	if score := w.get("score"); score > 0 {
		div := 100000
		started := false
		for i := 0; i < 6; i++ {
			digit := score / div % 10
			if digit != 0 {
				started = true
			}
			if started {
				s = append(s, hudSprite{0x7c0 + digit*4, 180 + i*18, 26, 2, 2})
			}
			div /= 10
		}
	}
	return s
}

func (w *NativeWorld) addScorePoint() {
	w.set("pending_score", w.get("pending_score")-1)
	// A pending unit advances the tens digit. The last display digit stays
	// zero; the independent extra-life counter still advances by one unit.
	score := w.get("score") + 10
	if score >= 1000000 {
		score -= 1000000
		// The original decimal carry reaches the adjacent video-tick byte.
		w.set("tick", (w.get("tick")+1)&255)
	}
	w.set("score", score)
	w.set("score_progress", w.get("score_progress")+1)
	threshold := 5000
	if w.get("difficulty") == 1 {
		threshold = 7500
	} else if w.get("difficulty") > 1 {
		threshold = 10000
	}
	if w.get("score_progress") >= threshold {
		w.set("score_progress", 0)
		w.set("lives", min(9, w.get("lives")+1))
		w.Sound(0x66, false)
	}
}

func (w *NativeWorld) updateScore() {
	if w.get("tick")&1 == 0 && w.get("pending_score") > 0 {
		w.addScorePoint()
	}
}

func (w *NativeWorld) updateHUDWave() {
	if w.get("health") > 0 && (w.get("health") >= 3 || w.get("tick")&15 >= 5) && len(w.Scripts.HUDWave) > 0 {
		// A source snapshot may point at the terminating zero word.
		if w.HUDWaveIndex >= len(w.Scripts.HUDWave) {
			w.HUDWaveIndex = 0
		}
		w.HUDWaveTile = w.Scripts.HUDWave[w.HUDWaveIndex]
		w.HUDWaveIndex = (w.HUDWaveIndex + 1) % len(w.Scripts.HUDWave)
	}
}

func (g *Game) drawHUD(dst *image.RGBA) {
	if g.World == nil || g.Project.HUD == nil {
		return
	}
	palette := g.renderPalette()
	for _, s := range g.World.hudSprites() {
		for tx := 0; tx < s.Columns; tx++ {
			for ty := 0; ty < s.Rows; ty++ {
				tile := s.Tile + tx*s.Rows + ty - 0x680
				if tile < 0 || tile >= 384 {
					continue
				}
				for y := 0; y < 8; y++ {
					for x := 0; x < 8; x++ {
						index := g.Project.HUD.ColorIndexAt((tile%16)*8+x, (tile/16)*8+y)
						if index != 0 {
							rgb := palette[48+int(index)]
							dst.SetRGBA(s.X+tx*8+x, s.Y+ty*8+y, color.RGBA{rgb[0], rgb[1], rgb[2], 255})
						}
					}
				}
			}
		}
	}
}

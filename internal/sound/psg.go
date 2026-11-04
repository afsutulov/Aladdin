package sound

// sn76489 models the Sega variant of the TI SN76489 PSG: three square-wave
// tone generators and one noise generator (16-bit LFSR, taps 0 and 3),
// attenuation in 2 dB steps, clocked at the Z80 clock / 16.
type sn76489 struct {
	latch       int
	latchVolume bool
	period      [4]int
	count       [4]int
	volume      [4]int
	polarity    [4]int32
	noise       int // noise control (low 3 bits)
	lfsr        uint16
	level       [16]int32
}

func newSN76489() *sn76489 {
	p := &sn76489{lfsr: 0x8000}
	for i := range p.volume {
		p.volume[i] = 15
		p.polarity[i] = 1
		p.period[i] = 0
	}
	// 2 dB per step; full scale per channel = psgFullScale.
	v := float64(psgFullScale)
	for i := 0; i < 15; i++ {
		p.level[i] = int32(v + 0.5)
		v /= 1.2589254117941673 // 10^(2/20)
	}
	p.level[15] = 0
	return p
}

const psgFullScale = 4096

func (p *sn76489) write(v byte) {
	if v&0x80 != 0 {
		p.latch = int(v>>5) & 3
		p.latchVolume = v&0x10 != 0
		p.data(int(v&0x0F), true)
		return
	}
	p.data(int(v&0x3F), false)
}

func (p *sn76489) data(d int, low bool) {
	switch {
	case p.latchVolume:
		p.volume[p.latch] = d & 0x0F
	case p.latch == 3:
		p.noise = d & 7
		p.lfsr = 0x8000
	case low:
		p.period[p.latch] = p.period[p.latch]&0x3F0 | d&0x0F
	default:
		p.period[p.latch] = p.period[p.latch]&0x0F | (d&0x3F)<<4
	}
}

// tick advances the generators by one PSG clock (Z80 clock / 16).
func (p *sn76489) tick() {
	for i := 0; i < 3; i++ {
		p.count[i]--
		if p.count[i] <= 0 {
			per := p.period[i]
			if per == 0 {
				per = 1
			}
			p.count[i] = per
			p.polarity[i] = -p.polarity[i]
		}
	}
	p.count[3]--
	if p.count[3] <= 0 {
		var per int
		switch p.noise & 3 {
		case 0:
			per = 0x10
		case 1:
			per = 0x20
		case 2:
			per = 0x40
		default:
			per = p.period[2]
			if per == 0 {
				per = 1
			}
		}
		p.count[3] = per
		p.polarity[3] = -p.polarity[3]
		if p.polarity[3] > 0 {
			// Shift on the rising edge of the noise clock.
			fb := p.lfsr & 1
			if p.noise&4 != 0 {
				fb = (p.lfsr ^ p.lfsr>>3) & 1
			}
			p.lfsr = p.lfsr>>1 | fb<<15
		}
	}
}

func (p *sn76489) output() int32 {
	var s int32
	for i := 0; i < 3; i++ {
		if p.period[i] <= 1 {
			s += p.level[p.volume[i]]
			continue
		}
		s += p.polarity[i] * p.level[p.volume[i]]
	}
	if p.lfsr&1 != 0 {
		s += p.level[p.volume[3]]
	} else {
		s -= p.level[p.volume[3]]
	}
	return s
}

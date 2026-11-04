package sound

import "math"

// ym2612 is a sample-level model of the Yamaha YM2612 FM synthesizer
// (6 channels x 4 operators, LFO, channel 3 special mode, DAC on channel 6),
// producing one stereo sample per chip sample (clock/144 = 53267 Hz).
// It follows the published chip behaviour: log-sine and exponent tables,
// envelope rate/increment tables, detune table, LFO PM/AM tables, 14-bit
// channel accumulators, 9-bit DAC quantisation and the discrete chip's DAC
// "ladder" offsets.

const (
	ymEnvBits    = 10
	ymMaxAtt     = 1<<ymEnvBits - 1
	ymSinBits    = 10
	ymSinLen     = 1 << ymSinBits
	ymTLResLen   = 256
	ymTLTabLen   = 13 * 2 * ymTLResLen
	ymEnvQuiet   = ymTLTabLen >> 3
	ymRateSteps  = 8
	egOff        = 0
	egRel        = 1
	egSus        = 2
	egDec        = 3
	egAtt        = 4
	ymPhaseBits  = 20 // phase accumulator: 10-bit sine index + 10 fraction bits
	ymPhaseMask  = 1<<ymPhaseBits - 1
	ymDetuneMask = 1<<17 - 1
)

var (
	ymTL          [ymTLTabLen]int32
	ymSin         [ymSinLen]uint32
	ymLFOPM       [128 * 8 * 32]int32
	ymSLTable     [16]int32
	ymDT          [8][32]int32
	ymEGIncrement = [19 * ymRateSteps]uint8{
		0, 1, 0, 1, 0, 1, 0, 1,
		0, 1, 0, 1, 1, 1, 0, 1,
		0, 1, 1, 1, 0, 1, 1, 1,
		0, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 2, 1, 1, 1, 2,
		1, 2, 1, 2, 1, 2, 1, 2,
		1, 2, 2, 2, 1, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 4, 2, 2, 2, 4,
		2, 4, 2, 4, 2, 4, 2, 4,
		2, 4, 4, 4, 2, 4, 4, 4,
		4, 4, 4, 4, 4, 4, 4, 4,
		4, 4, 4, 8, 4, 4, 4, 8,
		4, 8, 4, 8, 4, 8, 4, 8,
		4, 8, 8, 8, 4, 8, 8, 8,
		8, 8, 8, 8, 8, 8, 8, 8,
		16, 16, 16, 16, 16, 16, 16, 16,
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	ymEGRateSelect [32 + 64 + 32]uint16
	ymEGRateShift  [32 + 64 + 32]uint8
	ymFKTable      = [16]uint8{0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 3, 3, 3, 3, 3, 3}
	ymLFOSteps     = [8]int{108, 77, 71, 67, 62, 44, 8, 5}
	ymAMSShift     = [4]uint8{8, 3, 1, 0}
	ymDTBase       = [4 * 32]uint8{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3, 3, 4, 4, 4, 5, 5, 6, 6, 7, 8, 8, 8, 8,
		1, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3, 3, 4, 4, 4, 5, 5, 6, 6, 7, 8, 8, 9, 10, 11, 12, 13, 14, 16, 16, 16, 16,
		2, 2, 2, 2, 2, 3, 3, 3, 4, 4, 4, 5, 5, 6, 6, 7, 8, 8, 9, 10, 11, 12, 13, 14, 16, 17, 19, 20, 22, 22, 22, 22,
	}
	ymLFOPMOutput = [7 * 8][8]uint8{
		{0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 1, 1, 1, 1},
		{0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 1, 1, 1, 1}, {0, 0, 1, 1, 2, 2, 2, 3},
		{0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 1}, {0, 0, 0, 0, 1, 1, 1, 1}, {0, 0, 1, 1, 2, 2, 2, 3}, {0, 0, 2, 3, 4, 4, 5, 6},
		{0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 1, 1}, {0, 0, 0, 0, 1, 1, 1, 1},
		{0, 0, 0, 1, 1, 1, 1, 2}, {0, 0, 1, 1, 2, 2, 2, 3}, {0, 0, 2, 3, 4, 4, 5, 6}, {0, 0, 4, 6, 8, 8, 0xa, 0xc},
		{0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 1, 1, 1, 1}, {0, 0, 0, 1, 1, 1, 2, 2}, {0, 0, 1, 1, 2, 2, 3, 3},
		{0, 0, 1, 2, 2, 2, 3, 4}, {0, 0, 2, 3, 4, 4, 5, 6}, {0, 0, 4, 6, 8, 8, 0xa, 0xc}, {0, 0, 8, 0xc, 0x10, 0x10, 0x14, 0x18},
		{0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 2, 2, 2, 2}, {0, 0, 0, 2, 2, 2, 4, 4}, {0, 0, 2, 2, 4, 4, 6, 6},
		{0, 0, 2, 4, 4, 4, 6, 8}, {0, 0, 4, 6, 8, 8, 0xa, 0xc}, {0, 0, 8, 0xc, 0x10, 0x10, 0x14, 0x18}, {0, 0, 0x10, 0x18, 0x20, 0x20, 0x28, 0x30},
		{0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 4, 4, 4, 4}, {0, 0, 0, 4, 4, 4, 8, 8}, {0, 0, 4, 4, 8, 8, 0xc, 0xc},
		{0, 0, 4, 8, 8, 8, 0xc, 0x10}, {0, 0, 8, 0xc, 0x10, 0x10, 0x14, 0x18}, {0, 0, 0x10, 0x18, 0x20, 0x20, 0x28, 0x30}, {0, 0, 0x20, 0x30, 0x40, 0x40, 0x50, 0x60},
	}
)

func init() {
	for x := 0; x < ymTLResLen; x++ {
		m := math.Floor(65536 / math.Pow(2, float64(x+1)/256))
		n := int32(m) >> 4
		if n&1 != 0 {
			n = n>>1 + 1
		} else {
			n >>= 1
		}
		n <<= 2
		ymTL[x*2] = n
		ymTL[x*2+1] = -n
		for i := 1; i < 13; i++ {
			ymTL[x*2+i*2*ymTLResLen] = n >> uint(i)
			ymTL[x*2+1+i*2*ymTLResLen] = -(n >> uint(i))
		}
	}
	for i := 0; i < ymSinLen; i++ {
		m := math.Sin(float64(i*2+1) * math.Pi / ymSinLen)
		var o float64
		if m > 0 {
			o = 8 * math.Log(1/m) / math.Log(2)
		} else {
			o = 8 * math.Log(-1/m) / math.Log(2)
		}
		o *= 32
		n := int(2 * o)
		if n&1 != 0 {
			n = n>>1 + 1
		} else {
			n >>= 1
		}
		s := uint32(n * 2)
		if m < 0 {
			s++
		}
		ymSin[i] = s
	}
	for depth := 0; depth < 8; depth++ {
		for fnum := 0; fnum < 128; fnum++ {
			for step := 0; step < 8; step++ {
				v := int32(0)
				for bit := 0; bit < 7; bit++ {
					if fnum&(1<<bit) != 0 {
						v += int32(ymLFOPMOutput[bit*8+depth][step])
					}
				}
				base := fnum*32*8 + depth*32
				ymLFOPM[base+step] = v
				ymLFOPM[base+(step^7)+8] = v
				ymLFOPM[base+step+16] = -v
				ymLFOPM[base+(step^7)+24] = -v
			}
		}
	}
	for i := 0; i < 15; i++ {
		ymSLTable[i] = int32(i * 32)
	}
	ymSLTable[15] = 31 * 32
	for d := 0; d < 4; d++ {
		for k := 0; k < 32; k++ {
			ymDT[d][k] = int32(ymDTBase[d*32+k])
			ymDT[d+4][k] = -int32(ymDTBase[d*32+k])
		}
	}
	for i := range ymEGRateSelect {
		switch {
		case i < 32:
			ymEGRateSelect[i] = 18 * ymRateSteps
			ymEGRateShift[i] = 0
		case i < 32+48:
			ymEGRateSelect[i] = uint16((i-32)%4) * ymRateSteps
			ymEGRateShift[i] = uint8(11 - (i-32)/4)
		case i < 32+60:
			ymEGRateSelect[i] = uint16(4+(i-32-48)) * ymRateSteps
			ymEGRateShift[i] = 0
		default:
			ymEGRateSelect[i] = 16 * ymRateSteps
			ymEGRateShift[i] = 0
		}
	}
}

type ymSlot struct {
	dt                         int
	ksrShift                   uint8
	ar, d1r, d2r, rr           int
	ksr                        int
	mul                        uint32
	phase, incr                uint32
	state                      int
	tl                         int32
	volume                     int32
	sl                         int32
	volOut                     int32
	shAR, shD1, shD2, shRR     uint8
	selAR, selD1, selD2, selRR uint16
	key                        bool
	amMask                     int32
}

type ymChannel struct {
	slot        [4]ymSlot // register order S1, S3, S2, S4
	algo        uint8
	fb          uint8
	op1         [2]int32
	memValue    int32
	pms         int
	ams         uint8
	fn          uint32
	block       uint32
	kcode       int
	blockFnum   uint32
	needRefresh bool
	panL, panR  bool
}

type ym2612 struct {
	ch          [6]ymChannel
	addr        [2]byte
	fnH         byte
	sl3fnH      byte
	sl3fn       [3]uint32
	sl3block    [3]uint32
	sl3kcode    [3]int
	sl3bfn      [3]uint32
	mode        byte
	lfoOn       bool
	lfoTimer    int
	lfoOverflow int
	lfoCnt      int
	lfoAM       int32
	lfoPM       int
	egTimer     int
	egCnt       uint32
	dacOn       bool
	dacOut      int32
}

func newYM2612() *ym2612 {
	y := &ym2612{}
	y.reset()
	return y
}

func (y *ym2612) reset() {
	*y = ym2612{lfoAM: 126}
	for c := range y.ch {
		ch := &y.ch[c]
		ch.panL, ch.panR = true, true
		ch.needRefresh = true
		for s := range ch.slot {
			sl := &ch.slot[s]
			sl.mul = 1
			sl.state = egOff
			sl.volume = ymMaxAtt
			sl.volOut = ymMaxAtt
			sl.ksrShift = 3
			sl.rr = 34
			y.slotRates(sl)
		}
	}
}

func (y *ym2612) slotRates(sl *ymSlot) {
	k := sl.ksr
	if sl.ar+k < 32+62 {
		sl.shAR, sl.selAR = ymEGRateShift[sl.ar+k], ymEGRateSelect[sl.ar+k]
	} else {
		sl.shAR, sl.selAR = 0, 18*ymRateSteps
	}
	sl.shD1, sl.selD1 = ymEGRateShift[sl.d1r+k], ymEGRateSelect[sl.d1r+k]
	sl.shD2, sl.selD2 = ymEGRateShift[sl.d2r+k], ymEGRateSelect[sl.d2r+k]
	sl.shRR, sl.selRR = ymEGRateShift[sl.rr+k], ymEGRateSelect[sl.rr+k]
}

// slot index for register offsets +0, +4, +8, +C.
func ymSlotIndex(r byte) int { return int(r>>2) & 3 }

// write sets register reg of part (0: 000-0FF, 1: 100-1FF).
func (y *ym2612) write(part int, reg, v byte) {
	if part == 0 && reg < 0x30 {
		y.writeMode(reg, v)
		return
	}
	c := int(reg & 3)
	if c == 3 {
		return
	}
	if part == 1 {
		c += 3
	}
	ch := &y.ch[c]
	sl := &ch.slot[ymSlotIndex(reg)]
	switch reg & 0xF0 {
	case 0x30:
		sl.mul = uint32(v&0x0F) * 2
		if sl.mul == 0 {
			sl.mul = 1
		}
		sl.dt = int(v>>4) & 7
		ch.needRefresh = true
	case 0x40:
		sl.tl = int32(v&0x7F) << (ymEnvBits - 7)
		sl.volOut = sl.volume + sl.tl
	case 0x50:
		old := sl.ksrShift
		sl.ksrShift = 3 - v>>6
		sl.ar = 0
		if v&0x1F != 0 {
			sl.ar = 32 + int(v&0x1F)<<1
		}
		if old != sl.ksrShift {
			ch.needRefresh = true
		}
		y.slotRates(sl)
	case 0x60:
		sl.amMask = 0
		if v&0x80 != 0 {
			sl.amMask = -1
		}
		sl.d1r = 0
		if v&0x1F != 0 {
			sl.d1r = 32 + int(v&0x1F)<<1
		}
		y.slotRates(sl)
	case 0x70:
		sl.d2r = 0
		if v&0x1F != 0 {
			sl.d2r = 32 + int(v&0x1F)<<1
		}
		y.slotRates(sl)
	case 0x80:
		sl.sl = ymSLTable[v>>4]
		sl.rr = 34 + int(v&0x0F)<<2
		y.slotRates(sl)
		if sl.state == egDec && sl.volume >= sl.sl {
			sl.state = egSus
		}
	case 0x90:
		// SSG-EG is never enabled by this driver (written as 0).
	case 0xA0:
		switch reg & 0x0C {
		case 0x00:
			fn := uint32(y.fnH&7)<<8 | uint32(v)
			blk := uint32(y.fnH >> 3)
			ch.fn, ch.block = fn, blk
			ch.kcode = int(blk<<2) | int(ymFKTable[fn>>7])
			ch.blockFnum = blk<<11 | fn
			ch.needRefresh = true
		case 0x04:
			y.fnH = v & 0x3F
		case 0x08:
			if part == 0 {
				i := c
				fn := uint32(y.sl3fnH&7)<<8 | uint32(v)
				blk := uint32(y.sl3fnH >> 3)
				y.sl3fn[i], y.sl3block[i] = fn, blk
				y.sl3kcode[i] = int(blk<<2) | int(ymFKTable[fn>>7])
				y.sl3bfn[i] = blk<<11 | fn
				y.ch[2].needRefresh = true
			}
		case 0x0C:
			if part == 0 {
				y.sl3fnH = v & 0x3F
			}
		}
	case 0xB0:
		switch reg & 0x0C {
		case 0x00:
			fb := (v >> 3) & 7
			ch.fb = fb
			ch.algo = v & 7
		case 0x04:
			ch.panL = v&0x80 != 0
			ch.panR = v&0x40 != 0
			ch.ams = ymAMSShift[(v>>4)&3]
			ch.pms = int(v&7) * 32
		}
	}
}

func (y *ym2612) writeMode(reg, v byte) {
	switch reg {
	case 0x22:
		if v&8 != 0 {
			y.lfoOverflow = ymLFOSteps[v&7]
		} else {
			y.lfoOverflow = 0
			y.lfoTimer = 0
			y.lfoCnt = 0
			y.lfoPM = 0
			y.lfoAM = 126
		}
	case 0x27:
		if (y.mode^v)&0xC0 != 0 {
			y.ch[2].needRefresh = true
		}
		y.mode = v
	case 0x28:
		c := int(v & 3)
		if c == 3 {
			return
		}
		if v&4 != 0 {
			c += 3
		}
		ch := &y.ch[c]
		for i, s := range [4]int{0, 2, 1, 3} {
			if v&(0x10<<uint(i)) != 0 {
				y.keyOn(ch, s)
			} else {
				y.keyOff(ch, s)
			}
		}
	case 0x2A:
		y.dacOut = (int32(v) - 0x80) << 6
	case 0x2B:
		y.dacOn = v&0x80 != 0
	}
}

func (y *ym2612) keyOn(ch *ymChannel, s int) {
	sl := &ch.slot[s]
	if !sl.key {
		sl.phase = 0
		if ch.needRefresh {
			y.refresh(ch)
		}
		if sl.ar+sl.ksr < 32+62 {
			if sl.volume <= 0 {
				if sl.sl == 0 {
					sl.state = egSus
				} else {
					sl.state = egDec
				}
			} else {
				sl.state = egAtt
			}
		} else {
			sl.volume = 0
			if sl.sl == 0 {
				sl.state = egSus
			} else {
				sl.state = egDec
			}
		}
		sl.volOut = sl.volume + sl.tl
	}
	sl.key = true
}

func (y *ym2612) keyOff(ch *ymChannel, s int) {
	sl := &ch.slot[s]
	if sl.key && sl.state > egRel {
		sl.state = egRel
	}
	sl.key = false
}

func (y *ym2612) refreshSlot(sl *ymSlot, fn, blk uint32, kc int) {
	fc := int32((fn << blk) >> 1)
	fc += ymDT[sl.dt][kc]
	fc &= ymDetuneMask
	sl.incr = uint32(fc) * sl.mul >> 1
	k := kc >> sl.ksrShift
	if sl.ksr != k {
		sl.ksr = k
		y.slotRates(sl)
	}
}

func (y *ym2612) refresh(ch *ymChannel) {
	ch.needRefresh = false
	if ch == &y.ch[2] && y.mode&0xC0 != 0 {
		y.refreshSlot(&ch.slot[0], y.sl3fn[1], y.sl3block[1], y.sl3kcode[1])
		y.refreshSlot(&ch.slot[2], y.sl3fn[2], y.sl3block[2], y.sl3kcode[2])
		y.refreshSlot(&ch.slot[1], y.sl3fn[0], y.sl3block[0], y.sl3kcode[0])
		y.refreshSlot(&ch.slot[3], ch.fn, ch.block, ch.kcode)
		return
	}
	for s := range ch.slot {
		y.refreshSlot(&ch.slot[s], ch.fn, ch.block, ch.kcode)
	}
}

func ymOp(phase uint32, env int32, pm int32) int32 {
	idx := (int32(phase>>10) + pm>>1) & (ymSinLen - 1)
	p := uint32(env<<3) + ymSin[idx]
	if p >= ymTLTabLen {
		return 0
	}
	return ymTL[p]
}

func ymOp1(phase uint32, env int32, pm int32) int32 {
	idx := (int32(phase>>10) + pm>>10) & (ymSinLen - 1)
	p := uint32(env<<3) + ymSin[idx]
	if p >= ymTLTabLen {
		return 0
	}
	return ymTL[p]
}

// channel computes one channel output (14-bit accumulator before clipping)
// and advances its phases.
func (y *ym2612) channel(ch *ymChannel, carrierMask int32) int32 {
	if ch.needRefresh {
		y.refresh(ch)
	}
	am := y.lfoAM >> ch.ams
	var m2, c1, c2, mem, out int32
	// Restore the delayed (MEM) sample.
	switch ch.algo {
	case 0, 1, 2:
		m2 = ch.memValue
	case 3:
		c2 = ch.memValue
	case 5:
		m2 = ch.memValue
	}
	vol := func(s *ymSlot) int32 { return s.volOut + (am & s.amMask) }
	// S1 (feedback operator), output delayed by one sample.
	e := vol(&ch.slot[0])
	fbIn := ch.op1[0] + ch.op1[1]
	ch.op1[0] = ch.op1[1]
	o1 := ch.op1[0]
	switch ch.algo {
	case 0, 3:
		c1 += o1
	case 1:
		mem += o1
	case 2:
		c2 += o1
	case 4, 6:
		c1 += o1
	case 5:
		mem, c1, c2 = o1, o1, o1
	case 7:
		out += o1 & carrierMask
	}
	ch.op1[1] = 0
	if e < ymEnvQuiet {
		if ch.fb == 0 {
			fbIn = 0
		}
		ch.op1[1] = ymOp1(ch.slot[0].phase, e, fbIn<<ch.fb)
	}
	// S3 (M2)
	if e := vol(&ch.slot[1]); e < ymEnvQuiet {
		v := ymOp(ch.slot[1].phase, e, m2)
		switch ch.algo {
		case 0, 1, 2, 3, 4:
			c2 += v
		default:
			out += v & carrierMask
		}
	}
	// S2 (C1)
	if e := vol(&ch.slot[2]); e < ymEnvQuiet {
		v := ymOp(ch.slot[2].phase, e, c1)
		switch ch.algo {
		case 0, 1, 2, 3:
			mem += v
		default:
			out += v & carrierMask
		}
	}
	// S4 (C2), always a carrier.
	if e := vol(&ch.slot[3]); e < ymEnvQuiet {
		out += ymOp(ch.slot[3].phase, e, c2) & carrierMask
	}
	switch ch.algo {
	case 0, 1, 2, 3, 5:
		ch.memValue = mem
	}
	// Phase update after output.
	if ch.pms != 0 && y.lfoOverflow != 0 {
		y.lfoPhase(ch)
	} else {
		for s := range ch.slot {
			ch.slot[s].phase = (ch.slot[s].phase + ch.slot[s].incr) & ymPhaseMask
		}
	}
	return out
}

func (y *ym2612) lfoPhase(ch *ymChannel) {
	if ch == &y.ch[2] && y.mode&0xC0 != 0 {
		bf := [4]uint32{y.sl3bfn[1], y.sl3bfn[0], y.sl3bfn[2], ch.blockFnum}
		for s := range ch.slot {
			y.lfoSlotPhase(ch, &ch.slot[s], bf[s])
		}
		return
	}
	for s := range ch.slot {
		y.lfoSlotPhase(ch, &ch.slot[s], ch.blockFnum)
	}
}

func (y *ym2612) lfoSlotPhase(ch *ymChannel, sl *ymSlot, blockFnum uint32) {
	off := ymLFOPM[((blockFnum&0x7F0)>>4)<<8+uint32(ch.pms)+uint32(y.lfoPM)]
	if off == 0 {
		sl.phase = (sl.phase + sl.incr) & ymPhaseMask
		return
	}
	bf := int32(blockFnum)*2 + off
	blk := uint32(bf&0x7000) >> 12
	fn := uint32(bf) & 0xFFF
	kc := int(blk<<2) | int(ymFKTable[fn>>8])
	fc := int32((fn << blk) >> 2)
	fc += ymDT[sl.dt][kc]
	fc &= ymDetuneMask
	sl.phase = (sl.phase + uint32(fc)*sl.mul>>1) & ymPhaseMask
}

func (y *ym2612) advanceEG() {
	cnt := y.egCnt
	for c := range y.ch {
		for s := range y.ch[c].slot {
			sl := &y.ch[c].slot[s]
			switch sl.state {
			case egAtt:
				if cnt&(1<<sl.shAR-1) == 0 {
					inc := int32(ymEGIncrement[int(sl.selAR)+int(cnt>>sl.shAR&7)])
					sl.volume += (^sl.volume * inc) >> 4
					if sl.volume <= 0 {
						sl.volume = 0
						if sl.sl == 0 {
							sl.state = egSus
						} else {
							sl.state = egDec
						}
					}
					sl.volOut = sl.volume + sl.tl
				}
			case egDec:
				if cnt&(1<<sl.shD1-1) == 0 {
					sl.volume += int32(ymEGIncrement[int(sl.selD1)+int(cnt>>sl.shD1&7)])
					if sl.volume >= sl.sl {
						sl.state = egSus
					}
					sl.volOut = sl.volume + sl.tl
				}
			case egSus:
				if cnt&(1<<sl.shD2-1) == 0 {
					sl.volume += int32(ymEGIncrement[int(sl.selD2)+int(cnt>>sl.shD2&7)])
					if sl.volume >= ymMaxAtt {
						sl.volume = ymMaxAtt
					}
					sl.volOut = sl.volume + sl.tl
				}
			case egRel:
				if cnt&(1<<sl.shRR-1) == 0 {
					sl.volume += int32(ymEGIncrement[int(sl.selRR)+int(cnt>>sl.shRR&7)])
					if sl.volume >= ymMaxAtt {
						sl.volume = ymMaxAtt
						sl.state = egOff
					}
					sl.volOut = sl.volume + sl.tl
				}
			}
		}
	}
}

// sample produces one chip sample (left, right).
func (y *ym2612) sample() (int32, int32) {
	var outs [6]int32
	for c := range y.ch {
		if c == 5 && y.dacOn {
			// Channel 6 operators keep running silently while the DAC is on.
			y.channel(&y.ch[c], -32)
			outs[c] = y.dacOut
			continue
		}
		outs[c] = y.channel(&y.ch[c], -32) // carriers quantised to the 9-bit DAC
	}
	// LFO
	if y.lfoOverflow != 0 {
		y.lfoTimer++
		if y.lfoTimer >= y.lfoOverflow {
			y.lfoTimer = 0
			y.lfoCnt = (y.lfoCnt + 1) & 127
			if y.lfoCnt < 64 {
				y.lfoAM = int32(y.lfoCnt^63) << 1
			} else {
				y.lfoAM = int32(y.lfoCnt&63) << 1
			}
			y.lfoPM = y.lfoCnt >> 2
		}
	}
	// Envelope generator: one step every three samples.
	y.egTimer++
	if y.egTimer >= 3 {
		y.egTimer = 0
		y.egCnt++
		y.advanceEG()
	}
	var l, r int32
	for c, v := range outs {
		if v > 8191 {
			v = 8191
		} else if v < -8192 {
			v = -8192
		}
		ch := &y.ch[c]
		if ch.panL {
			l += v
		}
		if ch.panR {
			r += v
		}
		// Discrete YM2612 DAC "ladder effect" offsets (9-bit units).
		if v < 0 {
			l -= int32(4-boolInt(ch.panL)) << 5
			r -= int32(4-boolInt(ch.panR)) << 5
		} else {
			l += 4 << 5
			r += 4 << 5
		}
	}
	return l, r
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

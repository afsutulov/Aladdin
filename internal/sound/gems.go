package sound

// Native Go port of the GEMS 2.x sound driver logic used by the supplied
// Final Cut build (Z80 program at ROM 1E3C6E). The game's own music data is
// interpreted: patch bank 1E54F4, envelope bank 1E6734, sequence bank 1E675D
// and sample bank 1F2C31 (converted unchanged into gems.bin).
//
// The driver state keeps the original Z80 RAM layout (channel records at
// 1B80, voice lists at 1791/17BC/17D2, PSG envelopes at 0009, modulation
// slots at 0EAB, pitch tables at 0F76, ...). That makes every field
// comparable with the original driver's memory at each video boundary. No
// Z80 instructions are executed: each routine is rewritten in Go, and
// comments name the original routine addresses.
//
// Timing model: the driver's work happens at video boundaries (the Z80
// interrupt). Sound chip writes are applied at that instant; DAC samples are
// streamed at the YM2612 timer A rate as in routine 02B7.

type gemsDriver struct {
	ram  [0x2000]byte
	rom  []byte
	base uint32 // ROM address of rom[0]

	queue []byte // 68k command bytes not yet consumed

	// DAC streaming state (02B3..0301 and the 1F00 ring buffer).
	dacOn     bool // 02B7 is active (not RET)
	dacBuffer [256]byte
	dacRead   byte // E' (low byte of DE')
	dacNibble bool // 4-bit packed samples
	dacPhase  byte // C' rotating nibble selector
	dacRate   int  // timer A period in YM2612 samples
	timerCtl  byte // B' (08C4/149B): register 27 value written per DAC sample

	// Driver time (master cycles). While the DAC streams, every call of the
	// sample routine 02B7 waits for the next timer A overflow and outputs one
	// sample, which paces all driver work; otherwise the work is immediate.
	clk       int64
	lastTick  int64 // time of the last timer A overflow consumed
	dacPeriod int64
	dacIdle   bool // rates >= 10: alternate calls return at once (02CF)
	events    []chipEvent
}

type chipEvent struct {
	t        int64
	psg      bool
	part     int
	reg, val byte
}

// call accounts for n calls of the DAC routine 02B7.
func (g *gemsDriver) call(n int) {
	for ; n > 0; n-- {
		if !g.dacOn {
			continue
		}
		if g.dacPeriod >= 10*fmCycles {
			g.dacIdle = !g.dacIdle
			if g.dacIdle {
				continue
			}
		}
		g.tick()
	}
}

// tick waits for the next timer A overflow and outputs one DAC sample.
func (g *gemsDriver) tick() {
	next := g.lastTick + g.dacPeriod
	if next < g.clk {
		next += (g.clk - next + g.dacPeriod - 1) / g.dacPeriod * g.dacPeriod
	}
	g.clk = next
	g.lastTick = next
	if v, ok := g.dacSample(); ok {
		g.events = append(g.events, chipEvent{t: g.clk, reg: 0x2A, val: v})
	}
}

const (
	gemsPatchBank  = 0x1E54F4
	gemsEnvBank    = 0x1E6734
	gemsSeqBank    = 0x1E675D
	gemsSampleBank = 0x1F2C31
)

func (g *gemsDriver) r8(a uint16) byte       { return g.ram[a&0x1FFF] }
func (g *gemsDriver) w8(a uint16, v byte)    { g.ram[a&0x1FFF] = v }
func (g *gemsDriver) r16(a uint16) uint16    { return uint16(g.r8(a)) | uint16(g.r8(a+1))<<8 }
func (g *gemsDriver) w16(a uint16, v uint16) { g.w8(a, byte(v)); g.w8(a+1, byte(v>>8)) }
func (g *gemsDriver) set(a uint16, bit uint) { g.w8(a, g.r8(a)|1<<bit) }
func (g *gemsDriver) res(a uint16, bit uint) { g.w8(a, g.r8(a)&^(1<<bit)) }
func (g *gemsDriver) bit(a uint16, bit uint) bool {
	return g.r8(a)&(1<<bit) != 0
}
func (g *gemsDriver) r24(a uint16) uint32 {
	return uint32(g.r8(a)) | uint32(g.r8(a+1))<<8 | uint32(g.r8(a+2))<<16
}
func (g *gemsDriver) w24(a uint16, v uint32) {
	g.w8(a, byte(v))
	g.w8(a+1, byte(v>>8))
	g.w8(a+2, byte(v>>16))
}

func (g *gemsDriver) romByte(addr uint32) byte {
	addr &= 0xFFFFFF
	if addr < g.base || addr-g.base >= uint32(len(g.rom)) {
		return 0xFF
	}
	return g.rom[addr-g.base]
}

// copyROM is 01F9: C bytes from a 24-bit ROM address into driver RAM.
func (g *gemsDriver) copyROM(src uint32, dst uint16, n int) {
	if n == 0 {
		n = 256
	}
	g.call(copyCalls(n))
	for i := 0; i < n; i++ {
		g.w8(dst+uint16(i), g.romByte(src+uint32(i)))
	}
}

// ym writes a register with the busy-wait omitted.
func (g *gemsDriver) ym(part int, reg, val byte) {
	g.events = append(g.events, chipEvent{t: g.clk, part: part, reg: reg, val: val})
}

func (g *gemsDriver) psg(val byte) {
	g.events = append(g.events, chipEvent{t: g.clk, psg: true, val: val})
}

// copyCalls is the number of 02B7 calls made by 01F9 copying n bytes.
func copyCalls(n int) int {
	if n >= 7 {
		return 4 + (n-7)/6 + 1
	}
	return 4
}

func newGemsDriver(rom []byte) *gemsDriver {
	g := &gemsDriver{rom: rom, base: gemsPatchBank}
	g.reset()
	return g
}

// reset reproduces the driver's initial data and start-up code (0000, 08C3,
// 08D8) followed by the game's gemsinit (219EDA) bank command.
func (g *gemsDriver) reset() {
	g.ram = [0x2000]byte{}
	// PSG envelope attenuation (0009+04..07) starts silent.
	for i := uint16(0); i < 4; i++ {
		g.w8(0x0D+i, 0xFF)
	}
	g.w16(0x08BE, 0x00CC) // tempo increment: 120 BPM
	// Voice lists (7-byte records): FM 1791, PSG tone 17BC, noise 17D2.
	lists := []struct {
		at  uint16
		chs []byte
	}{{0x1791, []byte{0, 1, 4, 5, 6, 2}}, {0x17BC, []byte{0, 1, 2}}, {0x17D2, []byte{3}}}
	for _, l := range lists {
		a := l.at
		for _, ch := range l.chs {
			g.w8(a, 0x80|ch)
			g.w8(a+2, 0x50)
			a += 7
		}
		g.w8(a, 0xFF)
	}
	// Modulation slots 0EAB (four free) and their buffers.
	for i := uint16(0); i < 4; i++ {
		g.w8(0x0EAB+i, 0x40)
		g.w8(0x0EC4+i, byte(0x80+0x20*i))
	}
	g.w8(0x0EAF, 0xFF)
	// 08D8: per-channel patch caches start with an invalid type.
	for i := uint16(0); i < 16; i++ {
		g.w8(0x1886+i*0x27, 0xFF)
	}
	g.dacOn = false
	g.timerCtl = 0x15
	g.psg(0x9F)
	g.psg(0xBF)
	g.psg(0xDF)
	g.psg(0xFF)
	// gemsinit: bank pointers (command 0B).
	g.w24(0x0AA5, gemsPatchBank)
	g.w24(0x0AA8, gemsEnvBank)
	g.w24(0x0AAB, gemsSeqBank)
	g.w24(0x0AAE, gemsSampleBank)
}

// command queues a 68k request: FF, code, arguments (219E84).
func (g *gemsDriver) command(b ...byte) {
	g.queue = append(g.queue, 0xFF)
	g.queue = append(g.queue, b...)
}

// vblank is the Z80 interrupt plus the 0041 bookkeeping it triggers.
func (g *gemsDriver) vblank() {
	g.call(2)
	g.w8(0x0040, g.r8(0x0040)+1)
	g.w16(0x08C0, g.r16(0x08C0)+g.r16(0x08BE))
}

// run executes the main loop (08F7) until no timed work or request remains.
func (g *gemsDriver) run() {
	for {
		b := byte(0)
		g.call(1)      // 08FA
		g.serviceDAC() // 08FD
		g.call(1)      // 0900
		if v := g.r8(0x0040); v > 0 {
			g.w8(0x0040, v-1)
			g.psgEnvelopes()
			b = 1
		}
		g.call(1) // 091A
		if v := g.r8(0x08C1); v > 0 {
			g.w8(0x08C1, v-1)
			b |= 2
		}
		if b != 0 {
			g.w8(0x08C2, b)
			g.modulation()
			g.voiceDurations()
			g.sequencer()
		}
		g.applyPitch()
		if len(g.queue) == 0 {
			if b == 0 {
				return
			}
			continue
		}
		// 0945: only bytes following an FF marker are commands.
		c := g.next68k()
		if c != 0xFF {
			continue
		}
		g.request(g.next68k())
	}
}

func (g *gemsDriver) next68k() byte {
	if len(g.queue) == 0 {
		return 0
	}
	c := g.queue[0]
	g.queue = g.queue[1:]
	g.call(1) // 01D3
	g.serviceDAC()
	g.call(1) // 01E9
	return c
}

// request is the 68k command dispatcher (0958).
func (g *gemsDriver) request(c byte) {
	switch c {
	case 0x05: // tempo
		g.setTempo(g.next68k())
	case 0x0B: // bank pointers
		for i := uint16(0); i < 12; i++ {
			g.w8(0x0AA5+i, g.next68k())
		}
	case 0x0C: // pause all (0AB1)
		for i := uint16(0); i < 16; i++ {
			g.res(0x1B86+i*0x20, 4)
		}
		g.silenceIdleVoices()
	case 0x0D: // resume all
		g.resumeAll()
	case 0x10: // start sequence
		g.startSequence(g.next68k())
	case 0x12: // stop sequence
		g.stopSequence(g.next68k())
	case 0x16: // stop all (0AF2)
		for i := uint16(0); i < 16; i++ {
			ix := 0x1B80 + i*0x20
			g.w8(ix+6, 0)
			g.w8(ix+0x0B, 0)
			g.w8(ix+0x0C, 0)
		}
		g.silenceIdleVoices()
	}
}

// setTempo is 0E1B: increment = (A*0xDA*2)>>8 per video boundary (8.8).
func (g *gemsDriver) setTempo(a byte) {
	hl := uint32(mul16(0x00DA, a))
	g.w16(0x08BE, uint16((hl<<1)>>8))
}

// mul16 is 1159: DE*A modulo 65536.
func mul16(de uint16, a byte) uint16 { return de * uint16(a) }

// startSequence is 0BDF.
func (g *gemsDriver) startSequence(n byte) {
	g.w8(0x0BDE, n)
	bank := g.r24(0x0AAB)
	g.copyROM(bank+uint32(n)*2, 0x0BBD, 2)
	off := uint32(g.r16(0x0BBD))
	g.copyROM(bank+off, 0x0BBD, 0x21)
	tracks := g.r8(0x0BBD)
	if tracks == 0 {
		return
	}
	c := byte(0)
	hl := uint16(0x0BBE)
	for i := uint16(0); i < 16; i++ {
		ix := 0x1B80 + i*0x20
		iy := 0x0F76 + i
		if g.r8(ix+6)&0x21 != 0 {
			continue
		}
		g.w8(ix+6, 0x11)
		g.w24(ix+3, bank+uint32(g.r16(hl)))
		hl += 2
		g.w8(ix+0, 0xFF)
		g.w8(ix+1, 0xFF)
		g.w8(ix+2, 0xFF)
		g.w8(ix+7, 0xFF)
		g.w8(ix+8, 0xFF)
		g.w8(ix+0x0E, n)
		g.w8(ix+0x0F, c)
		for _, o := range []uint16{0x10, 0x13, 0x16, 0x19, 0x1D, 0x1C, 0x1E} {
			g.w8(ix+o, 0)
		}
		g.w8(iy+0x20, 0)
		g.w8(iy+0x30, 0)
		g.w8(iy+0x00, 0)
		g.w8(iy+0x10, 0)
		c++
		if c == tracks {
			return
		}
	}
}

// stopSequence is 0C9E (FF stops every unlocked sequence).
func (g *gemsDriver) stopSequence(n byte) {
	for i := uint16(0); i < 16; i++ {
		ix := 0x1B80 + i*0x20
		if !g.bit(ix+6, 0) || g.bit(ix+6, 5) {
			continue
		}
		if n != 0xFF && n != g.r8(ix+0x0E) {
			continue
		}
		g.w8(ix+6, 0)
		g.w8(ix+0x0B, 0)
		g.w8(ix+0x0C, 0)
	}
	g.silenceIdleVoices()
}

// resumeAll is 0B89.
func (g *gemsDriver) resumeAll() {
	for i := uint16(0); i < 16; i++ {
		ix := 0x1B80 + i*0x20
		if !g.bit(ix+6, 5) && g.bit(ix+6, 0) {
			g.set(ix+6, 4)
		}
	}
}

// silenceIdleVoices is 0CF9: release voices of channels that stopped playing.
func (g *gemsDriver) silenceIdleVoices() {
	g.silenceList(0x1791, false)
	g.silenceList(0x17BC, true)
	g.silenceList(0x17D2, true)
	for iy := uint16(0x0EAB); ; iy++ {
		a := g.r8(iy)
		if a&0x80 != 0 {
			return
		}
		if a&0x40 != 0 {
			continue
		}
		if g.bit(0x1B86+uint16(a&0x0F)*0x20, 4) {
			continue
		}
		g.set(iy, 6)
	}
}

func (g *gemsDriver) silenceList(ix uint16, psg bool) {
	for ; g.r8(ix) != 0xFF; ix += 7 {
		d := g.r8(ix)
		if g.bit(0x1B86+uint16(g.r8(ix+3)&0x0F)*0x20, 4) {
			continue
		}
		hw := d & 7
		g.w8(ix, hw|0x80)
		g.w8(ix+4, 0)
		g.w8(ix+5, 0)
		g.w8(ix+6, 0)
		g.w8(0x0CF8, hw)
		if psg {
			g.w8(0x0009+uint16(hw), 0x04)
			continue
		}
		if d&0x20 != 0 {
			g.stopDAC()
			continue
		}
		part, c := 0, hw
		if c >= 3 {
			c -= 4
			part = 1
		}
		for _, r := range []byte{0x40, 0x44, 0x48, 0x4C} {
			g.ym(part, r+c, 0x7F)
		}
		g.ym(0, 0x28, hw)
	}
}

// stopDAC is the shared 02B7/0302 = RET, reg 2B = 0 sequence.
func (g *gemsDriver) stopDAC() {
	g.dacOn = false
	g.ym(0, 0x2B, 0)
}

// sequencer is 0458: advance each playing channel on its time base.
func (g *gemsDriver) sequencer() {
	timing := g.r8(0x08C2)
	g.w16(0x0454, 0x1D80)
	g.w16(0x0456, 0x1886)
	for b := byte(0); b < 16; b++ {
		ix := 0x1B80 + uint16(b)*0x20
		if b > 0 {
			g.call(1) // 0472
			g.w16(0x0454, g.r16(0x0454)+0x10)
			g.w16(0x0456, g.r16(0x0456)+0x27)
		}
		if !g.bit(ix+6, 4) {
			continue
		}
		if g.bit(ix+6, 3) {
			if timing&1 == 0 {
				continue
			}
		} else if timing&2 == 0 {
			continue
		}
		g.call(1)      // 04A2
		g.serviceDAC() // 04A7
		g.channelTick(ix, b)
	}
}

// readSeq is 03EF: next byte of the channel's sequence through its cache.
func (g *gemsDriver) readSeq(ix uint16) byte {
	ptr := g.r24(ix + 3)
	cache := g.r24(ix)
	g.call(1) // 03EF
	if ptr-cache >= 0x10 || ptr < cache {
		g.call(1) // 042D
		g.w24(ix, ptr)
		g.copyROM(ptr, g.r16(0x0454), 0x10)
		cache = ptr
	}
	g.call(2) // 0410, 0429
	v := g.r8(g.r16(0x0454) + uint16(ptr-cache))
	g.w24(ix+3, (ptr+1)&0xFFFFFF)
	return v
}

// channelTick is 04B4/04BC.
func (g *gemsDriver) channelTick(ix uint16, b byte) {
	cnt := g.r16(ix+7) + 1
	g.w16(ix+7, cnt)
	if cnt != 0 {
		return
	}
	a := g.readSeq(ix)
	for {
		if a&0x80 != 0 {
			// Durations (80..BF) and delays (C0..FF) accumulate 6 bits per byte.
			class := a & 0xC0
			delay := class == 0xC0
			v := uint16(a & 0x3F)
			for {
				a = g.readSeq(ix)
				if a&0xC0 != class {
					break
				}
				v = v<<6 | uint16(a&0x3F)
			}
			if delay {
				g.w16(ix+9, -v)
			} else {
				g.w16(ix+0x0B, -v)
			}
			continue
		}
		if a < 0x60 {
			if !g.bit(ix+6, 1) {
				g.noteOn(ix, b, a)
			}
			if g.waitDelay(ix) {
				return
			}
			a = g.readSeq(ix)
			continue
		}
		next, wait := g.seqCommand(ix, b, a-0x60)
		if next == stopChannel {
			return
		}
		if wait && g.waitDelay(ix) {
			return
		}
		a = g.readSeq(ix)
	}
}

const stopChannel = -1

// waitDelay is 055B: true when a non-zero delay suspends the channel.
func (g *gemsDriver) waitDelay(ix uint16) bool {
	g.call(1) // 055B
	d := g.r16(ix + 9)
	if d == 0 {
		return false
	}
	g.w16(ix+7, d)
	return true
}

// seqCommand handles 60..72 (0570). The bool reports whether the command
// ends through 055B (wait for the current delay) rather than 04BC.
func (g *gemsDriver) seqCommand(ix uint16, b, c byte) (int, bool) {
	switch c {
	case 0x00: // 0606 end of sequence
		g.w8(ix+6, 0)
		g.w8(ix+0x0B, 0)
		g.w8(ix+0x0C, 0)
		return stopChannel, false
	case 0x01: // 0613 patch change
		g.w8(ix+0x0D, g.readSeq(ix))
		g.loadPatch(ix)
		return 0, true
	case 0x02: // 05C0 modulation change
		g.w8(ix+0x1D, g.readSeq(ix))
		if !g.bit(ix+6, 6) {
			g.startModulation(ix, b, g.r8(ix+0x1D))
		}
		return 0, true
	case 0x03: // 055B no operation
		return 0, true
	case 0x04: // 0621 loop start
		iy := ix + 0x10
		for g.r8(iy) != 0 {
			iy += 3
		}
		g.w8(iy, g.readSeq(ix))
		g.w8(iy+1, g.r8(ix+3))
		g.w8(iy+2, g.r8(ix+4))
		return 0, false
	case 0x05: // 064C loop end
		iy := ix + 0x19
		for g.r8(iy) == 0 {
			iy -= 3
		}
		n := g.r8(iy)
		if n != 0x7F {
			n--
			g.w8(iy, n)
			if n == 0 {
				return 0, false
			}
		}
		target := uint16(g.r8(iy+1)) | uint16(g.r8(iy+2))<<8
		old := g.r16(ix + 3)
		g.w16(ix+3, target)
		if target >= old {
			g.w8(ix+5, g.r8(ix+5)-1)
		}
		return 0, false
	case 0x06: // 05DD envelope retrigger flag
		if g.readSeq(ix) != 0 {
			g.set(ix+6, 6)
		} else {
			g.res(ix+6, 6)
		}
		return 0, true
	case 0x07: // 05F2
		if g.readSeq(ix) != 0 {
			g.set(ix+6, 7)
		} else {
			g.res(ix+6, 7)
		}
		return 0, true
	case 0x08: // 068B tempo
		g.setTempo(g.readSeq(ix) + 0x28)
		return 0, true
	case 0x09: // 0696 mute another track of this sequence
		h := g.readSeq(ix)
		for i := uint16(0); i < 16; i++ {
			iy := 0x1B80 + i*0x20
			if !g.bit(iy+6, 0) || g.r8(iy+0x0E) != g.r8(ix+0x0E) || h&0x0F != g.r8(iy+0x0F) {
				continue
			}
			if h&0x10 != 0 {
				g.res(iy+6, 1)
			} else {
				g.set(iy+6, 1)
			}
			break
		}
		return 0, true
	case 0x0A: // 06D3 priority
		g.w8(ix+0x1C, g.readSeq(ix))
		return 0, true
	case 0x0B: // 06DC start sequence
		g.startSequence(g.readSeq(ix))
		return 0, true
	case 0x0C: // 06EB pitch bend
		iy := 0x0F76 + uint16(b)
		g.w8(iy, g.readSeq(ix))
		g.w8(iy+0x10, g.readSeq(ix))
		g.set(iy+0x40, 0)
		g.w8(0x0F75, 1)
		return 0, true
	case 0x0D: // 070C video-rate time base (sound effect)
		g.set(ix+6, 3)
		return 0, true
	case 0x0E: // 0713 sample rate of a DAC patch
		d := g.readSeq(ix)
		p := g.r16(0x0456)
		if g.r8(p) == 1 {
			g.w8(p+1, d)
		}
		return 0, true
	case 0x0F: // 0725 relative jump
		l := g.readSeq(ix)
		h := g.readSeq(ix)
		g.jump(ix, uint32(int32(int16(uint16(h)<<8|uint16(l)))))
		return 0, false
	case 0x10: // 074B store
		v := g.readSeq(ix)
		g.w8(0x1B22+uint16(v), g.readSeq(ix))
		return 0, true
	case 0x11: // 0752 conditional skip
		v := g.readSeq(ix)
		hl := 0x1B22 + uint16(v)
		d := g.readSeq(ix)
		a := g.readSeq(ix)
		m := g.r8(hl)
		var take bool
		switch d {
		case 1:
			take = a == m
		case 2:
			take = a >= m
		case 3:
			take = a > m
		case 4:
			take = a <= m
		case 5:
			take = a < m
		default:
			take = a != m
		}
		if take {
			l := g.readSeq(ix)
			g.jump(ix, uint32(l))
		} else {
			g.readSeq(ix)
		}
		return 0, false
	case 0x12: // 07A6 extended
		d := g.readSeq(ix)
		e := g.readSeq(ix)
		switch d {
		case 0:
			g.stopSequence(e)
		case 1:
			g.pauseSequence(e)
		case 2:
			g.resumeAll()
		case 3:
			g.pauseSequence(g.r8(0x1B22))
		case 4:
			g.w8(0x15CF, e)
		case 5:
			g.w8(ix+0x1E, e)
		}
		return 0, true
	}
	return 0, false // 05BD unknown: continue
}

func (g *gemsDriver) jump(ix uint16, delta uint32) {
	g.w24(ix+3, (g.r24(ix+3)+delta)&0xFFFFFF)
}

// pauseSequence is 0CD1.
func (g *gemsDriver) pauseSequence(n byte) {
	for i := uint16(0); i < 16; i++ {
		ix := 0x1B80 + i*0x20
		if g.bit(ix+6, 0) && !g.bit(ix+6, 5) && n == g.r8(ix+0x0E) {
			g.res(ix+6, 4)
		}
	}
	g.silenceIdleVoices()
}

// loadPatch is 1737: copy the channel's patch into its 39-byte cache.
func (g *gemsDriver) loadPatch(ix uint16) {
	bank := g.r24(0x0AA5)
	g.copyROM(bank+uint32(g.r8(ix+0x0D))*2, 0x1726, 2)
	g.copyROM(bank+uint32(g.r16(0x1726)), g.r16(0x0456), 0x27)
}

// voiceDurations is 0806: count down timed notes and release ages.
func (g *gemsDriver) voiceDurations() {
	b := g.r8(0x08C2)
	for h, list := range []uint16{0x1791, 0x17BC, 0x17D2} {
		psg := h > 0
		for ix := list; g.r8(ix) != 0xFF; ix += 7 {
			a := g.r8(ix)
			g.call(1) // 082E
			if a&0x08 != 0 {
				if b&1 == 0 {
					continue
				}
			} else if b&2 == 0 {
				continue
			}
			if a&0x40 != 0 {
				v := g.r8(ix+6) - 1
				g.w8(ix+6, v)
				if v == 0 {
					g.w8(ix, a&^0x40)
				}
				continue
			}
			if a&0x10 == 0 {
				continue
			}
			c := a & 7
			n := g.r16(ix+4) + 1
			g.w16(ix+4, n)
			if n != 0 {
				continue
			}
			g.res(ix, 4)
			g.res(ix, 3)
			a = g.r8(ix)
			if a&0x2F == 0x26 {
				g.dacRelease()
				continue
			}
			g.w8(ix, a|0xC0)
			if psg {
				g.set(0x0009+uint16(c), 1)
			} else {
				g.ym(0, 0x28, c)
			}
		}
	}
}

// dacRelease is 1712.
func (g *gemsDriver) dacRelease() {
	f := g.r8(0x1421)
	if f&0x20 != 0 {
		g.w8(0x0301, 7)
	} else if f&0x10 != 0 {
		g.w8(0x0301, 6)
	}
}

// startModulation is 0E2D (E = channel, C = envelope).
func (g *gemsDriver) startModulation(chIX uint16, e, c byte) {
	flags := g.r8(chIX + 6)
	iy := 0x0F76 + uint16(e)
	slot := uint16(0)
	for s := uint16(0x0EAB); g.r8(s)&0x80 == 0; s++ {
		if g.r8(s) == e {
			slot = s
			break
		}
	}
	if slot == 0 {
		for s := uint16(0x0EAB); ; s++ {
			a := g.r8(s)
			if a&0x80 != 0 {
				return
			}
			if a&0x40 != 0 {
				slot = s
				break
			}
		}
	}
	if flags&0x08 != 0 {
		e |= 0x20
	}
	g.w8(slot, e)
	g.w8(slot+0x0D, 0)
	bank := g.r24(0x0AA8)
	g.copyROM(bank+uint32(c)*2, 0x1726, 2)
	buf := 0x1E00 | uint16(g.r8(slot+0x19))
	g.copyROM(bank+uint32(g.r16(0x1726)), buf, 0x20)
	g.w8(iy+0x20, g.r8(buf))
	g.w8(iy+0x30, g.r8(buf+1))
	p := buf + 2
	g.w8(slot+0x05, byte(p))
	g.w8(slot+0x09, byte(p>>8))
}

// modulation is 0EC8: pitch envelopes, one step per time-base tick.
func (g *gemsDriver) modulation() {
	timing := g.r8(0x08C2)
	for ix := uint16(0x0EAB); ; ix++ {
		g.call(1) // 0ECC
		c := g.r8(ix)
		if c&0x80 != 0 {
			return
		}
		if c&0x40 != 0 {
			continue
		}
		if c&0x20 != 0 {
			if timing&1 == 0 {
				continue
			}
		} else if timing&2 == 0 {
			continue
		}
		iy := 0x0F76 + uint16(c&^0x20)
		cnt := g.r8(ix + 0x0D)
		if cnt == 0 {
			hl := uint16(g.r8(ix+0x05)) | uint16(g.r8(ix+0x09))<<8
			n := g.r8(hl)
			if n == 0 {
				g.w8(iy+0x20, 0)
				g.w8(iy+0x30, 0)
				g.w8(ix, 0x40)
				g.set(iy+0x40, 0)
				g.w8(0x0F75, 1)
				continue
			}
			g.w8(ix+0x11, g.r8(hl+1))
			g.w8(ix+0x15, g.r8(hl+2))
			hl += 3
			g.call(1) // 0F0B
			g.w8(ix+0x05, byte(hl))
			g.w8(ix+0x09, byte(hl>>8))
			cnt = n
		}
		g.w8(ix+0x0D, cnt-1)
		lo := uint16(g.r8(iy+0x20)) + uint16(g.r8(ix+0x11))
		g.w8(iy+0x20, byte(lo))
		g.w8(iy+0x30, g.r8(iy+0x30)+g.r8(ix+0x15)+byte(lo>>8))
		g.set(iy+0x40, 0)
		g.w8(0x0F75, 1)
	}
}

// frequency is 10A8: FM block/F-number or PSG period for note C plus the
// channel's bend and modulation (8.8 semitones), interpolated in the tables.
func (g *gemsDriver) frequency(psg bool, ch, note byte) uint16 {
	g.call(6) // 10AA 10C0 10DB 1115 112C 114D
	ix := 0x0F76 + uint16(ch)
	lo := uint16(g.r8(ix)) + uint16(g.r8(ix+0x20))
	hi := g.r8(ix+0x10) + g.r8(ix+0x30) + byte(lo>>8)
	frac := byte(lo)
	a := note + hi
	if a >= 0x60 {
		if hi&0x80 != 0 {
			a, frac = 0, 0
		} else {
			frac, a = 0xFF, 0x5F
		}
	}
	var table []uint16
	block := byte(0)
	if !psg {
		if a >= 0x30 {
			a -= 0x30
			block |= 4
		}
		if a >= 0x18 {
			a -= 0x18
			block |= 2
		}
		if a >= 0x0C {
			a -= 0x0C
			block |= 1
		}
		table = gemsFMTable[:]
	} else {
		if a < 0x21 {
			a, frac = 0, 0
		} else {
			a -= 0x21
		}
		table = gemsPSGTable[:]
	}
	base, nxt := table[a], table[a+1]
	h := byte(mul16(nxt-base, frac) >> 8)
	de := base + uint16(int16(int8(h)))
	if !psg {
		de |= uint16(block) << 11
	}
	return de
}

// noteOn is 1206 (C = note, B = channel).
func (g *gemsDriver) noteOn(ix uint16, b, note byte) {
	g.call(1) // 1206
	att := g.r8(0x15CF) + g.r8(ix+0x1E)
	g.serviceDAC() // 121B
	if att&0x80 != 0 {
		att = 0x7F
	}
	g.w8(0x1205, att)
	g.w8(0x1202, note)
	g.w8(0x1203, b)
	p := g.r16(0x0456)
	switch g.r8(p) {
	case 0:
		g.noteFM(ix, b, note, p)
	case 1:
		g.noteDAC(ix, b, note, p)
	case 2:
		if v := g.allocate(ix, 0x17BC, b); v != 0 {
			g.notePSG(ix, b, note, p, v)
		}
	case 3:
		if v := g.fixedVoice(ix, 0x17D2); v != 0 {
			g.notePSG(ix, b, note, p, v)
		}
	}
}

// allocate is 17DE: free voice of this channel, else the longest released
// one, else steal the lowest priority not above this channel's.
func (g *gemsDriver) allocate(ix, list uint16, b byte) uint16 {
	c, l := byte(0xFF), byte(0xFF)
	h := g.r8(ix + 6)
	for iy := list; ; iy += 7 {
		g.call(1) // 17EC
		if g.r8(iy) == 0xFF {
			break
		}
		a := g.r8(iy)
		if a&0x20 != 0 {
			continue
		}
		if a&0x80 == 0 {
			if p := g.r8(iy + 1); p < c {
				c = p
				g.w16(0x17DA, iy)
			}
			continue
		}
		if g.r8(iy+3) == b && h&0x80 == 0 {
			return iy
		}
		if age := g.r8(iy + 6); age < l {
			l = age
			g.w16(0x17DC, iy)
		}
	}
	if l != 0xFF {
		return g.r16(0x17DC)
	}
	return g.steal(ix, c)
}

// steal is 1832: take the busy voice (17DA) if its priority does not
// exceed this channel's.
func (g *gemsDriver) steal(ix uint16, prio byte) uint16 {
	if prio <= g.r8(ix+0x1C) {
		return g.r16(0x17DA)
	}
	return 0
}

// fixedVoice is 1841.
func (g *gemsDriver) fixedVoice(ix, iy uint16) uint16 {
	g.call(1) // 1841
	if g.r8(iy)&0x80 != 0 {
		return iy
	}
	g.w16(0x17DA, iy)
	return g.steal(ix, g.r8(iy+1))
}

// recordVoice is 1659.
func (g *gemsDriver) recordVoice(ix uint16, b byte, v uint16) {
	a := g.r8(v) & 7
	g.w8(0x1204, a)
	dur := g.r16(ix + 0x0B)
	if dur != 0 {
		a |= 0x10
	}
	if g.bit(ix+6, 3) {
		a |= 0x08
	}
	g.w8(v, a)
	g.w8(v+1, g.r8(ix+0x1C))
	g.w8(v+2, g.r8(0x1202))
	g.w8(v+3, g.r8(0x1203))
	g.w16(v+4, dur)
	g.w8(v+6, 0xFE)
	g.call(1) // 1687
	if g.bit(ix+6, 6) {
		g.startModulation(ix, b, g.r8(ix+0x1D))
		g.call(1) // 1697
	}
}

func (g *gemsDriver) notePSG(ix uint16, b, note byte, p, v uint16) {
	g.recordVoice(ix, b, v)
	de := g.frequency(true, b, note)
	hw := uint16(g.r8(0x1204))
	iy := 0x0009 + hw
	g.w8(iy+0x1C, byte(de)&0x0F)
	g.w8(iy+0x20, byte(de>>4))
	if hw == 3 {
		g.res(0x17CA, 5)
		if g.r8(p+1)&3 == 3 {
			g.psg(g.r8(iy+0x1C) | 0xC0)
			g.psg(g.r8(iy + 0x20))
			g.w8(0x17CA, 0xA2)
			g.w8(0x17CC, g.r8(0x1202))
			g.w8(0x17CD, g.r8(0x1203))
			g.w8(0x000B, 0x04)
		}
		g.w8(iy+0x1C, g.r8(p+1))
	}
	g.w8(iy+0x08, g.r8(p+2))
	g.w8(iy+0x10, g.r8(p+3)<<4)
	g.w8(iy+0x24, g.r8(p+4)<<4)
	g.w8(iy+0x0C, g.r8(p+5))
	g.w8(iy+0x14, g.r8(p+6))
	g.set(iy, 0)
}

var gemsCarrierMask = [8]byte{0x08, 0x08, 0x08, 0x08, 0x0A, 0x0E, 0x0E, 0x0F}

// gemsPatchRegisters is the 161C table: register, patch offset (+1); 80 marks
// a total level scaled for carriers, 00 writes zero (SSG-EG).
var gemsPatchRegisters = []byte{
	0xB0, 0x02, 0xB4, 0x03,
	0x30, 0x04, 0x40, 0x85, 0x50, 0x06, 0x60, 0x07, 0x70, 0x08, 0x80, 0x09, 0x90, 0x00,
	0x38, 0x10, 0x48, 0x91, 0x58, 0x12, 0x68, 0x13, 0x78, 0x14, 0x88, 0x15, 0x98, 0x00,
	0x34, 0x0A, 0x44, 0x8B, 0x54, 0x0C, 0x64, 0x0D, 0x74, 0x0E, 0x84, 0x0F, 0x94, 0x00,
	0x3C, 0x16, 0x4C, 0x97, 0x5C, 0x18, 0x6C, 0x19, 0x7C, 0x1A, 0x8C, 0x1B, 0x9C, 0x00,
}

// noteFM is 142F.
func (g *gemsDriver) noteFM(ix uint16, b, note byte, p uint16) {
	d := g.r8(p + 2)
	var v uint16
	if d&0x40 != 0 {
		v = g.fixedVoice(ix, 0x17B4)
	} else {
		v = g.allocate(ix, 0x1791, b)
	}
	g.call(1) // 1446
	if v == 0 {
		return
	}
	if a := g.r8(v); a&0x80 == 0 {
		g.ym(0, 0x28, a&7)
	}
	g.recordVoice(ix, b, v)
	var freq uint16
	if d&0x40 == 0 {
		freq = g.frequency(false, b, note)
		g.w16(0x142D, freq)
		g.call(1) // 1480
	}
	q := p + 1
	hw := g.r8(0x1204)
	if hw == 2 {
		g.timerCtl = g.r8(q+1) | 0x15
		g.ym(0, 0x27, g.r8(q+1)|0x05)
	}
	part, e := 0, hw
	if e >= 3 {
		e -= 4
		part = 1
	}
	if h := g.r8(q); h&0x08 != 0 {
		g.ym(0, 0x22, h)
	}
	mask := gemsCarrierMask[g.r8(q+2)&7]
	att := uint16(g.r8(0x1205))
	for i := 0; i < len(gemsPatchRegisters); i += 2 {
		reg, src := gemsPatchRegisters[i], gemsPatchRegisters[i+1]
		val := byte(0)
		switch {
		case src == 0:
		case src&0x80 == 0:
			val = g.r8(q + uint16(src))
		default:
			tl := g.r8(q + uint16(src&0x7F))
			carrier := mask&1 != 0
			mask >>= 1
			if carrier {
				lvl := byte(0x7F) - tl
				h := byte(mul16(uint16(lvl)*2, byte(att)) >> 8)
				val = 0x7F - (lvl - h)
			} else {
				val = tl
			}
		}
		g.ym(part, reg+e, val)
	}
	if g.r8(q+1)&0x40 != 0 {
		for i, reg := range []byte{0xA6, 0xA2, 0xAC, 0xA8, 0xAD, 0xA9, 0xAE, 0xAA} {
			g.ym(0, reg, g.r8(q+0x1C+uint16(i)))
		}
	} else {
		g.ym(part, 0xA4+e, byte(freq>>8))
		g.ym(part, 0xA0+e, byte(freq))
	}
	g.ym(0, 0x28, g.r8(q+0x24)<<4|hw)
}

// noteDAC is 12F7: sample (note - 30) on FM channel 6.
func (g *gemsDriver) noteDAC(ix uint16, b, note byte, p uint16) {
	v := g.fixedVoice(ix, 0x17AD)
	if v == 0 {
		return
	}
	if a := g.r8(v); a&0x80 == 0 {
		// A busy voice is taken over: stop streaming, key off FM6.
		g.dacOn = false
		if a&0x20 == 0 {
			g.ym(0, 0x28, a&7)
		}
	}
	g.recordVoice(ix, b, v)
	g.set(0x17AD, 5)
	n := note - 0x30
	if note < 0x30 {
		n += 0x60
	}
	g.copyROM(g.r24(0x0AAE)+uint32(n)*12, 0x1421, 12)
	if g.r16(0x1427) == 0 {
		g.w8(0x17AD, 0xC6)
		return
	}
	if r := g.r8(p + 1); r != 4 {
		g.w8(0x1421, g.r8(0x1421)&0xF0|r)
	}
	rate := g.r8(0x1421) & 0x0F
	tick := (0x400 - int(rate)) & 0x3FF
	g.ym(0, 0x24, byte(tick>>2))
	g.ym(0, 0x25, byte(tick&3))
	g.ym(0, 0x2B, 0x80)
	g.ym(0, 0x27, g.timerCtl) // timer A on (B' value in the DAC routine)
	g.ym(1, 0xB6, 0xC0)
	start := g.r24(0x0AAE) + g.r24(0x1422) + uint32(g.r16(0x1425))
	g.w24(0x02FC, start)
	g.w16(0x02FF, g.r16(0x1427))
	g.w8(0x02FB, 0)
	mode := byte(4)
	if g.r8(0x1421)&0x10 != 0 {
		mode = 5
	}
	g.w8(0x0301, mode)
	g.dacRead = 0
	g.dacOn = false
	g.dacRate = int(rate)
	if g.dacRate == 0 {
		g.dacRate = 0x400
	}
	g.dacNibble = g.r8(0x1421)&0x80 != 0
	g.dacPhase = 0xAA
	g.refillDAC() // 0310, before 02B7 is enabled
	// 13F2: streaming starts on the timer loaded above.
	g.dacOn = true
	g.dacPeriod = int64(g.dacRate) * fmCycles
	g.lastTick = g.clk
	g.dacIdle = false
	g.serviceDAC()
}

// serviceDAC is 0303: refill once playback enters the half that is due.
func (g *gemsDriver) serviceDAC() {
	if g.dacOn && (g.r8(0x02FB)^g.dacRead)&0x80 != 0 {
		g.refillDAC()
	}
}

// refillDAC is 0314: copy the next 128 bytes into the free buffer half,
// handling loop and end of the sample.
func (g *gemsDriver) refillDAC() {
	g.call(1) // 0314
	if g.r8(0x0301) >= 7 {
		g.endDAC()
		return
	}
	half := g.r8(0x02FB)
	g.w8(0x02FB, half+0x80)
	src := g.r24(0x02FC)
	left := g.r16(0x02FF)
	if left > 0x80 {
		g.w16(0x02FF, left-0x80)
		g.fillDAC(half, src, 0x80)
		g.w24(0x02FC, src+0x80)
		return
	}
	// 035C: last part of the sample (1..128 bytes).
	c := uint32(left)
	g.fillDAC(half, src, int(c))
	if g.r8(0x0301) != 5 {
		// One-shot: streaming stops at once, as in the original.
		g.endDAC()
		return
	}
	// Looping sample: continue from its loop region.
	loop := uint32(g.r16(0x1429))
	src = src + c - loop
	g.w24(0x02FC, src)
	g.w16(0x02FF, uint16(loop))
	rest := 0x80 - c
	if rest == 0 {
		return
	}
	g.w16(0x02FF, uint16(loop-rest))
	g.call(copyCalls(int(rest)))
	for i := uint32(0); i < rest; i++ {
		g.dacBuffer[byte(uint32(half)+c+i)] = g.romByte(src + i)
	}
	g.w24(0x02FC, src+rest)
}

func (g *gemsDriver) fillDAC(half byte, src uint32, n int) {
	g.call(copyCalls(n))
	for i := 0; i < n; i++ {
		g.dacBuffer[byte(int(half)+i)] = g.romByte(src + uint32(i))
	}
}

// endDAC is 03CC.
func (g *gemsDriver) endDAC() {
	g.dacOn = false
	g.ym(0, 0x2B, 0)
	g.w8(0x17AD, 0xC6)
	g.w8(0x17B1, 0)
	g.w8(0x17B2, 0)
}

// dacSample is one timer A period of 02B7: the next DAC value, if any.
func (g *gemsDriver) dacSample() (byte, bool) {
	if !g.dacOn {
		return 0, false
	}
	a := g.dacBuffer[g.dacRead]
	if g.dacNibble {
		carry := g.dacPhase&1 != 0
		g.dacPhase = g.dacPhase>>1 | g.dacPhase<<7
		if carry {
			g.dacRead++
		} else {
			a <<= 4
		}
		a &= 0xF0
	} else {
		g.dacRead++
	}
	return a, true
}

// psgEnvelopes is 0066: PSG attack/decay/sustain/release at video rate.
func (g *gemsDriver) psgEnvelopes() {
	d := byte(0x80)
	for e := 4; e > 0; e-- {
		iy := uint16(0x0009 + 4 - e)
		g.call(1) // 0071
		c := g.r8(iy)
		g.w8(iy, 0)
		if c&4 != 0 {
			g.w8(iy+4, 0xFF)
			g.w8(iy+0x28, 1)
			g.w8(iy+0x18, 0)
			if e == 1 {
				g.res(0x17CA, 5)
			}
		}
		if c&2 != 0 && g.r8(iy+0x18) != 0 {
			g.w8(iy+0x28, 1)
			g.w8(iy+0x18, 4)
		}
		if c&1 != 0 {
			g.w8(iy+4, 0xFF)
			g.psg(g.r8(iy+0x1C) | d)
			if e != 1 {
				g.psg(g.r8(iy + 0x20))
			}
			g.w8(iy+0x28, 1)
			g.w8(iy+0x18, 1)
		}
		g.call(1) // 00C9
		switch g.r8(iy + 0x18) {
		case 1:
			g.w8(iy+0x28, 1)
			v, b := g.r8(iy+4), g.r8(iy+0x24)
			a := v - g.r8(iy+8)
			if v <= g.r8(iy+8) || a <= b {
				g.w8(iy+4, b)
				g.w8(iy+0x18, 2)
			} else {
				g.w8(iy+4, a)
			}
		case 2:
			g.w8(iy+0x28, 1)
			a, b := g.r8(iy+4), g.r8(iy+0x10)
			done := false
			switch {
			case a == b:
				done = true
			case a < b:
				s := uint16(a) + uint16(g.r8(iy+0x0C))
				if s > 0xFF || byte(s) >= b {
					done = true
				} else {
					a = byte(s)
				}
			default:
				if a < g.r8(iy+0x0C) {
					done = true
				} else if a -= g.r8(iy + 0x0C); a < b {
					done = true
				}
			}
			if done {
				g.w8(iy+4, b)
				g.w8(iy+0x18, 3)
			} else {
				g.w8(iy+4, a)
			}
		case 4:
			g.w8(iy+0x28, 1)
			s := uint16(g.r8(iy+4)) + uint16(g.r8(iy+0x14))
			if s > 0xFF {
				g.w8(iy+4, 0xFF)
				g.w8(iy+0x18, 0)
				if e == 1 {
					g.res(0x17CA, 5)
				}
			} else {
				g.w8(iy+4, byte(s))
			}
		}
		d += 0x20
	}
	g.call(1) // 0166
	for i := uint16(0); i < 4; i++ {
		if g.r8(0x0031+i)&1 != 0 {
			g.w8(0x0031+i, 0)
			g.psg(g.r8(0x000D+i)>>4 | byte(0x90+0x20*i))
		}
	}
	g.call(1) // 01CD
}

// applyPitch is 0FC6: re-tune voices whose channel pitch changed.
func (g *gemsDriver) applyPitch() {
	if g.r8(0x0F75) == 0 {
		return
	}
	g.w8(0x0F75, 0)
	for iy := uint16(0x1791); ; iy += 7 {
		g.call(1) // 0FD6
		if g.r8(iy) == 0xFF {
			break
		}
		hw := g.r8(iy) & 7
		ch := g.r8(iy + 3)
		if !g.bit(0x0F76+uint16(ch)+0x40, 0) {
			continue
		}
		de := g.frequency(false, ch, g.r8(iy+2))
		g.w16(0x142D, de)
		part, e := 0, hw
		if e >= 3 {
			e -= 4
			part = 1
		}
		g.ym(part, 0xA4+e, byte(de>>8))
		g.ym(part, 0xA0+e, byte(de))
	}
	for iy := uint16(0x17BC); ; iy += 7 {
		g.call(1) // 1047
		if g.r8(iy) == 0xFF {
			break
		}
		hw := g.r8(iy) & 7
		ch := g.r8(iy + 3)
		if !g.bit(0x0F76+uint16(ch)+0x40, 0) {
			continue
		}
		de := g.frequency(true, ch, g.r8(iy+2))
		g.call(1) // 106D
		g.psg(byte(de)&0x0F | 0x80 | hw<<5)
		g.psg(byte(de >> 4))
	}
	for i := uint16(0); i < 16; i++ {
		g.w8(0x0FB6+i, 0)
	}
}

// Note tables 1168 (YM2612 F-numbers, one octave) and 1182 (PSG periods).
var gemsFMTable = [13]uint16{0x284, 0x2AA, 0x2D3, 0x2FE, 0x32B, 0x35B, 0x38E, 0x3C5, 0x3FE, 0x43B, 0x47B, 0x4BF, 0x508}
var gemsPSGTable = [64]uint16{
	0x3F9, 0x3C0, 0x38A, 0x357, 0x327, 0x2FA, 0x2CF, 0x2A7, 0x281, 0x25D, 0x23B, 0x21B,
	0x1FC, 0x1E0, 0x1C5, 0x1AC, 0x194, 0x17D, 0x168, 0x153, 0x140, 0x12E, 0x11D, 0x10D,
	0x0FE, 0x0F0, 0x0E2, 0x0D6, 0x0CA, 0x0BE, 0x0B4, 0x0AA, 0x0A0, 0x097, 0x08F, 0x087,
	0x07F, 0x078, 0x071, 0x06B, 0x065, 0x05F, 0x05A, 0x055, 0x050, 0x04C, 0x047, 0x043,
	0x040, 0x03C, 0x039, 0x035, 0x032, 0x02F, 0x02D, 0x02A, 0x028, 0x026, 0x023, 0x021,
	0x020, 0x01E, 0x01C, 0x01C,
}

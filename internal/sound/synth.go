package sound

import "aladdin/internal/timing"

// Audio timeline of the native sound system: the GEMS driver port runs at
// each video boundary, the YM2612 at clock/144 and the PSG at Z80 clock/16,
// all on the NTSC master clock. Chip writes carry the driver's own time (see
// gemsDriver.call). Output is band-limited to 44.1 kHz by integrating the
// chips' piecewise-constant outputs over each output period.

const (
	masterClock = timing.MasterClock
	fmCycles    = 7 * 144            // master cycles per YM2612 sample
	psgCycles   = 15 * 16            // master cycles per PSG step
	frameCycles = timing.FrameCycles // master cycles per NTSC video boundary
	never       = int64(1) << 62
)

// Output gains (calibrated against the original's recorded output).
var (
	synthFMGain  = 0.92
	synthPSGGain = 0.52
)

type genesisSynth struct {
	drv *gemsDriver
	ym  *ym2612
	psg *sn76489

	t, nextFM, nextPSG, nextFrame int64
	outIndex                      int64

	fmL, fmR float64 // current chip outputs (held until the next step)
	psgOut   float64
	accL     float64
	accR     float64

	pending [][]byte    // requests applied after the next video boundary's work
	events  []chipEvent // timed chip writes not yet applied

	gainFM, gainPSG float64
}

func newGenesisSynth(rom []byte) *genesisSynth {
	s := &genesisSynth{ym: newYM2612(), psg: newSN76489(), gainFM: synthFMGain, gainPSG: synthPSGGain}
	s.drv = newGemsDriver(rom)
	s.nextFM = fmCycles
	s.nextPSG = psgCycles
	s.nextFrame = frameCycles
	s.collect()
	return s
}

// request queues a 68k-side GEMS request (code and arguments).
func (s *genesisSynth) request(b ...byte) {
	s.pending = append(s.pending, append([]byte(nil), b...))
}

func outBoundary(k int64) int64 { return k * masterClock / SampleRate }

func (s *genesisSynth) collect() {
	s.events = append(s.events, s.drv.events...)
	s.drv.events = s.drv.events[:0]
}

func (s *genesisSynth) apply(e chipEvent) {
	if e.psg {
		s.psg.write(e.val)
		s.psgOut = float64(s.psg.output())
		return
	}
	s.ym.write(e.part, e.reg, e.val)
}

// render fills dst with interleaved stereo samples.
func (s *genesisSynth) render(dst []float64) {
	for i := 0; i+1 < len(dst); i += 2 {
		end := outBoundary(s.outIndex + 1)
		start := s.t
		for s.t < end {
			ev := end
			if s.nextFM < ev {
				ev = s.nextFM
			}
			if s.nextPSG < ev {
				ev = s.nextPSG
			}
			if s.nextFrame < ev {
				ev = s.nextFrame
			}
			if len(s.events) > 0 && s.events[0].t < ev {
				ev = max(s.events[0].t, s.t)
			}
			dacTick := never
			if s.drv.dacOn {
				dacTick = s.drv.lastTick + s.drv.dacPeriod
				if dacTick < ev {
					ev = max(dacTick, s.t)
				}
			}
			dt := float64(ev - s.t)
			s.accL += (s.gainFM*s.fmL + s.gainPSG*s.psgOut) * dt
			s.accR += (s.gainFM*s.fmR + s.gainPSG*s.psgOut) * dt
			s.t = ev
			if s.t == s.nextFrame {
				s.frame()
				s.nextFrame += frameCycles
			}
			if s.drv.dacOn && dacTick <= s.t && s.drv.clk <= s.t {
				// Idle main loop: its 02B7 calls consume every timer tick.
				s.drv.clk = s.t
				s.drv.tick()
				s.drv.serviceDAC()
				s.collect()
			}
			for len(s.events) > 0 && s.events[0].t <= s.t {
				s.apply(s.events[0])
				s.events = s.events[1:]
			}
			if s.t == s.nextPSG {
				s.psg.tick()
				s.psgOut = float64(s.psg.output())
				s.nextPSG += psgCycles
			}
			if s.t == s.nextFM {
				l, r := s.ym.sample()
				s.fmL, s.fmR = float64(l), float64(r)
				s.nextFM += fmCycles
			}
		}
		span := float64(end - start)
		dst[i], dst[i+1] = s.accL/span, s.accR/span
		s.accL, s.accR = 0, 0
		s.outIndex++
	}
}

// frame is the video-boundary interrupt: the driver resumes when its
// previous work is done, handles the boundary, then the game's requests.
func (s *genesisSynth) frame() {
	d := s.drv
	if d.clk < s.t {
		d.clk = s.t
	}
	d.vblank()
	d.run()
	for _, b := range s.pending {
		d.command(b...)
	}
	s.pending = s.pending[:0]
	d.run()
	s.collect()
}

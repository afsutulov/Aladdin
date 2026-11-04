// Package sound plays the game's music and effects by running a native Go
// port of the original GEMS sound driver logic on the game's own sequence,
// patch, envelope and sample data, through Go models of the YM2612 FM and
// SN76489 PSG chips. No recordings, ROM image, CPU or Z80 program is used.
package sound

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/fs"
	"math"
	"sync"
)

const SampleRate = 44100

// gemsDataSHA256 identifies gems.bin: the original patch, envelope,
// sequence and sample banks (ROM 1E54F4..219D50), converted unchanged.
const gemsDataSHA256 = "8b9486a88295359c851085d956304097356959153e28cb1ce175234214360b4a"

// musicSequences use the music setting; all other requests are effects.
var musicSequences = map[int]bool{10: true, 15: true, 16: true, 17: true, 18: true, 20: true, 22: true, 26: true,
	27: true, 29: true, 35: true, 38: true, 50: true, 73: true, 82: true, 84: true, 85: true, 86: true}

const sequenceCount = 122

// Mixer is a concurrency-safe stereo signed-16-bit little-endian stream.
type Mixer struct {
	mu           sync.Mutex
	synth        *genesisSynth
	enabled      bool
	musicEnabled bool
	music        map[int]bool
	effects      map[int]bool
	buf          []float64
	tail         [4]byte
	tailPosition int
	hpL, hpR     dcBlocker
}

// dcBlocker removes the DC offset of the chip outputs (AC-coupled output).
type dcBlocker struct{ x1, y1 float64 }

func (d *dcBlocker) step(x float64) float64 {
	y := x - d.x1 + 0.9986*d.y1
	d.x1, d.y1 = x, y
	return y
}

func Load(files fs.FS) (*Mixer, error) {
	rom, err := fs.ReadFile(files, "gems.bin")
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(rom)
	if hex.EncodeToString(sum[:]) != gemsDataSHA256 {
		return nil, fmt.Errorf("sound data checksum mismatch")
	}
	return &Mixer{synth: newGenesisSynth(rom), enabled: true, musicEnabled: true, music: map[int]bool{}, effects: map[int]bool{}, tailPosition: 4}, nil
}

// StopAll is the driver's stop-all request.
func (m *Mixer) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synth.request(0x16)
	m.music = map[int]bool{}
	m.effects = map[int]bool{}
}

// SetEnabled switches sound effects; disabling stops the running ones.
func (m *Mixer) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.enabled && !enabled {
		for id := range m.effects {
			m.synth.request(0x12, byte(id))
		}
		m.effects = map[int]bool{}
	}
	m.enabled = enabled
}

// SetMusicEnabled controls music independently of effects. Re-enabling
// does not resurrect an old song: the game explicitly requests its new song.
func (m *Mixer) SetMusicEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.musicEnabled && !enabled {
		for id := range m.music {
			m.synth.request(0x12, byte(id))
		}
		m.music = map[int]bool{}
	}
	m.musicEnabled = enabled
}

// Command first stops the requested sequence, just as 219F48 does, then
// starts it (219F2A). A stop-only command never starts a replacement.
func (m *Mixer) Command(id int, stopOnly bool) error {
	if id < 0 || id >= sequenceCount {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synth.request(0x12, byte(id))
	delete(m.effects, id)
	delete(m.music, id)
	if stopOnly {
		return nil
	}
	if musicSequences[id] {
		if !m.musicEnabled {
			return nil
		}
		// 219F48 stops only this ID. Other sequences retain their channels;
		// GEMS priorities and explicit sequence commands decide any voice stealing.
		m.music[id] = true
	} else {
		if !m.enabled {
			return nil
		}
		m.effects[id] = true
	}
	m.synth.request(0x10, byte(id))
	return nil
}

func clampSample(v float64) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(math.Round(v))
}

// Read fills every request. Partial stereo-frame reads preserve byte order.
func (m *Mixer) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(p)
	for len(p) > 0 && m.tailPosition < 4 {
		c := copy(p, m.tail[m.tailPosition:])
		p = p[c:]
		m.tailPosition += c
	}
	frames := (len(p) + 3) / 4
	if frames == 0 {
		return n, nil
	}
	if cap(m.buf) < frames*2 {
		m.buf = make([]float64, frames*2)
	}
	buf := m.buf[:frames*2]
	m.synth.render(buf)
	for i := 0; i < frames; i++ {
		l := clampSample(m.hpL.step(buf[i*2]))
		r := clampSample(m.hpR.step(buf[i*2+1]))
		var f [4]byte
		binary.LittleEndian.PutUint16(f[:2], uint16(l))
		binary.LittleEndian.PutUint16(f[2:], uint16(r))
		c := copy(p, f[:])
		p = p[c:]
		if c < 4 {
			m.tail = f
			m.tailPosition = c
		}
	}
	return n, nil
}

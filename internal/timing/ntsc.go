// Package timing keeps game updates and sound on the same NTSC clock.
package timing

const (
	MasterClock = 53693175
	FrameCycles = 262 * 3420
	HostTPS     = 60
)

// Clock schedules NTSC game frames inside Ebitengine's integer 60 Hz updates.
// It distributes the occasional repeated display frame without changing the
// simulation step or the pitch/tempo of the original sound sequences.
type Clock struct {
	phase   uint64
	started bool
}

func (c *Clock) Advance() bool {
	const period = uint64(FrameCycles * HostTPS)
	if !c.started {
		c.started = true
		c.phase = period - 1
	}
	c.phase += MasterClock
	if c.phase < period {
		return false
	}
	c.phase -= period
	return true
}

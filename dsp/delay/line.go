package delay

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/interp"
)

// Line is a circular delay line with configurable fractional-sample
// interpolation.
type Line struct {
	buffer       []float64
	writePos     int
	mode         interp.Mode
	sincHalfN    int     // half-width for Sinc mode (default 8 = 16 taps)
	allpassState float64 // one-sample state for Allpass mode
}

// Option configures a Line.
type Option func(*Line)

// WithMode sets the interpolation mode used by ReadFractional.
// The default is interp.Hermite.
func WithMode(m interp.Mode) Option {
	return func(l *Line) { l.mode = m }
}

// WithSincN sets the half-width for Sinc mode (2*n taps total).
// Ignored for other modes. Default is 8.
func WithSincN(n int) Option {
	return func(l *Line) {
		if n > 0 {
			l.sincHalfN = n
		}
	}
}

// New returns a delay line of fixed size.
func New(size int, opts ...Option) (*Line, error) {
	if size <= 0 {
		return nil, fmt.Errorf("delay size must be > 0: %d", size)
	}
	l := &Line{
		buffer:    make([]float64, size),
		mode:      interp.Hermite,
		sincHalfN: 8,
	}
	for _, o := range opts {
		o(l)
	}
	return l, nil
}

// Len returns internal buffer size.
func (d *Line) Len() int {
	return len(d.buffer)
}

// Write writes one sample and advances the write pointer.
func (d *Line) Write(sample float64) {
	d.buffer[d.writePos] = sample
	d.writePos++
	if d.writePos >= len(d.buffer) {
		d.writePos = 0
	}
}

// Read reads an integer delay in samples. delay=1 returns the most
// recently written sample; delay=0 returns the sample at the write
// head (oldest when the buffer is full).
func (d *Line) Read(delay int) float64 {
	size := len(d.buffer)
	if size == 0 {
		return 0
	}
	readPos := (d.writePos - delay + size) % size
	return d.buffer[readPos]
}

// ReadFractional reads at a fractional delay using the configured
// interpolation mode.
func (d *Line) ReadFractional(delay float64) float64 {
	size := len(d.buffer)
	if size == 0 {
		return 0
	}
	if delay < 0 {
		delay = 0
	}

	switch d.mode {
	case interp.Linear:
		return d.readLinear(delay, size)
	case interp.Hermite:
		return d.readHermite(delay, size)
	case interp.Lagrange3:
		return d.readLagrange(delay, size)
	case interp.Lanczos3:
		return d.readLanczos(delay, size)
	case interp.Sinc:
		return d.readSinc(delay, size)
	case interp.Allpass:
		return d.readAllpass(delay, size)
	default:
		return d.readHermite(delay, size)
	}
}

// Reset clears line state.
func (d *Line) Reset() {
	for i := range d.buffer {
		d.buffer[i] = 0
	}
	d.writePos = 0
	d.allpassState = 0
}

// --- interpolation read helpers ---

func (d *Line) readLinear(delay float64, size int) float64 {
	maxDelay := float64(size - 1)
	if delay > maxDelay {
		delay = maxDelay
	}
	p := int(math.Floor(delay))
	t := delay - float64(p)
	x0 := d.Read(p)
	x1 := d.Read(minInt(p+1, size-1))
	return interp.Linear2(t, x0, x1)
}

func (d *Line) readHermite(delay float64, size int) float64 {
	maxDelay := float64(size - 3)
	if maxDelay < 0 {
		maxDelay = 0
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	p := int(math.Floor(delay))
	t := delay - float64(p)
	xm1 := d.Read(maxInt(0, p-1))
	x0 := d.Read(p)
	x1 := d.Read(p + 1)
	x2 := d.Read(p + 2)
	return interp.Hermite4(t, xm1, x0, x1, x2)
}

func (d *Line) readLagrange(delay float64, size int) float64 {
	maxDelay := float64(size - 3)
	if maxDelay < 0 {
		maxDelay = 0
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	p := int(math.Floor(delay))
	t := delay - float64(p)
	xm1 := d.Read(maxInt(0, p-1))
	x0 := d.Read(p)
	x1 := d.Read(p + 1)
	x2 := d.Read(p + 2)
	return interp.Lagrange4(t, xm1, x0, x1, x2)
}

func (d *Line) readLanczos(delay float64, size int) float64 {
	const a = 3
	maxDelay := float64(size - (2*a - 1))
	if maxDelay < 0 {
		maxDelay = 0
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	p := int(math.Floor(delay))
	t := delay - float64(p)
	var samples [2 * a]float64
	for i := 0; i < 2*a; i++ {
		idx := p - (a - 1) + i
		if idx < 0 {
			idx = 0
		}
		samples[i] = d.Read(idx)
	}
	return interp.Lanczos6(t, samples[:])
}

func (d *Line) readSinc(delay float64, size int) float64 {
	n := d.sincHalfN
	taps := 2 * n
	maxDelay := float64(size - taps)
	if maxDelay < 0 {
		maxDelay = 0
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	p := int(math.Floor(delay))
	t := delay - float64(p)
	samples := make([]float64, taps)
	for i := 0; i < taps; i++ {
		idx := p - (n - 1) + i
		if idx < 0 {
			idx = 0
		}
		samples[i] = d.Read(idx)
	}
	return interp.SincInterp(t, samples, n)
}

func (d *Line) readAllpass(delay float64, size int) float64 {
	maxDelay := float64(size - 1)
	if delay > maxDelay {
		delay = maxDelay
	}
	p := int(math.Floor(delay))
	t := delay - float64(p)
	x0 := d.Read(p)
	x1 := d.Read(minInt(p+1, size-1))
	return interp.AllpassTick(t, x0, x1, &d.allpassState)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

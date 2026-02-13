package delay

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/interp"
)

// Line is a circular delay line.
type Line struct {
	buffer   []float64
	writePos int
}

// New returns a delay line of fixed size.
func New(size int) (*Line, error) {
	if size <= 0 {
		return nil, fmt.Errorf("delay size must be > 0: %d", size)
	}
	return &Line{buffer: make([]float64, size)}, nil
}

// Len returns internal buffer size.
func (d *Line) Len() int {
	return len(d.buffer)
}

// Write writes one sample.
func (d *Line) Write(sample float64) {
	d.buffer[d.writePos] = sample
	d.writePos++
	if d.writePos >= len(d.buffer) {
		d.writePos = 0
	}
}

// Read reads an integer delay in samples.
func (d *Line) Read(delay int) float64 {
	size := len(d.buffer)
	if size == 0 {
		return 0
	}
	readPos := (d.writePos - delay + size) % size
	return d.buffer[readPos]
}

// ReadFractional reads with cubic Hermite interpolation.
func (d *Line) ReadFractional(delay float64) float64 {
	size := len(d.buffer)
	if size == 0 {
		return 0
	}
	if delay < 0 {
		delay = 0
	}
	maxDelay := float64(size - 3)
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

// Reset clears line state.
func (d *Line) Reset() {
	for i := range d.buffer {
		d.buffer[i] = 0
	}
	d.writePos = 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

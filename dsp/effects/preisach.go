package effects

import (
	"fmt"
	"math"
)

// Preisach is a discrete, uniformly weighted magnetic hysteresis model.
// Construct it with NewPreisach and use one instance per signal. Each
// triangular row stores the number of consecutive positive relays.
// Processing uses linear storage in the resolution and allocates no memory.
type Preisach struct {
	rows     []int
	scale    float32
	lastStep int
	output   float32
}

// NewPreisach creates a model with resolution from 2 through 32768 and all
// relays positive. The triangular grid contains resolution*(resolution-1)/2
// relays.
func NewPreisach(resolution int) (*Preisach, error) {
	if resolution < 2 || resolution > 32768 {
		return nil, fmt.Errorf("Preisach resolution %d is outside [2, 32768]", resolution)
	}
	cells := (resolution - 1) * resolution / 2
	p := &Preisach{
		rows:  make([]int, resolution-1),
		scale: float32(1 / float64(cells)),
	}
	p.Reset()
	return p, nil
}

// Reset sets all relays to their positive state.
func (p *Preisach) Reset() {
	p.setPositive()
	p.lastStep = len(p.rows) + 2
}

func (p *Preisach) setPositive() {
	for i := range p.rows {
		p.rows[i] = len(p.rows) - i
	}
}

// ProcessSample advances the grid using DAV's nearest-even quantization and
// returns its normalized magnetization. Inputs saturate outside [-1, 1].
// NaN is treated as zero.
func (p *Preisach) ProcessSample(input float32) float32 {
	if math.IsNaN(float64(input)) {
		input = 0
	}
	input = min(max(input, -1), 1)
	resolution := len(p.rows) + 1
	step := int(math.RoundToEven(float64(input) * float64(resolution)))
	if step == p.lastStep {
		return p.output
	}
	p.lastStep = step

	x, y := (resolution+step)/2, (resolution-step)/2
	switch {
	case x <= 0:
		clear(p.rows)
		p.output = -1
	case y <= 0:
		p.setPositive()
		p.output = 1
	default:
		split := min(y, len(p.rows))
		// Above the switch row, clearing a suffix preserves each positive
		// prefix. At and below it, every relay is positive.
		positive := 0
		for i, prefix := range p.rows[:split] {
			p.rows[i] = min(prefix, x)
			positive += p.rows[i]
		}
		for i := split; i < len(p.rows); i++ {
			p.rows[i] = len(p.rows) - i
			positive += p.rows[i]
		}
		p.output = float32(2*float64(p.scale)*float64(positive) - 1)
	}
	return p.output
}

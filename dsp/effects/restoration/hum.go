package restoration

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// HumRemover is a narrow notch comb at a mains frequency and its harmonics.
// One caller owns its filter state; processing does not allocate.
type HumRemover struct{ sections []*biquad.Section }

// NewHumRemover builds 1..16 notches at 50 or 60 Hz, with Q in [5,100].
func NewHumRemover(sampleRate, frequency, q float64, harmonics int) (*HumRemover, error) {
	if !finite(sampleRate) || sampleRate < 8000 || sampleRate > 384000 || (frequency != 50 && frequency != 60) || !finite(q) || q < 5 || q > 100 || harmonics < 1 || harmonics > 16 || frequency*float64(harmonics) >= sampleRate/2 {
		return nil, fmt.Errorf("restoration.hum: invalid settings")
	}

	h := &HumRemover{}

	for i := 1; i <= harmonics; i++ {
		w := 2 * math.Pi * frequency * float64(i) / sampleRate
		alpha := math.Sin(w) / (2 * q)
		den := 1 + alpha
		c := biquad.Coefficients{B0: 1 / den, B1: -2 * math.Cos(w) / den, B2: 1 / den, A1: -2 * math.Cos(w) / den, A2: (1 - alpha) / den}
		h.sections = append(h.sections, biquad.NewSection(c))
	}

	return h, nil
}

// ProcessInPlace filters finite samples atomically after validation.
func (h *HumRemover) ProcessInPlace(samples []float64) error {
	if h == nil {
		return fmt.Errorf("restoration.hum: nil processor")
	}

	for _, x := range samples {
		if !finite(x) {
			return fmt.Errorf("restoration.hum: nonfinite input")
		}
	}

	for _, s := range h.sections {
		s.ProcessBlock(samples)
	}

	return nil
}

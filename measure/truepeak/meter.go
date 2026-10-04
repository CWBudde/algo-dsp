// Package truepeak provides bounded four-times-oversampled peak measurement.
package truepeak

import (
	"fmt"
	"math"
)

// FIR phases from ITU-R BS.1770-5 Annex 2, the published 48-coefficient
// interpolator. Floating-point arithmetic needs no integer headroom attenuation.
var phases = [4][12]float64{
	{.001708984375, .010986328125, -.0196533203125, .033203125, -.0594482421875, .1373291015625, .97216796875, -.102294921875, .047607421875, -.026611328125, .014892578125, -.00830078125},
	{-.0291748046875, .029296875, -.0517578125, .089111328125, -.16650390625, .465087890625, .77978515625, -.2003173828125, .1015625, -.0582275390625, .0330810546875, -.0189208984375},
	{-.0189208984375, .0330810546875, -.0582275390625, .1015625, -.2003173828125, .77978515625, .465087890625, -.16650390625, .089111328125, -.0517578125, .029296875, -.0291748046875},
	{-.00830078125, .014892578125, -.026611328125, .047607421875, -.102294921875, .97216796875, .1373291015625, -.0594482421875, .033203125, -.0196533203125, .010986328125, .001708984375},
}

// Meter interpolates each input channel at four phases and retains its maximum
// absolute amplitude. It includes the original samples in the maximum. FIR
// history crosses block boundaries; Flush supplies the final twelve zero frames.
// This is a 4x digital estimate, not an analogue reconstruction guarantee.
type Meter struct {
	history  [][24]float64
	peaks    []float64
	position int
	flushed  bool
}

// NewMeter reserves histories for one to thirty-two packed channels.
func NewMeter(channels int) (*Meter, error) {
	if channels < 1 || channels > 32 {
		return nil, fmt.Errorf("truepeak.new: channels must be in [1,32]")
	}

	return &Meter{history: make([][24]float64, channels), peaks: make([]float64, channels)}, nil
}

// Reset clears peaks, the FIR clock and tail state without allocation.
func (m *Meter) Reset() { clear(m.history); clear(m.peaks); m.position = 0; m.flushed = false }

// ProcessPlanar reads finite equal-length float64 blocks of at most65536 frames.
// Validation is atomic. Magnitudes above1e100 are rejected to bound arithmetic.
func (m *Meter) ProcessPlanar(block [][]float64) error { return process(m, block, nil, false) }

// ProcessPlanar32 reads float32 blocks without a widening copy.
func (m *Meter) ProcessPlanar32(block [][]float32) error { return process(m, block, nil, false) }

// ProcessInterleaved32 reads complete packed float32 frames without allocation.
func (m *Meter) ProcessInterleaved32(block []float32) error { return process(m, nil, block, true) }

func process[T ~float32 | ~float64](m *Meter, planar [][]T, packed []T, interleaved bool) error {
	if m == nil || len(m.peaks) == 0 || m.flushed {
		return fmt.Errorf("truepeak.process: unconfigured or flushed meter")
	}

	channels := len(m.peaks)
	frames := 0

	if interleaved {
		if len(packed)%channels != 0 {
			return fmt.Errorf("truepeak.process: incomplete frame")
		}

		frames = len(packed) / channels
	} else {
		if len(planar) != channels {
			return fmt.Errorf("truepeak.process: channel count")
		}

		frames = len(planar[0])
		for _, channel := range planar {
			if len(channel) != frames {
				return fmt.Errorf("truepeak.process: unequal channel lengths")
			}
		}
	}

	if frames > 65536 {
		return fmt.Errorf("truepeak.process: block exceeds65536 frames")
	}

	for frame := 0; frame < frames; frame++ {
		for ch := 0; ch < channels; ch++ {
			var value T
			if interleaved {
				value = packed[frame*channels+ch]
			} else {
				value = planar[ch][frame]
			}

			x := float64(value)
			if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1e100 {
				return fmt.Errorf("truepeak.process: nonfinite or unrepresentable input")
			}
		}
	}

	for frame := 0; frame < frames; frame++ {
		for ch := 0; ch < channels; ch++ {
			var value T
			if interleaved {
				value = packed[frame*channels+ch]
			} else {
				value = planar[ch][frame]
			}

			m.sample(ch, float64(value))
		}

		m.position++
		if m.position == 12 {
			m.position = 0
		}
	}

	return nil
}

func (m *Meter) sample(channel int, value float64) {
	history := &m.history[channel]
	history[m.position] = value
	history[m.position+12] = value
	m.peaks[channel] = math.Max(m.peaks[channel], math.Abs(value))
	window := history[m.position+1 : m.position+13]

	for _, phase := range phases {
		sum := 0.0
		for i, x := range window {
			sum += x * phase[i]
		}

		m.peaks[channel] = math.Max(m.peaks[channel], math.Abs(sum))
	}
}

// Flush measures the zero-padded FIR tail once. Further input requires Reset.
func (m *Meter) Flush() {
	if m.flushed {
		return
	}

	for range 12 {
		for ch := range m.peaks {
			m.sample(ch, 0)
		}

		m.position++
		if m.position == 12 {
			m.position = 0
		}
	}

	m.flushed = true
}

// PeaksInto copies per-channel absolute linear maxima into caller-owned storage.
// A short destination returns an error without modifying it.
func (m *Meter) PeaksInto(dst []float64) error {
	if len(dst) < len(m.peaks) {
		return fmt.Errorf("truepeak.peaks: short destination")
	}

	copy(dst, m.peaks)

	return nil
}

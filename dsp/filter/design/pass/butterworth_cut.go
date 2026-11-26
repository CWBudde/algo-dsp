package pass

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ButterworthCut keeps the normalization before the sections, as in the
// Delphi ASIO & VST Butterworth cut filters. This retains the same section
// state scaling when cutoff is automated.
type ButterworthCut struct {
	pairs [32]biquad.Section
	order int
	gain  float64
}

// Configure changes the cutoff and order while retaining section state.
// Supported orders are 0 through 64, with order zero passing the input.
// A cutoff at or above Nyquist passes a low-pass and mutes a high-pass, one at
// or below zero mutes a low-pass and passes a high-pass. A NaN cutoff or a
// non-finite or non-positive sample rate bypasses the filter.
func (f *ButterworthCut) Configure(cutoff, sampleRate float64, order int, highpass bool) {
	f.gain = 1

	f.order = min(max(order, 0), 64)
	if f.order == 0 {
		return
	}

	switch {
	case math.IsNaN(cutoff) || !(sampleRate > 0) || math.IsInf(sampleRate, 0):
		f.order = 0

		return
	case cutoff >= sampleRate/2:
		f.order = 0
		if highpass {
			f.gain = 0
		}

		return
	case cutoff <= 0:
		f.order = 0
		if !highpass {
			f.gain = 0
		}

		return
	}

	k := math.Tan(math.Pi * cutoff / sampleRate)

	k2 := k * k
	for i := range f.order / 2 {
		a := 2 * k * math.Sin(math.Pi*float64(2*i+1)/(2*float64(f.order)))
		inv := 1 / (1 + a + k2)

		stage := biquad.Coefficients{B0: 1, B1: 2, B2: 1, A1: 2 * (k2 - 1) * inv, A2: (1 + k2 - a) * inv}
		if highpass {
			stage.B1 = -2
		} else {
			f.gain *= k2
		}

		f.gain *= inv
		f.pairs[i].Coefficients = stage
	}

	if f.order%2 != 0 {
		inv := 1 / (1 + k)

		stage := biquad.Coefficients{B0: 1, B1: 1, A1: (k - 1) * inv}
		if highpass {
			stage.B1 = -1
		} else {
			f.gain *= k
		}

		f.gain *= inv
		f.pairs[f.order/2].Coefficients = stage
	}
}

// ProcessSample applies the x87 path's reverse order for the pole pairs,
// followed by the first-order stage when the order is odd.
func (f *ButterworthCut) ProcessSample(input float32) float32 {
	return float32(f.ProcessSample64(float64(input)))
}

// ProcessSample64 applies the same filter and shares its state with ProcessSample.
func (f *ButterworthCut) ProcessSample64(input float64) float64 {
	x := f.gain * input
	for i := f.order/2 - 1; i >= 0; i-- {
		x = f.pairs[i].ProcessSample(x)
	}

	if f.order%2 != 0 {
		stage := &f.pairs[f.order/2]
		state := stage.State()
		y := x + state[0]
		state[0] = stage.B1*x - stage.A1*y
		stage.SetState(state)

		x = y
	}

	return x
}

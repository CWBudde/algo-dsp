package pass

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

const (
	// ButterworthCutFlushThreshold is the magnitude (about -600 dBFS) below
	// which ButterworthCut returns zero.
	ButterworthCutFlushThreshold = 1e-30
	// ButterworthCutDivergenceLimit is the output magnitude (+80 dBFS) above
	// which ButterworthCut clears its state and returns zero. Static filters
	// never reach it; only state pumped up by cutoff automation does.
	ButterworthCutDivergenceLimit = 1e4
)

// ButterworthCut follows the Delphi ASIO & VST Butterworth cut filters: the
// same coefficients, the reversed pole-pair order and section state retained
// while the cutoff is automated. Unlike the original, each section carries
// its share of the gain, which keeps that retained state at signal level so
// cutoff jumps at high orders cannot grow it without bound, and a change of
// order clears the state.
type ButterworthCut struct {
	pairs [32]biquad.Section
	order int
	gain  float64
}

// Configure changes the cutoff while retaining section state. A change of
// order clears it.
// Supported orders are 0 through 64, with order zero passing the input.
// A cutoff at or above Nyquist passes a low-pass and mutes a high-pass, one at
// or below zero mutes a low-pass and passes a high-pass. A NaN cutoff or a
// non-finite or non-positive sample rate bypasses the filter.
func (f *ButterworthCut) Configure(cutoff, sampleRate float64, order int, highpass bool) {
	f.gain = 1

	order = min(max(order, 0), 64)
	if order != f.order {
		// A new order moves every pole pair into a different section, where
		// the old state no longer matches its coefficients.
		f.Reset()
	}

	f.order = order
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

		g, sign := k2*inv, 1.0
		if highpass {
			g, sign = inv, -1
		}

		f.pairs[i].Coefficients = biquad.Coefficients{B0: g, B1: 2 * sign * g, B2: g, A1: 2 * (k2 - 1) * inv, A2: (1 + k2 - a) * inv}
	}

	if f.order%2 != 0 {
		inv := 1 / (1 + k)

		g, sign := k*inv, 1.0
		if highpass {
			g, sign = inv, -1
		}

		f.pairs[f.order/2].Coefficients = biquad.Coefficients{B0: g, B1: sign * g, A1: (k - 1) * inv}
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
		y := stage.B0*x + state[0]
		state[0] = stage.B1*x - stage.A1*y
		stage.SetState(state)

		x = y
	}

	switch magnitude := math.Abs(x); {
	case magnitude < ButterworthCutFlushThreshold:
		// Deep low-pass tails would otherwise reach a float32 output as
		// subnormals.
		return 0
	case !(magnitude <= ButterworthCutDivergenceLimit):
		// Fast cutoff automation can still pump the retained state of a
		// high-order cascade without bound; restart instead of emitting Inf.
		f.Reset()

		return 0
	}

	return x
}

// Reset clears the section history.
func (f *ButterworthCut) Reset() {
	for i := range f.pairs {
		f.pairs[i].Reset()
	}
}

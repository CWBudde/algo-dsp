package moog_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/moog"
	"github.com/cwbudde/algo-dsp/dsp/spectrum"
)

// ExampleFilter_ProcessSample shows that the ladder lowpass settles a DC input
// to its steady-state gain.
func ExampleFilter_ProcessSample() {
	f, _ := moog.New(1000, 48000)

	var y float64
	for range 5000 {
		y = f.ProcessSample(1.0)
	}

	fmt.Printf("DC steady state: %.3f\n", y)
	// Output:
	// DC steady state: 1.000
}

// ExampleFilter_SetResonanceNormalized demonstrates the musical [0, 1]
// resonance control mapping onto the raw feedback amount.
func ExampleFilter_SetResonanceNormalized() {
	f, _ := moog.New(1000, 48000)

	_ = f.SetResonanceNormalized(0.5)
	fmt.Printf("feedback: %.1f\n", f.Resonance())

	_ = f.SetResonanceNormalized(1.0)
	fmt.Printf("feedback: %.1f\n", f.Resonance())
	// Output:
	// feedback: 2.0
	// feedback: 4.0
}

// ExampleFilter_lowpass compares the attenuation of a low and a high tone.
func ExampleFilter_lowpass() {
	const (
		sr     = 48000.0
		cutoff = 1000.0
		n      = 8192
	)

	rms := func(x []float64) float64 {
		var sum float64
		for _, v := range x {
			sum += v * v
		}

		return math.Sqrt(sum / float64(len(x)))
	}

	tone := func(freq float64) []float64 {
		buf := make([]float64, n)
		for i := range buf {
			buf[i] = 0.05 * math.Sin(2*math.Pi*freq*float64(i)/sr)
		}

		return buf
	}

	low := tone(100)
	high := tone(12000)

	fLow, _ := moog.New(cutoff, sr)
	fHigh, _ := moog.New(cutoff, sr)

	fLow.ProcessInPlace(low)
	fHigh.ProcessInPlace(high)

	fmt.Printf("100 Hz passes, 12 kHz attenuated: %v\n", rms(high[n/4:]) < 0.1*rms(low[n/4:]))
	// Output:
	// 100 Hz passes, 12 kHz attenuated: true
}

// ExampleFilter_oversampling drives a 9 kHz tone hard into saturation. At the
// base rate the 5th harmonic folds back to 3 kHz (aliasing); the high-quality
// oversampled path removes it before decimation.
func ExampleFilter_oversampling() {
	const (
		sr     = 48000.0
		f0     = 9000.0
		cutoff = 10000.0
		n      = 19200
	)

	aliasPower := func(os int) float64 {
		f, _ := moog.New(cutoff, sr, moog.WithOversampling(os), moog.WithResonance(2))

		in := make([]float64, n)
		for i := range in {
			in[i] = 40 * math.Sin(2*math.Pi*f0*float64(i)/sr)
		}

		f.ProcessInPlace(in)

		g, _ := spectrum.NewGoertzel(3000, sr)
		g.ProcessBlock(in[n-9600:])

		return g.Power()
	}

	base := aliasPower(1)
	os4 := aliasPower(4)

	fmt.Printf("4x oversampling cuts 3 kHz alias below 10%%: %v\n", os4 < 0.1*base)
	// Output:
	// 4x oversampling cuts 3 kHz alias below 10%: true
}

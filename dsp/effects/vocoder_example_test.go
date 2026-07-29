package effects_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

// ExampleNewVocoder shows the default configuration: a 1/3-octave analysis and
// synthesis filter bank with a fast envelope follower.
func ExampleNewVocoder() {
	v, err := effects.NewVocoder(48000)
	if err != nil {
		fmt.Println("error")
		return
	}

	fmt.Printf("bands=%d attack=%.1fms release=%.1fms\n", v.NumBands(), v.Attack(), v.Release())
	// Output:
	// bands=32 attack=0.5ms release=2.0ms
}

// ExampleVocoder_ProcessBlock imposes the spectral envelope of a modulator
// (here a gated 300 Hz tone) onto a harmonically rich carrier. The vocoded
// output follows the modulator's amplitude, so the gated half is quieter.
func ExampleVocoder_ProcessBlock() {
	const (
		sampleRate = 48000.0
		n          = 4096
	)

	v, err := effects.NewVocoder(sampleRate, effects.WithVocoderRelease(10))
	if err != nil {
		fmt.Println("error")
		return
	}

	modulator := make([]float64, n)
	carrier := make([]float64, n)
	output := make([]float64, n)

	for i := range n {
		t := float64(i) / sampleRate

		// Modulator: a 300 Hz tone that is silenced for the second half.
		if i < n/2 {
			modulator[i] = math.Sin(2 * math.Pi * 300 * t)
		}

		// Carrier: a few harmonics of 110 Hz to give the bank something to shape.
		for h := 1; h <= 8; h++ {
			carrier[i] += math.Sin(2*math.Pi*110*float64(h)*t) / float64(h)
		}
	}

	err = v.ProcessBlock(modulator, carrier, output)
	if err != nil {
		fmt.Println("error")
		return
	}

	loud := rms(output[n/4 : n/2])
	quiet := rms(output[3*n/4:])

	fmt.Printf("len=%d gated quieter=%t\n", len(output), quiet < loud)
	// Output:
	// len=4096 gated quieter=true
}

// ExampleWithBandLayout selects the 24-band Bark layout and overrides the
// synthesis Q. Without the override, Bark synthesis uses the same per-band Q
// values as the analysis bank.
func ExampleWithBandLayout() {
	v, err := effects.NewVocoder(
		48000,
		effects.WithBandLayout(effects.BandLayoutBark),
		effects.WithVocoderSynthesisQ(6),
	)
	if err != nil {
		fmt.Println("error")
		return
	}

	fmt.Printf("bands=%d bark=%t synthQ=%.1f\n",
		v.NumBands(), v.Layout() == effects.BandLayoutBark, v.SynthesisQ())
	// Output:
	// bands=24 bark=true synthQ=6.0
}

// ExampleWithDownsampling enables multirate analysis. Low-frequency bands run
// their analysis filter and envelope follower at a reduced rate, while
// synthesis always stays at the full sample rate.
func ExampleWithDownsampling() {
	v, err := effects.NewVocoder(48000, effects.WithDownsampling(true))
	if err != nil {
		fmt.Println("error")
		return
	}

	factors := v.DownsampleFactors()

	fmt.Printf("enabled=%t lowest band=%dx highest band=%dx\n",
		v.Downsampling(), factors[0], factors[len(factors)-1])
	// Output:
	// enabled=true lowest band=128x highest band=1x
}

func rms(x []float64) float64 {
	sum := 0.0
	for _, s := range x {
		sum += s * s
	}

	return math.Sqrt(sum / float64(len(x)))
}

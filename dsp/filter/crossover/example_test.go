package crossover_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/crossover"
)

func ExampleNew() {
	xo, _ := crossover.New(1000, 4, 48000) // LR4 at 1 kHz

	fmt.Printf("order=%d freq=%.0f Hz\n", xo.Order(), xo.Freq())
	fmt.Printf("LP sections=%d HP sections=%d\n", xo.LP().NumSections(), xo.HP().NumSections())
	fmt.Printf("LP at 100 Hz:  %.2f dB\n", xo.LP().MagnitudeDB(100, 48000))
	fmt.Printf("LP at 1000 Hz: %.2f dB\n", xo.LP().MagnitudeDB(1000, 48000))
	fmt.Printf("HP at 1000 Hz: %.2f dB\n", xo.HP().MagnitudeDB(1000, 48000))
	fmt.Printf("HP at 10 kHz:  %.2f dB\n", xo.HP().MagnitudeDB(10000, 48000))
	// Output:
	// order=4 freq=1000 Hz
	// LP sections=2 HP sections=2
	// LP at 100 Hz:  -0.00 dB
	// LP at 1000 Hz: -6.02 dB
	// HP at 1000 Hz: -6.02 dB
	// HP at 10 kHz:  -0.00 dB
}

func ExampleCrossover_ProcessSample() {
	xo, _ := crossover.New(1000, 4, 48000)

	// Feed an impulse and several zeros, accumulate energy of sum.
	// For an allpass crossover, the total energy of the summed impulse
	// response equals the input energy (1.0).
	energy := 0.0

	for i := range 4096 {
		x := 0.0
		if i == 0 {
			x = 1.0
		}

		lo, hi := xo.ProcessSample(x)
		s := lo + hi
		energy += s * s
	}

	fmt.Printf("allpass impulse energy=%.4f\n", energy)
	// Output:
	// allpass impulse energy=1.0000
}

func ExampleNewMultiBand() {
	mb, _ := crossover.NewMultiBand([]float64{500, 5000}, 4, 48000)

	fmt.Printf("bands=%d stages=%d\n", mb.NumBands(), len(mb.Stages()))
	fmt.Printf("crossover 1: %.0f Hz LR%d\n", mb.Stages()[0].Freq(), mb.Stages()[0].Order())
	fmt.Printf("crossover 2: %.0f Hz LR%d\n", mb.Stages()[1].Freq(), mb.Stages()[1].Order())
	// Output:
	// bands=3 stages=2
	// crossover 1: 500 Hz LR4
	// crossover 2: 5000 Hz LR4
}

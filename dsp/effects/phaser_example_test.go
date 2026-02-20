package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExamplePhaser_ProcessInPlace() {
	phaser, err := effects.NewPhaser(48000,
		effects.WithPhaserRateHz(0.35),
		effects.WithPhaserFrequencyRangeHz(280, 1400),
		effects.WithPhaserStages(6),
		effects.WithPhaserFeedback(0.25),
		effects.WithPhaserMix(0.5),
	)
	if err != nil {
		fmt.Println("error")
		return
	}

	buf := []float64{1, 0, 0, 0}
	if err := phaser.ProcessInPlace(buf); err != nil {
		fmt.Println("error")
		return
	}

	fmt.Printf("len=%d\n", len(buf))
	// Output:
	// len=4
}

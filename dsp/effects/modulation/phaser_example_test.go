package modulation_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
)

func ExamplePhaser_ProcessInPlace() {
	phaser, err := modulation.NewPhaser(48000,
		modulation.WithPhaserRateHz(0.35),
		modulation.WithPhaserFrequencyRangeHz(280, 1400),
		modulation.WithPhaserStages(6),
		modulation.WithPhaserFeedback(0.25),
		modulation.WithPhaserMix(0.5),
	)
	if err != nil {
		fmt.Println("error")
		return
	}

	buf := []float64{1, 0, 0, 0}

	err = phaser.ProcessInPlace(buf)
	if err != nil {
		fmt.Println("error")
		return
	}

	fmt.Printf("len=%d\n", len(buf))
	// Output:
	// len=4
}

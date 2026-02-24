package modulation_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/filter/hilbert"
)

func ExampleFrequencyShifter_ProcessSample() {
	shifter, err := modulation.NewFrequencyShifter(48000,
		modulation.WithFrequencyShiftHz(100),
		modulation.WithFrequencyShifterHilbertPreset(hilbert.PresetBalanced),
	)
	if err != nil {
		fmt.Println("error")
		return
	}

	up, down := shifter.ProcessSample(1)
	fmt.Printf("up=%.6f down=%.6f\n", up, down)
	// Output:
	// up=0.000125 down=0.000125
}

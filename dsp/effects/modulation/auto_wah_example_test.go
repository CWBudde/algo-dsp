package modulation_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
)

func ExampleAutoWah_ProcessInPlace() {
	autoWah, err := modulation.NewAutoWah(48000,
		modulation.WithAutoWahFrequencyRangeHz(350, 2200),
		modulation.WithAutoWahSensitivity(3.0),
		modulation.WithAutoWahMix(0.8),
	)
	if err != nil {
		fmt.Println("error")
		return
	}

	buf := []float64{1, 0, 0, 0}

	err = autoWah.ProcessInPlace(buf)
	if err != nil {
		fmt.Println("error")
		return
	}

	fmt.Printf("len=%d\n", len(buf))
	// Output:
	// len=4
}

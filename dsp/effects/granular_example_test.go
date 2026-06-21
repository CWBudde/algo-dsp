package effects_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleGranular_ProcessInPlace() {
	granular, err := effects.NewGranular(48000)
	if err != nil {
		fmt.Println("error")
		return
	}

	_ = granular.SetGrainSeconds(0.05)
	_ = granular.SetOverlap(0.7)
	_ = granular.SetSpray(0.2)
	_ = granular.SetPitch(1.0)
	_ = granular.SetMix(0.8)

	buf := make([]float64, 512)
	for i := range 128 {
		buf[i] = math.Sin(2 * math.Pi * 330 * float64(i) / 48000)
	}

	granular.ProcessInPlace(buf)
	fmt.Printf("len=%d\n", len(buf))
	// Output:
	// len=512
}

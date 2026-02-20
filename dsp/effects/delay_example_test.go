package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleDelay_ProcessInPlace() {
	delay, err := effects.NewDelay(48000)
	if err != nil {
		fmt.Println("error")
		return
	}
	_ = delay.SetTime(0.2)
	_ = delay.SetFeedback(0.4)
	_ = delay.SetMix(0.3)

	buf := []float64{1, 0, 0, 0}
	delay.ProcessInPlace(buf)

	fmt.Printf("len=%d\n", len(buf))
	// Output:
	// len=4
}

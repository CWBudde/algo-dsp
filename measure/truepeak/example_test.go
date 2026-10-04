package truepeak_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/measure/truepeak"
)

func ExampleNewMeter() {
	meter, _ := truepeak.NewMeter(1)
	if err := meter.ProcessInterleaved32([]float32{0, 1, 0}); err != nil {
		panic(err)
	}

	meter.Flush()

	peaks := make([]float64, 1)
	_ = meter.PeaksInto(peaks)
	fmt.Println(peaks[0])
	// Output: 1
}

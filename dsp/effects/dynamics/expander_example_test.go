package dynamics_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

// ExampleExpander demonstrates basic downward expander usage.
func ExampleExpander() {
	exp, err := dynamics.NewExpander(48000)
	if err != nil {
		panic(err)
	}

	_ = exp.ProcessSample(0.05)

	fmt.Println("Expander processed one sample")
	// Output:
	// Expander processed one sample
}

// ExampleExpander_configuration demonstrates configuring expander parameters.
func ExampleExpander_configuration() {
	exp, _ := dynamics.NewExpander(48000)

	_ = exp.SetThreshold(-30.0)
	_ = exp.SetRatio(4.0)
	_ = exp.SetKnee(6.0)
	_ = exp.SetAttack(2.0)
	_ = exp.SetRelease(120.0)
	_ = exp.SetRange(-70.0)

	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = 0.2 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	exp.ProcessInPlace(buf)

	fmt.Println("Configured expander parameters:")
	fmt.Printf("Threshold: %.1f dB\n", exp.Threshold())
	fmt.Printf("Ratio: %.1f:1\n", exp.Ratio())
	fmt.Printf("Knee: %.1f dB\n", exp.Knee())
	fmt.Printf("Range: %.1f dB\n", exp.Range())
	// Output:
	// Configured expander parameters:
	// Threshold: -30.0 dB
	// Ratio: 4.0:1
	// Knee: 6.0 dB
	// Range: -70.0 dB
}

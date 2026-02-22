package dynamics_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

func ExampleLookaheadLimiter_configuration() {
	l, err := dynamics.NewLookaheadLimiter(48000)
	if err != nil {
		panic(err)
	}

	_ = l.SetThreshold(-1.0)
	_ = l.SetRelease(80)
	_ = l.SetLookahead(3.0)

	buf := []float64{0.0, 0.3, 1.2, 0.8, 0.1, 0.0}
	l.ProcessInPlace(buf)

	fmt.Printf("threshold=%.1f lookahead=%.1fms\n", l.Threshold(), l.Lookahead())
	// Output:
	// threshold=-1.0 lookahead=3.0ms
}

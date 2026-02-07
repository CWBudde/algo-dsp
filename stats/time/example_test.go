package time_test

import (
	"fmt"

	timestats "github.com/cwbudde/algo-dsp/stats/time"
)

func ExampleCalculate() {
	s := timestats.Calculate([]float64{1, -1, 1, -1})
	fmt.Printf("rms=%.1f zc=%d\n", s.RMS, s.ZeroCrossings)

	// Output:
	// rms=1.0 zc=3
}

func ExampleStreamingStats() {
	s := timestats.NewStreamingStats()
	s.Update([]float64{1, -1})
	s.Update([]float64{1, -1})
	m := s.Result()
	fmt.Printf("len=%d dc=%.1f\n", m.Length, m.DC)

	// Output:
	// len=4 dc=0.0
}

package signal_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/signal"
)

func ExamplePlanPeakNormalization() {
	plan, err := signal.PlanPeakNormalization(0.25, -6.020599913279624)
	if err != nil {
		panic(err)
	}

	fmt.Printf("linked gain %.1f; sample peak %.2f\n", plan.Gain, plan.PredictedSamplePeak)
	// Output: linked gain 2.0; sample peak 0.50
}

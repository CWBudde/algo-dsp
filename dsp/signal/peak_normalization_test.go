package signal

import (
	"math"
	"testing"
)

func TestPlanPeakNormalization(t *testing.T) {
	for _, test := range []struct {
		name            string
		peak, target    float64
		gain, predicted float64
	}{
		{"half", 1, -6.020599913279624, 0.5, 0.5},
		{"double", 0.25, -6.020599913279624, 2, 0.5},
		{"identity", 0.5, -6.020599913279624, 1, 0.5},
		{"silence", 0, -1, 1, 0},
		{"silence extreme finite target", 0, math.MaxFloat64, 1, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := PlanPeakNormalization(test.peak, test.target)
			if err != nil || math.Abs(plan.Gain-test.gain) > 1e-12 || math.Abs(plan.PredictedSamplePeak-test.predicted) > 1e-12 {
				t.Fatalf("got %+v error=%v", plan, err)
			}

			if test.peak == 0 && plan.GainDB != 0 {
				t.Fatalf("silence was not identity: %+v", plan)
			}
		})
	}

	for _, test := range []struct{ peak, target float64 }{
		{-1, -1},
		{math.NaN(), -1},
		{math.Inf(1), -1},
		{1, math.NaN()},
		{0, math.Inf(1)},
		{1, math.Inf(-1)},
		{1, 1e6},
		{1, -1e6},
		{math.SmallestNonzeroFloat64, 0},
	} {
		if _, err := PlanPeakNormalization(test.peak, test.target); err == nil {
			t.Fatalf("invalid/unrepresentable plan accepted: peak=%g target=%g", test.peak, test.target)
		}
	}
}

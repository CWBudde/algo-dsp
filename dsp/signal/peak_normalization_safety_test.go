package signal_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/signal"
)

func TestPeakNormalizationExtremeFinitePlansAndAllocation(t *testing.T) {
	for _, tt := range []struct{ peak, target, want float64 }{
		{float64(math.SmallestNonzeroFloat32), 0, 1},
		{float64(math.MaxFloat32), -120, 0.000001},
		{1e-310, -620, 1e-31},
		{math.MaxFloat64, 0, 1},
	} {
		plan, err := signal.PlanPeakNormalization(tt.peak, tt.target)
		if err != nil || math.Abs(plan.PredictedSamplePeak/tt.want-1) > 1e-12 || math.IsInf(plan.GainDB, 0) {
			t.Fatalf("extreme plan peak=%g target=%g: %+v error=%v", tt.peak, tt.target, plan, err)
		}

		wantDB := tt.target - 20*math.Log10(tt.peak)
		if tt.peak < 0x1p-1022 {
			m, e := math.Frexp(tt.peak)
			wantDB = tt.target - 20*(math.Log10(m)+float64(e)*math.Log10(2))
		}

		if math.Abs(plan.GainDB-wantDB) > 1e-9 {
			t.Fatalf("incorrect gain dB: %g != %g", plan.GainDB, wantDB)
		}
	}

	var failure error

	allocations := testing.AllocsPerRun(100, func() { _, failure = signal.PlanPeakNormalization(0.5, -1) })
	if failure != nil || allocations != 0 {
		t.Fatalf("peak planning allocates: %g error=%v", allocations, failure)
	}
}

func BenchmarkPlanPeakNormalization(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		if _, err := signal.PlanPeakNormalization(0.5, -1); err != nil {
			b.Fatal(err)
		}
	}
}

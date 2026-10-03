package loudness

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"testing"
)

// Exhaustive independent interval enumeration checks the streaming solver's
// linear-time cursor and bounded merge machinery against small arbitrary gate
// sets, including ties and multiple valid solutions. The test oracle re-gates
// every candidate directly rather than using production suffix means/cursors.
func TestTargetIndependentIntervalEnumeration(t *testing.T) {
	rng := rand.New(rand.NewPCG(57291, 82317))
	for trial := 0; trial < 300; trial++ {
		count := 1 + rng.IntN(50)

		energies := make([]float64, count)
		for i := range energies {
			// Avoid exact absolute-gate ties caused by decade powers at a
			// decade target: exp(log A-2 log gain) and A/gain² may straddle
			// the same strict boundary by one ulp. Deliberate equal-energy
			// groups remain, without conflating that with output rounding.
			energies[i] = math.Pow(10, float64(rng.IntN(20)-14)) * (1 + rng.Float64()/8)
			if i > 0 && i%4 == 0 {
				energies[i] = energies[i-1]
			}
		}

		target := []float64{-69, -50, -23, 0, 10}[trial%5]

		a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: int64(count+3) * 4800}, target)
		if err != nil {
			t.Fatal(err)
		}

		a.energies = append(a.energies, energies...)
		a.frames, a.peak = int64(count+3)*4800, 0.5

		absolute := math.Pow(10, (-70+0.691)/10)
		for _, energy := range energies {
			if energy > absolute {
				a.absCount++
				a.absMean += (energy - a.absMean) / float64(a.absCount)
			}
		}

		got := finishTargetBasic(t, a)

		ordered := append([]float64(nil), energies...)
		sort.Float64s(ordered)

		wantGain := math.Inf(1)

		for i, energy := range ordered {
			if i > 0 && energy == ordered[i-1] {
				continue
			}

			mean := 0.0
			for _, value := range ordered[i:] {
				mean += value / float64(count-i)
			}

			finalMean, finalCount := 0.0, 0

			for _, value := range ordered[i:] {
				if value > 0.1*mean {
					finalMean += value
					finalCount++
				}
			}

			gain := math.Pow(10, (target-(-0.691+10*math.Log10(finalMean/float64(finalCount))))/20)

			cutoff := absolute / (gain * gain)
			if energy <= cutoff || (i > 0 && ordered[i-1] > cutoff) {
				continue
			}

			if gain < wantGain {
				wantGain = gain
			}
		}

		if math.IsInf(wantGain, 0) || math.Abs(got.Plan.Gain/wantGain-1) > 1e-12 {
			t.Fatalf("trial %d: gain=%g want=%g energies=%v", trial, got.Plan.Gain, wantGain, energies)
		}
	}
}

func TestTargetSubnormalEnergyAndUnrepresentableGain(t *testing.T) {
	for _, tt := range []struct {
		energy, peak float64
		wantError    bool
	}{
		{math.SmallestNonzeroFloat64, math.Sqrt(math.SmallestNonzeroFloat64), false},
		{1e-8, math.MaxFloat64, true},
	} {
		a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 19200}, -23)
		if err != nil {
			t.Fatal(err)
		}

		a.energies = append(a.energies, tt.energy)

		a.frames, a.peak = 19200, tt.peak
		if tt.wantError {
			if _, err := a.FinishStep(math.MaxInt); !errors.Is(err, ErrNumericalOverflow) {
				t.Fatal("overflow plan published", err)
			}

			if _, err := a.Result(); !errors.Is(err, ErrNumericalOverflow) {
				t.Fatal("overflow not terminal", err)
			}
		} else {
			got := finishTargetBasic(t, a)
			if math.Abs(got.PredictedLUFS+23) > 0.01 || !integratedFinite(got.Plan.Gain) || got.HasMeasuredLUFS {
				t.Fatalf("lost subnormal energy: %+v", got)
			}
		}
	}
}

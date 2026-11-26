package pass

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func TestButterworthCutResponse(t *testing.T) {
	for _, rate := range []float64{44100, 48000, 96000} {
		for _, order := range []int{0, 1, 2, 3, 8, 16} {
			for _, highpass := range []bool{false, true} {
				const hz = 1000

				var filter ButterworthCut
				filter.Configure(hz, rate, order, highpass)

				var reference []biquad.Coefficients
				if highpass {
					reference = ButterworthHP(hz, order, rate)
				} else {
					reference = ButterworthLP(hz, order, rate)
				}

				chain := make([]*biquad.Section, len(reference))
				for i, coefficients := range reference {
					chain[i] = biquad.NewSection(coefficients)
				}

				for i := range 250 {
					var input float32
					if i == 0 {
						input = 1
					}

					want := float64(input)
					for _, section := range chain {
						want = section.ProcessSample(want)
					}

					got := filter.ProcessSample(input)
					if math.Abs(float64(got)-want) > 1e-7 {
						t.Fatalf("rate %g order %d highpass %t sample %d: got %g, want %g",
							rate, order, highpass, i, got, want)
					}
				}
			}
		}
	}
}

func TestButterworthCutBoundaries(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)

	for _, tc := range []struct {
		cutoff, rate float64
		lp, hp       float32
	}{
		{nan, 48000, 1, 1},
		{1000, nan, 1, 1},
		{1000, inf, 1, 1},
		{1000, -inf, 1, 1},
		{1000, 0, 1, 1},
		{1000, -48000, 1, 1},
		{24000, 48000, 1, 0},
		{inf, 48000, 1, 0},
		{0, 48000, 0, 1},
		{-1000, 48000, 0, 1},
		{-inf, 48000, 0, 1},
	} {
		for _, highpass := range []bool{false, true} {
			want := tc.lp
			if highpass {
				want = tc.hp
			}

			var filter ButterworthCut
			// Leave state behind that a broken design could amplify.
			filter.Configure(1000, 48000, 5, highpass)

			for range 16 {
				filter.ProcessSample(1)
			}

			filter.Configure(tc.cutoff, tc.rate, 5, highpass)

			for i, x := range []float32{1, -0.5, 0.25} {
				if got := filter.ProcessSample(x); got != want*x {
					t.Errorf("cutoff %g rate %g highpass %t sample %d: got %g, want %g",
						tc.cutoff, tc.rate, highpass, i, got, want*x)
				}
			}
		}
	}
}

func TestButterworthCutOrderChangeClearsState(t *testing.T) {
	var changed, fresh ButterworthCut

	changed.Configure(1000, 48000, 4, false)

	for range 64 {
		changed.ProcessSample64(1)
	}

	changed.Configure(1000, 48000, 6, false)
	fresh.Configure(1000, 48000, 6, false)

	for i := range 64 {
		if got, want := changed.ProcessSample64(.5), fresh.ProcessSample64(.5); got != want {
			t.Fatalf("sample %d after order change = %g, want fresh %g", i, got, want)
		}
	}
}

func TestButterworthCutCutoffChangeKeepsState(t *testing.T) {
	var changed, fresh ButterworthCut

	changed.Configure(1000, 48000, 4, false)

	for range 64 {
		changed.ProcessSample64(1)
	}

	changed.Configure(2000, 48000, 4, false)
	fresh.Configure(2000, 48000, 4, false)

	if changed.ProcessSample64(0) == fresh.ProcessSample64(0) {
		t.Fatal("cutoff change cleared the section state")
	}
}

func TestButterworthCutCutoffAutomationStaysFinite(t *testing.T) {
	// pluginval's Automation test changes the cutoff every 32 samples. The
	// retained state of a high-order cascade can grow without bound then, in
	// the original as well; the divergence limit restarts the filter.
	for _, order := range []int{8, 16, 32, 64} {
		for _, highpass := range []bool{false, true} {
			random := rand.New(rand.NewPCG(uint64(order), 1))

			var filter ButterworthCut

			for block := range 8000 {
				cutoff := 20 * math.Pow(1000, random.Float64())
				if block%4 == 0 {
					// Periodic extremes are the worst case.
					cutoff = 20 + 19980*float64(block/4%2)
				}

				filter.Configure(cutoff, 44100, order, highpass)

				for range 32 {
					y := filter.ProcessSample(float32(2*random.Float64() - 1))
					if math.IsNaN(float64(y)) || math.Abs(float64(y)) > ButterworthCutDivergenceLimit {
						t.Fatalf("order %d highpass %v block %d: output %g", order, highpass, block, y)
					}
				}
			}
		}
	}
}

func TestButterworthCutFlushesTinyOutput(t *testing.T) {
	// An impulse tail decays through the float32 subnormal range unless the
	// filter flushes it.
	var filter ButterworthCut

	filter.Configure(1000, 44100, 4, false)

	input := float32(1)
	for i := range 20000 {
		y := filter.ProcessSample(input)
		input = 0

		if y != 0 && math.Abs(float64(y)) < 0x1p-126 {
			t.Fatalf("sample %d is a float32 subnormal: %g", i, y)
		}
	}
}

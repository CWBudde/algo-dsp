package mixedphase

import (
	"errors"
	"math"
	"math/cmplx"
	"testing"
)

func TestMinimumPhaseMethodString(t *testing.T) {
	cases := []struct {
		method MinimumPhaseMethod
		want   string
	}{
		{MethodCepstrum, "cepstrum"},
		{MethodHilbert, "hilbert"},
		{MinimumPhaseMethod(7), "unknown"},
	}

	for _, tc := range cases {
		if got := tc.method.String(); got != tc.want {
			t.Fatalf("String() = %q, want %q", got, tc.want)
		}
	}
}

// TestMinimumPhaseMethodsAgree pins the practical equivalence of the two
// reconstructions: once the dense spectrum is truncated back to the tap budget,
// the truncation error dominates and both methods yield the same FIR.
func TestMinimumPhaseMethodsAgree(t *testing.T) {
	prototypes := map[string][]float64{
		"wide":   lowpassPrototype(129, 0.12),
		"narrow": lowpassPrototype(257, 0.03),
	}

	for name, prototype := range prototypes {
		t.Run(name, func(t *testing.T) {
			cepstrum, err := MinimumPhaseWith(
				prototype,
				MinimumPhaseConfig{FFTSize: 8192, Method: MethodCepstrum},
			)
			if err != nil {
				t.Fatalf("MinimumPhaseWith(cepstrum) error = %v", err)
			}

			hilbert, err := MinimumPhaseWith(
				prototype,
				MinimumPhaseConfig{FFTSize: 8192, Method: MethodHilbert},
			)
			if err != nil {
				t.Fatalf("MinimumPhaseWith(hilbert) error = %v", err)
			}

			peak := 0.0
			for _, value := range cepstrum {
				peak = max(peak, math.Abs(value))
			}

			for i := range cepstrum {
				if math.Abs(cepstrum[i]-hilbert[i]) > 1e-10*peak {
					t.Fatalf(
						"taps[%d]: cepstrum = %.16g, hilbert = %.16g",
						i,
						cepstrum[i],
						hilbert[i],
					)
				}
			}
		})
	}
}

func TestMinimumPhaseDefaultsToCepstrum(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	want, err := MinimumPhaseWith(
		prototype,
		MinimumPhaseConfig{FFTSize: 4096, Method: MethodCepstrum},
	)
	if err != nil {
		t.Fatalf("MinimumPhaseWith() error = %v", err)
	}

	got, err := MinimumPhase(prototype, 4096)
	if err != nil {
		t.Fatalf("MinimumPhase() error = %v", err)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("taps[%d] = %.16g, want %.16g", i, got[i], want[i])
		}
	}
}

// TestHilbertReproducesTargetMagnitude measures the spectral-factorisation
// error on the dense design grid, before truncation hides it.
//
// The Hilbert route pairs the transformed phase with the floored target
// magnitude, so the magnitude is exact by construction and stays exact when the
// magnitude floor is lowered by twelve decades. The cepstral route recovers the
// magnitude through the complex exponential of the folded log spectrum, so its
// deviation grows with the log-domain dynamic range; the observed values are
// logged for comparison.
func TestHilbertReproducesTargetMagnitude(t *testing.T) {
	prototype := lowpassPrototype(257, 0.03)
	size := 8192

	w, err := newFFTWorkspace(size)
	if err != nil {
		t.Fatalf("newFFTWorkspace() error = %v", err)
	}

	targetSpectrum, err := w.forwardReal(prototype)
	if err != nil {
		t.Fatalf("forwardReal() error = %v", err)
	}

	targetMagnitude := magnitude(targetSpectrum)

	for _, requested := range []float64{1e-6, 1e-12, 1e-18} {
		epsilon := defaultEpsilon(targetMagnitude, requested)
		floored := flooredMagnitude(size, targetMagnitude, epsilon)

		deviations := make(map[MinimumPhaseMethod]float64)

		for _, method := range []MinimumPhaseMethod{
			MethodCepstrum,
			MethodHilbert,
		} {
			reconstructed, designErr := minimumPhaseSpectrum(
				w,
				targetMagnitude,
				epsilon,
				method,
			)
			if designErr != nil {
				t.Fatalf("minimumPhaseSpectrum(%v) error = %v", method, designErr)
			}

			worst := 0.0

			for i := range floored {
				deviation := math.Abs(cmplx.Abs(reconstructed[i])-floored[i]) /
					floored[i]
				worst = max(worst, deviation)
			}

			deviations[method] = worst
		}

		t.Logf(
			"epsilon=%g: cepstrum max relative magnitude error = %.3e, "+
				"hilbert = %.3e",
			requested,
			deviations[MethodCepstrum],
			deviations[MethodHilbert],
		)

		if deviations[MethodHilbert] > 1e-12 {
			t.Fatalf(
				"epsilon=%g: hilbert max relative magnitude error = %.3e, "+
					"want <= 1e-12",
				requested,
				deviations[MethodHilbert],
			)
		}
	}
}

// TestHilbertMinimumPhaseMovesEnergyForward mirrors the cepstral endpoint test:
// the Hilbert reconstruction must front-load the energy while preserving the
// prototype's magnitude response.
func TestHilbertMinimumPhaseMovesEnergyForward(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)
	size := 4096

	taps, err := MinimumPhaseWith(
		prototype,
		MinimumPhaseConfig{FFTSize: size, Method: MethodHilbert},
	)
	if err != nil {
		t.Fatalf("MinimumPhaseWith() error = %v", err)
	}

	linearMetrics, err := Analyze(prototype, prototype, size)
	if err != nil {
		t.Fatalf("Analyze(linear) error = %v", err)
	}

	metrics, err := Analyze(prototype, taps, size)
	if err != nil {
		t.Fatalf("Analyze(hilbert) error = %v", err)
	}

	t.Logf("hilbert minimum-phase metrics: %+v", metrics)

	if metrics.EnergyCentroid >= linearMetrics.EnergyCentroid/2 {
		t.Fatalf(
			"hilbert centroid = %f, linear centroid = %f",
			metrics.EnergyCentroid,
			linearMetrics.EnergyCentroid,
		)
	}

	if metrics.RelativeMagnitudeError > 2e-4 {
		t.Fatalf(
			"relative magnitude error = %g, want <= 2e-4",
			metrics.RelativeMagnitudeError,
		)
	}
}

func TestMinimumPhaseWithValidation(t *testing.T) {
	_, err := MinimumPhaseWith(nil, MinimumPhaseConfig{})
	if !errors.Is(err, ErrEmptyPrototype) {
		t.Fatalf("empty prototype error = %v", err)
	}

	_, err = MinimumPhaseWith([]float64{1}, MinimumPhaseConfig{Epsilon: -1})
	if !errors.Is(err, ErrInvalidEpsilon) {
		t.Fatalf("negative epsilon error = %v", err)
	}

	_, err = MinimumPhaseWith(
		[]float64{1},
		MinimumPhaseConfig{Method: MinimumPhaseMethod(7)},
	)
	if !errors.Is(err, ErrInvalidMethod) {
		t.Fatalf("unknown method error = %v", err)
	}
}

// TestIterativeMethodsReachComparableQuality documents that the practical
// equivalence of the two reconstructions does not carry over to the alternating
// factorisation.
//
// Each pass truncates a factor and divides by the other one, and the
// regularised division is ill-conditioned wherever the residual has a deep
// null. The two reconstructions floor those nulls slightly differently, so the
// iteration paths separate and the final errors differ by a few dB in either
// direction without one method being systematically better. The test therefore
// only requires both to converge to a usable design.
func TestIterativeMethodsReachComparableQuality(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)

	for _, delay := range []int{4, 16, 32} {
		for _, method := range []MinimumPhaseMethod{
			MethodCepstrum,
			MethodHilbert,
		} {
			result, err := DesignIterative(prototype, IterativeConfig{
				Length:     129,
				Delay:      delay,
				Iterations: 8,
				FFTSize:    4096,
				Method:     method,
			})
			if err != nil {
				t.Fatalf("DesignIterative(%v) error = %v", method, err)
			}

			t.Logf(
				"delay=%d %v: rms=%.4f dB max=%.4f dB relative=%.4e",
				delay,
				method,
				result.Metrics.RMSMagnitudeErrorDB,
				result.Metrics.MaxMagnitudeErrorDB,
				result.Metrics.RelativeMagnitudeError,
			)

			if result.Metrics.RelativeMagnitudeError > 5e-2 {
				t.Fatalf(
					"delay=%d %v: relative magnitude error = %g, want <= 5e-2",
					delay,
					method,
					result.Metrics.RelativeMagnitudeError,
				)
			}

			if result.Metrics.PeakIndex >= 64 {
				t.Fatalf(
					"delay=%d %v: PeakIndex = %d, want < 64",
					delay,
					method,
					result.Metrics.PeakIndex,
				)
			}
		}
	}
}

// TestDesignsAcceptHilbertMethod covers the method plumbing of both design
// entry points, including rejection of unknown methods.
func TestDesignsAcceptHilbertMethod(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)

	interpolated, err := DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{
			Length:  129,
			Mix:     0.5,
			FFTSize: 4096,
			Method:  MethodHilbert,
		},
	)
	if err != nil {
		t.Fatalf("DesignPhaseInterpolation(hilbert) error = %v", err)
	}

	if interpolated.Metrics.RelativeMagnitudeError > 1e-2 {
		t.Fatalf(
			"relative magnitude error = %g, want <= 1e-2",
			interpolated.Metrics.RelativeMagnitudeError,
		)
	}

	_, err = DesignIterative(
		prototype,
		IterativeConfig{Method: MinimumPhaseMethod(7)},
	)
	if !errors.Is(err, ErrInvalidMethod) {
		t.Fatalf("iterative unknown method error = %v", err)
	}

	_, err = DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{Method: MinimumPhaseMethod(7)},
	)
	if !errors.Is(err, ErrInvalidMethod) {
		t.Fatalf("interpolation unknown method error = %v", err)
	}
}

func BenchmarkMinimumPhase(b *testing.B) {
	prototype := lowpassPrototype(513, 0.1)

	for _, method := range []MinimumPhaseMethod{
		MethodCepstrum,
		MethodHilbert,
	} {
		b.Run(method.String(), func(b *testing.B) {
			cfg := MinimumPhaseConfig{FFTSize: 8192, Method: method}

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				if _, err := MinimumPhaseWith(prototype, cfg); err != nil {
					b.Fatalf("MinimumPhaseWith() error = %v", err)
				}
			}
		})
	}
}

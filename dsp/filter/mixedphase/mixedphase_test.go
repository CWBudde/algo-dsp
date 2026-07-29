package mixedphase

import (
	"errors"
	"math"
	"testing"
)

func TestMinimumPhaseMovesEnergyForward(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	taps, err := MinimumPhase(prototype, 4096)
	if err != nil {
		t.Fatalf("MinimumPhase() error = %v", err)
	}

	linearMetrics, err := Analyze(prototype, prototype, 4096)
	if err != nil {
		t.Fatalf("Analyze(linear) error = %v", err)
	}

	minimumMetrics, err := Analyze(prototype, taps, 4096)
	if err != nil {
		t.Fatalf("Analyze(minimum) error = %v", err)
	}

	t.Logf("minimum-phase metrics: %+v", minimumMetrics)

	if minimumMetrics.EnergyCentroid >= linearMetrics.EnergyCentroid/2 {
		t.Fatalf(
			"minimum-phase centroid = %f, linear centroid = %f",
			minimumMetrics.EnergyCentroid,
			linearMetrics.EnergyCentroid,
		)
	}

	if minimumMetrics.RelativeMagnitudeError > 2e-4 {
		t.Fatalf(
			"minimum-phase relative magnitude error = %g, want <= 2e-4",
			minimumMetrics.RelativeMagnitudeError,
		)
	}
}

func TestDesignIterativeHonoursTapBudget(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	result, err := DesignIterative(prototype, IterativeConfig{
		Length:     129,
		Delay:      16,
		Iterations: 8,
		FFTSize:    4096,
	})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	t.Logf("iterative metrics: %+v", result.Metrics)

	if len(result.Taps) != 129 {
		t.Fatalf("len(Taps) = %d, want 129", len(result.Taps))
	}

	if len(result.LinearPhasePart) != 33 {
		t.Fatalf(
			"len(LinearPhasePart) = %d, want 33",
			len(result.LinearPhasePart),
		)
	}

	if len(result.MinimumPhasePart) != 97 {
		t.Fatalf(
			"len(MinimumPhasePart) = %d, want 97",
			len(result.MinimumPhasePart),
		)
	}

	if result.Metrics.RelativeMagnitudeError > 0.01 {
		t.Fatalf(
			"relative magnitude error = %g, want <= 0.01",
			result.Metrics.RelativeMagnitudeError,
		)
	}

	if result.Metrics.PeakIndex >= 32 {
		t.Fatalf("PeakIndex = %d, want < 32", result.Metrics.PeakIndex)
	}
}

func TestIterativeZeroDelayIsMinimumPhaseEndpoint(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	want, err := MinimumPhase(prototype, 4096)
	if err != nil {
		t.Fatalf("MinimumPhase() error = %v", err)
	}

	result, err := DesignIterative(prototype, IterativeConfig{
		Length:  129,
		Delay:   0,
		FFTSize: 4096,
	})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	if len(result.LinearPhasePart) != 1 ||
		result.LinearPhasePart[0] != 1 {
		t.Fatalf(
			"LinearPhasePart = %v, want identity factor",
			result.LinearPhasePart,
		)
	}

	for i := range want {
		if math.Abs(result.Taps[i]-want[i]) > 1e-12 {
			t.Fatalf(
				"Taps[%d] = %.16g, want %.16g",
				i,
				result.Taps[i],
				want[i],
			)
		}
	}
}

func TestIterativeImprovesUncorrectedFactorisation(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)

	initial, err := DesignIterative(prototype, IterativeConfig{
		Length:      129,
		Delay:       12,
		Iterations:  -1,
		FFTSize:     4096,
		ToleranceDB: -1,
	})
	if err != nil {
		t.Fatalf("initial DesignIterative() error = %v", err)
	}

	corrected, err := DesignIterative(prototype, IterativeConfig{
		Length:      129,
		Delay:       12,
		Iterations:  8,
		FFTSize:     4096,
		ToleranceDB: -1,
	})
	if err != nil {
		t.Fatalf("corrected DesignIterative() error = %v", err)
	}

	t.Logf(
		"uncorrected/corrected RMS magnitude error: %f/%f dB",
		initial.Metrics.RMSMagnitudeErrorDB,
		corrected.Metrics.RMSMagnitudeErrorDB,
	)

	if corrected.Metrics.RMSMagnitudeErrorDB >=
		initial.Metrics.RMSMagnitudeErrorDB {
		t.Fatalf(
			"corrected error = %f dB, initial error = %f dB",
			corrected.Metrics.RMSMagnitudeErrorDB,
			initial.Metrics.RMSMagnitudeErrorDB,
		)
	}
}

func TestIterativeBeatsDirectPhaseProjectionAtSameDelay(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)
	delay := 16
	maxDelay := (len(prototype) - 1) / 2

	iterative, err := DesignIterative(prototype, IterativeConfig{
		Length:     len(prototype),
		Delay:      delay,
		Iterations: 8,
		FFTSize:    4096,
	})
	if err != nil {
		t.Fatalf("DesignIterative() error = %v", err)
	}

	direct, err := DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{
			Length:  len(prototype),
			Mix:     float64(delay) / float64(maxDelay),
			FFTSize: 4096,
		},
	)
	if err != nil {
		t.Fatalf("DesignPhaseInterpolation() error = %v", err)
	}

	if iterative.Metrics.RelativeMagnitudeError >=
		direct.Metrics.RelativeMagnitudeError {
		t.Fatalf(
			"iterative relative error = %g, direct relative error = %g",
			iterative.Metrics.RelativeMagnitudeError,
			direct.Metrics.RelativeMagnitudeError,
		)
	}
}

func TestPhaseInterpolationMovesPeakContinuously(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	minimum, err := DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{Length: 129, Mix: 0, FFTSize: 4096},
	)
	if err != nil {
		t.Fatalf("minimum DesignPhaseInterpolation() error = %v", err)
	}

	mixed, err := DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{Length: 129, Mix: 0.5, FFTSize: 4096},
	)
	if err != nil {
		t.Fatalf("mixed DesignPhaseInterpolation() error = %v", err)
	}

	linear, err := DesignPhaseInterpolation(
		prototype,
		PhaseInterpolationConfig{Length: 129, Mix: 1, FFTSize: 4096},
	)
	if err != nil {
		t.Fatalf("linear DesignPhaseInterpolation() error = %v", err)
	}

	t.Logf(
		"interpolation metrics minimum/mixed/linear: %+v / %+v / %+v",
		minimum.Metrics,
		mixed.Metrics,
		linear.Metrics,
	)

	if minimum.Metrics.PeakIndex >= mixed.Metrics.PeakIndex ||
		mixed.Metrics.PeakIndex >= linear.Metrics.PeakIndex {
		t.Fatalf(
			"peak indices minimum/mixed/linear = %d/%d/%d",
			minimum.Metrics.PeakIndex,
			mixed.Metrics.PeakIndex,
			linear.Metrics.PeakIndex,
		)
	}
}

func TestDesignValidation(t *testing.T) {
	_, err := DesignIterative(nil, IterativeConfig{})
	if !errors.Is(err, ErrEmptyPrototype) {
		t.Fatalf("empty prototype error = %v", err)
	}

	_, err = DesignIterative(
		[]float64{1},
		IterativeConfig{Length: 5, Delay: 3},
	)
	if !errors.Is(err, ErrInvalidDelay) {
		t.Fatalf("invalid delay error = %v", err)
	}

	_, err = DesignPhaseInterpolation(
		[]float64{1},
		PhaseInterpolationConfig{Mix: 1.1},
	)
	if !errors.Is(err, ErrInvalidPhaseMix) {
		t.Fatalf("invalid phase mix error = %v", err)
	}
}

func lowpassPrototype(length int, cutoff float64) []float64 {
	taps := make([]float64, length)
	middle := float64(length-1) / 2
	sum := 0.0

	for i := range taps {
		x := float64(i) - middle

		sinc := 2 * cutoff
		if x != 0 {
			sinc = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
		}

		windowValue := 0.5 -
			0.5*math.Cos(2*math.Pi*float64(i)/float64(length-1))
		taps[i] = sinc * windowValue
		sum += taps[i]
	}

	for i := range taps {
		taps[i] /= sum
	}

	return taps
}

package mixedphase

import (
	"errors"
	"math"
	"testing"
)

// weightedBandLimit is the normalised frequency below which the emphasis tests
// raise the design weight.
const weightedBandLimit = 0.3

// TestUniformWeightMatchesPhaseInterpolation pins the analytic relationship
// between the two direct designs.
//
// On a uniform DFT grid with uniform weights the autocorrelation matrix of the
// normal equations is the identity, so the least-squares solution is the
// truncated inverse transform of the prescribed response — which is exactly
// what [DesignPhaseInterpolation] computes. The two must therefore agree bit
// for bit, and any deviation means the normal equations are being assembled
// wrongly.
func TestUniformWeightMatchesPhaseInterpolation(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	for _, mix := range []float64{0, 0.3, 1} {
		leastSquares, err := DesignComplexLeastSquares(
			prototype,
			ComplexLeastSquaresConfig{Length: 129, Mix: mix, FFTSize: 4096},
		)
		if err != nil {
			t.Fatalf("mix=%g: DesignComplexLeastSquares() error = %v", mix, err)
		}

		interpolated, err := DesignPhaseInterpolation(
			prototype,
			PhaseInterpolationConfig{Length: 129, Mix: mix, FFTSize: 4096},
		)
		if err != nil {
			t.Fatalf("mix=%g: DesignPhaseInterpolation() error = %v", mix, err)
		}

		for i := range interpolated.Taps {
			if leastSquares.Taps[i] != interpolated.Taps[i] {
				t.Fatalf(
					"mix=%g taps[%d]: least squares = %.17g, interpolation = %.17g",
					mix,
					i,
					leastSquares.Taps[i],
					interpolated.Taps[i],
				)
			}
		}
	}
}

// TestMinimaxTradesRMSForPeak measures the effect of the Lawson reweighting:
// the peak complex error must fall and the RMS error must rise, driving the
// peak-to-RMS ratio towards the equiripple value of one.
func TestMinimaxTradesRMSForPeak(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	baseline, err := DesignComplexLeastSquares(
		prototype,
		ComplexLeastSquaresConfig{Length: 65, Mix: 0.25, FFTSize: 4096},
	)
	if err != nil {
		t.Fatalf("DesignComplexLeastSquares(wls) error = %v", err)
	}

	if baseline.Iterations != 0 {
		t.Fatalf("Iterations = %d, want 0 without reweighting", baseline.Iterations)
	}

	refined, err := DesignComplexLeastSquares(
		prototype,
		ComplexLeastSquaresConfig{
			Length:            65,
			Mix:               0.25,
			FFTSize:           4096,
			MinimaxIterations: 20,
			MinimaxTolerance:  -1,
		},
	)
	if err != nil {
		t.Fatalf("DesignComplexLeastSquares(irls) error = %v", err)
	}

	t.Logf(
		"wls: rms=%.5e peak=%.5e ratio=%.3f",
		baseline.ComplexError.RMS,
		baseline.ComplexError.Peak,
		baseline.ComplexError.Peak/baseline.ComplexError.RMS,
	)
	t.Logf(
		"irls: passes=%d rms=%.5e peak=%.5e ratio=%.3f",
		refined.Iterations,
		refined.ComplexError.RMS,
		refined.ComplexError.Peak,
		refined.ComplexError.Peak/refined.ComplexError.RMS,
	)

	if refined.ComplexError.Peak >= baseline.ComplexError.Peak {
		t.Fatalf(
			"minimax peak error = %g, want below the least-squares peak %g",
			refined.ComplexError.Peak,
			baseline.ComplexError.Peak,
		)
	}

	if refined.ComplexError.RMS <= baseline.ComplexError.RMS {
		t.Fatalf(
			"minimax RMS error = %g, want above the least-squares RMS %g",
			refined.ComplexError.RMS,
			baseline.ComplexError.RMS,
		)
	}

	baselineRatio := baseline.ComplexError.Peak / baseline.ComplexError.RMS
	refinedRatio := refined.ComplexError.Peak / refined.ComplexError.RMS

	if refinedRatio >= baselineRatio {
		t.Fatalf(
			"peak-to-RMS ratio = %g, want below the least-squares ratio %g",
			refinedRatio,
			baselineRatio,
		)
	}
}

// TestMinimaxKeepsBestIterate covers the non-monotone nature of Lawson
// reweighting: running further must never return a worse design than a shorter
// run, because the best iterate is retained.
func TestMinimaxKeepsBestIterate(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	worst := 0.0
	previous := math.Inf(1)

	for _, iterations := range []int{2, 5, 10, 20, 40} {
		result, err := DesignComplexLeastSquares(
			prototype,
			ComplexLeastSquaresConfig{
				Length:            65,
				Mix:               1,
				FFTSize:           4096,
				MinimaxIterations: iterations,
				MinimaxTolerance:  -1,
			},
		)
		if err != nil {
			t.Fatalf("iterations=%d error = %v", iterations, err)
		}

		t.Logf("iterations=%d: peak=%.6e", iterations, result.ComplexError.Peak)

		if result.ComplexError.Peak > previous {
			t.Fatalf(
				"iterations=%d: peak error = %g rose above %g",
				iterations,
				result.ComplexError.Peak,
				previous,
			)
		}

		previous = result.ComplexError.Peak
		worst = max(worst, result.ComplexError.Peak)
	}

	if worst == 0 {
		t.Fatal("peak error is identically zero, the measurement is not meaningful")
	}
}

// TestMinimaxToleranceStopsEarly checks that a loose tolerance shortens the
// reweighting without materially changing the result.
func TestMinimaxToleranceStopsEarly(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)

	loose, err := DesignComplexLeastSquares(
		prototype,
		ComplexLeastSquaresConfig{
			Length:            65,
			Mix:               0.3,
			FFTSize:           2048,
			MinimaxIterations: 50,
			MinimaxTolerance:  1e-2,
		},
	)
	if err != nil {
		t.Fatalf("DesignComplexLeastSquares(loose) error = %v", err)
	}

	tight, err := DesignComplexLeastSquares(
		prototype,
		ComplexLeastSquaresConfig{
			Length:            65,
			Mix:               0.3,
			FFTSize:           2048,
			MinimaxIterations: 50,
			MinimaxTolerance:  -1,
		},
	)
	if err != nil {
		t.Fatalf("DesignComplexLeastSquares(tight) error = %v", err)
	}

	t.Logf(
		"loose: passes=%d peak=%.6e; exhaustive: passes=%d peak=%.6e",
		loose.Iterations,
		loose.ComplexError.Peak,
		tight.Iterations,
		tight.ComplexError.Peak,
	)

	if loose.Iterations >= 50 {
		t.Fatalf("loose tolerance performed %d passes, want early stopping", loose.Iterations)
	}

	if loose.ComplexError.Peak > 1.1*tight.ComplexError.Peak {
		t.Fatalf(
			"early stop peak error = %g, more than 10%% above the exhaustive %g",
			loose.ComplexError.Peak,
			tight.ComplexError.Peak,
		)
	}
}

// TestWeightShapesErrorDistribution verifies that the weight vector does what
// it promises: emphasising a band lowers the error there at the expense of the
// rest of the grid.
func TestWeightShapesErrorDistribution(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)
	size := 4096
	half := size / 2

	weight := make([]float64, half+1)
	for i := range weight {
		weight[i] = 1
		if float64(i)/float64(half) < weightedBandLimit {
			weight[i] = 100
		}
	}

	uniform, err := DesignComplexLeastSquares(
		prototype,
		ComplexLeastSquaresConfig{Length: 65, Mix: 0.3, FFTSize: size},
	)
	if err != nil {
		t.Fatalf("DesignComplexLeastSquares(uniform) error = %v", err)
	}

	shaped, err := DesignComplexLeastSquares(
		prototype,
		ComplexLeastSquaresConfig{
			Length:  65,
			Mix:     0.3,
			FFTSize: size,
			Weight:  weight,
		},
	)
	if err != nil {
		t.Fatalf("DesignComplexLeastSquares(shaped) error = %v", err)
	}

	uniformInBand := peakInWeightedBand(
		t, prototype, uniform.Taps, size, weightedBandLimit,
	)
	shapedInBand := peakInWeightedBand(
		t, prototype, shaped.Taps, size, weightedBandLimit,
	)

	t.Logf(
		"peak error in the emphasised band: uniform=%.4e shaped=%.4e",
		uniformInBand,
		shapedInBand,
	)

	if shapedInBand >= uniformInBand {
		t.Fatalf(
			"emphasised-band peak error = %g, want below the uniform %g",
			shapedInBand,
			uniformInBand,
		)
	}

	if shaped.ComplexError.RMS <= uniform.ComplexError.RMS {
		t.Fatalf(
			"shaped overall RMS = %g, want above the uniform %g",
			shaped.ComplexError.RMS,
			uniform.ComplexError.RMS,
		)
	}
}

// peakInWeightedBand reports the largest complex deviation from the prescribed
// response below the given normalised frequency.
func peakInWeightedBand(
	t *testing.T,
	prototype, taps []float64,
	size int,
	limit float64,
) float64 {
	t.Helper()

	w, err := newFFTWorkspace(size)
	if err != nil {
		t.Fatalf("newFFTWorkspace() error = %v", err)
	}

	targetSpectrum, err := w.forwardReal(prototype)
	if err != nil {
		t.Fatalf("forwardReal() error = %v", err)
	}

	targetMagnitude := magnitude(targetSpectrum)

	minimumSpectrum, err := minimumPhaseSpectrum(
		w,
		targetMagnitude,
		defaultEpsilon(targetMagnitude, 0),
		MethodCepstrum,
	)
	if err != nil {
		t.Fatalf("minimumPhaseSpectrum() error = %v", err)
	}

	desired := prescribedResponse(
		w,
		targetMagnitude,
		minimumSpectrum,
		0.3,
		float64(len(taps)-1)/2,
	)

	deviation, err := complexDeviation(w, taps, desired)
	if err != nil {
		t.Fatalf("complexDeviation() error = %v", err)
	}

	half := size / 2
	peak := 0.0

	for i, value := range deviation {
		if float64(i)/float64(half) < limit {
			peak = max(peak, value)
		}
	}

	return peak
}

// TestUnweightedBandsAreUnconstrained documents the consequence of a weight
// that is exactly zero over part of the grid: the objective says nothing about
// those bins, so the response there is free to diverge even though the
// weighted band is matched closely.
func TestUnweightedBandsAreUnconstrained(t *testing.T) {
	prototype := lowpassPrototype(129, 0.12)
	size := 2048
	half := size / 2

	weight := make([]float64, half+1)
	for i := range weight {
		if float64(i)/float64(half) < 0.05 {
			weight[i] = 1
		}
	}

	result, err := DesignComplexLeastSquares(
		prototype,
		ComplexLeastSquaresConfig{
			Length:  65,
			Mix:     0.3,
			FFTSize: size,
			Weight:  weight,
		},
	)
	if err != nil {
		t.Fatalf("DesignComplexLeastSquares() error = %v", err)
	}

	inBand := peakInWeightedBand(t, prototype, result.Taps, size, 0.05)

	t.Logf(
		"narrow weight: in-band peak=%.4e, overall peak=%.4e",
		inBand,
		result.ComplexError.Peak,
	)

	if inBand > 1e-2 {
		t.Fatalf("weighted-band peak error = %g, want <= 1e-2", inBand)
	}
}

func TestComplexLeastSquaresValidation(t *testing.T) {
	prototype := lowpassPrototype(33, 0.2)

	cases := []struct {
		name string
		cfg  ComplexLeastSquaresConfig
		want error
	}{
		{"mix below range", ComplexLeastSquaresConfig{Mix: -0.1}, ErrInvalidPhaseMix},
		{"mix above range", ComplexLeastSquaresConfig{Mix: 1.1}, ErrInvalidPhaseMix},
		{"negative epsilon", ComplexLeastSquaresConfig{Epsilon: -1}, ErrInvalidEpsilon},
		{
			"unknown method",
			ComplexLeastSquaresConfig{Method: MinimumPhaseMethod(7)},
			ErrInvalidMethod,
		},
		{
			"negative iterations",
			ComplexLeastSquaresConfig{MinimaxIterations: -1},
			ErrInvalidLength,
		},
		{
			"short weight",
			ComplexLeastSquaresConfig{FFTSize: 256, Weight: make([]float64, 4)},
			ErrInvalidWeight,
		},
		{
			"negative weight",
			ComplexLeastSquaresConfig{FFTSize: 256, Weight: negativeWeight(129)},
			ErrInvalidWeight,
		},
		{
			"zero weight",
			ComplexLeastSquaresConfig{FFTSize: 256, Weight: make([]float64, 129)},
			ErrInvalidWeight,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DesignComplexLeastSquares(prototype, tc.cfg)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}

	if _, err := DesignComplexLeastSquares(
		nil,
		ComplexLeastSquaresConfig{},
	); !errors.Is(err, ErrEmptyPrototype) {
		t.Fatalf("empty prototype error = %v", err)
	}

	if _, err := DesignComplexLeastSquares(
		prototype,
		ComplexLeastSquaresConfig{Length: -1},
	); !errors.Is(err, ErrInvalidLength) {
		t.Fatalf("negative length error = %v", err)
	}
}

func negativeWeight(length int) []float64 {
	out := make([]float64, length)
	for i := range out {
		out[i] = 1
	}

	out[length/2] = -1

	return out
}

func TestComplexLeastSquaresAcceptsHilbertMethod(t *testing.T) {
	prototype := lowpassPrototype(129, 0.08)

	for _, method := range []MinimumPhaseMethod{MethodCepstrum, MethodHilbert} {
		result, err := DesignComplexLeastSquares(
			prototype,
			ComplexLeastSquaresConfig{
				Length:  129,
				Mix:     0.5,
				FFTSize: 4096,
				Method:  method,
			},
		)
		if err != nil {
			t.Fatalf("%v: DesignComplexLeastSquares() error = %v", method, err)
		}

		t.Logf(
			"%v: relative magnitude error = %g, complex peak = %g",
			method,
			result.Metrics.RelativeMagnitudeError,
			result.ComplexError.Peak,
		)

		if result.Metrics.RelativeMagnitudeError > 1e-2 {
			t.Fatalf(
				"%v: relative magnitude error = %g, want <= 1e-2",
				method,
				result.Metrics.RelativeMagnitudeError,
			)
		}
	}
}

func TestSolveNormalEquationsRejectsDegenerateInput(t *testing.T) {
	if _, err := solveNormalEquations(nil, nil); !errors.Is(err, ErrInvalidLength) {
		t.Fatalf("empty system error = %v", err)
	}

	if _, err := solveNormalEquations(
		[]float64{1},
		[]float64{1, 2},
	); !errors.Is(err, ErrInvalidLength) {
		t.Fatalf("short autocorrelation error = %v", err)
	}

	if _, err := solveNormalEquations(
		[]float64{0, 0},
		[]float64{1, 2},
	); !errors.Is(err, ErrSingularSystem) {
		t.Fatalf("zero autocorrelation error = %v", err)
	}

	// An autocorrelation whose matrix is indefinite at every ridge on the
	// ladder must be reported rather than silently returning garbage.
	if _, err := solveNormalEquations(
		[]float64{1, 1e9},
		[]float64{1, 2},
	); !errors.Is(err, ErrSingularSystem) {
		t.Fatalf("indefinite system error = %v", err)
	}
}

// TestSolveNormalEquationsSolvesKnownSystem checks the Cholesky path against a
// system whose solution is known in closed form.
func TestSolveNormalEquationsSolvesKnownSystem(t *testing.T) {
	// A tridiagonal Toeplitz matrix with 2 on the diagonal and 1 off it.
	autocorrelation := []float64{2, 1, 0, 0}
	want := []float64{1, -2, 3, -1}

	size := len(want)
	rhs := make([]float64, size)

	for m := range size {
		for n := range size {
			rhs[m] += autocorrelation[abs(m-n)] * want[n]
		}
	}

	got, err := solveNormalEquations(autocorrelation, rhs)
	if err != nil {
		t.Fatalf("solveNormalEquations() error = %v", err)
	}

	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("solution[%d] = %.17g, want %.17g", i, got[i], want[i])
		}
	}
}

func BenchmarkDesignComplexLeastSquares(b *testing.B) {
	prototype := lowpassPrototype(513, 0.1)

	cases := []struct {
		name       string
		iterations int
	}{
		{"wls", 0},
		{"irls8", 8},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			cfg := ComplexLeastSquaresConfig{
				Length:            257,
				Mix:               0.4,
				FFTSize:           8192,
				MinimaxIterations: tc.iterations,
				MinimaxTolerance:  -1,
			}

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				if _, err := DesignComplexLeastSquares(prototype, cfg); err != nil {
					b.Fatalf("DesignComplexLeastSquares() error = %v", err)
				}
			}
		})
	}
}

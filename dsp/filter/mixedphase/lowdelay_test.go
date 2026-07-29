package mixedphase

import (
	"errors"
	"math"
	"testing"
)

// lowDelayTestConfig keeps the optimiser small enough for the test suite while
// still exercising every penalty stage.
func lowDelayTestConfig(toleranceDB float64) LowGroupDelayConfig {
	return LowGroupDelayConfig{
		FFTSize:     256,
		ToleranceDB: toleranceDB,
		Iterations:  60,
	}
}

// startingDelay reports what the default starting point achieves under the same
// problem definition, which is the only fair reference for the optimised value.
func startingDelay(
	t *testing.T,
	prototype []float64,
	cfg LowGroupDelayConfig,
) GroupDelayMetrics {
	t.Helper()

	problem, taps, err := newLowDelayProblem(prototype, cfg)
	if err != nil {
		t.Fatalf("newLowDelayProblem() error = %v", err)
	}

	return problem.summarise(taps)
}

// TestLowGroupDelayGradientMatchesFiniteDifferences guards the analytic
// derivative of the penalised group-delay objective. Every other property of
// this design rests on it, and a sign error there would still produce
// plausible-looking filters.
func TestLowGroupDelayGradientMatchesFiniteDifferences(t *testing.T) {
	prototype := lowpassPrototype(33, 0.1)

	cfg := LowGroupDelayConfig{Length: 17, FFTSize: 64, ToleranceDB: 0.5}

	problem, taps, err := newLowDelayProblem(prototype, cfg)
	if err != nil {
		t.Fatalf("newLowDelayProblem() error = %v", err)
	}

	// A penalty large enough that the constraint term is active, so both
	// branches of the objective are covered.
	problem.penalty = 3.7

	analytic := make([]float64, len(taps))
	problem.evaluate(taps, analytic)

	scratch := make([]float64, len(taps))

	for n := range taps {
		const step = 1e-6

		original := taps[n]

		taps[n] = original + step
		plus := problem.evaluate(taps, scratch)

		taps[n] = original - step
		minus := problem.evaluate(taps, scratch)

		taps[n] = original

		numeric := (plus - minus) / (2 * step)

		deviation := math.Abs(numeric-analytic[n]) / max(1, math.Abs(numeric))
		if deviation > 1e-5 {
			t.Errorf(
				"tap %d: analytic gradient = %g, finite difference = %g",
				n,
				analytic[n],
				numeric,
			)
		}
	}
}

// TestLowGroupDelayUndercutsMinimumPhase is the point of the whole method: a
// truncated minimum-phase design is not delay-optimal once the magnitude is
// allowed to move within a stated tolerance.
func TestLowGroupDelayUndercutsMinimumPhase(t *testing.T) {
	prototype := lowpassPrototype(33, 0.12)
	cfg := lowDelayTestConfig(6)

	before := startingDelay(t, prototype, cfg)

	result, err := DesignLowGroupDelay(prototype, cfg)
	if err != nil {
		t.Fatalf("DesignLowGroupDelay() error = %v", err)
	}

	if result.GroupDelay.Mean >= before.Mean*0.995 {
		t.Errorf(
			"mean group delay = %v, want at least 0.5%% below the "+
				"minimum-phase start %v",
			result.GroupDelay.Mean,
			before.Mean,
		)
	}

	if result.GroupDelay.Peak >= before.Peak {
		t.Errorf(
			"peak group delay = %v, want below the minimum-phase start %v",
			result.GroupDelay.Peak,
			before.Peak,
		)
	}

	if result.GroupDelay.ConstraintViolation > 1e-3 {
		t.Errorf(
			"constraint violation = %v, want a feasible design",
			result.GroupDelay.ConstraintViolation,
		)
	}

	if result.Iterations == 0 {
		t.Error("Iterations = 0, want the optimiser to accept steps")
	}
}

// TestLowGroupDelayRestoresFeasibility covers the opposite case: the default
// starting point already violates a tight tolerance, so the penalty ladder has
// to buy accuracy back with delay.
func TestLowGroupDelayRestoresFeasibility(t *testing.T) {
	prototype := lowpassPrototype(33, 0.12)
	cfg := lowDelayTestConfig(0.25)

	before := startingDelay(t, prototype, cfg)
	if before.ConstraintViolation <= 0.1 {
		t.Fatalf(
			"minimum-phase start violation = %v, want an infeasible start for "+
				"this test to mean anything",
			before.ConstraintViolation,
		)
	}

	result, err := DesignLowGroupDelay(prototype, cfg)
	if err != nil {
		t.Fatalf("DesignLowGroupDelay() error = %v", err)
	}

	if result.GroupDelay.ConstraintViolation > 1e-3 {
		t.Errorf(
			"constraint violation = %v, want the ladder to restore feasibility",
			result.GroupDelay.ConstraintViolation,
		)
	}
}

// TestLowGroupDelayToleranceBuysDelay documents the trade-off the tolerance
// controls.
func TestLowGroupDelayToleranceBuysDelay(t *testing.T) {
	prototype := lowpassPrototype(33, 0.12)

	tight, err := DesignLowGroupDelay(prototype, lowDelayTestConfig(0.5))
	if err != nil {
		t.Fatalf("DesignLowGroupDelay() error = %v", err)
	}

	loose, err := DesignLowGroupDelay(prototype, lowDelayTestConfig(6))
	if err != nil {
		t.Fatalf("DesignLowGroupDelay() error = %v", err)
	}

	if loose.GroupDelay.Mean >= tight.GroupDelay.Mean {
		t.Errorf(
			"mean group delay at 6 dB = %v, at 0.5 dB = %v, want the looser "+
				"tolerance to yield the lower delay",
			loose.GroupDelay.Mean,
			tight.GroupDelay.Mean,
		)
	}

	if loose.Metrics.RelativeMagnitudeError <=
		tight.Metrics.RelativeMagnitudeError {
		t.Errorf(
			"relative magnitude error at 6 dB = %v, at 0.5 dB = %v, want the "+
				"looser tolerance to be paid for in magnitude accuracy",
			loose.Metrics.RelativeMagnitudeError,
			tight.Metrics.RelativeMagnitudeError,
		)
	}
}

// TestLowGroupDelayDependsOnInitialisation records that the problem is not
// convex. The linear-phase start sits in a basin the optimiser cannot leave,
// because escaping it means moving a zero across the unit circle.
func TestLowGroupDelayDependsOnInitialisation(t *testing.T) {
	prototype := lowpassPrototype(33, 0.12)

	cfg := lowDelayTestConfig(1)

	fromMinimum, err := DesignLowGroupDelay(prototype, cfg)
	if err != nil {
		t.Fatalf("DesignLowGroupDelay() error = %v", err)
	}

	linear := cfg
	linear.InitialTaps = prototype

	fromLinear, err := DesignLowGroupDelay(prototype, linear)
	if err != nil {
		t.Fatalf("DesignLowGroupDelay() error = %v", err)
	}

	if fromLinear.GroupDelay.Mean <= fromMinimum.GroupDelay.Mean*1.5 {
		t.Errorf(
			"mean group delay from the linear-phase start = %v, from the "+
				"minimum-phase start = %v, want a clearly worse local optimum",
			fromLinear.GroupDelay.Mean,
			fromMinimum.GroupDelay.Mean,
		)
	}

	// Both are genuine local optima: each improves on its own starting point.
	problem, _, err := newLowDelayProblem(prototype, linear)
	if err != nil {
		t.Fatalf("newLowDelayProblem() error = %v", err)
	}

	if before := problem.summarise(prototype); fromLinear.GroupDelay.Mean >=
		before.Mean {
		t.Errorf(
			"mean group delay from the linear-phase start = %v, want below its "+
				"own start %v",
			fromLinear.GroupDelay.Mean,
			before.Mean,
		)
	}
}

// TestLowGroupDelayWeightSelectsBand checks that a supplied weight really does
// steer the objective, measured under that same weight.
func TestLowGroupDelayWeightSelectsBand(t *testing.T) {
	prototype := lowpassPrototype(33, 0.12)

	cfg := lowDelayTestConfig(1)

	uniform := make([]float64, cfg.FFTSize/2+1)
	for i := range uniform {
		uniform[i] = 1
	}

	weighted := cfg
	weighted.DelayWeight = uniform

	defaultDesign, err := DesignLowGroupDelay(prototype, cfg)
	if err != nil {
		t.Fatalf("DesignLowGroupDelay() error = %v", err)
	}

	uniformDesign, err := DesignLowGroupDelay(prototype, weighted)
	if err != nil {
		t.Fatalf("DesignLowGroupDelay() error = %v", err)
	}

	problem, _, err := newLowDelayProblem(prototype, weighted)
	if err != nil {
		t.Fatalf("newLowDelayProblem() error = %v", err)
	}

	underUniform := problem.summarise(uniformDesign.Taps).Mean
	defaultUnderUniform := problem.summarise(defaultDesign.Taps).Mean

	if underUniform >= defaultUnderUniform {
		t.Errorf(
			"uniformly weighted design scores %v under its own weight, the "+
				"magnitude-weighted design scores %v, want the former to win",
			underUniform,
			defaultUnderUniform,
		)
	}
}

// TestLowGroupDelayNegativeIterationsReturnsStart gives callers a way to
// measure the starting point through the same reporting path.
func TestLowGroupDelayNegativeIterationsReturnsStart(t *testing.T) {
	prototype := lowpassPrototype(33, 0.12)

	cfg := lowDelayTestConfig(1)
	cfg.Iterations = -1

	result, err := DesignLowGroupDelay(prototype, cfg)
	if err != nil {
		t.Fatalf("DesignLowGroupDelay() error = %v", err)
	}

	if result.Iterations != 0 {
		t.Errorf("Iterations = %d, want 0", result.Iterations)
	}

	expected, err := MinimumPhaseWith(
		prototype,
		MinimumPhaseConfig{FFTSize: cfg.FFTSize},
	)
	if err != nil {
		t.Fatalf("MinimumPhaseWith() error = %v", err)
	}

	for i, tap := range result.Taps {
		if math.Abs(tap-expected[i]) > 1e-12 {
			t.Fatalf(
				"tap %d = %v, want the untouched minimum-phase start %v",
				i,
				tap,
				expected[i],
			)
		}
	}
}

func TestLowGroupDelayValidation(t *testing.T) {
	prototype := lowpassPrototype(17, 0.2)

	tests := []struct {
		name string
		cfg  LowGroupDelayConfig
		want error
	}{
		{
			name: "negative length",
			cfg:  LowGroupDelayConfig{Length: -1},
			want: ErrInvalidLength,
		},
		{
			name: "negative epsilon",
			cfg:  LowGroupDelayConfig{Epsilon: -1},
			want: ErrInvalidEpsilon,
		},
		{
			name: "negative tolerance",
			cfg:  LowGroupDelayConfig{ToleranceDB: -1},
			want: ErrInvalidTolerance,
		},
		{
			name: "negative penalty",
			cfg:  LowGroupDelayConfig{InitialPenalty: -1},
			want: ErrInvalidTolerance,
		},
		{
			name: "negative stages",
			cfg:  LowGroupDelayConfig{PenaltyStages: -1},
			want: ErrInvalidLength,
		},
		{
			name: "unknown method",
			cfg:  LowGroupDelayConfig{Method: MinimumPhaseMethod(7)},
			want: ErrInvalidMethod,
		},
		{
			name: "short FFT",
			cfg:  LowGroupDelayConfig{FFTSize: 8},
			want: ErrInvalidLength,
		},
		{
			name: "wrong initial length",
			cfg:  LowGroupDelayConfig{InitialTaps: make([]float64, 3)},
			want: ErrInvalidLength,
		},
		{
			name: "wrong weight length",
			cfg:  LowGroupDelayConfig{DelayWeight: make([]float64, 3)},
			want: ErrInvalidWeight,
		},
		{
			name: "negative weight",
			cfg: LowGroupDelayConfig{
				FFTSize:     256,
				DelayWeight: negativeWeight(129),
			},
			want: ErrInvalidWeight,
		},
		{
			name: "zero weight",
			cfg: LowGroupDelayConfig{
				FFTSize:     256,
				DelayWeight: make([]float64, 129),
			},
			want: ErrInvalidWeight,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DesignLowGroupDelay(prototype, test.cfg); !errors.Is(
				err,
				test.want,
			) {
				t.Errorf("DesignLowGroupDelay() error = %v, want %v", err, test.want)
			}
		})
	}

	if _, err := DesignLowGroupDelay(nil, LowGroupDelayConfig{}); !errors.Is(
		err,
		ErrEmptyPrototype,
	) {
		t.Errorf("DesignLowGroupDelay(nil) error = %v, want %v", err, ErrEmptyPrototype)
	}

	if _, err := DesignLowGroupDelay(
		make([]float64, 8),
		LowGroupDelayConfig{},
	); err == nil {
		t.Error("DesignLowGroupDelay() with a silent prototype error = nil, want error")
	}
}

func BenchmarkDesignLowGroupDelay(b *testing.B) {
	prototype := lowpassPrototype(65, 0.08)

	cfg := LowGroupDelayConfig{
		FFTSize:     512,
		ToleranceDB: 1,
		Iterations:  50,
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, err := DesignLowGroupDelay(prototype, cfg); err != nil {
			b.Fatalf("DesignLowGroupDelay() error = %v", err)
		}
	}
}

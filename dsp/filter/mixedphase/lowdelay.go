package mixedphase

import (
	"fmt"
	"math"
)

const (
	defaultMagnitudeToleranceDB = 1.0
	defaultPenaltyStages        = 6
	defaultOptimiserIterations  = 200
	defaultInitialPenalty       = 1.0

	penaltyGrowth = 10.0
	lbfgsMemory   = 8

	// lowDelayStopbandFloor keeps the magnitude constraint finite where the
	// target is numerically zero: the permitted band never narrows below this
	// fraction of the target peak.
	lowDelayStopbandFloor = 1e-3

	// lowDelayDenominatorFloor bounds the 1/|H|^2 factor of the group delay,
	// relative to the squared target peak. Without it a deep null would
	// dominate the gradient even though nobody hears its phase.
	lowDelayDenominatorFloor = 1e-6

	// lowDelayPeakWeightShare selects which bins count towards the reported
	// peak group delay.
	lowDelayPeakWeightShare = 1e-2
)

// DesignLowGroupDelay minimises passband group delay directly, subject to a
// magnitude constraint, in the spirit of the Wu-Gao-Teo formulation.
//
// The other designs in this package fix the phase first — either implicitly
// through a latency budget, or explicitly as a prescribed curve — and then
// approximate the resulting complex response. Here the phase is free: the
// optimiser is asked only to keep |H(omega)| inside a tolerance band around the
// prototype magnitude and to make the weighted group delay as small as it can
// under that restriction.
//
// That objective is smooth but not convex. It is solved by a penalty ladder
// around a limited-memory BFGS minimiser: each stage multiplies the constraint
// penalty by ten and re-optimises from the previous solution, so early stages
// move freely and late stages enforce feasibility. Convergence is therefore to
// a local minimum, and [LowGroupDelayConfig.InitialTaps] chooses which one.
//
// The returned [Result] reports the achieved group delay and the worst
// remaining constraint violation; a non-zero violation means the tolerance was
// too tight for the tap budget.
func DesignLowGroupDelay(
	prototype []float64,
	cfg LowGroupDelayConfig,
) (Result, error) {
	problem, taps, err := newLowDelayProblem(prototype, cfg)
	if err != nil {
		return Result{}, err
	}

	iterations := runPenaltyLadder(problem, taps, cfg)

	metrics, err := Analyze(prototype, taps, problem.size)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Taps:       taps,
		Iterations: iterations,
		Metrics:    metrics,
		GroupDelay: problem.summarise(taps),
	}, nil
}

// runPenaltyLadder re-optimises taps with a geometrically increasing constraint
// penalty and returns the total number of accepted steps.
func runPenaltyLadder(
	problem *lowDelayProblem,
	taps []float64,
	cfg LowGroupDelayConfig,
) int {
	if cfg.Iterations < 0 {
		return 0
	}

	iterations := cfg.Iterations
	if iterations == 0 {
		iterations = defaultOptimiserIterations
	}

	stages := cfg.PenaltyStages
	if stages == 0 {
		stages = defaultPenaltyStages
	}

	penalty := cfg.InitialPenalty
	if penalty == 0 {
		penalty = defaultInitialPenalty
	}

	performed := 0

	for range stages {
		problem.penalty = penalty
		performed += minimizeLBFGS(problem.evaluate, taps, lbfgsMemory, iterations)
		penalty *= penaltyGrowth
	}

	return performed
}

// lowDelayProblem holds the fixed data of one design: the trigonometric tables
// of the design grid, the permitted magnitude band, and the delay weight.
type lowDelayProblem struct {
	size   int
	bins   int
	length int

	// cosine and sine are row-major [bin][tap] tables of cos(omega*n) and
	// sin(omega*n). They cost bins*length floats but remove all transcendental
	// calls from the inner loop, which runs once per objective evaluation.
	cosine []float64
	sine   []float64

	target    []float64
	tolerance []float64
	weight    []float64

	denominatorFloor float64
	penalty          float64
}

func newLowDelayProblem(
	prototype []float64,
	cfg LowGroupDelayConfig,
) (*lowDelayProblem, []float64, error) {
	if len(prototype) == 0 {
		return nil, nil, ErrEmptyPrototype
	}

	if cfg.Epsilon < 0 {
		return nil, nil, fmt.Errorf("%w: got %g", ErrInvalidEpsilon, cfg.Epsilon)
	}

	if cfg.ToleranceDB < 0 {
		return nil, nil, fmt.Errorf(
			"%w: got %g dB",
			ErrInvalidTolerance,
			cfg.ToleranceDB,
		)
	}

	if cfg.InitialPenalty < 0 {
		return nil, nil, fmt.Errorf(
			"%w: initial penalty must not be negative, got %g",
			ErrInvalidTolerance,
			cfg.InitialPenalty,
		)
	}

	if cfg.PenaltyStages < 0 {
		return nil, nil, fmt.Errorf(
			"%w: penalty stages must not be negative, got %d",
			ErrInvalidLength,
			cfg.PenaltyStages,
		)
	}

	if !cfg.Method.valid() {
		return nil, nil, fmt.Errorf("%w: %d", ErrInvalidMethod, int(cfg.Method))
	}

	length := cfg.Length
	if length == 0 {
		length = len(prototype)
	}

	if length <= 0 {
		return nil, nil, ErrInvalidLength
	}

	size, err := nextDesignFFTSize(max(length, len(prototype)), cfg.FFTSize)
	if err != nil {
		return nil, nil, err
	}

	w, err := newFFTWorkspace(size)
	if err != nil {
		return nil, nil, err
	}

	targetSpectrum, err := w.forwardReal(prototype)
	if err != nil {
		return nil, nil, err
	}

	targetMagnitude := magnitude(targetSpectrum)

	peak := 0.0
	for i := range size/2 + 1 {
		peak = max(peak, targetMagnitude[i])
	}

	if peak == 0 {
		return nil, nil, fmt.Errorf(
			"mixedphase: reference magnitude is identically zero",
		)
	}

	problem := &lowDelayProblem{
		size:             size,
		bins:             size/2 + 1,
		length:           length,
		target:           targetMagnitude[:size/2+1],
		denominatorFloor: lowDelayDenominatorFloor * peak * peak,
	}

	problem.buildTables()
	problem.buildTolerance(cfg.ToleranceDB, peak)

	problem.weight, err = delayWeights(cfg.DelayWeight, problem.target)
	if err != nil {
		return nil, nil, err
	}

	taps, err := lowDelayStart(w, cfg, targetMagnitude, length)
	if err != nil {
		return nil, nil, err
	}

	return problem, taps, nil
}

func (p *lowDelayProblem) buildTables() {
	p.cosine = make([]float64, p.bins*p.length)
	p.sine = make([]float64, p.bins*p.length)

	for k := range p.bins {
		omega := 2 * math.Pi * float64(k) / float64(p.size)
		row := k * p.length

		for n := range p.length {
			angle := omega * float64(n)
			p.cosine[row+n] = math.Cos(angle)
			p.sine[row+n] = math.Sin(angle)
		}
	}
}

// buildTolerance converts the dB tolerance into an absolute allowance per bin.
//
// A relative band alone would demand infinite precision at a spectral null, so
// the reference magnitude is floored at a small fraction of the peak.
func (p *lowDelayProblem) buildTolerance(toleranceDB, peak float64) {
	if toleranceDB == 0 {
		toleranceDB = defaultMagnitudeToleranceDB
	}

	relative := math.Pow(10, toleranceDB/20) - 1
	floor := peak * lowDelayStopbandFloor

	p.tolerance = make([]float64, p.bins)
	for i, value := range p.target {
		p.tolerance[i] = relative * max(value, floor)
	}
}

// delayWeights validates a supplied weight or derives the default one.
//
// The squared target magnitude is the natural default: it places the objective
// where the filter actually passes energy and vanishes in the stopband, where
// group delay is both meaningless and numerically fragile.
func delayWeights(requested, target []float64) ([]float64, error) {
	out := make([]float64, len(target))
	total := 0.0

	if requested == nil {
		for i, value := range target {
			out[i] = value * value
			total += out[i]
		}
	} else {
		if len(requested) != len(target) {
			return nil, fmt.Errorf(
				"%w: got %d delay weights, want %d",
				ErrInvalidWeight,
				len(requested),
				len(target),
			)
		}

		for i, value := range requested {
			if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf(
					"%w: delay weight[%d] = %g",
					ErrInvalidWeight,
					i,
					value,
				)
			}

			out[i] = value
			total += value
		}
	}

	if total == 0 {
		return nil, fmt.Errorf("%w: all delay weights are zero", ErrInvalidWeight)
	}

	for i := range out {
		out[i] /= total
	}

	return out, nil
}

// lowDelayStart returns the starting point of the optimisation.
func lowDelayStart(
	w *fftWorkspace,
	cfg LowGroupDelayConfig,
	targetMagnitude []float64,
	length int,
) ([]float64, error) {
	if cfg.InitialTaps != nil {
		if len(cfg.InitialTaps) != length {
			return nil, fmt.Errorf(
				"%w: got %d initial taps, want %d",
				ErrInvalidLength,
				len(cfg.InitialTaps),
				length,
			)
		}

		return append([]float64(nil), cfg.InitialTaps...), nil
	}

	epsilon := defaultEpsilon(targetMagnitude, cfg.Epsilon)

	minimumSpectrum, err := minimumPhaseSpectrum(
		w,
		targetMagnitude,
		epsilon,
		cfg.Method,
	)
	if err != nil {
		return nil, err
	}

	impulse, err := w.inverseReal(minimumSpectrum)
	if err != nil {
		return nil, err
	}

	return append([]float64(nil), impulse[:length]...), nil
}

// evaluate returns the penalised objective at taps and writes its gradient.
//
// Per bin the response is H = A + jB and its frequency derivative is
// dH/domega = -j*(C + jD), from which the group delay follows as
// (A*C + B*D)/|H|^2. Differentiating that quotient with respect to a tap only
// needs the same four sums, so one pass accumulates them and a second spreads
// the result back over the taps.
func (p *lowDelayProblem) evaluate(taps, grad []float64) float64 {
	for i := range grad {
		grad[i] = 0
	}

	total := 0.0

	for k := range p.bins {
		row := k * p.length
		cosRow := p.cosine[row : row+p.length]
		sinRow := p.sine[row : row+p.length]

		responseReal, responseImag, slopeReal, slopeImag := 0.0, 0.0, 0.0, 0.0

		for n, tap := range taps {
			cosine, sine := cosRow[n], sinRow[n]
			responseReal += tap * cosine
			responseImag -= tap * sine
			slopeReal += float64(n) * tap * cosine
			slopeImag -= float64(n) * tap * sine
		}

		squared := responseReal*responseReal + responseImag*responseImag
		denominator := max(squared, p.denominatorFloor)

		numerator := responseReal*slopeReal + responseImag*slopeImag
		delay := numerator / denominator

		weight := p.weight[k]
		total += weight * delay

		// The derivative of the denominator vanishes once it is floored, which
		// is exactly the intended effect: the null stops steering the search.
		derivativeScale := 2.0
		if squared <= p.denominatorFloor {
			derivativeScale = 0
		}

		firstTerm := weight * (slopeReal - derivativeScale*delay*responseReal) /
			denominator
		secondTerm := weight * (slopeImag - derivativeScale*delay*responseImag) /
			denominator
		delayScale := weight / denominator

		magnitudeScale := p.penaltyGradient(k, squared, &total)

		for n := range taps {
			cosine, sine := cosRow[n], sinRow[n]
			// Derivative of |H|^2/2 with respect to tap n.
			energy := responseReal*cosine - responseImag*sine

			grad[n] += cosine*firstTerm - sine*secondTerm +
				float64(n)*energy*delayScale +
				magnitudeScale*energy
		}
	}

	return total
}

// penaltyGradient adds the constraint penalty of one bin to total and returns
// the factor by which the derivative of |H|^2/2 enters the gradient.
func (p *lowDelayProblem) penaltyGradient(
	k int,
	squared float64,
	total *float64,
) float64 {
	tolerance := p.tolerance[k]
	if tolerance <= 0 {
		return 0
	}

	response := math.Sqrt(squared)

	deviation := response - p.target[k]
	excess := math.Abs(deviation) - tolerance

	if excess <= 0 || response == 0 {
		return 0
	}

	scaled := excess / tolerance
	*total += p.penalty * scaled * scaled

	sign := 1.0
	if deviation < 0 {
		sign = -1
	}

	// d/dh of penalty = 2*penalty*excess/tolerance^2 * sign * d|H|/dh, and
	// d|H|/dh is the derivative of |H|^2/2 divided by |H|.
	return 2 * p.penalty * scaled * sign / (tolerance * response)
}

// summarise reports the group delay of taps and the worst constraint violation.
func (p *lowDelayProblem) summarise(taps []float64) GroupDelayMetrics {
	largestWeight := 0.0
	for _, value := range p.weight {
		largestWeight = max(largestWeight, value)
	}

	threshold := largestWeight * lowDelayPeakWeightShare

	out := GroupDelayMetrics{Peak: math.Inf(-1)}

	for k := range p.bins {
		row := k * p.length
		cosRow := p.cosine[row : row+p.length]
		sinRow := p.sine[row : row+p.length]

		responseReal, responseImag, slopeReal, slopeImag := 0.0, 0.0, 0.0, 0.0

		for n, tap := range taps {
			cosine, sine := cosRow[n], sinRow[n]
			responseReal += tap * cosine
			responseImag -= tap * sine
			slopeReal += float64(n) * tap * cosine
			slopeImag -= float64(n) * tap * sine
		}

		squared := responseReal*responseReal + responseImag*responseImag
		denominator := max(squared, p.denominatorFloor)
		delay := (responseReal*slopeReal + responseImag*slopeImag) / denominator

		out.Mean += p.weight[k] * delay

		if p.weight[k] >= threshold {
			out.Peak = max(out.Peak, delay)
		}

		if p.tolerance[k] > 0 {
			excess := math.Abs(math.Sqrt(squared)-p.target[k]) - p.tolerance[k]
			out.ConstraintViolation = max(
				out.ConstraintViolation,
				excess/p.tolerance[k],
			)
		}
	}

	if math.IsInf(out.Peak, -1) {
		out.Peak = 0
	}

	return out
}

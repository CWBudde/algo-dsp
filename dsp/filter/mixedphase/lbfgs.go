package mixedphase

import "math"

const (
	lbfgsArmijoConstant    = 1e-4
	lbfgsBacktrackFactor   = 0.5
	lbfgsCurvatureFloor    = 1e-12
	lbfgsGradientTolerance = 1e-10
	lbfgsMaxBacktrackSteps = 40
)

// objectiveFunc evaluates the objective at x and stores its gradient in grad.
//
// The gradient slice is owned by the caller and has the same length as x.
type objectiveFunc func(x, grad []float64) float64

// lbfgsPair is one curvature pair of the limited-memory history.
type lbfgsPair struct {
	s   []float64
	y   []float64
	rho float64
}

// minimizeLBFGS minimises objective from x using limited-memory BFGS with an
// Armijo backtracking line search, overwriting x with the best point found and
// returning the number of accepted iterations.
//
// The design problem this serves is smooth but non-convex, so a plain descent
// method with a sufficient-decrease condition is the right level of machinery:
// it needs no second derivatives, cannot increase the objective, and stops as
// soon as no acceptable step exists.
func minimizeLBFGS(
	objective objectiveFunc,
	x []float64,
	memory int,
	maxIterations int,
) int {
	count := len(x)

	grad := make([]float64, count)
	next := make([]float64, count)
	nextGrad := make([]float64, count)
	direction := make([]float64, count)
	alpha := make([]float64, memory)
	history := make([]lbfgsPair, 0, memory)

	value := objective(x, grad)
	performed := 0

	for range maxIterations {
		if infinityNorm(grad) <= lbfgsGradientTolerance {
			break
		}

		lbfgsDirection(direction, grad, history, alpha)

		slope := dot(direction, grad)
		if slope >= 0 {
			// The stored curvature no longer describes the local geometry;
			// steepest descent is always a descent direction.
			for i := range direction {
				direction[i] = -grad[i]
			}

			slope = -dot(grad, grad)
			history = history[:0]
		}

		step := 1.0
		if len(history) == 0 {
			step = 1 / max(1, infinityNorm(direction))
		}

		nextValue, ok := lbfgsLineSearch(
			objective,
			x,
			direction,
			next,
			nextGrad,
			value,
			slope,
			step,
		)
		if !ok {
			break
		}

		history = pushCurvature(history, x, next, grad, nextGrad, memory)

		copy(x, next)
		copy(grad, nextGrad)

		value = nextValue
		performed++
	}

	return performed
}

// lbfgsLineSearch backtracks along direction until the Armijo condition holds,
// leaving the accepted point in next and its gradient in nextGrad.
func lbfgsLineSearch(
	objective objectiveFunc,
	x []float64,
	direction []float64,
	next []float64,
	nextGrad []float64,
	value float64,
	slope float64,
	step float64,
) (float64, bool) {
	for range lbfgsMaxBacktrackSteps {
		for i := range next {
			next[i] = x[i] + step*direction[i]
		}

		// A non-finite trial value fails the comparison and is backtracked
		// away from, so no separate guard is needed.
		nextValue := objective(next, nextGrad)
		if nextValue <= value+lbfgsArmijoConstant*step*slope {
			return nextValue, true
		}

		step *= lbfgsBacktrackFactor
	}

	return value, false
}

// lbfgsDirection applies the two-loop recursion, writing the quasi-Newton
// search direction into direction.
func lbfgsDirection(
	direction []float64,
	grad []float64,
	history []lbfgsPair,
	alpha []float64,
) {
	copy(direction, grad)

	for i := len(history) - 1; i >= 0; i-- {
		pair := history[i]
		alpha[i] = pair.rho * dot(pair.s, direction)

		for j := range direction {
			direction[j] -= alpha[i] * pair.y[j]
		}
	}

	if len(history) > 0 {
		last := history[len(history)-1]

		if denominator := dot(last.y, last.y); denominator > 0 {
			scale := dot(last.s, last.y) / denominator
			for j := range direction {
				direction[j] *= scale
			}
		}
	}

	for i := range history {
		pair := history[i]
		beta := pair.rho * dot(pair.y, direction)

		for j := range direction {
			direction[j] += (alpha[i] - beta) * pair.s[j]
		}
	}

	for j := range direction {
		direction[j] = -direction[j]
	}
}

// pushCurvature appends the step and gradient difference to the history,
// dropping the oldest pair once the memory is full.
//
// Pairs that fail the curvature condition are skipped rather than stored: they
// would make the implicit inverse Hessian indefinite.
func pushCurvature(
	history []lbfgsPair,
	x, next []float64,
	grad, nextGrad []float64,
	memory int,
) []lbfgsPair {
	s := make([]float64, len(x))
	y := make([]float64, len(x))

	for i := range s {
		s[i] = next[i] - x[i]
		y[i] = nextGrad[i] - grad[i]
	}

	curvature := dot(s, y)
	if curvature <= lbfgsCurvatureFloor*math.Sqrt(dot(s, s)*dot(y, y)) {
		return history
	}

	if len(history) == memory {
		history = append(history[:0], history[1:]...)
	}

	return append(history, lbfgsPair{s: s, y: y, rho: 1 / curvature})
}

func dot(a, b []float64) float64 {
	total := 0.0
	for i, value := range a {
		total += value * b[i]
	}

	return total
}

func infinityNorm(values []float64) float64 {
	largest := 0.0
	for _, value := range values {
		largest = max(largest, math.Abs(value))
	}

	return largest
}

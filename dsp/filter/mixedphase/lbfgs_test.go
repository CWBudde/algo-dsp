package mixedphase

import (
	"math"
	"testing"
)

// rosenbrock is the standard curved-valley test problem: a quasi-Newton method
// must handle it, a fixed-step gradient method will not.
func rosenbrock(x, grad []float64) float64 {
	first := 1 - x[0]
	second := x[1] - x[0]*x[0]

	grad[0] = -2*first - 400*x[0]*second
	grad[1] = 200 * second

	return first*first + 100*second*second
}

func TestMinimizeLBFGSSolvesRosenbrock(t *testing.T) {
	x := []float64{-1.2, 1}

	iterations := minimizeLBFGS(rosenbrock, x, 8, 500)
	if iterations == 0 {
		t.Fatal("minimizeLBFGS() accepted no step")
	}

	for i, want := range []float64{1, 1} {
		if math.Abs(x[i]-want) > 1e-5 {
			t.Errorf("x[%d] = %v, want %v", i, x[i], want)
		}
	}
}

// TestMinimizeLBFGSStopsAtOptimum covers the gradient-tolerance exit: starting
// at the solution there is nothing left to do.
func TestMinimizeLBFGSStopsAtOptimum(t *testing.T) {
	x := []float64{1, 1}

	if iterations := minimizeLBFGS(rosenbrock, x, 8, 100); iterations != 0 {
		t.Errorf("minimizeLBFGS() = %d accepted steps, want 0", iterations)
	}
}

// TestMinimizeLBFGSHandlesNonFiniteObjective covers the line search running out
// of backtracking steps, which is how the optimiser gives up rather than
// wandering into a region it cannot evaluate.
func TestMinimizeLBFGSHandlesNonFiniteObjective(t *testing.T) {
	objective := func(x, grad []float64) float64 {
		grad[0] = 1

		if x[0] != 0 {
			return math.NaN()
		}

		return 0
	}

	x := []float64{0}

	if iterations := minimizeLBFGS(objective, x, 4, 50); iterations != 0 {
		t.Errorf("minimizeLBFGS() = %d accepted steps, want 0", iterations)
	}

	if x[0] != 0 {
		t.Errorf("x[0] = %v, want the starting point to be kept", x[0])
	}
}

// TestMinimizeLBFGSRecoversFromBadCurvature drives the fallback to steepest
// descent by minimising a function whose curvature changes sign along the path.
func TestMinimizeLBFGSRecoversFromBadCurvature(t *testing.T) {
	objective := func(x, grad []float64) float64 {
		// A quartic with a flat shoulder; the stored curvature is a poor model
		// there and the two-loop direction can turn uphill.
		value := 0.0

		for i, xi := range x {
			value += math.Pow(xi-float64(i), 4)
			grad[i] = 4 * math.Pow(xi-float64(i), 3)
		}

		return value
	}

	x := []float64{-8, 9, -3}

	if iterations := minimizeLBFGS(objective, x, 3, 400); iterations == 0 {
		t.Fatal("minimizeLBFGS() accepted no step")
	}

	for i := range x {
		if math.Abs(x[i]-float64(i)) > 1e-2 {
			t.Errorf("x[%d] = %v, want %v", i, x[i], float64(i))
		}
	}
}

func TestMinimizeLBFGSRespectsIterationBudget(t *testing.T) {
	x := []float64{-1.2, 1}

	if iterations := minimizeLBFGS(rosenbrock, x, 8, 3); iterations != 3 {
		t.Errorf("minimizeLBFGS() = %d accepted steps, want 3", iterations)
	}
}

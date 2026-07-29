package mixedphase

import (
	"fmt"
	"math"
)

// ridgeLadder lists the relative diagonal loadings tried in turn when the
// weighted normal equations are too ill-conditioned to factor. Reweighting can
// drive the weight to zero over wide bands, which leaves the autocorrelation
// matrix only positive semi-definite.
var ridgeLadder = []float64{0, 1e-12, 1e-9, 1e-6, 1e-3}

// solveNormalEquations solves the symmetric Toeplitz system R h = rhs, where
// R[m][n] = autocorrelation[|m-n|].
//
// The matrix is formed explicitly and factored with Cholesky. Filter lengths in
// this package are in the hundreds of taps, so the cubic cost is negligible
// next to the transforms, and the dense factorisation is far less sensitive to
// a near-singular weighting than the O(n^2) Levinson recursion.
func solveNormalEquations(autocorrelation, rhs []float64) ([]float64, error) {
	size := len(rhs)
	if size == 0 || len(autocorrelation) < size {
		return nil, ErrInvalidLength
	}

	scale := math.Abs(autocorrelation[0])
	if scale == 0 {
		return nil, fmt.Errorf("%w: zero autocorrelation", ErrSingularSystem)
	}

	for _, ridge := range ridgeLadder {
		matrix := make([]float64, size*size)
		for m := range size {
			for n := range size {
				matrix[m*size+n] = autocorrelation[abs(m-n)]
			}

			matrix[m*size+m] += ridge * scale
		}

		if !choleskyInPlace(matrix, size) {
			continue
		}

		return choleskySolve(matrix, size, rhs), nil
	}

	return nil, fmt.Errorf(
		"%w: Cholesky failed up to a relative ridge of %g",
		ErrSingularSystem,
		ridgeLadder[len(ridgeLadder)-1],
	)
}

// choleskyInPlace overwrites the lower triangle of matrix with its Cholesky
// factor and reports whether the matrix was positive definite.
func choleskyInPlace(matrix []float64, size int) bool {
	for m := range size {
		for n := 0; n <= m; n++ {
			sum := matrix[m*size+n]
			for k := range n {
				sum -= matrix[m*size+k] * matrix[n*size+k]
			}

			if m == n {
				if sum <= 0 || math.IsNaN(sum) {
					return false
				}

				matrix[m*size+m] = math.Sqrt(sum)

				continue
			}

			matrix[m*size+n] = sum / matrix[n*size+n]
		}
	}

	return true
}

// choleskySolve applies forward and back substitution using the factor produced
// by [choleskyInPlace].
func choleskySolve(factor []float64, size int, rhs []float64) []float64 {
	out := make([]float64, size)
	copy(out, rhs)

	for m := range size {
		sum := out[m]
		for k := range m {
			sum -= factor[m*size+k] * out[k]
		}

		out[m] = sum / factor[m*size+m]
	}

	for m := size - 1; m >= 0; m-- {
		sum := out[m]
		for k := m + 1; k < size; k++ {
			sum -= factor[k*size+m] * out[k]
		}

		out[m] = sum / factor[m*size+m]
	}

	return out
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}

package effects

import (
	"errors"
	"math"
)

const maxChebyshevPowerTerms = 25

var (
	errChebyshevPowerLength = errors.New("chebyshev power transform: requires 1 to 25 weights and a destination of at least that length")
	errChebyshevPowerFinite = errors.New("chebyshev power transform: requires finite weights and coefficients")
)

var chebyshevPowerBasis = func() [maxChebyshevPowerTerms][maxChebyshevPowerTerms]float64 {
	var basis [maxChebyshevPowerTerms][maxChebyshevPowerTerms]float64

	basis[0][0] = 1
	basis[1][1] = 1

	for order := 2; order < len(basis); order++ {
		// T_n(x) = 2*x*T_{n-1}(x) - T_{n-2}(x).
		for power := range order {
			basis[order][power+1] = 2 * basis[order-1][power]
		}

		for power := range order - 1 {
			basis[order][power] -= basis[order-2][power]
		}
	}

	return basis
}()

// ChebyshevPowerCoefficients converts weights of T_0 through T_24 into
// coefficients in ascending powers of x. Provide 1 to 25 weights and a
// destination at least as long. The destination and weights may overlap,
// and any destination elements after the result are left untouched.
//
// The transform allocates no memory. It returns an error and leaves dst
// untouched if the lengths are invalid or a weight or coefficient is nonfinite.
func ChebyshevPowerCoefficients(dst, weights []float64) error {
	count := len(weights)
	if count == 0 || count > maxChebyshevPowerTerms || len(dst) < count {
		return errChebyshevPowerLength
	}

	var coefficients [maxChebyshevPowerTerms]float64

	for order, weight := range weights {
		if math.IsNaN(weight) || math.IsInf(weight, 0) {
			return errChebyshevPowerFinite
		}

		if weight == 0 {
			continue
		}

		for power, term := range chebyshevPowerBasis[order][:order+1] {
			coefficients[power] += weight * term
		}
	}

	for _, value := range coefficients[:count] {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errChebyshevPowerFinite
		}
	}

	copy(dst, coefficients[:count])

	return nil
}

// EvaluatePowerPolynomial evaluates ascending power coefficients at x using
// Horner's method, without clamping the input or output. An empty polynomial
// evaluates to zero.
func EvaluatePowerPolynomial(coefficients []float64, x float64) float64 {
	if len(coefficients) == 0 {
		return 0
	}

	value := coefficients[len(coefficients)-1]
	for i := len(coefficients) - 2; i >= 0; i-- {
		value = value*x + coefficients[i]
	}

	return value
}

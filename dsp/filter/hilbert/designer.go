package hilbert

import (
	"fmt"
	"math"
)

const (
	// DefaultCoefficientCount is the legacy default order control.
	DefaultCoefficientCount = 8
	// DefaultTransition is the legacy normalized transition bandwidth.
	DefaultTransition = 0.1
)

// DesignCoefficients computes polyphase Hilbert allpass coefficients for the
// given number of coefficients and normalized transition bandwidth.
func DesignCoefficients(numberOfCoeffs int, transition float64) ([]float64, error) {
	if err := validateDesignParams(numberOfCoeffs, transition); err != nil {
		return nil, err
	}

	k, q := computeTransitionParam(transition)
	order := numberOfCoeffs*2 + 1

	coeffs := make([]float64, numberOfCoeffs)
	for i := range numberOfCoeffs {
		coeffs[i] = computeCoefficient(i, k, q, order)
	}

	return coeffs, nil
}

// AttenuationFromOrderTBW computes stopband attenuation in dB for the given
// coefficient count and transition bandwidth.
func AttenuationFromOrderTBW(numberOfCoeffs int, transition float64) (float64, error) {
	if err := validateDesignParams(numberOfCoeffs, transition); err != nil {
		return 0, err
	}

	_, q := computeTransitionParam(transition)
	order := numberOfCoeffs*2 + 1

	return computeAttenuation(q, order), nil
}

func validateDesignParams(numberOfCoeffs int, transition float64) error {
	if numberOfCoeffs < 1 {
		return fmt.Errorf("hilbert: number of coefficients must be >= 1: %d", numberOfCoeffs)
	}
	if !isFinite(transition) || transition <= 0 || transition >= 0.5 {
		return fmt.Errorf("hilbert: transition must be finite and in (0, 0.5): %g", transition)
	}

	return nil
}

func validateCoefficients64(coeffs []float64) error {
	if len(coeffs) < 1 {
		return fmt.Errorf("hilbert: coefficients must not be empty")
	}

	for i, c := range coeffs {
		if !isFinite(c) {
			return fmt.Errorf("hilbert: coefficient[%d] is not finite", i)
		}
		if math.Abs(c) >= 1 {
			return fmt.Errorf("hilbert: coefficient[%d] magnitude must be < 1 for stability: %g", i, c)
		}
	}

	return nil
}

func validateCoefficients32(coeffs []float32) error {
	if len(coeffs) < 1 {
		return fmt.Errorf("hilbert: coefficients must not be empty")
	}

	for i, c := range coeffs {
		if !isFinite(float64(c)) {
			return fmt.Errorf("hilbert: coefficient[%d] is not finite", i)
		}
		if math.Abs(float64(c)) >= 1 {
			return fmt.Errorf("hilbert: coefficient[%d] magnitude must be < 1 for stability: %g", i, c)
		}
	}

	return nil
}

func computeTransitionParam(transition float64) (k, q float64) {
	k = math.Pow(math.Tan((1-transition*2)*math.Pi*0.25), 2)
	kksqrt := math.Pow(1-k*k, 0.25)
	e := 0.5 * (1 - kksqrt) / (1 + kksqrt)
	e4 := e * e * e * e
	q = e * (1 + e4*(2+e4*(15+150*e4)))

	return k, q
}

func computeAttenuation(q float64, order int) float64 {
	v := 4 * math.Exp(float64(order)*0.5*math.Log(q))
	return -10 * math.Log10(v/(1+v))
}

func computeCoefficient(index int, k, q float64, order int) float64 {
	c := index + 1
	num := computeACCNum(q, order, c) * math.Pow(q, 0.25)
	den := computeACCDen(q, order, c) + 0.5
	ww := (num * num) / (den * den)

	r := math.Sqrt((1-ww*k)*(1-ww/k)) / (1 + ww)
	return (1 - r) / (1 + r)
}

func computeACCNum(q float64, order, c int) float64 {
	result := 0.0
	i := 0
	sign := 1.0
	for {
		term := math.Pow(q, float64(i*(i+1))) * (math.Sin(float64(i*2+1)*float64(c)*math.Pi/float64(order)) * sign)
		result += term
		sign = -sign
		i++
		if math.Abs(term) <= 1e-100 {
			break
		}
	}

	return result
}

func computeACCDen(q float64, order, c int) float64 {
	result := 0.0
	i := 1
	sign := -1.0
	for {
		term := math.Pow(q, float64(i*i)) * math.Cos(2*float64(i)*float64(c)*math.Pi/float64(order)) * sign
		result += term
		sign = -sign
		i++
		if math.Abs(term) <= 1e-100 {
			break
		}
	}

	return result
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

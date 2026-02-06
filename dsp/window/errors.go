package window

import (
	"errors"
	"fmt"
)

var (
	errEmptyCoeffs      = errors.New("window coefficients must not be empty")
	errZeroCoherentGain = errors.New("window coherent gain is zero")
	errMismatchedLength = errors.New("samples and coefficients must have same length")
)

func validateLength(size int) error {
	if size <= 0 {
		return fmt.Errorf("window size must be > 0: %d", size)
	}
	return nil
}

func validateKaiser(size int, beta float64) error {
	if size <= 0 {
		return validateLength(size)
	}
	if beta < 0 {
		return fmt.Errorf("kaiser beta must be >= 0: %f", beta)
	}
	return nil
}

func validateTukey(size int, alpha float64) error {
	if size <= 0 {
		return validateLength(size)
	}
	if alpha < 0 || alpha > 1 {
		return fmt.Errorf("tukey alpha must be in [0,1]: %f", alpha)
	}
	return nil
}

func validateGauss(size int, alpha float64) error {
	if size <= 0 {
		return validateLength(size)
	}
	if alpha <= 0 {
		return fmt.Errorf("gauss alpha must be > 0: %f", alpha)
	}
	return nil
}

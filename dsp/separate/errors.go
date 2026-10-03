package separate

import "errors"

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrNilOption reports a nil option passed to a constructor.
	ErrNilOption = errors.New("separate: nil option")
	// ErrInvalidKernel reports a median-filter length that is not odd and
	// positive.
	ErrInvalidKernel = errors.New("separate: invalid kernel size")
	// ErrInvalidPower reports a mask power that is not > 0 (or is NaN).
	ErrInvalidPower = errors.New("separate: invalid power")
	// ErrInvalidMargin reports an HPSS margin below 1 or not finite.
	ErrInvalidMargin = errors.New("separate: invalid margin")
	// ErrInvalidExponent reports a centre-mask exponent that is not finite
	// and > 0.
	ErrInvalidExponent = errors.New("separate: invalid exponent")
	// ErrInvalidSTFT reports STFT settings rejected by dsp/stft.
	ErrInvalidSTFT = errors.New("separate: invalid STFT settings")
	// ErrShapeMismatch reports slices or spectrograms whose lengths do not
	// match, or an empty source list.
	ErrShapeMismatch = errors.New("separate: shape mismatch")
	// ErrInvalidValue reports a NaN input to a median filter, or a
	// non-finite or negative magnitude where a magnitude is required.
	ErrInvalidValue = errors.New("separate: invalid value")
)

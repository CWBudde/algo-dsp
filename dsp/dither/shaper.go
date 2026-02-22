package dither

// NoiseShaper applies spectral shaping to quantization error via feedback filtering.
// The typical usage cycle per sample is:
//  1. shaped := shaper.Shape(scaledInput)
//  2. quantized := round(shaped + dither)
//  3. shaper.RecordError(float64(quantized) - shaped)
type NoiseShaper interface {
	// Shape applies the noise-shaping filter to the input sample using
	// previously recorded quantization errors.
	Shape(input float64) float64

	// RecordError stores the quantization error from the current sample
	// for use in subsequent Shape calls.
	RecordError(quantizationError float64)

	// Reset clears all internal state (error history, filter state).
	Reset()
}

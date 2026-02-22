package dither

// FIRShaper implements error-feedback noise shaping with FIR coefficients
// and a circular buffer for quantization error history.
type FIRShaper struct {
	coeffs  []float64
	history []float64
	pos     int
	order   int
}

// NewFIRShaper creates a new FIR noise shaper with the given coefficients.
// A nil or empty slice creates a pass-through (no shaping).
func NewFIRShaper(coeffs []float64) *FIRShaper {
	order := len(coeffs)
	c := make([]float64, order)
	copy(c, coeffs)
	var hist []float64
	if order > 0 {
		hist = make([]float64, order)
	}
	return &FIRShaper{
		coeffs:  c,
		history: hist,
		order:   order,
	}
}

// Shape applies FIR error-feedback filtering. The filter subtracts
// weighted past quantization errors from the input.
func (s *FIRShaper) Shape(input float64) float64 {
	if s.order == 0 {
		return input
	}
	// Apply FIR convolution: subtract weighted past errors.
	for i := 0; i < s.order; i++ {
		idx := (s.order + s.pos - i) % s.order
		input -= s.coeffs[i] * s.history[idx]
	}
	// Advance ring buffer position.
	s.pos = (s.pos + 1) % s.order
	return input
}

// RecordError stores the quantization error for the current sample.
// Must be called once after each Shape call.
func (s *FIRShaper) RecordError(quantizationError float64) {
	if s.order == 0 {
		return
	}
	s.history[s.pos] = quantizationError
}

// Reset clears the error history and resets the ring buffer position.
func (s *FIRShaper) Reset() {
	for i := range s.history {
		s.history[i] = 0
	}
	s.pos = 0
}

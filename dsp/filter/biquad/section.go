package biquad

// Coefficients holds the transfer function coefficients for a single
// second-order section (biquad). a0 is normalized to 1 and not stored.
//
// The sign convention follows Direct Form II Transposed:
//
//	y  = B0*x + d0
//	d0 = B1*x - A1*y + d1
//	d1 = B2*x - A2*y
type Coefficients struct {
	B0, B1, B2 float64 // feedforward (numerator)
	A1, A2     float64 // feedback (denominator)
}

// Section is a single biquad filter with coefficients and internal state.
// It implements Direct Form II Transposed processing.
type Section struct {
	Coefficients
	d0, d1 float64
}

// NewSection returns a Section initialized with the given coefficients
// and zero state.
func NewSection(c Coefficients) *Section {
	return &Section{Coefficients: c}
}

// ProcessSample filters one input sample and returns the output.
//
// This is a Direct Form II Transposed implementation, ported from
// MFFilter.pas TMFDSPBiquadIIRFilter.ProcessSample (lines 737–743).
func (s *Section) ProcessSample(x float64) float64 {
	y := s.B0*x + s.d0
	s.d0 = s.B1*x - s.A1*y + s.d1
	s.d1 = s.B2*x - s.A2*y
	return y
}

// ProcessBlock filters a block of samples in-place. Zero-alloc.
func (s *Section) ProcessBlock(buf []float64) {
	for i, x := range buf {
		y := s.B0*x + s.d0
		s.d0 = s.B1*x - s.A1*y + s.d1
		s.d1 = s.B2*x - s.A2*y
		buf[i] = y
	}
}

// ProcessBlockTo filters src into dst. Both slices must have the same length.
// Zero-alloc.
func (s *Section) ProcessBlockTo(dst, src []float64) {
	_ = dst[len(src)-1] // bounds check hint
	for i, x := range src {
		y := s.B0*x + s.d0
		s.d0 = s.B1*x - s.A1*y + s.d1
		s.d1 = s.B2*x - s.A2*y
		dst[i] = y
	}
}

// Reset clears the delay line to zero.
func (s *Section) Reset() {
	s.d0 = 0
	s.d1 = 0
}

// State returns the current delay-line state [d0, d1].
func (s *Section) State() [2]float64 {
	return [2]float64{s.d0, s.d1}
}

// SetState restores a previously saved delay-line state.
func (s *Section) SetState(state [2]float64) {
	s.d0 = state[0]
	s.d1 = state[1]
}

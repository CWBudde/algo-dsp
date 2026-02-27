//nolint:funcorder
package biquad

import (
	"sync"

	archregistry "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"
	"github.com/cwbudde/algo-vecmath/cpu"
)

// Coefficients holds the transfer function coefficients for a single
// second-order section (biquad). a0 is normalized to 1 and not stored.
//
// The sign convention follows Direct Form II Transposed:
//
//	y  = B0*x + d0
//	= B1*x - A1*y + d1
//	= B2*x - A2*y
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

var (
	processBlockImpl     archregistry.ProcessBlockFn
	processBlockInitOnce sync.Once
)

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
	processBlockInitOnce.Do(initProcessBlockKernel)

	coeffs := archregistry.Coefficients{
		B0: s.B0,
		B1: s.B1,
		B2: s.B2,
		A1: s.A1,
		A2: s.A2,
	}

	s.d0, s.d1 = processBlockImpl(coeffs, s.d0, s.d1, buf)
}

func initProcessBlockKernel() {
	entry := archregistry.Global.Lookup(cpu.DetectFeatures())
	if entry == nil {
		panic("biquad: no ProcessBlock kernel registered (missing generic fallback?)")
	}

	if entry.ProcessBlock == nil {
		panic("biquad: selected kernel missing ProcessBlock")
	}

	processBlockImpl = entry.ProcessBlock
}

func (s *Section) processBlockScalar(buf []float64) {
	for i, x := range buf {
		y := s.B0*x + s.d0
		s.d0 = s.B1*x - s.A1*y + s.d1
		s.d1 = s.B2*x - s.A2*y
		buf[i] = y
	}
}

// processBlockUnrolled2 is a manual 2x-unrolled scalar implementation of
// ProcessBlock that reduces loop overhead and improves ILP.
func (s *Section) processBlockUnrolled2(buf []float64) {
	b0, b1, b2 := s.B0, s.B1, s.B2
	a1, a2 := s.A1, s.A2
	d0, d1 := s.d0, s.d1

	i := 0

	n := len(buf)
	for ; i+1 < n; i += 2 {
		x0 := buf[i]
		y0 := b0*x0 + d0
		d0n := b1*x0 - a1*y0 + d1
		d1n := b2*x0 - a2*y0

		x1 := buf[i+1]
		y1 := b0*x1 + d0n
		d0 = b1*x1 - a1*y1 + d1n
		d1 = b2*x1 - a2*y1

		buf[i] = y0
		buf[i+1] = y1
	}

	if i < n {
		x := buf[i]
		y := b0*x + d0
		d0 = b1*x - a1*y + d1
		d1 = b2*x - a2*y
		buf[i] = y
	}

	s.d0, s.d1 = d0, d1
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

package biquad

import (
	"math"
	"math/cmplx"
)

// Response computes the complex frequency response H(e^jw) of a biquad
// at the given frequency (Hz) and sample rate (Hz).
func (c *Coefficients) Response(freqHz, sampleRate float64) complex128 {
	w := 2 * math.Pi * freqHz / sampleRate
	ejw := cmplx.Exp(complex(0, -w))
	ej2w := cmplx.Exp(complex(0, -2*w))

	num := complex(c.B0, 0) + complex(c.B1, 0)*ejw + complex(c.B2, 0)*ej2w
	den := complex(1, 0) + complex(c.A1, 0)*ejw + complex(c.A2, 0)*ej2w
	return num / den
}

// MagnitudeSquared returns |H(f)|^2 using a closed-form expression.
//
// This avoids computing complex exponentials and matches the legacy
// MFFilter.pas TMFDSPBiquadIIRFilter.MagnitudeSquared (lines 702–708).
func (c *Coefficients) MagnitudeSquared(freqHz, sampleRate float64) float64 {
	cw := 2 * math.Cos(2*math.Pi*freqHz/sampleRate)
	b0, b1, b2 := c.B0, c.B1, c.B2
	a1, a2 := c.A1, c.A2

	num := (b0-b2)*(b0-b2) + b1*b1 + (b1*(b0+b2)+b0*b2*cw)*cw
	den := (1-a2)*(1-a2) + a1*a1 + (a1*(a2+1)+cw*a2)*cw
	return num / den
}

// MagnitudeDB returns 10*log10(|H(f)|^2).
//
// Matches legacy MFFilter.pas TMFDSPBiquadIIRFilter.MagnitudeLog10 (lines 694–699).
func (c *Coefficients) MagnitudeDB(freqHz, sampleRate float64) float64 {
	return 10 * math.Log10(c.MagnitudeSquared(freqHz, sampleRate))
}

// Phase returns the phase response in radians at the given frequency.
// The result is in [-pi, pi], consistent with the standard DSP convention
// H(e^{-jw}).
func (c *Coefficients) Phase(freqHz, sampleRate float64) float64 {
	return cmplx.Phase(c.Response(freqHz, sampleRate))
}

// Response computes the complex frequency response of the full cascade
// as the product of individual section responses.
func (c *Chain) Response(freqHz, sampleRate float64) complex128 {
	h := complex(c.gain, 0)
	for i := range c.sections {
		h *= c.sections[i].Response(freqHz, sampleRate)
	}
	return h
}

// MagnitudeDB returns the cascaded magnitude response in dB.
func (c *Chain) MagnitudeDB(freqHz, sampleRate float64) float64 {
	h := c.Response(freqHz, sampleRate)
	return 20 * math.Log10(cmplx.Abs(h))
}

// ImpulseResponse computes n samples of the impulse response h[n]
// by feeding an impulse through the section. The filter state is
// saved and restored so this method does not modify the section.
//
// Ported from MFFilter.pas TMFDSPBiquadIIRFilter.GetIR (lines 620–639).
func (s *Section) ImpulseResponse(n int) []float64 {
	if n <= 0 {
		return nil
	}
	saved := s.State()
	s.Reset()
	ir := make([]float64, n)
	ir[0] = s.ProcessSample(1)
	for i := 1; i < n; i++ {
		ir[i] = s.ProcessSample(0)
	}
	s.SetState(saved)
	return ir
}

// ImpulseResponse computes n samples of the cascade impulse response.
// The chain state is saved and restored.
func (c *Chain) ImpulseResponse(n int) []float64 {
	if n <= 0 {
		return nil
	}
	saved := c.State()
	c.Reset()
	ir := make([]float64, n)
	ir[0] = c.ProcessSample(1)
	for i := 1; i < n; i++ {
		ir[i] = c.ProcessSample(0)
	}
	c.SetState(saved)
	return ir
}

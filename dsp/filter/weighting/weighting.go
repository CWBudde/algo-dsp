package weighting

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// IEC 61672 analog prototype pole frequencies (Hz).
const (
	f1 = 20.598997 // double pole for A, B, C
	f2 = 107.65265 // single pole for A, B
	f3 = 158.48932 // single pole for B only
	f4 = 737.86223 // single pole for A only
	f5 = 12194.217 // double pole for A, B, C
)

// Type identifies a frequency weighting curve.
type Type int

const (
	// TypeA is the A-weighting curve per IEC 61672.
	// It approximates the 40-phon equal-loudness contour and is the most
	// widely used weighting for noise measurements.
	TypeA Type = iota

	// TypeB is the B-weighting curve per IEC 61672.
	// It approximates the 70-phon equal-loudness contour and is rarely
	// used in modern practice.
	TypeB

	// TypeC is the C-weighting curve per IEC 61672.
	// It approximates the 100-phon equal-loudness contour and is used
	// for peak measurements and C-A difference calculations.
	TypeC

	// TypeZ is the Z-weighting (zero-weighting) per IEC 61672.
	// It applies no frequency weighting (unity gain at all frequencies).
	TypeZ
)

// String returns a human-readable name for the weighting type.
func (t Type) String() string {
	switch t {
	case TypeA:
		return "A"
	case TypeB:
		return "B"
	case TypeC:
		return "C"
	case TypeZ:
		return "Z"
	default:
		return "Unknown"
	}
}

// New returns a [biquad.Chain] configured for the given weighting curve
// at the specified sample rate. The chain is normalized so that the
// magnitude response at 1 kHz is 0 dB.
//
// Panics if sampleRate <= 0.
func New(t Type, sampleRate float64) *biquad.Chain {
	if sampleRate <= 0 {
		panic("weighting: sample rate must be positive")
	}

	switch t {
	case TypeA:
		return newAWeighting(sampleRate)
	case TypeB:
		return newBWeighting(sampleRate)
	case TypeC:
		return newCWeighting(sampleRate)
	case TypeZ:
		return newZWeighting()
	default:
		panic("weighting: unknown type")
	}
}

// newAWeighting builds an A-weighting filter (6th order).
//
// The analog prototype is:
//
//	H_A(s) = K_A * s^4 / ((s+ω1)^2 * (s+ω2) * (s+ω4) * (s+ω5)^2)
//
// where ω_i = 2*π*f_i. The s^4 numerator yields 4 zeros at DC, distributed:
//   - 2nd-order HP at f1 (2 zeros at DC, 2 poles)
//   - 1st-order HP at f2 (1 zero at DC, 1 pole)
//   - 1st-order HP at f4 (1 zero at DC, 1 pole)
//   - 1st-order LP at f5 (0 zeros, 1 pole) x2
func newAWeighting(sr float64) *biquad.Chain {
	coeffs := []biquad.Coefficients{
		hpSecondOrder(f1, sr),
		lpFirstOrder(sr),
		lpFirstOrder(sr),
		hpFirstOrder(f2, sr),
		hpFirstOrder(f4, sr),
	}
	gain := normalizationGain(coeffs, sr)

	return biquad.NewChain(coeffs, biquad.WithGain(gain))
}

// newBWeighting builds a B-weighting filter (5th order).
//
// The analog prototype is:
//
//	H_B(s) = K_B * s^3 / ((s+ω1)^2 * (s+ω3) * (s+ω5)^2)
//
// Sections:
//   - 2nd-order HP at f1 (2 zeros at DC, 2 poles)
//   - 1st-order HP at f3 (1 zero at DC, 1 pole)
//   - 1st-order LP at f5 (0 zeros, 1 pole) x2
func newBWeighting(sr float64) *biquad.Chain {
	coeffs := []biquad.Coefficients{
		hpSecondOrder(f1, sr),
		lpFirstOrder(sr),
		lpFirstOrder(sr),
		hpFirstOrder(f3, sr),
	}
	gain := normalizationGain(coeffs, sr)

	return biquad.NewChain(coeffs, biquad.WithGain(gain))
}

// newCWeighting builds a C-weighting filter (4th order).
//
// The analog prototype is:
//
//	H_C(s) = K_C * s^2 / ((s+ω1)^2 * (s+ω5)^2)
//
// Sections:
//   - 2nd-order HP at f1 (2 zeros at DC, 2 poles)
//   - 1st-order LP at f5 (0 zeros, 1 pole) x2
func newCWeighting(sr float64) *biquad.Chain {
	coeffs := []biquad.Coefficients{
		hpSecondOrder(f1, sr),
		lpFirstOrder(sr),
		lpFirstOrder(sr),
	}
	gain := normalizationGain(coeffs, sr)

	return biquad.NewChain(coeffs, biquad.WithGain(gain))
}

// newZWeighting builds a Z-weighting filter (unity gain).
func newZWeighting() *biquad.Chain {
	return biquad.NewChain([]biquad.Coefficients{
		{B0: 1},
	})
}

// lpFirstOrder computes a 1st-order low-pass biquad section for the
// fixed weighting pole at f5 using the bilinear transform.
//
// The analog prototype is H(s) = omega / (s + omega).
// Using K = tan(pi*f/sr):
//
//	B0 = K/(1+K), B1 = K/(1+K), B2 = 0
//	A1 = (K-1)/(K+1), A2 = 0
func lpFirstOrder(sr float64) biquad.Coefficients {
	k := math.Tan(math.Pi * f5 / sr)
	d := 1 + k

	return biquad.Coefficients{
		B0: k / d,
		B1: k / d,
		A1: (k - 1) / d,
	}
}

// hpSecondOrder computes a 2nd-order high-pass biquad section for a
// double pole at frequency f using the bilinear transform.
//
// The analog prototype is H(s) = s^2 / (s + omega)^2 where omega = 2*pi*f.
// Using K = tan(pi*f/sr) as the frequency-warped variable:
//
//	denom = 1 + 2*K + K^2
//	B0 = 1/denom, B1 = -2/denom, B2 = 1/denom
//	A1 = 2*(K^2 - 1)/denom, A2 = (1 - 2*K + K^2)/denom
func hpSecondOrder(f, sr float64) biquad.Coefficients {
	k := math.Tan(math.Pi * f / sr)
	k2 := k * k
	d := 1 + 2*k + k2

	return biquad.Coefficients{
		B0: 1 / d,
		B1: -2 / d,
		B2: 1 / d,
		A1: 2 * (k2 - 1) / d,
		A2: (1 - 2*k + k2) / d,
	}
}

// hpFirstOrder computes a 1st-order high-pass biquad section for a
// single pole at frequency f using the bilinear transform.
//
// The analog prototype is H(s) = s / (s + omega).
// Using K = tan(pi*f/sr):
//
//	B0 = 1/(1+K), B1 = -1/(1+K), B2 = 0
//	A1 = (K-1)/(K+1), A2 = 0
func hpFirstOrder(f, sr float64) biquad.Coefficients {
	k := math.Tan(math.Pi * f / sr)
	d := 1 + k

	return biquad.Coefficients{
		B0: 1 / d,
		B1: -1 / d,
		A1: (k - 1) / d,
	}
}

// normalizationGain computes the gain factor needed to make the cascade
// magnitude equal to 1 (0 dB) at 1 kHz.
func normalizationGain(coeffs []biquad.Coefficients, sr float64) float64 {
	h := complex(1, 0)
	for i := range coeffs {
		h *= coeffs[i].Response(1000, sr)
	}

	return 1 / cmplx.Abs(h)
}

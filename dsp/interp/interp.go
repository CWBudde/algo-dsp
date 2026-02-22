package interp

import "math"

// Mode selects the interpolation algorithm used by delay lines
// and other fractional-sample DSP blocks.
type Mode int

const (
	// Linear performs 2-point linear interpolation.
	// Cheapest but introduces high-frequency roll-off and audible
	// artifacts on pitched or modulated delay lines.
	Linear Mode = iota

	// Hermite performs 4-point cubic Hermite interpolation.
	// Good default: smooth, no overshoot on monotone data, cheap.
	Hermite

	// Lagrange3 performs 4-point third-order Lagrange interpolation.
	// Similar quality to Hermite with slightly different spectral
	// characteristics.
	Lagrange3

	// Lanczos3 performs 6-point windowed-sinc interpolation using a
	// Lanczos kernel with a = 3.
	// Better frequency-domain accuracy than cubic methods at moderate cost.
	Lanczos3

	// Sinc performs windowed-sinc interpolation with a configurable
	// number of zero crossings. Use SincN to set the half-width;
	// the default is 8 (16-tap kernel).
	Sinc

	// Allpass performs first-order allpass interpolation.
	// Unity magnitude response; introduces frequency-dependent phase
	// shift. Very cheap and well-suited for feedback delay lines.
	Allpass
)

// --- standalone interpolation functions ---

// Linear2 performs linear interpolation between x0 and x1.
// t is the fractional position in [0, 1].
func Linear2(t, x0, x1 float64) float64 {
	return x0 + t*(x1-x0)
}

// Hermite4 computes cubic 4-point Hermite interpolation.
// It interpolates from x0 toward x1 using neighbor points xm1 and x2.
// t is the fractional position in [0, 1].
func Hermite4(t, xm1, x0, x1, x2 float64) float64 {
	c0 := x0
	c1 := 0.5 * (x1 - xm1)
	c2 := xm1 - 2.5*x0 + 2*x1 - 0.5*x2
	c3 := 0.5*(x2-xm1) + 1.5*(x0-x1)

	return ((c3*t+c2)*t+c1)*t + c0
}

// Lagrange4 computes 4-point third-order Lagrange interpolation.
// Points xm1, x0, x1, x2 are equally spaced; t is the fractional
// position in [0, 1] between x0 and x1.
func Lagrange4(t, xm1, x0, x1, x2 float64) float64 {
	// Lagrange basis polynomials evaluated at t (shifted so nodes are -1,0,1,2).
	d0 := t
	d1 := t - 1
	d2 := t + 1
	l0 := -d0 * d1 * (t - 2) / 6.0 // basis for xm1 at node -1
	l1 := d2 * d1 * (t - 2) / 2.0  // basis for x0 at node 0
	l2 := -d2 * d0 * (t - 2) / 2.0 // basis for x1 at node 1
	l3 := d2 * d0 * d1 / 6.0       // basis for x2 at node 2

	return l0*xm1 + l1*x0 + l2*x1 + l3*x2
}

// sincNormalized returns sin(pi*x)/(pi*x), with sinc(0) = 1.
func sincNormalized(x float64) float64 {
	if x == 0 {
		return 1
	}

	px := math.Pi * x

	return math.Sin(px) / px
}

// lanczosWindow returns the Lanczos window for kernel half-width a.
func lanczosWindow(x float64, a int) float64 {
	fa := float64(a)
	if x <= -fa || x >= fa {
		return 0
	}

	return sincNormalized(x / fa)
}

// LanczosN performs windowed-sinc interpolation using a Lanczos kernel
// of half-width a. samples must contain 2*a values centered on the
// interpolation point: samples[a-1] and samples[a] bracket the
// fractional position t in [0, 1].
//
// For the common case a = 3 (6-tap), pass 6 samples where
// samples[2]..samples[3] is the integer bracket.
func LanczosN(t float64, samples []float64, a int) float64 {
	var sum, wsum float64

	for i := 0; i < 2*a; i++ {
		// distance from sample i to the fractional point
		d := float64(i-(a-1)) - t
		w := sincNormalized(d) * lanczosWindow(d, a)
		sum += w * samples[i]
		wsum += w
	}

	if wsum == 0 {
		return 0
	}

	return sum / wsum
}

// Lanczos6 is a convenience wrapper for LanczosN with a = 3 (6 samples).
// samples must have length >= 6; samples[2]..samples[3] bracket t.
func Lanczos6(t float64, samples []float64) float64 {
	return LanczosN(t, samples, 3)
}

// SincInterp performs windowed-sinc interpolation with half-width n
// (2*n taps total). The window used is a Blackman window for good
// stop-band attenuation.
//
// samples must contain 2*n values; samples[n-1]..samples[n] bracket
// fractional position t in [0, 1].
func SincInterp(t float64, samples []float64, n int) float64 {
	taps := 2 * n

	var sum, wsum float64

	for i := 0; i < taps; i++ {
		d := float64(i-(n-1)) - t
		// Blackman window over the kernel span.
		wpos := float64(i) + (1 - t)
		wn := wpos / float64(taps)
		bw := 0.42 - 0.5*math.Cos(2*math.Pi*wn) + 0.08*math.Cos(4*math.Pi*wn)
		w := sincNormalized(d) * bw
		sum += w * samples[i]
		wsum += w
	}

	if wsum == 0 {
		return 0
	}

	return sum / wsum
}

// AllpassCoeff computes the optimal first-order allpass interpolation
// coefficient for fractional delay t in [0, 1]:
//
//	eta = (1 - t) / (1 + t)
//
// This is the Thiran first-order approximation.
func AllpassCoeff(t float64) float64 {
	return (1 - t) / (1 + t)
}

// AllpassTick performs one sample of first-order allpass interpolation.
// x0, x1 are the two bracketing samples; state is the one-sample filter
// state (pass a pointer so it persists across calls). t is the fractional
// delay in [0, 1].
func AllpassTick(t, x0, x1 float64, state *float64) float64 {
	eta := AllpassCoeff(t)
	out := x1 + eta*(x0-*state)
	*state = out

	return out
}

// --- LagrangeInterpolator (legacy convenience wrapper) ---

// LagrangeInterpolator provides configurable fractional interpolation.
type LagrangeInterpolator struct {
	order int
}

// NewLagrangeInterpolator creates an interpolator.
// order: 1 = linear, 3 = cubic (Hermite-style 4-point interpolation).
func NewLagrangeInterpolator(order int) *LagrangeInterpolator {
	return &LagrangeInterpolator{order: order}
}

// Interpolate interpolates around frac in [0,1].
// For order 1, samples must contain at least 2 values.
// For order 3, samples must contain at least 4 values and interpolates between samples[1] and samples[2].
func (l *LagrangeInterpolator) Interpolate(samples []float64, frac float64) float64 {
	if len(samples) == 0 {
		return 0
	}

	if l.order == 1 {
		if len(samples) < 2 {
			return samples[0]
		}

		return samples[0] + frac*(samples[1]-samples[0])
	}

	if l.order == 3 {
		if len(samples) < 4 {
			if len(samples) < 2 {
				return samples[0]
			}

			return samples[0] + frac*(samples[1]-samples[0])
		}

		return Hermite4(frac, samples[0], samples[1], samples[2], samples[3])
	}

	if len(samples) < 2 {
		return samples[0]
	}

	return samples[0] + frac*(samples[1]-samples[0])
}

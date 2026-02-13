package interp

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

// Hermite4 computes cubic 4-point interpolation.
// It interpolates from x0 to x1 using neighbor points xm1 and x2.
func Hermite4(t, xm1, x0, x1, x2 float64) float64 {
	c0 := x0
	c1 := 0.5 * (x1 - xm1)
	c2 := xm1 - 2.5*x0 + 2*x1 - 0.5*x2
	c3 := 0.5*(x2-xm1) + 1.5*(x0-x1)
	return ((c3*t+c2)*t+c1)*t + c0
}

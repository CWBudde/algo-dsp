package dynamics

import (
	"fmt"
	"math"
)

// DAVLevelingGain is the transfer curve of the Delphi ASIO & VST leveling
// amplifier. The caller supplies a nonnegative detector peak and owns the
// envelope, wet mix and gain staging.
type DAVLevelingGain struct {
	threshold   float64
	kneeSquared float64
	exponent    float64
}

// NewDAVLevelingGain prepares a normalized knee in [0, 1] and an input/output
// ratio of at least one.
func NewDAVLevelingGain(knee, ratio float64) (DAVLevelingGain, error) {
	if !(knee >= 0 && knee <= 1 && ratio >= 1) || math.IsInf(ratio, 1) {
		return DAVLevelingGain{}, fmt.Errorf("invalid DAV leveling gain: knee %g, ratio %g", knee, ratio)
	}
	corner := 1 - knee
	return DAVLevelingGain{
		threshold: corner * corner, kneeSquared: knee * knee, exponent: 1/ratio - 1,
	}, nil
}

// Gain evaluates the release curve with its original binary32 polynomial and
// rounding boundaries.
func (g DAVLevelingGain) Gain(peak float64) float64 {
	d := g.overThreshold(peak)
	tanh := float64(float32(davLevelingTanh(d)))
	adjusted := float32(d - g.kneeSquared*tanh)
	logPeak := davLog2(float32(1 + float64(adjusted)))
	return float64(davPower2(float32(float64(logPeak) * g.exponent)))
}

// LegacyGain evaluates the earlier release's curve with double precision
// after its original binary32 detector boundary and an exact power function.
func (g DAVLevelingGain) LegacyGain(peak float64) float64 {
	d := g.overThreshold(peak)
	adjusted := d - g.kneeSquared*davLevelingTanh(d)
	return math.Pow(1+adjusted, g.exponent)
}

func (g DAVLevelingGain) overThreshold(peak float64) float64 {
	if !(peak > g.threshold) {
		return 0
	}
	return float64(float32(min(peak-g.threshold, math.MaxFloat32)))
}

func davLevelingTanh(x float64) float64 {
	polynomial := 1.26175667589988239 + x*(-0.54699348440059470+x*2.66559097474027817)
	return polynomial * x / (1 + polynomial*x)
}

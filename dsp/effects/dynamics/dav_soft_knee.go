package dynamics

import (
	"fmt"
	"math"
)

// DAVSoftKneeGain is the float32 transfer curve used by the Delphi ASIO & VST
// Lightweight Soft Knee Limiter. It maps a positive linear peak estimate to
// attenuation. Detection, envelope timing and makeup gain belong to the caller.
type DAVSoftKneeGain struct {
	thresholdLog2 float32
	kneeSquared   float32
}

// NewDAVSoftKneeGain prepares the curve for a threshold and a nonnegative knee
// in dB. A zero knee selects the hard transition.
func NewDAVSoftKneeGain(thresholdDB, kneeDB float64) (DAVSoftKneeGain, error) {
	const dbToLog2 = float64(float32(1.6609640474436811739351597147447e-1))

	threshold := float32(float64(float32(thresholdDB)) * dbToLog2)
	knee := 2 * float64(float32(kneeDB)) * dbToLog2

	squared := float32(knee * knee)
	if !(kneeDB >= 0) || math.IsInf(float64(squared), 0) ||
		math.IsNaN(float64(threshold)) || math.IsInf(float64(threshold), 0) {
		return DAVSoftKneeGain{}, fmt.Errorf("invalid DAV soft knee: threshold %g dB, knee %g dB", thresholdDB, kneeDB)
	}

	return DAVSoftKneeGain{thresholdLog2: threshold, kneeSquared: squared}, nil
}

// Gain returns the linear attenuation for peak. A nonpositive peak returns
// unity and positive infinity returns zero. The compatibility curve rounds
// the intermediate values at the same float32 boundaries as the reference.
func (c DAVSoftKneeGain) Gain(peak float64) float64 {
	if !(peak > 0) {
		return 1
	}

	if math.IsInf(peak, 1) {
		return 0
	}

	logPeak := davLog2(float32(min(max(peak, 0x1p-126), math.MaxFloat32)))
	delta := float64(c.thresholdLog2 - logPeak)
	squared := delta*delta + float64(c.kneeSquared)

	root := davSqrt(float32(squared))
	if math.IsInf(float64(root), 1) {
		// Keep the normal float32 path and handle extremely large settings
		// with finite float64 intermediates.
		return math.Exp2(.5 * (delta - math.Sqrt(squared)))
	}

	return float64(davPower2(float32(.5 * (delta - float64(root)))))
}

// FeedbackLimiterGainAndTiming returns the attenuation and the next detector
// coefficients of the DAV Lightweight Feedback Limiter. Each cycle is
// -1/(timeMS * 0.001 * sampleRate), rounded to float32. Apply the returned
// attack interpolation and release pole to the next input peak update.
// Invalid peaks, cycles or degenerate timing leave the detector frozen
// with attack 0 and release 1.
func (c DAVSoftKneeGain) FeedbackLimiterGainAndTiming(peak float64, attackCycle, releaseCycle float32) (gain, attack, release float64) {
	if !(peak > 0) || math.IsInf(peak, 1) {
		return c.Gain(peak), 0, 1
	}

	logPeak := davLog2(float32(min(max(peak, 0x1p-126), math.MaxFloat32)))
	delta := .5 * float64(c.thresholdLog2-logPeak)
	squared := delta*delta + .25*float64(c.kneeSquared)

	root := float64(davSqrt(float32(squared)))
	if math.IsInf(root, 1) {
		return c.Gain(peak), 0, 1
	}

	gain = float64(davPower2(float32(delta - root)))

	const limiterRatio = 1e10

	denominator := (limiterRatio+1)*root + (limiterRatio-1)*delta
	if !(denominator > 0 && attackCycle <= 0 && releaseCycle <= 0) ||
		math.IsInf(float64(attackCycle), -1) || math.IsInf(float64(releaseCycle), -1) {
		return gain, 0, 1
	}

	// The release rounds the time scale and both powers to float32.
	timeScale := float32(2 * root / denominator)
	attack = 1 - float64(davPower2(timeScale*attackCycle))
	release = float64(davPower2(timeScale * releaseCycle))

	return gain, attack, release
}

func davLog2(value float32) float32 {
	return float32(davLog2Extended(value))
}

func davLog2Extended(value float32) float64 {
	const (
		p4 = float64(float32(-8.21343513178931783e-2))
		p3 = float64(float32(6.49732456739820052e-1))
		p2 = float64(float32(-2.13417801862571777))
		p1 = float64(float32(4.08642207062728868))
		p0 = float64(float32(-1.51984215742349793))
	)

	bits := math.Float32bits(value)
	mantissa := float64(math.Float32frombits(bits&0x007fffff | 0x3f800000))
	exponent := int(bits>>23) - 128

	return float64(exponent) + ((((p4*mantissa+p3)*mantissa+p2)*mantissa+p1)*mantissa + p0)
}

func davSqrt(value float32) float32 {
	if value < 0x1p-126 || math.IsInf(float64(value), 1) {
		return float32(math.Sqrt(float64(value)))
	}

	seed := davSqrtSeed(value)
	squared := float64(value)
	step := float64(float32(seed + squared/seed))

	return float32(.25*step + squared/step)
}

func davSqrtSeed(value float32) float64 {
	bits := (math.Float32bits(value)-0x00800000)>>1 + 0x20000000

	return float64(math.Float32frombits(bits))
}

func davPower2(value float32) float32 {
	return float32(davPower2Extended(float64(value)))
}

func davPower2Extended(value float64) float64 {
	if value < -1075 {
		return 0
	}

	if value > 1024 {
		return math.Inf(1)
	}

	exponent := math.RoundToEven(value)
	residual := value - exponent

	return math.Ldexp(davPower2Residual(residual), int(exponent))
}

func davPower2Residual(x float64) float64 {
	const (
		p1 = float64(float32(6.93292707161004662e-1))
		p2 = float64(float32(2.42162975514835621e-1))
		p3 = float64(float32(5.48668824216034384e-2))
	)

	return 1 + x*(p1+x*(p2+x*p3))
}

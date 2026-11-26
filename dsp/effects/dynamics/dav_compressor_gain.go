package dynamics

import (
	"fmt"
	"math"
)

// DAVSoftKneeCompressorGain reproduces the x87 transfer curve used by the
// Delphi ASIO & VST compressor. Settings and the detector peak enter the
// curve as binary32 values. The polynomial and gain calculation use wider
// intermediates.
type DAVSoftKneeCompressorGain struct {
	thresholdLog2 float32
	kneeSquared   float32
	slope         float32
	ratio         float32
}

// NewDAVSoftKneeCompressorGain prepares a threshold and nonnegative knee in
// dB and a positive input/output ratio. Ratios below one attenuate peaks below
// the threshold.
func NewDAVSoftKneeCompressorGain(thresholdDB, kneeDB, ratio float64) (DAVSoftKneeCompressorGain, error) {
	threshold, squared, valid := davCompressorCurveInputs(thresholdDB, kneeDB)

	slope := float32(.5 * (1/float64(float32(ratio)) - 1))
	if !(ratio > 0 && ratio <= math.MaxFloat32) || !valid || math.IsInf(float64(slope), 0) {
		return DAVSoftKneeCompressorGain{}, fmt.Errorf("invalid DAV compressor curve: threshold %g dB, knee %g dB, ratio %g",
			thresholdDB, kneeDB, ratio)
	}

	return DAVSoftKneeCompressorGain{
		thresholdLog2: threshold, kneeSquared: squared, slope: slope, ratio: float32(ratio),
	}, nil
}

// Gain evaluates the transfer curve for a nonnegative linear peak. Zero and
// infinity use the corresponding limiting gains. Results below the floating
// point range underflow to zero.
func (c DAVSoftKneeCompressorGain) Gain(peak float64) float64 {
	if c.slope != 0 {
		switch {
		case !(peak > 0):
			if c.slope > 0 {
				return 0
			}

			return 1
		case math.IsInf(peak, 1):
			if c.slope > 0 {
				return 1
			}

			return 0
		}
	}

	delta, root := c.curveTerms(peak)

	return davPower2Extended(delta - root)
}

// SpectralGateGain evaluates the binary32 gain used to build a DAV spectral
// gate's filter. The original exponent-word packing wraps below the binary32
// exponent range, which can yield signed finite coefficients. The filter
// discards gains outside [-1, 1]. Ordinary compressor callers should use Gain.
func (c DAVSoftKneeCompressorGain) SpectralGateGain(peak float64) float32 {
	if !(c.ratio > 0 && peak > 0 && peak <= math.MaxFloat64) {
		return 0
	}

	delta, root := c.curveTerms(peak)
	exponent := delta - root

	whole := math.RoundToEven(exponent)
	if !(whole >= math.MinInt32 && whole <= math.MaxInt32) {
		return 0
	}

	packed := math.Float32frombits(uint32(int64(whole)+127) << 23)

	gain := float32(float64(packed) * davPower2Residual(exponent-whole))
	if gain >= -1 && gain <= 1 {
		return gain
	}

	return 0
}

func (c DAVSoftKneeCompressorGain) curveTerms(peak float64) (delta, root float64) {
	if c.slope != 0 {
		logPeak := davLog2Extended(float32(min(max(peak, 0x1p-126), math.MaxFloat32)))
		delta = float64(c.slope) * (logPeak - float64(c.thresholdLog2))
	}

	return delta, davCompressorRoot(delta*delta + float64(c.kneeSquared))
}

// FeedbackLikeGainAndTiming returns the attenuation, attack interpolation,
// and release pole of the DAV Smooth Multiband Compressor's Smooth detector.
// Each cycle is -1/(timeMS * 0.001 * sampleRate) rounded to float32. Use the
// returned coefficients to update the next detector sample. An invalid peak,
// cycle, or uninitialized curve leaves the detector frozen.
func (c DAVSoftKneeCompressorGain) FeedbackLikeGainAndTiming(peak float64, attackCycle, releaseCycle float32) (gain, attack, release float64) {
	if !(peak > 0) || math.IsInf(peak, 1) || !(c.ratio > 0) {
		return c.Gain(peak), 0, 1
	}

	delta, root := c.curveTerms(peak)

	gain = davPower2Extended(delta - root)
	if !(attackCycle <= 0 && releaseCycle <= 0) ||
		math.IsInf(float64(attackCycle), -1) || math.IsInf(float64(releaseCycle), -1) {
		return gain, 0, 1
	}

	ratio := float64(c.ratio)
	// At a zero-width knee exactly on the corner, use the center slope.
	scale := 2 / (ratio + 1)
	if root != 0 {
		// Both groupings equal (ratio+1)*root + (ratio-1)*delta. The direct
		// form can lose about log2(ratio) or log2(1/ratio) bits to
		// cancellation, so extreme ratios use nonnegative terms instead.
		denominator := (ratio+1)*root + (ratio-1)*delta
		if !(ratio > 0x1p-26 && ratio < 0x1p26) {
			denominator = ratio*max(root+delta, 0) + max(root-delta, 0)
		}

		if !(denominator > 0) {
			return gain, 0, 1
		}

		scale = 2 * root / denominator
	}

	return gain, 1 - davPower2Extended(scale*float64(attackCycle)),
		davPower2Extended(scale * float64(releaseCycle))
}

func davCompressorCurveInputs(thresholdDB, kneeDB float64) (threshold, kneeSquared float32, valid bool) {
	const dbToLog2 = float64(float32(1.6609640474436811739351597147447e-1))

	threshold = float32(float64(float32(thresholdDB)) * dbToLog2)
	knee := float64(float32(kneeDB)) * dbToLog2
	kneeSquared = float32(knee * knee)
	valid = kneeDB >= 0 && !math.IsInf(float64(kneeSquared), 0) &&
		!math.IsNaN(float64(threshold)) && !math.IsInf(float64(threshold), 0)

	return threshold, kneeSquared, valid
}

func davCompressorRoot(value float64) float64 {
	rounded := float32(value)
	if rounded < 0x1p-126 || math.IsInf(float64(rounded), 1) {
		return math.Sqrt(value)
	}

	seed := davSqrtSeed(rounded)
	step := seed + value/seed

	return .25*step + value/step
}

// DAVCompressorTiming returns the attack interpolation coefficient and the
// release pole for the compressor's detector loop. Times are in milliseconds
// and sampleRate is the effective detector rate, including oversampling.
func DAVCompressorTiming(attackMS, releaseMS, sampleRate float64) (attack, release float64, err error) {
	return davCompressorTiming(attackMS, releaseMS, sampleRate, davPower2)
}

// DAVSpectralGateTiming returns the attack interpolation and release pole used
// by the legacy DAV spectral gate. Supply the detector rate in FFT hops per
// second and times in milliseconds. The original binary32 exponent packing
// can produce signed poles outside [0, 1] at short times and low detector rates.
// Nonfinite packed poles use the ordinary finite exponential result.
func DAVSpectralGateTiming(attackMS, releaseMS, sampleRate float64) (attack, release float64, err error) {
	return davCompressorTiming(attackMS, releaseMS, sampleRate, davSpectralGatePole)
}

func davCompressorTiming(attackMS, releaseMS, sampleRate float64, power2 func(float32) float32) (attack, release float64, err error) {
	if !(attackMS > 0 && attackMS <= math.MaxFloat32 &&
		releaseMS > 0 && releaseMS <= math.MaxFloat32 &&
		sampleRate > 0 && sampleRate <= math.MaxFloat32) {
		return 0, 0, fmt.Errorf("invalid DAV compressor timing: attack %g ms, release %g ms, rate %g Hz",
			attackMS, releaseMS, sampleRate)
	}

	pole := func(ms float64) float64 {
		exponent := float32(-1 / (float64(float32(ms)) * .001 * sampleRate))
		return float64(power2(exponent))
	}

	return 1 - pole(attackMS), pole(releaseMS), nil
}

func davSpectralGatePole(exponent float32) float32 {
	value := float64(exponent)
	whole := math.RoundToEven(value)
	if !(whole >= math.MinInt32 && whole <= math.MaxInt32) {
		return davPower2(exponent)
	}
	packed := math.Float32frombits(uint32(int64(whole)+127) << 23)
	pole := float32(float64(packed) * davPower2Residual(value-whole))
	if math.Abs(float64(pole)) <= math.MaxFloat32 {
		return pole
	}
	return davPower2(exponent)
}

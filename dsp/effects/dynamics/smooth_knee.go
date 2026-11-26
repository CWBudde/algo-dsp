package dynamics

import "math"

// SmoothKneeSettings configures a peak-following soft-knee detector. AttackMS
// and ReleaseMS express half-lives in milliseconds; zero makes that direction
// follow the key instantly. Ratio must be positive and may be smaller than
// one. KneeDB is the depth of the curve at threshold.
type SmoothKneeSettings struct {
	AttackMS, ReleaseMS float64
	ThresholdDB, KneeDB float64
	Ratio, MakeupDB     float64
	AutoMakeup          bool
}

// SmoothKneeDetector is the transfer and envelope of the Delphi ASIO & VST
// Project's TLightweightSoftKneeCompressor. Configure before processing and
// keep one detector for each independent key. A shared key needs one detector.
type SmoothKneeDetector struct {
	peak, attack, release float64
	threshold, ratio      float64
	kneeSquared, makeup   float64
}

const (
	smoothKneeFloor = float64(float32(1e-24))
	dbToLog2        = float64(float32(0.1660964047443681173935))
)

// Configure recalculates coefficients without discarding the envelope history.
// The caller supplies a positive sample rate and finite settings.
func (d *SmoothKneeDetector) Configure(sampleRate float64, settings SmoothKneeSettings) {
	// A zero half-life is the limit of the formula below: an instant attack.
	d.attack = 1
	if settings.AttackMS > 0 {
		d.attack = 1 - float64(float32(smoothKneePow2(float64(float32(-1/(settings.AttackMS*0.001*sampleRate))))))
	}

	d.release = 0
	if settings.ReleaseMS > 0 {
		d.release = float64(float32(smoothKneePow2(float64(float32(-1 / (settings.ReleaseMS * 0.001 * sampleRate))))))
	}

	d.threshold = float64(float32(settings.ThresholdDB * dbToLog2))
	d.ratio = float64(float32(0.5 * (1/settings.Ratio - 1)))
	knee := settings.KneeDB * dbToLog2
	d.kneeSquared = float64(float32(knee * knee))

	makeup := settings.MakeupDB
	if settings.AutoMakeup {
		atZero := float64(float32(-settings.ThresholdDB * d.ratio))
		makeup = float64(float32(smoothKneeSqrt(atZero*atZero+settings.KneeDB*settings.KneeDB))) - atZero
	}

	d.makeup = math.Exp(makeup * 0.1151292546497022842)
	if d.peak == 0 {
		d.Reset()
	}
}

// Reset returns the peak envelope to its initial silence floor.
func (d *SmoothKneeDetector) Reset() { d.peak = smoothKneeFloor }

// Gain updates the peak with one detector sample and returns gain reduction as
// a linear multiplier. The caller can apply it to a separate carrier sample.
func (d *SmoothKneeDetector) Gain(key float64) float64 {
	level := math.Abs(key) + smoothKneeFloor
	if level > d.peak {
		d.peak += (level - d.peak) * d.attack
	} else {
		d.peak = level + (d.peak-level)*d.release
	}

	level = d.ratio * (smoothKneeLog2(float32(d.peak)) - d.threshold)

	return smoothKneePow2(level - smoothKneeSqrt(level*level+d.kneeSquared))
}

// MakeupGain returns the linear automatic or manual makeup multiplier.
func (d *SmoothKneeDetector) MakeupGain() float64 { return d.makeup }

// SmoothKneeSoftClip applies the fourth-order tanh approximation used after
// the dry/wet mix by the lightweight compressor.
func SmoothKneeSoftClip(value float32) float32 {
	// At this amplitude the tanh result rounds to unity in float32.
	if value >= 10 {
		return 1
	}

	if value <= -10 {
		return -1
	}

	x := float64(float32(value * float32(2.8853900817779268)))
	exponent := math.RoundToEven(x)
	x -= exponent
	y := 1 + x*(float64(float32(0.693118326815805763))+
		x*(float64(float32(0.240239617833581720))+
			x*(float64(float32(0.0559538423222786241))+x*float64(float32(0.00960540553453761992)))))
	power := float64(float32(y * math.Ldexp(1, int(exponent))))

	return float32((power - 1) / (power + 1))
}

// The numerical kernels follow the original x87 detector. Coefficients and
// approximate exponent/mantissa seeds are Single, intermediate math is wider.
func smoothKneePow2(x float64) float64 {
	exponent := math.RoundToEven(x)
	if exponent < -126 {
		// Legal ratios below one can exceed the original float exponent field.
		return 0
	}

	x -= exponent
	y := 1 + x*(float64(float32(0.693292707161004662))+
		x*(float64(float32(0.242162975514835621))+x*float64(float32(0.0548668824216034384))))

	return y * math.Ldexp(1, int(exponent))
}

func smoothKneeLog2(x float32) float64 {
	bits := math.Float32bits(x)
	exponent := int(bits>>23&0xff) - 128
	mantissa := float64(math.Float32frombits(bits&0x7fffff | 0x3f800000))

	return float64(exponent) + (((float64(float32(-0.0821343513178931783))*mantissa+
		float64(float32(0.649732456739820052)))*mantissa+float64(float32(-2.13417801862571777)))*
		mantissa+float64(float32(4.08642207062728868)))*mantissa + float64(float32(-1.51984215742349793))
}

func smoothKneeSqrt(x float64) float64 {
	seed := (math.Float32bits(float32(x))-0x00800000)>>1 + 0x20000000
	estimate := float64(math.Float32frombits(seed))
	sum := estimate + x/estimate

	return 0.25*sum + x/sum
}

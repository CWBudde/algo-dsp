package dynamics

import "math"

// SimpleFeedbackSettings sets the ratio and envelope of a hard-knee feedback
// compressor. Attack and release are half-lives in milliseconds before
// scaling by Ratio. Supply a ratio of at least one and positive times.
type SimpleFeedbackSettings struct {
	AttackMS, ReleaseMS float64
	ThresholdDB, Ratio  float64
	AutoMakeup          bool
}

// SimpleFeedbackCompressor follows Delphi ASIO & VST's TSimpleFeedbackCompressor.
// The detector follows the previous compressed sample before makeup.
type SimpleFeedbackCompressor struct {
	peak, previous         float64
	attack, release        float64
	threshold, ratioFactor float64
	reductionScale, makeup float64
}

// Configure updates the coefficients while preserving feedback history.
// Supply a positive finite sample rate and finite settings.
func (c *SimpleFeedbackCompressor) Configure(sampleRate float64, settings SimpleFeedbackSettings) {
	ratioReciprocal := 1 / settings.Ratio
	designRatio := 1 / ratioReciprocal
	c.attack = 1 - math.Exp(-math.Ln2/(settings.AttackMS*0.001*sampleRate*designRatio))
	c.release = math.Exp(-math.Ln2 / (settings.ReleaseMS * 0.001 * sampleRate * designRatio))
	c.threshold = math.Exp(settings.ThresholdDB * 0.1151292546497022842)
	c.ratioFactor = 1 - designRatio
	c.reductionScale = math.Pow(c.threshold, (1-ratioReciprocal)*designRatio)

	c.makeup = 1
	if settings.AutoMakeup {
		c.makeup = math.Pow(c.threshold, ratioReciprocal-1)
	}
}

// ProcessSample compresses one sample and advances the feedback detector.
func (c *SimpleFeedbackCompressor) ProcessSample(input float64) float64 {
	if c.previous > c.peak {
		c.peak += (c.previous - c.peak) * c.attack
	} else {
		c.peak = c.previous + (c.peak-c.previous)*c.release
	}

	compressed := input

	if c.peak >= c.threshold {
		// Near the lowest thresholds and highest ratios the separate
		// terms can underflow and overflow even though the gain is finite.
		if c.reductionScale < 0x1p-1022 {
			compressed *= math.Pow(c.peak/c.threshold, c.ratioFactor)
		} else {
			compressed *= c.reductionScale * math.Pow(c.peak, c.ratioFactor)
		}
	}

	c.previous = math.Abs(compressed)

	return c.makeup * compressed
}

package design

// PeakOption configures the Peak and PeakCascade designers.
//
// Without options, Peak uses the standard RBJ peaking-EQ formula.
// Supplying WithDCGain and/or WithNyquistGain activates the Orfanidis
// algorithm which supports prescribed gain at DC and Nyquist.
type PeakOption func(*peakConfig)

type peakConfig struct {
	dcGain       float64
	nyquistGain  float64
	bandEdgeGain float64 // 0 means "use default sqrt(G)"
	hasDCGain    bool
	hasNyqGain   bool
	hasBEGain    bool
}

// WithDCGain sets the DC gain (linear) for Orfanidis-style peaking design.
// Typical value is 1.0 (unity at DC). Setting this activates the Orfanidis
// algorithm instead of the default RBJ formula.
func WithDCGain(g float64) PeakOption {
	return func(c *peakConfig) {
		c.dcGain = g
		c.hasDCGain = true
	}
}

// WithNyquistGain sets the Nyquist gain (linear) for Orfanidis-style peaking
// design. Typical value is 1.0 (unity at Nyquist). Setting this activates the
// Orfanidis algorithm instead of the default RBJ formula.
func WithNyquistGain(g float64) PeakOption {
	return func(c *peakConfig) {
		c.nyquistGain = g
		c.hasNyqGain = true
	}
}

// WithBandEdgeGain sets the band-edge gain (linear) for Orfanidis-style
// peaking design. If not set, defaults to sqrt(G) (the classic half-gain
// convention).
func WithBandEdgeGain(g float64) PeakOption {
	return func(c *peakConfig) {
		c.bandEdgeGain = g
		c.hasBEGain = true
	}
}

func applyPeakOpts(opts []PeakOption) peakConfig {
	var cfg peakConfig
	for _, o := range opts {
		o(&cfg)
	}

	return cfg
}

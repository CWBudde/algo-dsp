package effects

// Limiter implements a simple peak limiter using a high-ratio compressor.
// It is configured with a high compression ratio (100:1) and fast attack (0.1ms)
// to prevent signal peaks from exceeding the threshold.
type Limiter struct {
	comp *Compressor
}

// NewLimiter creates a new limiter instance.
func NewLimiter(sampleRate float64) (*Limiter, error) {
	c, err := NewCompressor(sampleRate)
	if err != nil {
		return nil, err
	}
	// Configure as limiter
	if err := c.SetRatio(100.0); err != nil {
		return nil, err
	}
	if err := c.SetAttack(0.1); err != nil { // 0.1 ms fast attack
		return nil, err
	}
	if err := c.SetKnee(0.0); err != nil { // Hard knee
		return nil, err
	}
	if err := c.SetAutoMakeup(false); err != nil {
		return nil, err
	}
	if err := c.SetMakeupGain(0.0); err != nil {
		return nil, err
	}

	return &Limiter{comp: c}, nil
}

// SetThreshold sets the limiting threshold in dB (ceiling).
// Signals above this level will be heavily compressed.
func (l *Limiter) SetThreshold(dB float64) error {
	return l.comp.SetThreshold(dB)
}

// SetRelease sets the release time in milliseconds.
func (l *Limiter) SetRelease(ms float64) error {
	return l.comp.SetRelease(ms)
}

// SetSampleRate updates the sample rate.
func (l *Limiter) SetSampleRate(sr float64) error {
	return l.comp.SetSampleRate(sr)
}

// ProcessSample processes one sample through the limiter.
func (l *Limiter) ProcessSample(input float64) float64 {
	return l.comp.ProcessSample(input)
}

// CalculateOutputLevel computes the steady-state output level for a given input magnitude.
func (l *Limiter) CalculateOutputLevel(inputMagnitude float64) float64 {
	return l.comp.CalculateOutputLevel(inputMagnitude)
}

// Reset clears the internal state.
func (l *Limiter) Reset() {
	l.comp.Reset()
}

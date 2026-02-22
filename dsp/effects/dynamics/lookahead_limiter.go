package dynamics

import (
	"fmt"
	"math"
)

const (
	defaultLookaheadLimiterThresholdDB = -0.1
	defaultLookaheadLimiterReleaseMs   = 100.0
	defaultLookaheadLimiterLookaheadMs = 3.0

	minLookaheadLimiterThresholdDB = -24.0
	maxLookaheadLimiterThresholdDB = 0.0
	minLookaheadLimiterReleaseMs   = 1.0
	maxLookaheadLimiterReleaseMs   = 5000.0
	minLookaheadLimiterLookaheadMs = 0.0
	maxLookaheadLimiterLookaheadMs = 200.0
)

// LookaheadLimiter is a limiter with delayed program path and optional
// sidechain detector input.
type LookaheadLimiter struct {
	comp *Compressor

	sampleRate  float64
	thresholdDB float64
	releaseMs   float64
	lookaheadMs float64

	delayBuf []float64
	writePos int
}

// NewLookaheadLimiter creates a lookahead limiter with production defaults.
func NewLookaheadLimiter(sampleRate float64) (*LookaheadLimiter, error) {
	if err := validateSampleRate(sampleRate); err != nil {
		return nil, fmt.Errorf("lookahead limiter %w", err)
	}

	c, err := NewCompressor(sampleRate)
	if err != nil {
		return nil, fmt.Errorf("lookahead limiter compressor init: %w", err)
	}

	if err := c.SetRatio(100.0); err != nil {
		return nil, err
	}

	if err := c.SetAttack(0.1); err != nil {
		return nil, err
	}

	if err := c.SetKnee(0.0); err != nil {
		return nil, err
	}

	if err := c.SetAutoMakeup(false); err != nil {
		return nil, err
	}

	if err := c.SetMakeupGain(0.0); err != nil {
		return nil, err
	}

	l := &LookaheadLimiter{
		comp:        c,
		sampleRate:  sampleRate,
		thresholdDB: defaultLookaheadLimiterThresholdDB,
		releaseMs:   defaultLookaheadLimiterReleaseMs,
		lookaheadMs: defaultLookaheadLimiterLookaheadMs,
	}
	if err := l.SetThreshold(l.thresholdDB); err != nil {
		return nil, err
	}

	if err := l.SetRelease(l.releaseMs); err != nil {
		return nil, err
	}

	if err := l.SetLookahead(l.lookaheadMs); err != nil {
		return nil, err
	}

	return l, nil
}

// SetThreshold sets the limiting threshold in dB.
func (l *LookaheadLimiter) SetThreshold(dB float64) error {
	if dB < minLookaheadLimiterThresholdDB || dB > maxLookaheadLimiterThresholdDB || !isFinite(dB) {
		return fmt.Errorf("lookahead limiter threshold must be in [%f, %f]: %f",
			minLookaheadLimiterThresholdDB, maxLookaheadLimiterThresholdDB, dB)
	}

	if err := l.comp.SetThreshold(dB); err != nil {
		return err
	}

	l.thresholdDB = dB

	return nil
}

// SetRelease sets release time in milliseconds.
func (l *LookaheadLimiter) SetRelease(ms float64) error {
	if ms < minLookaheadLimiterReleaseMs || ms > maxLookaheadLimiterReleaseMs || !isFinite(ms) {
		return fmt.Errorf("lookahead limiter release must be in [%f, %f]: %f",
			minLookaheadLimiterReleaseMs, maxLookaheadLimiterReleaseMs, ms)
	}

	if err := l.comp.SetRelease(ms); err != nil {
		return err
	}

	l.releaseMs = ms

	return nil
}

// SetLookahead sets lookahead time in milliseconds.
func (l *LookaheadLimiter) SetLookahead(ms float64) error {
	if ms < minLookaheadLimiterLookaheadMs || ms > maxLookaheadLimiterLookaheadMs || !isFinite(ms) {
		return fmt.Errorf("lookahead limiter lookahead must be in [%f, %f]: %f",
			minLookaheadLimiterLookaheadMs, maxLookaheadLimiterLookaheadMs, ms)
	}

	l.lookaheadMs = ms
	l.rebuildDelayBuffer()

	return nil
}

// SetSampleRate updates sample rate and internal coefficients/buffers.
func (l *LookaheadLimiter) SetSampleRate(sr float64) error {
	if err := validateSampleRate(sr); err != nil {
		return fmt.Errorf("lookahead limiter %w", err)
	}

	if err := l.comp.SetSampleRate(sr); err != nil {
		return err
	}

	l.sampleRate = sr
	l.rebuildDelayBuffer()

	return nil
}

// Threshold returns threshold in dB.
func (l *LookaheadLimiter) Threshold() float64 { return l.thresholdDB }

// Release returns release time in milliseconds.
func (l *LookaheadLimiter) Release() float64 { return l.releaseMs }

// Lookahead returns lookahead time in milliseconds.
func (l *LookaheadLimiter) Lookahead() float64 { return l.lookaheadMs }

// SampleRate returns sample rate in Hz.
func (l *LookaheadLimiter) SampleRate() float64 { return l.sampleRate }

// Reset clears limiter and delay state.
func (l *LookaheadLimiter) Reset() {
	l.comp.Reset()
	l.writePos = 0
	for i := range l.delayBuf {
		l.delayBuf[i] = 0
	}
}

// ProcessSample processes one sample using the input as both program and detector.
func (l *LookaheadLimiter) ProcessSample(input float64) float64 {
	return l.ProcessSampleSidechain(input, input)
}

// ProcessSampleSidechain processes one sample using a separate detector input.
func (l *LookaheadLimiter) ProcessSampleSidechain(input, sidechain float64) float64 {
	if len(l.delayBuf) == 0 {
		return 0
	}

	// Run detector ahead of the delayed program path.
	_, gain := l.comp.core.ProcessSample(0, sidechain)

	l.delayBuf[l.writePos] = input
	readPos := l.writePos + 1
	if readPos >= len(l.delayBuf) {
		readPos = 0
	}

	delayed := l.delayBuf[readPos]
	l.writePos = readPos

	return delayed * gain
}

// ProcessInPlace processes a block in place using the signal itself as detector.
func (l *LookaheadLimiter) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = l.ProcessSample(buf[i])
	}
}

// ProcessInPlaceSidechain processes program buffer in place using a separate
// sidechain detector buffer.
func (l *LookaheadLimiter) ProcessInPlaceSidechain(program, sidechain []float64) {
	for i := range program {
		det := 0.0
		if i < len(sidechain) {
			det = sidechain[i]
		}

		program[i] = l.ProcessSampleSidechain(program[i], det)
	}
}

func (l *LookaheadLimiter) rebuildDelayBuffer() {
	delaySamples := int(math.Round(l.lookaheadMs * l.sampleRate / 1000.0))
	if delaySamples < 0 {
		delaySamples = 0
	}

	size := delaySamples + 1
	if size < 1 {
		size = 1
	}

	l.delayBuf = make([]float64, size)
	l.writePos = 0
}

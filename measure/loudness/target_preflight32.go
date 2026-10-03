package loudness

import (
	"fmt"
	"math"
)

func validateTargetPlanar32Block(a *TargetAnalyzer, block [][]float32) error {
	if a == nil || a.maxFrames <= 0 || len(a.blockEnergy) == 0 {
		return fmt.Errorf("loudness.target.process: unconfigured analyzer: %w", ErrState)
	}

	if a.failure != nil {
		return a.failure
	}

	if a.phase != targetInput {
		return fmt.Errorf("loudness.target.process: input finalized: %w", ErrState)
	}

	if len(block) != len(a.weights) || len(block) == 0 {
		return fmt.Errorf("loudness.target.process: channel count: %w", ErrInvalid)
	}

	frames := len(block[0])
	if frames > MaxIntegratedBlockFrames || int64(frames) > a.maxFrames-a.frames {
		return fmt.Errorf("loudness.target.process: frame count: %w", ErrLimit)
	}

	for _, channel := range block {
		if len(channel) != frames {
			return fmt.Errorf("loudness.target.process: unequal channel lengths: %w", ErrInvalid)
		}
	}

	return nil
}

func preflightTargetPlanar32(a *TargetAnalyzer, block [][]float32) (float64, error) {
	if err := validateTargetPlanar32Block(a, block); err != nil {
		return 0, err
	}
	// Positive finite IEEE float32 encodings are monotonically ordered. Ignore
	// sign only for the local peak, classify both signed infinities and every
	// NaN payload, and never rewrite caller samples (including signed zero).
	var peakBits uint32

	for _, channel := range block {
		for _, sample := range channel {
			bits := math.Float32bits(sample) & 0x7fffffff
			if bits >= 0x7f800000 {
				return 0, fmt.Errorf("loudness.target.process: sample: %w", ErrNonFinite)
			}

			if bits > peakBits {
				peakBits = bits
			}
		}
	}

	peak := float64(math.Float32frombits(peakBits))
	// Keep the previous float64 diagnostic exactly: callers may mix planar
	// precisions, including a prior peak larger than float32's finite range.
	if a.peak > peak {
		peak = a.peak
	}

	return peak, nil
}

package loudness

import (
	"fmt"
	"math"
)

// ProcessCertifiedPlanar32 measures the actual samples like ProcessPlanar32,
// using a caller-proven finite sample peak to omit its sample validation scan.
// The caller MUST prove every sample in every supplied channel is finite and
// samplePeak is the exact maximum absolute float32 sample widened to float64
// (including zero-weight channels). The empty block's peak is zero. An immutable
// block's previously computed finite energy and exact sample peak can supply
// that proof; estimates, true-peak measurements and loudness predictions cannot.
// Samples must stay unchanged during this call. Use ProcessPlanar32 when these
// obligations cannot be guaranteed. Violating them has no promised atomic
// sample-rejection behavior; the method deliberately does not reverify samples.
//
// State, shape, frame limits and certificate representation are still validated
// atomically. samplePeak must be finite, nonnegative and exactly representable
// as a float32; zero is canonicalized to positive zero. This method still runs
// the actual K-weighting filters, window aggregation and finalization over the
// supplied samples. It never certifies or predicts their loudness, retains no
// caller slices, and allocates nothing on successful processing.
func (a *TargetAnalyzer) ProcessCertifiedPlanar32(block [][]float32, samplePeak float64) error {
	if err := validateTargetPlanar32Block(a, block); err != nil {
		return err
	}

	if !integratedFinite(samplePeak) || samplePeak < 0 || samplePeak > math.MaxFloat32 ||
		float64(float32(samplePeak)) != samplePeak || (len(block[0]) == 0 && samplePeak != 0) {
		return fmt.Errorf("loudness.target.certified: exact finite nonnegative float32 sample peak required: %w", ErrInvalid)
	}

	if samplePeak == 0 {
		samplePeak = 0
	}

	if a.peak > samplePeak {
		samplePeak = a.peak
	}

	return processPreparedTargetPlanar(a, block, samplePeak)
}

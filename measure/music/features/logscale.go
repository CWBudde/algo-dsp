package features

import (
	"fmt"
	"math"
)

// LogScale maps a one-sided FFT power spectrum onto a logarithmic frequency
// grid of Bins() bins between fmin and fmax.
//
// Log bin b covers [e_b, e_{b+1}) with e_b = fmin·(fmax/fmin)^(b/bins).
// FFT bin k (frequency f_k = k·sampleRate/fftSize, spacing Δ) is treated as
// a uniform power density over [f_k-Δ/2, f_k+Δ/2], and log bin b receives
// the fraction of it that overlaps [e_b, e_{b+1}):
//
//	L[b] = Σ_k P[k]·|[f_k-Δ/2, f_k+Δ/2] ∩ [e_b, e_{b+1})| / Δ
//
// The mapping is energy preserving (Σ_b L[b] equals the power of the FFT
// bins inside [fmin, fmax], with partial bins weighted by their overlap),
// and every log bin overlaps at least one FFT bin, so no row of a
// spectrogram is empty. Log bins narrower than Δ (the low end of a typical
// music spectrogram) receive a proportional share of the FFT bins they
// overlap instead of either nothing or a whole bin.
//
// A LogScale is immutable and safe for concurrent use.
type LogScale struct {
	edges   []float64
	centres []float64
	first   []int       // per log bin: first FFT bin with a non-zero weight
	weights [][]float64 // per log bin: weights of FFT bins first, first+1, ...
	nBins   int         // number of FFT bins, fftSize/2+1
}

// NewLogScale returns the log-frequency mapping of bins log bins between
// fmin and fmax (Hz) for an FFT of fftSize points at sampleRate. It requires
// bins >= 1, 0 < fmin < fmax <= sampleRate/2 and fftSize >= 2.
func NewLogScale(bins int, fmin, fmax, sampleRate float64, fftSize int) (*LogScale, error) {
	switch {
	case bins < 1:
		return nil, fmt.Errorf("%w: log bins %d", ErrInvalidArgument, bins)
	case fftSize < 2:
		return nil, fmt.Errorf("%w: FFT size %d", ErrInvalidArgument, fftSize)
	case !(sampleRate > 0) || math.IsInf(sampleRate, 0):
		return nil, fmt.Errorf("%w: sample rate %v", ErrInvalidArgument, sampleRate)
	case !(fmin > 0) || !(fmax > fmin) || fmax > sampleRate/2:
		return nil, fmt.Errorf("%w: log range %v..%v Hz at %v Hz", ErrInvalidArgument, fmin, fmax, sampleRate)
	}

	s := &LogScale{
		edges:   make([]float64, bins+1),
		centres: make([]float64, bins),
		first:   make([]int, bins),
		weights: make([][]float64, bins),
		nBins:   fftSize/2 + 1,
	}

	ratio := math.Log(fmax / fmin)
	for b := range s.edges {
		s.edges[b] = fmin * math.Exp(ratio*float64(b)/float64(bins))
	}

	s.edges[0], s.edges[bins] = fmin, fmax

	delta := sampleRate / float64(fftSize)

	for b := range bins {
		lo, hi := s.edges[b], s.edges[b+1]
		s.centres[b] = math.Sqrt(lo * hi)

		// FFT bins whose interval [f-Δ/2, f+Δ/2] overlaps [lo, hi).
		k0 := max(0, int(math.Floor(lo/delta+0.5)))
		k1 := min(s.nBins-1, int(math.Ceil(hi/delta-0.5)))

		s.first[b] = k0
		for k := k0; k <= k1; k++ {
			f := float64(k) * delta
			overlap := min(f+delta/2, hi) - max(f-delta/2, lo)
			s.weights[b] = append(s.weights[b], max(0, overlap)/delta)
		}
	}

	return s, nil
}

// Bins returns the number of log bins.
func (s *LogScale) Bins() int { return len(s.centres) }

// BinFrequencies returns the geometric centre frequency sqrt(e_b·e_{b+1})
// of each log bin in Hz, for labelling a spectrogram axis.
func (s *LogScale) BinFrequencies() []float64 {
	return append([]float64(nil), s.centres...)
}

// Edges returns the Bins()+1 log bin edges in Hz.
func (s *LogScale) Edges() []float64 {
	return append([]float64(nil), s.edges...)
}

// Apply maps the one-sided power spectrum power (fftSize/2+1 bins) onto the
// log grid: dst[b] = Σ_k w[b][k]·power[k]. dst must have Bins() elements.
// Apply does not allocate.
func (s *LogScale) Apply(dst, power []float64) error {
	if len(dst) != len(s.centres) || len(power) != s.nBins {
		return fmt.Errorf("%w: dst %d (want %d), power %d (want %d)",
			ErrInvalidArgument, len(dst), len(s.centres), len(power), s.nBins)
	}

	s.apply(dst, power)

	return nil
}

func (s *LogScale) apply(dst, power []float64) {
	for b, w := range s.weights {
		acc := 0.0
		p := power[s.first[b]:]

		for i, v := range w {
			acc += v * p[i]
		}

		dst[b] = acc
	}
}

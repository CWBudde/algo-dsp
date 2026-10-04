package spectrum

import (
	"fmt"
	"math"
	"sort"
)

// PowerAverager retains a bounded rolling window of linear-power spectra.
// Add copies input; ValuesInto rebases positive sums to avoid subtractive tails
// after a loud-to-quiet transition. Processing and Reset allocate nothing.
type PowerAverager struct {
	history                       []float64
	bins, frames, position, count int
}

// NewPowerAverager reserves up to1024 spectra of at most32769 bins, within64MiB.
func NewPowerAverager(bins, frames int) (*PowerAverager, error) {
	if bins < 1 || bins > 32769 || frames < 1 || frames > 1024 || int64(bins)*int64(frames) > 64<<20/8 {
		return nil, fmt.Errorf("spectrum.average.new: invalid or oversized geometry")
	}

	return &PowerAverager{history: make([]float64, bins*frames), bins: bins, frames: frames}, nil
}

func validatePower(power []float64, bins int) error {
	if len(power) != bins {
		return fmt.Errorf("spectrum: power bin count")
	}

	for _, value := range power {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1e200 {
			return fmt.Errorf("spectrum: nonfinite, negative or unrepresentable power")
		}
	}

	return nil
}

// Add accepts one finite nonnegative spectrum, bounded to1e200 per bin.
func (a *PowerAverager) Add(power []float64) error {
	if a == nil || a.bins == 0 {
		return fmt.Errorf("spectrum.average.add: unconfigured")
	}

	if err := validatePower(power, a.bins); err != nil {
		return err
	}

	copy(a.history[a.position*a.bins:], power)
	a.position = (a.position + 1) % a.frames
	a.count = min(a.count+1, a.frames)

	return nil
}

// ValuesInto writes the current arithmetic mean, or zeros before the first Add.
func (a *PowerAverager) ValuesInto(dst []float64) error {
	if a == nil || len(dst) < a.bins {
		return fmt.Errorf("spectrum.average.values: short destination")
	}

	clear(dst[:a.bins])

	if a.count == 0 {
		return nil
	}

	for frame := 0; frame < a.count; frame++ {
		values := a.history[frame*a.bins : (frame+1)*a.bins]
		for bin, value := range values {
			dst[bin] += value
		}
	}

	for bin := 0; bin < a.bins; bin++ {
		dst[bin] /= float64(a.count)
	}

	return nil
}

// Reset clears the retained averaging window without allocation.
func (a *PowerAverager) Reset() { clear(a.history); a.position = 0; a.count = 0 }

// PowerAccumulator averages any bounded number of spectra without retaining
// frames. Its fixed workspace suits spectrogram pixels containing many hops.
type PowerAccumulator struct {
	sums  []float64
	count int64
}

// NewPowerAccumulator reserves a fixed sum per bin, up to32769 bins.
func NewPowerAccumulator(bins int) (*PowerAccumulator, error) {
	if bins < 1 || bins > 32769 {
		return nil, fmt.Errorf("spectrum.accumulator.new: invalid bins")
	}

	return &PowerAccumulator{sums: make([]float64, bins)}, nil
}

// Add accepts one finite nonnegative spectrum. At most2^52 spectra are accepted.
func (a *PowerAccumulator) Add(power []float64) error {
	if a == nil || len(a.sums) == 0 || a.count >= 1<<52 {
		return fmt.Errorf("spectrum.accumulator.add: unconfigured or count limit")
	}

	if err := validatePower(power, len(a.sums)); err != nil {
		return err
	}

	for bin, value := range power {
		a.sums[bin] += value
	}

	a.count++

	return nil
}

// ValuesInto writes the arithmetic mean, or zeros for an empty accumulation.
func (a *PowerAccumulator) ValuesInto(dst []float64) error {
	if a == nil || len(dst) < len(a.sums) {
		return fmt.Errorf("spectrum.accumulator.values: short destination")
	}

	for bin, sum := range a.sums {
		dst[bin] = 0
		if a.count > 0 {
			dst[bin] = sum / float64(a.count)
		}
	}

	return nil
}

// Reset begins another accumulation without reallocating.
func (a *PowerAccumulator) Reset() { clear(a.sums); a.count = 0 }

// PowerInto writes squared magnitudes of finite complex64/complex128 bins.
func PowerInto[C ~complex64 | ~complex128](dst []float64, bins []C) error {
	if len(dst) < len(bins) {
		return fmt.Errorf("spectrum.power: short destination")
	}

	for _, bin := range bins {
		c := complex128(bin)
		if math.IsNaN(real(c)) || math.IsNaN(imag(c)) || math.IsInf(real(c), 0) || math.IsInf(imag(c), 0) || math.Abs(real(c)) > 1e100 || math.Abs(imag(c)) > 1e100 {
			return fmt.Errorf("spectrum.power: nonfinite or unrepresentable bins")
		}
	}

	for i, bin := range bins {
		c := complex128(bin)
		dst[i] = real(c)*real(c) + imag(c)*imag(c)
	}

	return nil
}

// PowerToDBInto calibrates a one-sided real FFT to amplitude dBFS using the
// analysis window's coherent gain (sum of its coefficients). Interior bins get
// the factor-two amplitude correction; DC and an even-size Nyquist bin do not.
// Zero power maps to -Inf. Source and destination may be the same slice.
func PowerToDBInto(dst, power []float64, windowSum float64, fftSize int) error {
	if fftSize < 2 || len(power) != fftSize/2+1 || len(dst) < len(power) || windowSum <= 0 || math.IsNaN(windowSum) || math.IsInf(windowSum, 0) {
		return fmt.Errorf("spectrum.db: invalid geometry or window sum")
	}

	if err := validatePower(power, len(power)); err != nil {
		return err
	}

	base := -20 * math.Log10(windowSum)
	for bin, value := range power {
		offset := base
		if bin > 0 && (fftSize%2 != 0 || bin < len(power)-1) {
			offset += 20 * math.Log10(2)
		}

		dst[bin] = 10*math.Log10(value) + offset
	}

	return nil
}

// SmoothFractionalOctaveInto applies the existing arithmetic-mean band
// smoothing without allocation. DC is omitted: frequencies must be positive,
// finite and strictly increasing. Destination must not overlap either input.
func SmoothFractionalOctaveInto(dst, freqHz, values []float64, fraction int) error {
	if len(freqHz) == 0 || len(freqHz) != len(values) || len(dst) < len(values) || fraction < 1 {
		return fmt.Errorf("spectrum.smooth: invalid geometry")
	}

	for i, f := range freqHz {
		if f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) || (i > 0 && f <= freqHz[i-1]) || math.IsNaN(values[i]) || math.IsInf(values[i], 0) {
			return fmt.Errorf("spectrum.smooth: invalid input")
		}
	}

	half := math.Pow(2, 1/(2*float64(fraction)))
	for i, f := range freqHz {
		lo := sort.SearchFloat64s(freqHz, f/half)
		hi := sort.Search(len(freqHz), func(j int) bool { return freqHz[j] > f*half })

		sum := 0.0
		for j := lo; j < hi; j++ {
			sum += values[j]
		}

		dst[i] = sum / float64(hi-lo)
	}

	return nil
}

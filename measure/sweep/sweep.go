package sweep

import (
	"errors"
	"fmt"
	"math"

	algofft "github.com/cwbudde/algo-fft"
)

// Errors returned by sweep functions.
var (
	ErrInvalidFrequency  = errors.New("sweep: frequency must be positive")
	ErrInvalidDuration   = errors.New("sweep: duration must be positive")
	ErrInvalidSampleRate = errors.New("sweep: sample rate must be positive")
	ErrFrequencyOrder    = errors.New("sweep: start frequency must be less than end frequency")
	ErrEmptyResponse     = errors.New("sweep: response signal is empty")
	ErrMaxHarmonic       = errors.New("sweep: max harmonic must be >= 2")
)

// LogSweep generates a logarithmic sine sweep and provides deconvolution
// methods for impulse response measurement.
//
// A logarithmic sweep has the property that each octave takes the same
// amount of time, making it ideal for room acoustic measurements.
// The corresponding inverse filter, when convolved with the recorded
// response, yields the impulse response plus separated harmonic distortion IRs.
type LogSweep struct {
	StartFreq  float64 // start frequency in Hz
	EndFreq    float64 // end frequency in Hz
	Duration   float64 // sweep duration in seconds
	SampleRate float64 // sample rate in Hz
}

// Validate checks that the LogSweep parameters are valid.
func (s *LogSweep) Validate() error {
	if s.StartFreq <= 0 || s.EndFreq <= 0 {
		return ErrInvalidFrequency
	}

	if s.StartFreq >= s.EndFreq {
		return ErrFrequencyOrder
	}

	if s.Duration <= 0 {
		return ErrInvalidDuration
	}

	if s.SampleRate <= 0 {
		return ErrInvalidSampleRate
	}

	return nil
}

// samples returns the total number of samples for the sweep.
func (s *LogSweep) samples() int {
	return int(math.Round(s.Duration * s.SampleRate))
}

// Generate creates the logarithmic sine sweep signal.
//
// The instantaneous frequency increases exponentially from StartFreq to EndFreq:
//
//	f(t) = f1 * exp(t/T * ln(f2/f1))
//
// The phase integral gives:
//
//	x(t) = sin(2π * f1 * T / ln(f2/f1) * (exp(t/T * ln(f2/f1)) - 1))
func (s *LogSweep) Generate() ([]float64, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	n := s.samples()
	out := make([]float64, n)

	T := s.Duration
	ratio := s.EndFreq / s.StartFreq
	lnRatio := math.Log(ratio)

	for i := range out {
		t := float64(i) / s.SampleRate
		phase := 2 * math.Pi * s.StartFreq * T / lnRatio * (math.Exp(t/T*lnRatio) - 1)
		out[i] = math.Sin(phase)
	}

	return out, nil
}

// InverseFilter creates the inverse filter for deconvolution.
//
// For a log sweep, the inverse filter is the time-reversed sweep with an
// amplitude envelope proportional to the instantaneous frequency
// (Farina's method):
//
//	h_inv(t) = x(T-t) * (f(T-t)/f2)
//
// A log sweep spends equal time per octave, so its power spectral density
// falls at -3 dB/octave and |X(f)| is proportional to 1/sqrt(f). The
// convolution x * h_inv has magnitude |X(f)| * |X(f)| * env(f); a spectrally
// flat result therefore needs env proportional to f, i.e. the low-frequency
// (late) portion of the reversed sweep is attenuated by 6 dB/octave.
// (Using env proportional to 1/f — attenuating the high-frequency end —
// tilts the deconvolved spectrum by -12 dB/octave instead.)
//
// The filter is scaled so that convolving the sweep with it yields an
// impulse of unit amplitude at sample len(inv)-1.
func (s *LogSweep) InverseFilter() ([]float64, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	n := s.samples()

	sweep, err := s.Generate()
	if err != nil {
		return nil, err
	}

	T := s.Duration
	ratio := s.EndFreq / s.StartFreq
	lnRatio := math.Log(ratio)

	inv := make([]float64, n)
	normFactor := 0.0

	for i := range inv {
		// Reverse index into the original sweep
		j := n - 1 - i

		// Time in the original sweep for sample j
		t := float64(j) / s.SampleRate

		// Instantaneous frequency at time t
		fInst := s.StartFreq * math.Exp(t/T*lnRatio)

		// Amplitude compensation proportional to instantaneous frequency,
		// normalized so the envelope peaks at 1 at the high-frequency end.
		amp := fInst / s.EndFreq

		inv[i] = sweep[j] * amp

		// The identity-system convolution peak at lag n-1 is exactly
		// sum(sweep[j]^2 * amp[j]); accumulate it for exact unity scaling.
		normFactor += sweep[j] * sweep[j] * amp
	}

	if normFactor > 0 {
		scale := 1.0 / normFactor
		for i := range inv {
			inv[i] *= scale
		}
	}

	return inv, nil
}

// Deconvolve recovers the impulse response from a recorded sweep response.
//
// The recorded response is convolved (via FFT) with the inverse filter.
// For a causal system the linear impulse response starts at sample
// len(inverse)-1 of the returned slice, with harmonic distortion IRs at
// predictable offsets before it (see ExtractHarmonicIRs).
func (s *LogSweep) Deconvolve(response []float64) ([]float64, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	if len(response) == 0 {
		return nil, ErrEmptyResponse
	}

	inv, err := s.InverseFilter()
	if err != nil {
		return nil, err
	}

	// Use FFT-based convolution for efficiency
	n := len(response) + len(inv) - 1
	fftSize := nextPowerOf2(n)

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("sweep: failed to create FFT plan: %w", err)
	}

	// Zero-pad and FFT the response
	respPadded := make([]complex128, fftSize)
	for i, v := range response {
		respPadded[i] = complex(v, 0)
	}

	respFreq := make([]complex128, fftSize)
	if err := plan.Forward(respFreq, respPadded); err != nil {
		return nil, fmt.Errorf("sweep: forward FFT failed: %w", err)
	}

	// Zero-pad and FFT the inverse filter
	invPadded := make([]complex128, fftSize)
	for i, v := range inv {
		invPadded[i] = complex(v, 0)
	}

	invFreq := make([]complex128, fftSize)
	if err := plan.Forward(invFreq, invPadded); err != nil {
		return nil, fmt.Errorf("sweep: forward FFT failed: %w", err)
	}

	// Multiply in frequency domain (convolution)
	resultFreq := make([]complex128, fftSize)
	for i := range resultFreq {
		resultFreq[i] = respFreq[i] * invFreq[i]
	}

	// Inverse FFT
	resultTime := make([]complex128, fftSize)
	if err := plan.Inverse(resultTime, resultFreq); err != nil {
		return nil, fmt.Errorf("sweep: inverse FFT failed: %w", err)
	}

	// Extract real part - the IR starts around the center (at len(inv)-1)
	// For a causal system, the main IR peak appears at offset = len(inv) - 1
	result := make([]float64, n)
	for i := range result {
		result[i] = real(resultTime[i])
	}

	return result, nil
}

// ExtractHarmonicIRs separates the harmonic impulse responses from
// a deconvolved sweep response.
//
// When a log sweep passes through a nonlinear system, the deconvolved
// response contains the linear IR plus separate harmonic distortion IRs
// that appear at predictable time offsets before the main IR:
//
//	Δt_k = T * ln(k) / ln(f2/f1)
//
// where k is the harmonic order and T is the sweep duration.
//
// maxHarmonic specifies the highest harmonic to extract (e.g., 5 for H2-H5).
// Returns a slice of IRs: [linear IR, H2 IR, H3 IR, ...].
func (s *LogSweep) ExtractHarmonicIRs(response []float64, maxHarmonic int) ([][]float64, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	if maxHarmonic < 2 {
		return nil, ErrMaxHarmonic
	}

	deconv, err := s.Deconvolve(response)
	if err != nil {
		return nil, err
	}

	invLen := s.samples()
	T := s.Duration
	lnRatio := math.Log(s.EndFreq / s.StartFreq)

	// The main (linear) IR peak is at offset invLen - 1 in the deconvolved signal
	mainOffset := invLen - 1

	// Calculate time offsets for each harmonic
	// Harmonic k appears at Δt_k = T * ln(k) / ln(f2/f1) before the main IR
	type harmonicRegion struct {
		center int // sample offset in deconv
	}

	regions := make([]harmonicRegion, maxHarmonic+1) // index 1 = linear, 2 = H2, etc.

	for k := 1; k <= maxHarmonic; k++ {
		dtSamples := int(math.Round(T * math.Log(float64(k)) / lnRatio * s.SampleRate))
		regions[k] = harmonicRegion{center: mainOffset - dtSamples}
	}

	// Determine window size for each harmonic IR extraction.
	// Use half the distance to the next harmonic as the window half-width.
	results := make([][]float64, maxHarmonic)

	for k := 1; k <= maxHarmonic; k++ {
		center := regions[k].center

		// Window half-width: half distance to adjacent harmonic
		var halfWidth int

		if k == 1 {
			// For linear IR, use distance to H2 divided by 2
			if maxHarmonic >= 2 {
				halfWidth = (regions[1].center - regions[2].center) / 2
			} else {
				halfWidth = invLen / 4
			}
		} else if k < maxHarmonic {
			halfWidth = (regions[k-1].center - regions[k].center) / 2
		} else {
			// Last harmonic: use same width as previous
			if k >= 3 {
				halfWidth = (regions[k-1].center - regions[k].center) / 2
			} else {
				halfWidth = (regions[1].center - regions[2].center) / 2
			}
		}

		if halfWidth < 1 {
			halfWidth = 1
		}

		// Extract windowed region
		start := center - halfWidth
		end := center + halfWidth

		if start < 0 {
			start = 0
		}

		if end > len(deconv) {
			end = len(deconv)
		}

		irLen := end - start
		if irLen <= 0 {
			results[k-1] = []float64{0}
			continue
		}

		ir := make([]float64, irLen)
		copy(ir, deconv[start:end])
		results[k-1] = ir
	}

	return results, nil
}

// LinearSweep generates a linear (chirp) sine sweep for comparison/testing.
type LinearSweep struct {
	StartFreq  float64
	EndFreq    float64
	Duration   float64
	SampleRate float64
}

// Validate checks that the LinearSweep parameters are valid.
func (s *LinearSweep) Validate() error {
	if s.StartFreq <= 0 || s.EndFreq <= 0 {
		return ErrInvalidFrequency
	}

	if s.StartFreq >= s.EndFreq {
		return ErrFrequencyOrder
	}

	if s.Duration <= 0 {
		return ErrInvalidDuration
	}

	if s.SampleRate <= 0 {
		return ErrInvalidSampleRate
	}

	return nil
}

// Generate creates the linear frequency sweep signal.
//
// The instantaneous frequency increases linearly:
//
//	f(t) = f1 + (f2-f1) * t / T
func (s *LinearSweep) Generate() ([]float64, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	n := int(math.Round(s.Duration * s.SampleRate))
	out := make([]float64, n)

	T := s.Duration
	k := (s.EndFreq - s.StartFreq) / T

	for i := range out {
		t := float64(i) / s.SampleRate
		phase := 2 * math.Pi * (s.StartFreq*t + 0.5*k*t*t)
		out[i] = math.Sin(phase)
	}

	return out, nil
}

// InverseFilter creates the inverse (matched) filter for the linear sweep.
//
// A linear chirp has a flat power spectral density within its band, so its
// matched filter — the time-reversed sweep — already yields a spectrally
// flat deconvolution; no amplitude envelope is required. The filter is
// scaled by the sweep energy so that convolving the sweep with it yields an
// impulse of unit amplitude at sample len(inv)-1.
//
// (An earlier implementation computed a regularized spectral inverse over a
// zero-padded FFT and then truncated the result to the sweep length, which
// discarded the non-causal half of the filter energy — self-deconvolution
// peaked at the wrong index with amplitude far below unity.)
func (s *LinearSweep) InverseFilter() ([]float64, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	sweep, err := s.Generate()
	if err != nil {
		return nil, err
	}

	n := len(sweep)

	energy := 0.0
	for _, v := range sweep {
		energy += v * v
	}

	if energy <= 0 {
		return nil, ErrEmptyResponse
	}

	inv := make([]float64, n)
	for i := range inv {
		inv[i] = sweep[n-1-i] / energy
	}

	return inv, nil
}

// Deconvolve recovers the impulse response from a recorded linear sweep response.
//
// For a causal system the impulse response starts at sample
// len(inverse)-1 of the returned slice.
func (s *LinearSweep) Deconvolve(response []float64) ([]float64, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	if len(response) == 0 {
		return nil, ErrEmptyResponse
	}

	inv, err := s.InverseFilter()
	if err != nil {
		return nil, err
	}

	n := len(response) + len(inv) - 1
	fftSize := nextPowerOf2(n)

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("sweep: failed to create FFT plan: %w", err)
	}

	respPadded := make([]complex128, fftSize)
	for i, v := range response {
		respPadded[i] = complex(v, 0)
	}

	respFreq := make([]complex128, fftSize)
	if err := plan.Forward(respFreq, respPadded); err != nil {
		return nil, fmt.Errorf("sweep: forward FFT failed: %w", err)
	}

	invPadded := make([]complex128, fftSize)
	for i, v := range inv {
		invPadded[i] = complex(v, 0)
	}

	invFreq := make([]complex128, fftSize)
	if err := plan.Forward(invFreq, invPadded); err != nil {
		return nil, fmt.Errorf("sweep: forward FFT failed: %w", err)
	}

	resultFreq := make([]complex128, fftSize)
	for i := range resultFreq {
		resultFreq[i] = respFreq[i] * invFreq[i]
	}

	resultTime := make([]complex128, fftSize)
	if err := plan.Inverse(resultTime, resultFreq); err != nil {
		return nil, fmt.Errorf("sweep: inverse FFT failed: %w", err)
	}

	result := make([]float64, n)
	for i := range result {
		result[i] = real(resultTime[i])
	}

	return result, nil
}

// nextPowerOf2 returns the next power of 2 >= n.
func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}

	p := 1
	for p < n {
		p *= 2
	}

	return p
}

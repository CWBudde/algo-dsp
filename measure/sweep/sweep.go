package sweep

import (
	"errors"
	"fmt"
	"math"
	"math/cmplx"

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
	err := s.Validate()
	if err != nil {
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
// For a log sweep, the inverse filter is the time-reversed sweep with
// amplitude compensation that decreases at 6 dB/octave (to compensate
// for the sweep's increasing energy per frequency band):
//
//	h_inv(t) = x(T-t) * (f1/f(T-t))
//
// This ensures that convolution of the sweep with its inverse yields
// an impulse (Dirac delta).
func (s *LogSweep) InverseFilter() ([]float64, error) {
	err := s.Validate()
	if err != nil {
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

	// The amplitude envelope for the inverse filter compensates for
	// the log sweep's increasing instantaneous frequency.
	// At time t, the instantaneous frequency is f1*exp(t*ln(f2/f1)/T).
	// The inverse filter at position i corresponds to sweep time T - t_inv.
	inv := make([]float64, n)
	for i := range inv {
		// Reverse index into the original sweep
		j := n - 1 - i

		// Time in the original sweep for sample j
		t := float64(j) / s.SampleRate

		// Instantaneous frequency at time t
		fInst := s.StartFreq * math.Exp(t/T*lnRatio)

		// Amplitude compensation: normalize by instantaneous frequency
		// (6 dB/octave rolloff to flatten the energy spectrum)
		amp := s.StartFreq / fInst

		inv[i] = sweep[j] * amp
	}

	// Normalize so that the convolution peak is unity
	// The normalization factor is the integral of the squared inverse
	// which for a log sweep evaluates to T*f1/ln(f2/f1)
	normFactor := T * s.StartFreq / lnRatio * s.SampleRate
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
// Given a system response to the log sweep, this performs FFT-based
// deconvolution by dividing the response spectrum by the sweep spectrum
// (with regularization). The result is the system's impulse response.
func (s *LogSweep) Deconvolve(response []float64) ([]float64, error) {
	err := s.Validate()
	if err != nil {
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

	err = plan.Forward(respFreq, respPadded)
	if err != nil {
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
	err := s.Validate()
	if err != nil {
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

// InverseFilter creates an inverse filter for the linear sweep using
// spectral division with regularization.
func (s *LinearSweep) InverseFilter() ([]float64, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	sweep, err := s.Generate()
	if err != nil {
		return nil, err
	}

	n := len(sweep)
	fftSize := nextPowerOf2(2 * n)

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("sweep: failed to create FFT plan: %w", err)
	}

	// FFT the sweep
	sweepPadded := make([]complex128, fftSize)
	for i, v := range sweep {
		sweepPadded[i] = complex(v, 0)
	}

	sweepFreq := make([]complex128, fftSize)
	if err := plan.Forward(sweepFreq, sweepPadded); err != nil {
		return nil, fmt.Errorf("sweep: forward FFT failed: %w", err)
	}

	// Compute regularized inverse: conj(H) / (|H|^2 + epsilon)
	const epsilon = 1e-6

	invFreq := make([]complex128, fftSize)
	for i := range invFreq {
		hConj := cmplx.Conj(sweepFreq[i])
		hMagSq := real(sweepFreq[i])*real(sweepFreq[i]) + imag(sweepFreq[i])*imag(sweepFreq[i])
		invFreq[i] = hConj / complex(hMagSq+epsilon, 0)
	}

	// Inverse FFT
	invTime := make([]complex128, fftSize)
	if err := plan.Inverse(invTime, invFreq); err != nil {
		return nil, fmt.Errorf("sweep: inverse FFT failed: %w", err)
	}

	result := make([]float64, n)
	for i := range result {
		result[i] = real(invTime[i])
	}

	return result, nil
}

// Deconvolve recovers the impulse response from a recorded linear sweep response.
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

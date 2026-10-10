package design

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// ErrInvalidFirwin2 reports parameters [Firwin2] cannot design a filter for.
// Errors are wrapped with context, so test for it with errors.Is.
var ErrInvalidFirwin2 = errors.New("design: invalid firwin2 parameters")

// Option configures [Firwin2].
type Option func(*firwin2Config) error

type firwin2Config struct {
	nfreqs     int // 0 selects the default
	fs         float64
	noWindow   bool
	windowType window.Type
	windowOpts []window.Option
}

func defaultFirwin2Config() firwin2Config {
	return firwin2Config{fs: 2, windowType: window.TypeHamming}
}

// WithNFreqs sets the size of the interpolation mesh used to sample the
// desired response (scipy's nfreqs). It must be greater than numtaps. The
// default is 1 + 2^ceil(log2(numtaps)).
func WithNFreqs(n int) Option {
	return func(c *firwin2Config) error {
		if n < 2 {
			return fmt.Errorf("%w: nfreqs %d < 2", ErrInvalidFirwin2, n)
		}

		c.nfreqs = n

		return nil
	}
}

// WithSampleRate sets the sample rate the frequencies are given in (scipy's
// fs). The band edges then run from 0 to fs/2. The default is 2, so that the
// frequencies are normalized to the Nyquist frequency.
func WithSampleRate(fs float64) Option {
	return func(c *firwin2Config) error {
		if !(fs > 0) || math.IsInf(fs, 0) {
			return fmt.Errorf("%w: sample rate %g", ErrInvalidFirwin2, fs)
		}

		c.fs = fs

		return nil
	}
}

// WithWindow selects the window applied to the taps. It is generated in its
// symmetric form, window.Generate(t, numtaps, opts...), as scipy's
// get_window(window, numtaps, fftbins=False) does; opts are passed through,
// for example window.WithAlpha for the Kaiser beta. The default is Hamming.
// A single-tap filter is never windowed, as in scipy.
func WithWindow(t window.Type, opts ...window.Option) Option {
	optsCopy := append([]window.Option(nil), opts...)

	return func(c *firwin2Config) error {
		c.noWindow = false
		c.windowType = t
		c.windowOpts = optsCopy

		return nil
	}
}

// WithoutWindow leaves the truncated inverse transform unwindowed (scipy's
// window=None).
func WithoutWindow() Option {
	return func(c *firwin2Config) error {
		c.noWindow = true

		return nil
	}
}

// Firwin2 designs a linear-phase FIR filter with numtaps taps by the
// frequency-sampling method, matching scipy.signal.firwin2.
//
// freq and gain describe the desired magnitude response as a piecewise-linear
// curve: gain[i] at freq[i]. freq must start at 0, end at the Nyquist
// frequency (1 by default, fs/2 with [WithSampleRate]) and be non-decreasing.
// A frequency may appear twice to describe a step; 0 and the Nyquist
// frequency may not. The response is interpolated on a uniform mesh of
// nfreqs points (see [WithNFreqs]), phase-shifted so that the first numtaps
// samples of its inverse real FFT are the centred impulse response, truncated
// to numtaps and multiplied by the window (Hamming unless [WithWindow] or
// [WithoutWindow] is given).
//
// An odd numtaps gives a type I filter and an even numtaps a type II filter,
// which must have zero gain at Nyquist. The antisymmetric types III and IV are
// not supported.
//
// The returned slice is newly allocated. freq and gain are not modified.
func Firwin2(numtaps int, freq, gain []float64, opts ...Option) ([]float64, error) {
	cfg := defaultFirwin2Config()

	for _, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: nil option", ErrInvalidFirwin2)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	err := validateFirwin2(numtaps, freq, gain, cfg)
	if err != nil {
		return nil, err
	}

	nyq := 0.5 * cfg.fs

	nfreqs := cfg.nfreqs
	if nfreqs == 0 {
		nfreqs = 1 + 1<<ceilLog2(numtaps)
	}

	// Separate repeated frequencies by one machine epsilon of the Nyquist
	// frequency so the interpolation sees a strictly increasing grid, as
	// scipy does.
	f := append([]float64(nil), freq...)
	eps := 0x1p-52 * nyq

	for k := range len(f) - 1 {
		if f[k] == f[k+1] {
			f[k] -= eps
			f[k+1] += eps
		}
	}

	for k := range len(f) - 1 {
		if f[k+1] <= f[k] {
			return nil, fmt.Errorf("%w: frequencies too close (within %g) to a repeated value", ErrInvalidFirwin2, eps)
		}
	}

	// Desired response on the uniform mesh x[i] = i*nyq/(nfreqs-1), linearly
	// phase-shifted by (numtaps-1)/2 samples.
	step := nyq / float64(nfreqs-1)
	re := make([]float64, nfreqs)
	im := make([]float64, nfreqs)
	delay := -float64(numtaps-1) / 2

	for i := range nfreqs {
		x := float64(i) * step
		if i == nfreqs-1 {
			x = nyq
		}

		fx := interpLinear(x, f, gain)
		theta := delay * math.Pi * x / nyq
		re[i] = fx * math.Cos(theta)
		im[i] = fx * math.Sin(theta)
	}

	taps := irfftHead(re, im, numtaps)

	// scipy's get_window returns [1] for a single tap whatever the window;
	// window.Generate would evaluate the window at its edge instead.
	if !cfg.noWindow && numtaps > 1 {
		w := window.Generate(cfg.windowType, numtaps, cfg.windowOpts...)
		for i := range taps {
			taps[i] *= w[i]
		}
	}

	return taps, nil
}

func validateFirwin2(numtaps int, freq, gain []float64, cfg firwin2Config) error {
	nyq := 0.5 * cfg.fs

	switch {
	case numtaps < 1:
		return fmt.Errorf("%w: numtaps %d < 1", ErrInvalidFirwin2, numtaps)
	case len(freq) != len(gain):
		return fmt.Errorf("%w: %d frequencies but %d gains", ErrInvalidFirwin2, len(freq), len(gain))
	case len(freq) < 2:
		return fmt.Errorf("%w: need at least 2 frequencies, got %d", ErrInvalidFirwin2, len(freq))
	case cfg.nfreqs != 0 && numtaps >= cfg.nfreqs:
		return fmt.Errorf("%w: numtaps %d must be less than nfreqs %d", ErrInvalidFirwin2, numtaps, cfg.nfreqs)
	}

	for i := range freq {
		if math.IsNaN(freq[i]) || math.IsInf(freq[i], 0) || math.IsNaN(gain[i]) || math.IsInf(gain[i], 0) {
			return fmt.Errorf("%w: non-finite value at index %d", ErrInvalidFirwin2, i)
		}
	}

	last := len(freq) - 1

	switch {
	case freq[0] != 0 || freq[last] != nyq:
		return fmt.Errorf("%w: freq must start with 0 and end with %g", ErrInvalidFirwin2, nyq)
	case freq[1] == 0:
		return fmt.Errorf("%w: 0 must not be repeated in freq", ErrInvalidFirwin2)
	case freq[last-1] == nyq:
		return fmt.Errorf("%w: %g must not be repeated in freq", ErrInvalidFirwin2, nyq)
	}

	for k := range last {
		if freq[k+1] < freq[k] {
			return fmt.Errorf("%w: freq must be non-decreasing (index %d)", ErrInvalidFirwin2, k+1)
		}

		if k+2 <= last && freq[k] == freq[k+2] {
			return fmt.Errorf("%w: frequency %g occurs more than twice", ErrInvalidFirwin2, freq[k])
		}
	}

	if numtaps%2 == 0 && gain[last] != 0 {
		return fmt.Errorf("%w: a type II filter (even numtaps) must have zero gain at Nyquist", ErrInvalidFirwin2)
	}

	return nil
}

// interpLinear evaluates the piecewise-linear curve through (xp, fp) at x the
// way numpy.interp does, clamping outside [xp[0], xp[last]].
func interpLinear(x float64, xp, fp []float64) float64 {
	last := len(xp) - 1

	switch {
	case x <= xp[0]:
		return fp[0]
	case x >= xp[last]:
		return fp[last]
	}

	// Largest j with xp[j] <= x; xp is strictly increasing here.
	lo, hi := 0, last
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if xp[mid] <= x {
			lo = mid
		} else {
			hi = mid
		}
	}

	if x == xp[lo] {
		return fp[lo]
	}

	slope := (fp[lo+1] - fp[lo]) / (xp[lo+1] - xp[lo])

	return slope*(x-xp[lo]) + fp[lo]
}

// irfftHead returns the first n samples of the inverse real DFT of the
// half spectrum re + i*im (length m), whose full length is 2*(m-1). As in
// numpy's irfft, the imaginary parts of the DC and Nyquist bins are ignored.
func irfftHead(re, im []float64, n int) []float64 {
	m := len(re)
	size := 2 * (m - 1)
	out := make([]float64, n)

	// Twiddles cos/sin(2*pi*j/size) indexed by (k*t) mod size keep every
	// angle exactly reduced.
	cosT := make([]float64, size)
	sinT := make([]float64, size)

	for j := range size {
		angle := 2 * math.Pi * float64(j) / float64(size)
		cosT[j] = math.Cos(angle)
		sinT[j] = math.Sin(angle)
	}

	for t := range n {
		sum := re[0]
		if t%2 == 0 {
			sum += re[m-1]
		} else {
			sum -= re[m-1]
		}

		acc := 0.0

		for k := 1; k < m-1; k++ {
			j := (k * t) % size
			acc += re[k]*cosT[j] - im[k]*sinT[j]
		}

		out[t] = (sum + 2*acc) / float64(size)
	}

	return out
}

// ceilLog2 returns ceil(log2(n)) for n >= 1.
func ceilLog2(n int) int {
	k := 0
	for 1<<k < n {
		k++
	}

	return k
}

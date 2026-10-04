package restoration

import (
	"fmt"
	"math"
	"math/cmplx"
)

// NoiseProfile owns average, unnormalised FFT powers for one channel. Capture
// uses the same FFT size, window and hop as subsequent reduction.
type NoiseProfile struct {
	FFTSize    int
	SampleRate float64
	power      []float64
	count      int64
}

// NewNoiseProfile constructs an empty capture. FFTSize must be a power of two
// in [256,8192], and sampleRate must be finite and positive.
func NewNoiseProfile(fftSize int, sampleRate float64) (*NoiseProfile, error) {
	if fftSize < 256 || fftSize > 8192 || fftSize&(fftSize-1) != 0 || !finite(sampleRate) || sampleRate <= 0 {
		return nil, fmt.Errorf("restoration.profile: invalid format")
	}

	return &NoiseProfile{FFTSize: fftSize, SampleRate: sampleRate, power: make([]float64, fftSize/2+1)}, nil
}

// AddSpectrum updates the running mean atomically after checking every bin.
func (p *NoiseProfile) AddSpectrum(bins []complex128) error {
	if p == nil || len(bins) != len(p.power) || p.count >= 1<<52 {
		return fmt.Errorf("restoration.profile: invalid bins or count")
	}

	for _, b := range bins {
		if !finite(real(b)) || !finite(imag(b)) || !finite(real(b)*real(b)+imag(b)*imag(b)) {
			return fmt.Errorf("restoration.profile: nonfinite spectrum")
		}
	}

	p.count++

	for i, b := range bins {
		power := real(b)*real(b) + imag(b)*imag(b)
		p.power[i] += (power - p.power[i]) / float64(p.count)
	}

	return nil
}

// Frames returns the number of captured spectral frames.
func (p *NoiseProfile) Frames() int64 {
	if p == nil {
		return 0
	}

	return p.count
}

// Powers returns a defensive copy of the captured noise powers.
func (p *NoiseProfile) Powers() []float64 { return append([]float64(nil), p.power...) }

// NoiseReducer applies profile-based Wiener, subtraction or gate gains. Scratch
// is owned by one caller; ProcessSpectrum is allocation-free.
type NoiseReducer struct {
	profile                 []float64
	gain, previous, scratch []float64
	floor                   float64
	method                  string
}

// NewNoiseReducer freezes a profile. reductionDB is the maximum attenuation
// [0,60]; method is "wiener", "subtraction", or "gate".
func NewNoiseReducer(profile *NoiseProfile, reductionDB float64, method string) (*NoiseReducer, error) {
	if profile == nil || profile.count < 2 || !finite(reductionDB) || reductionDB < 0 || reductionDB > 60 || (method != "wiener" && method != "subtraction" && method != "gate") {
		return nil, fmt.Errorf("restoration.reduce: invalid profile or settings")
	}

	n := len(profile.power)

	r := &NoiseReducer{profile: profile.Powers(), gain: make([]float64, n), previous: make([]float64, n), scratch: make([]float64, n), floor: math.Pow(10, -reductionDB/20), method: method}
	for i := range r.gain {
		r.gain[i] = r.floor
	}

	return r, nil
}

// ProcessSpectrum modifies finite bins in place; invalid input changes no state.
func (r *NoiseReducer) ProcessSpectrum(bins []complex128) error {
	if r == nil || len(bins) != len(r.profile) {
		return fmt.Errorf("restoration.reduce: invalid bins")
	}

	for _, b := range bins {
		if !finite(real(b)) || !finite(imag(b)) || !finite(cmplx.Abs(b)*cmplx.Abs(b)) {
			return fmt.Errorf("restoration.reduce: nonfinite bins")
		}
	}

	for k, b := range bins {
		power := real(b)*real(b) + imag(b)*imag(b)
		noise := math.Max(r.profile[k], 1e-24)
		gamma := power / noise
		gain := 1.0

		switch r.method {
		case "wiener":
			xi := 0.96*r.previous[k]/noise + 0.04*math.Max(gamma-1, 0)
			gain = xi / (1 + xi)
		case "subtraction":
			gain = math.Sqrt(math.Max(0, 1-1.5/math.Max(gamma, 1e-24)))
		case "gate":
			if gamma < 2 {
				gain = r.floor
			}
		}

		r.scratch[k] = math.Max(r.floor, math.Min(1, gain))
	}

	for k, b := range bins {
		// A five-bin neighbourhood suppresses isolated random gain openings.
		sum, weight := 0.0, 0.0

		for j := max(0, k-2); j <= min(len(bins)-1, k+2); j++ {
			w := float64(3 - absInt(j-k))
			sum += r.scratch[j] * w
			weight += w
		}

		target := 0.9*r.scratch[k] + 0.1*sum/weight

		coefficient := 0.18
		if target < r.gain[k] {
			coefficient = 0.08
		}

		r.gain[k] += coefficient * (target - r.gain[k])
		bins[k] = b * complex(r.gain[k], 0)
		r.previous[k] = real(bins[k])*real(bins[k]) + imag(bins[k])*imag(bins[k])
	}

	return nil
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}

	return x
}

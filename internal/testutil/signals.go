package testutil

import (
	"math"
	"math/rand"
)

// DeterministicSine generates a deterministic sine wave.
func DeterministicSine(freqHz, sampleRate, amplitude float64, length int) []float64 {
	out := make([]float64, length)
	step := 2 * math.Pi * freqHz / sampleRate
	for i := range out {
		out[i] = amplitude * math.Sin(step*float64(i))
	}
	return out
}

// DeterministicNoise generates white noise with a fixed seed for reproducibility.
func DeterministicNoise(seed int64, amplitude float64, length int) []float64 {
	out := make([]float64, length)
	rng := rand.New(rand.NewSource(seed))
	for i := range out {
		out[i] = (rng.Float64()*2 - 1) * amplitude
	}
	return out
}

// Impulse generates a unit impulse at the given position.
func Impulse(length, pos int) []float64 {
	out := make([]float64, length)
	if pos >= 0 && pos < length {
		out[pos] = 1
	}
	return out
}

// DC generates a constant-valued signal.
func DC(value float64, length int) []float64 {
	out := make([]float64, length)
	for i := range out {
		out[i] = value
	}
	return out
}

// Ones returns a slice of length n filled with 1.0.
func Ones(n int) []float64 {
	return DC(1.0, n)
}

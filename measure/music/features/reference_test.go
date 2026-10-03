//nolint:all // Verbatim reference copy; kept byte-for-byte close to the original.
package features_test

// Verbatim copy (adapted to standalone functions) of the AudioVisualizer
// reference implementation in internal/audioanalysis (features.go, rhythm.go,
// types.go, load.go). The parity tests compare this package against it
// bit for bit. Do not "fix" or reformat the arithmetic below.

import (
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/dsp/window"
	frequency "github.com/cwbudde/algo-dsp/stats/frequency"
	fft "github.com/cwbudde/algo-fft"
)

const (
	SampleRate = 24000
	FFTSize    = 2048
	Hop        = 240
)

var BandEdges = [6]float64{25, 140, 400, 2000, 6000, 12000}

type refTrack struct {
	RMS         []float64
	Peak        []float64
	Centroid    []float64
	Width       []float64
	Flux        []float64
	Bands       [5][]float64
	Spectrogram []float64
}

func DB(v float64) float64 { return 20 * math.Log10(math.Max(v, 1e-6)) }

func refAnalyze(channels [][]float64) (*refTrack, error) {
	length := len(channels[0])
	count := (length + Hop - 1) / Hop
	t := &refTrack{RMS: make([]float64, count), Peak: make([]float64, count), Centroid: make([]float64, count), Width: make([]float64, count), Flux: make([]float64, count), Spectrogram: make([]float64, count*64)}
	for b := range t.Bands {
		t.Bands[b] = make([]float64, count)
	}
	plan, err := fft.NewPlanReal64(FFTSize)
	if err != nil {
		return nil, err
	}
	w, err := window.Hann(FFTSize, window.WithPeriodic())
	if err != nil {
		return nil, err
	}
	windowPower := 0.0
	for _, v := range w {
		windowPower += v * v
	}
	buf := make([]float64, FFTSize)
	sp := make([]complex128, FFTSize/2+1)
	power := make([]float64, len(sp))
	mag := make([]float64, len(sp))
	previous := make([]float64, len(sp))
	for frame := 0; frame < count; frame++ {
		center := frame * Hop
		clear(power)
		for _, ch := range channels {
			for i := range buf {
				j := center + i - FFTSize/2
				buf[i] = 0
				if j >= 0 && j < length {
					buf[i] = ch[j] * w[i]
				}
			}
			if err := plan.Forward(sp, buf); err != nil {
				return nil, err
			}
			for k, v := range sp {
				power[k] += (real(v)*real(v) + imag(v)*imag(v)) / float64(len(channels))
			}
		}
		for k, p := range power {
			mag[k] = math.Sqrt(p)
			hz := float64(k) * SampleRate / FFTSize
			logmag := math.Log1p(mag[k])
			if frame > 0 {
				t.Flux[frame] += math.Max(0, logmag-previous[k])
			}
			previous[k] = logmag
			for b := range t.Bands {
				if hz >= BandEdges[b] && hz < BandEdges[b+1] {
					t.Bands[b][frame] += 2 * p / (FFTSize * windowPower)
				}
			}
			if hz >= 25 && hz < 12000 {
				bin := min(63, int(math.Log(hz/25)/math.Log(12000.0/25)*64))
				t.Spectrogram[frame*64+bin] += 2 * p / (FFTSize * windowPower)
			}
		}
		for b := range t.Bands {
			t.Bands[b][frame] = math.Sqrt(t.Bands[b][frame])
		}
		for b := 0; b < 64; b++ {
			t.Spectrogram[frame*64+b] = DB(math.Sqrt(t.Spectrogram[frame*64+b]))
		}
		t.Centroid[frame] = frequency.Centroid(mag, SampleRate)
		n, energy, mid, side := 0.0, 0.0, 0.0, 0.0
		for j := max(0, center-Hop); j < min(length, center+Hop); j++ {
			for _, ch := range channels {
				v := ch[j]
				energy += v * v
				n++
				t.Peak[frame] = math.Max(t.Peak[frame], math.Abs(v))
			}
			if len(channels) == 2 {
				l, r := channels[0][j], channels[1][j]
				mid += (l + r) * (l + r) / 4
				side += (l - r) * (l - r) / 4
			}
		}
		t.RMS[frame] = math.Sqrt(energy / math.Max(1, n))
		if mid+side > 1e-12 {
			t.Width[frame] = side / (mid + side)
		}
		if t.RMS[frame] < 1e-4 {
			t.Centroid[frame] = 0
			t.Flux[frame] = 0
		}
	}
	return t, nil
}

func percentile(x []float64, p float64) float64 {
	if len(x) == 0 {
		return 0
	}
	y := append([]float64(nil), x...)
	sort.Float64s(y)
	return y[int(math.Round(p*float64(len(y)-1)))]
}

func refNormalize(x []float64, attack, release float64) []float64 {
	active := make([]float64, 0, len(x))
	for _, v := range x {
		if v > 1e-4 {
			active = append(active, v)
		}
	}
	hi := percentile(active, 0.95)
	y := make([]float64, len(x))
	if hi < 1e-4 {
		return y
	}
	state := 0.0
	for i, v := range x {
		target := math.Min(1, math.Max(0, (v-1e-4)/(hi-1e-4)))
		seconds := release
		if target > state {
			seconds = attack
		}
		alpha := 1 - math.Exp(-float64(Hop)/SampleRate/seconds)
		state += alpha * (target - state)
		y[i] = state
	}
	return y
}

type refInterval struct {
	Start float64
	End   float64
}

func refFindSilence(channels [][]float64) []refInterval {
	threshold := math.Pow(10, -45.0/20)
	intervals := []refInterval{}
	start := -1
	for i := 0; i <= len(channels[0]); i++ {
		quiet := i < len(channels[0])
		if quiet {
			for _, ch := range channels {
				if math.Abs(ch[i]) > threshold {
					quiet = false
					break
				}
			}
		}
		if quiet && start < 0 {
			start = i
		}
		if !quiet && start >= 0 {
			if i-start >= int(0.15*SampleRate) {
				intervals = append(intervals, refInterval{float64(start) / SampleRate, float64(i) / SampleRate})
			}
			start = -1
		}
	}
	return intervals
}

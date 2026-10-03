package features_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
	"github.com/cwbudde/algo-dsp/measure/music/features"
)

// logSweep returns an exponential sine sweep from f0 to f1 Hz.
func logSweep(f0, f1, seconds, amplitude float64) []float64 {
	n := int(seconds * SampleRate)
	x := make([]float64, n)
	k := math.Log(f1 / f0)

	for i := range x {
		t := float64(i) / SampleRate
		phase := 2 * math.Pi * f0 * seconds / k * (math.Exp(t/seconds*k) - 1)
		x[i] = amplitude * math.Sin(phase)
	}

	return x
}

func TestLogSpectrogramSweepHasNoEmptyRows(t *testing.T) {
	t.Parallel()

	const bins = 64

	x := logSweep(20, 12500, 6, 0.5)

	f, err := features.Extract([][]float64{x}, features.DefaultConfig(), features.WithLogSpectrogram(bins, 25, 12000))
	if err != nil {
		t.Fatal(err)
	}

	s := f.Spectrogram
	if s.Scale.Bins() != bins || len(s.DB) != f.Len()*bins {
		t.Fatalf("shape %d bins, %d values for %d frames", s.Scale.Bins(), len(s.DB), f.Len())
	}

	rowMax := make([]float64, bins)
	for b := range rowMax {
		rowMax[b] = math.Inf(-1)
	}

	for i := range f.Len() {
		for b, v := range s.Frame(i) {
			rowMax[b] = math.Max(rowMax[b], v)
		}
	}

	// Every row lights up when the sweep passes it. The 0.5 sine is -9 dBFS
	// RMS; the narrow low rows get a proportional share of an FFT bin.
	for b, v := range rowMax {
		if v < -25 {
			t.Errorf("row %d (%.1f Hz) peaks at %.1f dB: empty row", b, s.BinFrequencies()[b], v)
		}

		if b > 0 && math.Abs(v-rowMax[b-1]) > 3 {
			t.Errorf("row %d jumps %.1f dB from its neighbour", b, v-rowMax[b-1])
		}
	}

	// Above the region where log bins are narrower than an FFT bin, a row
	// holds the whole sine and reads ≈ its RMS.
	if v := rowMax[bins-10]; math.Abs(v-(-9.03)) > 1.5 {
		t.Errorf("high row peaks at %.2f dB, want ≈ -9 dB", v)
	}
}

func TestLogSpectrogramToneLandsInItsRow(t *testing.T) {
	t.Parallel()

	for _, hz := range []float64{300, 1000, 5000, 10000} {
		x := testutil.DeterministicSine(hz, SampleRate, 0.5, SampleRate)

		f, err := features.Extract([][]float64{x}, features.DefaultConfig(), features.WithLogSpectrogram(64, 25, 12000))
		if err != nil {
			t.Fatal(err)
		}

		row := f.Spectrogram.Frame(50)
		peak := 0

		for b, v := range row {
			if v > row[peak] {
				peak = b
			}
		}

		edges := f.Spectrogram.Scale.Edges()
		// Allow the neighbour row when the tone sits on a row edge.
		lo, hi := edges[max(0, peak-1)], edges[min(len(edges)-1, peak+2)]

		if hz < lo || hz >= hi {
			t.Errorf("%v Hz peaks in row %d (%.1f–%.1f Hz)", hz, peak, edges[peak], edges[peak+1])
		}
	}
}

func TestLogScaleEnergyPreserving(t *testing.T) {
	t.Parallel()

	const (
		n          = 2048
		sampleRate = 24000.0
	)

	s, err := features.NewLogScale(64, 25, 12000, sampleRate, n)
	if err != nil {
		t.Fatal(err)
	}

	// A flat spectrum of unit power per FFT bin carries (fmax-fmin)/Δ.
	power := make([]float64, n/2+1)
	for k := range power {
		power[k] = 1
	}

	dst := make([]float64, s.Bins())

	err = s.Apply(dst, power)
	if err != nil {
		t.Fatal(err)
	}

	total := 0.0

	for b, v := range dst {
		if v <= 0 {
			t.Fatalf("log bin %d is empty", b)
		}

		total += v
	}

	want := (12000 - 25) / (sampleRate / n)
	if math.Abs(total-want) > 1e-9 {
		t.Fatalf("total power %v, want %v", total, want)
	}

	freqs := s.BinFrequencies()
	edges := s.Edges()

	if len(freqs) != 64 || len(edges) != 65 || edges[0] != 25 || edges[64] != 12000 {
		t.Fatalf("grid %d centres, %d edges %v..%v", len(freqs), len(edges), edges[0], edges[64])
	}

	for b, f := range freqs {
		if !(f > edges[b] && f < edges[b+1]) {
			t.Fatalf("centre %v outside [%v, %v)", f, edges[b], edges[b+1])
		}
	}
}

func TestLogScaleErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bins       int
		fmin, fmax float64
		rate       float64
		n          int
	}{
		{"bins", 0, 25, 12000, 24000, 2048},
		{"fft", 8, 25, 12000, 24000, 1},
		{"rate", 8, 25, 12000, 0, 2048},
		{"fmin", 8, 0, 12000, 24000, 2048},
		{"order", 8, 100, 50, 24000, 2048},
		{"nyquist", 8, 25, 12001, 24000, 2048},
	}

	for _, tc := range tests {
		_, err := features.NewLogScale(tc.bins, tc.fmin, tc.fmax, tc.rate, tc.n)
		if !errors.Is(err, features.ErrInvalidArgument) {
			t.Errorf("%s: err = %v", tc.name, err)
		}
	}

	s, err := features.NewLogScale(8, 25, 12000, 24000, 2048)
	if err != nil {
		t.Fatal(err)
	}

	err = s.Apply(make([]float64, 7), make([]float64, 1025))
	if !errors.Is(err, features.ErrInvalidArgument) {
		t.Errorf("Apply with short dst: err = %v", err)
	}
}

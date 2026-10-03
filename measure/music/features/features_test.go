package features_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/stft"
	"github.com/cwbudde/algo-dsp/internal/testutil"
	"github.com/cwbudde/algo-dsp/measure/music/features"
)

func TestStereoPowerAndFrequencyBands(t *testing.T) {
	t.Parallel()

	x := testutil.DeterministicSine(1000, SampleRate, 0.5, SampleRate)
	opposite := make([]float64, len(x))

	for i, v := range x {
		opposite[i] = -v
	}

	f, err := features.Extract([][]float64{x, opposite}, features.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	const i = 50

	if math.Abs(f.RMS[i]-0.5/math.Sqrt2) > 0.001 {
		t.Fatalf("anti-phase stereo lost energy: %f", f.RMS[i])
	}

	if math.Abs(f.Centroid[i]-1000) > 20 || f.Width[i] < 0.999 {
		t.Fatalf("wrong spectral shape: centroid %f width %f", f.Centroid[i], f.Width[i])
	}

	if f.Bands[2][i] < 0.34 {
		t.Fatalf("tone missing from 400–2000 Hz band: %f", f.Bands[2][i])
	}

	for b := range f.Bands {
		if b != 2 && f.Bands[b][i] > 0.005 {
			t.Fatalf("tone leaks into band %d: %f", b, f.Bands[b][i])
		}
	}
}

// TestFullRangeBandParseval checks the one-sided band scaling at the
// spectrum edges: a band over every FFT bin, DC and Nyquist included, must
// read the window-weighted RMS of the frame, sqrt(Σ(x·w)²/Σw²). DC and the
// Nyquist bin of an even FFT size have no negative-frequency partner and
// count once; weighting them twice inflated a constant signal by sqrt(5/3).
func TestFullRangeBandParseval(t *testing.T) {
	t.Parallel()

	const (
		n     = 2048
		hop   = 64
		frame = 10
	)

	constant := make([]float64, n)
	alternating := make([]float64, n)

	for i := range constant {
		constant[i] = 0.5
		alternating[i] = 0.5 * float64(1-2*(i%2))
	}

	noise := testutil.DeterministicNoise(5, 0.5, n)

	tests := []struct {
		name    string
		fftSize int
		x       []float64
	}{
		{"DC even FFT", 256, constant},
		{"DC odd FFT", 255, constant},
		{"Nyquist even FFT", 256, alternating},
		{"noise even FFT", 256, noise},
		{"noise odd FFT", 255, noise},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := features.Config{
				SampleRate: SampleRate,
				FFTSize:    tc.fftSize,
				Hop:        hop,
				BandEdges:  []float64{0, SampleRate},
			}

			f, err := features.Extract([][]float64{tc.x}, cfg)
			if err != nil {
				t.Fatal(err)
			}

			tr, err := stft.New(tc.fftSize, hop)
			if err != nil {
				t.Fatal(err)
			}

			start := frame*hop - tc.fftSize/2
			energy, windowPower := 0.0, 0.0

			for k, w := range tr.Window() {
				v := tc.x[start+k] * w
				energy += v * v
				windowPower += w * w
			}

			want := math.Sqrt(energy / windowPower)
			if got := f.Bands[0][frame]; math.Abs(got-want) > 1e-12*want {
				t.Fatalf("band RMS %.15g, want %.15g", got, want)
			}
		})
	}
}

func TestSilenceFinite(t *testing.T) {
	t.Parallel()

	x := make([]float64, SampleRate)

	f, err := features.Extract([][]float64{x}, features.DefaultConfig(), features.WithLogSpectrogram(64, 25, 12000))
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range [][]float64{f.RMS, f.Peak, f.Centroid, f.Width, f.Flux, f.Spectrogram.DB} {
		testutil.RequireFinite(t, s)
	}

	if f.Spectrogram.DB[0] != -120 {
		t.Fatalf("silent spectrogram %v dB, want -120", f.Spectrogram.DB[0])
	}

	y, err := features.Normalize(f.RMS, features.DefaultGate, 0.01, 0.15, f.FrameRate())
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range y {
		if v != 0 {
			t.Fatal("silent controls nonzero")
		}
	}

	intervals, err := features.Silence([][]float64{x}, SampleRate, -45, 0.15)
	if err != nil {
		t.Fatal(err)
	}

	if len(intervals) != 1 || intervals[0].Start != 0 || intervals[0].End != 1 {
		t.Fatalf("silence intervals: %+v", intervals)
	}
}

func TestSilenceGapLength(t *testing.T) {
	t.Parallel()

	x := testutil.DeterministicSine(300, SampleRate, 0.5, 2*SampleRate)
	clear(x[12000:18000]) // 250 ms gap at 0.5 s
	clear(x[30000:33000]) // 125 ms gap: shorter than the minimum

	intervals, err := features.Silence([][]float64{x}, SampleRate, -45, 0.15)
	if err != nil {
		t.Fatal(err)
	}

	// A 300 Hz sine crosses zero inside the threshold for a few samples, so
	// the detected gap may extend by up to a sample or two at each edge.
	if len(intervals) != 1 || math.Abs(intervals[0].Start-0.5) > 1e-3 || math.Abs(intervals[0].End-0.75) > 1e-3 {
		t.Fatalf("intervals %+v, want one at 0.5..0.75 s", intervals)
	}
}

func TestNormalizeBoundsAndSmoothing(t *testing.T) {
	t.Parallel()

	x := make([]float64, 300)
	for i := 100; i < 200; i++ {
		x[i] = 1
	}

	y, err := features.Normalize(x, 1e-4, 0.01, 0.1, 100)
	if err != nil {
		t.Fatal(err)
	}

	for i, v := range y {
		if v < 0 || v > 1 || math.IsNaN(v) {
			t.Fatalf("y[%d] = %v out of [0, 1]", i, v)
		}
	}

	// Attack τ = 10 ms at 100 frames/s: one step covers 1-1/e of the gap.
	if math.Abs(y[100]-(1-math.Exp(-1))) > 1e-12 {
		t.Fatalf("attack step %v", y[100])
	}

	// Release τ = 100 ms: ten frames after the drop the state is ≈ 1/e.
	if math.Abs(y[209]-y[199]*math.Exp(-1)) > 1e-12 {
		t.Fatalf("release %v -> %v", y[199], y[209])
	}

	// Zero time constants follow the target instantly.
	y, err = features.Normalize(x, 0, 0, 0, 100)
	if err != nil {
		t.Fatal(err)
	}

	if y[99] != 0 || y[100] != 1 || y[200] != 0 {
		t.Fatalf("instant follower %v %v %v", y[99], y[100], y[200])
	}
}

func TestPercentile(t *testing.T) {
	t.Parallel()

	x := []float64{5, 1, 4, 2, 3}
	tests := []struct {
		p, want float64
	}{
		{0, 1}, {0.5, 3}, {1, 5}, {0.95, 5}, {-1, 1}, {2, 5}, {math.NaN(), 1},
	}

	for _, tc := range tests {
		if got := features.Percentile(x, tc.p); got != tc.want {
			t.Errorf("Percentile(%v) = %v, want %v", tc.p, got, tc.want)
		}
	}

	if features.Percentile(nil, 0.5) != 0 {
		t.Error("empty percentile not zero")
	}

	if x[0] != 5 {
		t.Error("Percentile modified its input")
	}
}

func TestTiming(t *testing.T) {
	t.Parallel()

	tm := features.DefaultConfig().Timing()
	if tm.FrameRate() != 100 || tm.FrameTime(150) != 1.5 || tm.FramePosition(1.5) != 150 {
		t.Fatalf("timing %v %v %v", tm.FrameRate(), tm.FrameTime(150), tm.FramePosition(1.5))
	}
}

func TestExtractErrors(t *testing.T) {
	t.Parallel()

	good := [][]float64{make([]float64, 100)}
	mod := func(f func(*features.Config)) features.Config {
		c := features.DefaultConfig()
		f(&c)

		return c
	}

	tests := []struct {
		name     string
		channels [][]float64
		cfg      features.Config
		opts     []features.Option
		want     error
	}{
		{"no channels", nil, features.DefaultConfig(), nil, features.ErrNoAudio},
		{"empty channel", [][]float64{{}}, features.DefaultConfig(), nil, features.ErrNoAudio},
		{"unequal", [][]float64{make([]float64, 10), make([]float64, 9)}, features.DefaultConfig(), nil, features.ErrChannelLength},
		{"rate", good, mod(func(c *features.Config) { c.SampleRate = 0 }), nil, features.ErrInvalidConfig},
		{"rate inf", good, mod(func(c *features.Config) { c.SampleRate = math.Inf(1) }), nil, features.ErrInvalidConfig},
		{"hop", good, mod(func(c *features.Config) { c.Hop = 0 }), nil, features.ErrInvalidConfig},
		{"fft", good, mod(func(c *features.Config) { c.FFTSize = 1 }), nil, features.ErrInvalidConfig},
		{"edges", good, mod(func(c *features.Config) { c.BandEdges = []float64{1} }), nil, features.ErrInvalidConfig},
		{"edge nan", good, mod(func(c *features.Config) { c.BandEdges = []float64{math.NaN(), 1} }), nil, features.ErrInvalidConfig},
		{"edge order", good, mod(func(c *features.Config) { c.BandEdges = []float64{2, 1} }), nil, features.ErrInvalidConfig},
		{"gate", good, mod(func(c *features.Config) { c.Gate = -1 }), nil, features.ErrInvalidConfig},
		{"nil option", good, features.DefaultConfig(), []features.Option{nil}, features.ErrNilOption},
		{"log bins", good, features.DefaultConfig(), []features.Option{features.WithLogSpectrogram(0, 25, 12000)}, features.ErrInvalidArgument},
		{"log range", good, features.DefaultConfig(), []features.Option{features.WithLogSpectrogram(8, 25, 20000)}, features.ErrInvalidArgument},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := features.Extract(tc.channels, tc.cfg, tc.opts...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNormalizeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                           string
		gate, attack, release, frameRt float64
	}{
		{"gate", -1, 0, 0, 100},
		{"attack", 0, math.NaN(), 0, 100},
		{"release", 0, 0, math.Inf(1), 100},
		{"rate", 0, 0, 0, 0},
		{"rate inf", 0, 0, 0, math.Inf(1)},
	}

	for _, tc := range tests {
		_, err := features.Normalize([]float64{1}, tc.gate, tc.attack, tc.release, tc.frameRt)
		if !errors.Is(err, features.ErrInvalidArgument) {
			t.Errorf("%s: err = %v", tc.name, err)
		}
	}
}

func TestSilenceErrors(t *testing.T) {
	t.Parallel()

	good := [][]float64{make([]float64, 10)}
	tests := []struct {
		name     string
		channels [][]float64
		rate, db float64
		min      float64
		want     error
	}{
		{"empty", nil, 1, -45, 0, features.ErrNoAudio},
		{"rate", good, 0, -45, 0, features.ErrInvalidArgument},
		{"db", good, 1, math.NaN(), 0, features.ErrInvalidArgument},
		{"min", good, 1, -45, -1, features.ErrInvalidArgument},
	}

	for _, tc := range tests {
		_, err := features.Silence(tc.channels, tc.rate, tc.db, tc.min)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestFloorSamples(t *testing.T) {
	t.Parallel()

	tests := []struct {
		x    float64
		want int
	}{
		{0.15 * 24000, 3600}, {0.005 * 44100, 220}, {0.005 * 24000, 120}, {0.002 * 48000, 96}, {2.5, 2}, {0, 0},
	}

	for _, tc := range tests {
		if got := features.FloorSamples(tc.x); got != tc.want {
			t.Errorf("FloorSamples(%v) = %d, want %d", tc.x, got, tc.want)
		}
	}
}

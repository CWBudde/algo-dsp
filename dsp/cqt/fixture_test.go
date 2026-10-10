package cqt

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// fixtureConfig mirrors the "config" object written by
// scripts/fixtures/cqt/gen.py.
type fixtureConfig struct {
	SR             float64 `json:"sr"`
	Hop            int     `json:"hop"`
	FMin           float64 `json:"fmin"`
	NBins          int     `json:"n_bins"`
	BinsPerOctave  int     `json:"bins_per_octave"`
	FilterScale    float64 `json:"filter_scale"`
	BasisNorm      int     `json:"basis_norm"`
	Window         string  `json:"window"`
	PadMode        string  `json:"pad_mode"`
	EarlyDownsampl bool    `json:"earlydownsample"`
	Normalization  string  `json:"normalization"`
	Output         string  `json:"output"`
}

// fixture is one transform case of dsp/cqt/testdata.
type fixture struct {
	Name             string        `json:"-"`
	Config           fixtureConfig `json:"config"`
	Length           int           `json:"length"`
	NFFT             int           `json:"n_fft"`
	NOctaves         int           `json:"n_octaves"`
	DownsampleFactor int           `json:"downsample_factor"`
	HopEffective     int           `json:"hop_effective"`
	ReflectFallback  bool          `json:"reflect_fallback"`
	Frequencies      []float64     `json:"frequencies"`
	Lengths          []float64     `json:"lengths"`
	KernelsReal      []float64     `json:"kernels_real"`
	KernelsImag      []float64     `json:"kernels_imag"`
	Lowpass          []float64     `json:"lowpass"`
	EarlyLowpass     []float64     `json:"early_lowpass"`
	Frames           int           `json:"frames"`
	F64              []float64     `json:"f64"`
	F32              []float64     `json:"f32"`
	Librosa          []float64     `json:"librosa"`
}

func readGzipJSON(tb testing.TB, path string, v any) {
	tb.Helper()

	fh, err := os.Open(path)
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = fh.Close() }()

	gz, err := gzip.NewReader(fh)
	if err != nil {
		tb.Fatalf("%s: %v", path, err)
	}
	defer func() { _ = gz.Close() }()

	err = json.NewDecoder(gz).Decode(v)
	if err != nil {
		tb.Fatalf("%s: %v", path, err)
	}
}

var (
	signalOnce sync.Once
	signalData []float64
)

// testSignal returns the shared fixture input: 43844 samples at 22050 Hz,
// each exactly representable in float32.
func testSignal(tb testing.TB) []float64 {
	tb.Helper()

	signalOnce.Do(func() {
		var s struct {
			X []float64 `json:"x"`
		}

		readGzipJSON(tb, filepath.Join("testdata", "signal.json.gz"), &s)
		signalData = s.X
	})

	if len(signalData) == 0 {
		tb.Fatal("empty fixture signal")
	}

	return signalData
}

// loadFixtures returns every transform case in testdata, sorted by name.
func loadFixtures(tb testing.TB) []fixture {
	tb.Helper()

	paths, err := filepath.Glob(filepath.Join("testdata", "*.json.gz"))
	if err != nil {
		tb.Fatal(err)
	}

	slices.Sort(paths)

	var out []fixture

	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".json.gz")
		if name == "signal" {
			continue
		}

		var fx fixture
		readGzipJSON(tb, p, &fx)
		fx.Name = name
		out = append(out, fx)
	}

	if len(out) == 0 {
		tb.Fatal("no fixtures found")
	}

	return out
}

func loadFixture(tb testing.TB, name string) fixture {
	tb.Helper()

	var fx fixture
	readGzipJSON(tb, filepath.Join("testdata", name+".json.gz"), &fx)
	fx.Name = name

	return fx
}

// options translates a fixture configuration into Options.
func (c fixtureConfig) options(tb testing.TB) []Option {
	tb.Helper()

	windows := map[string]window.Type{
		"hann":     window.TypeHann,
		"hamming":  window.TypeHamming,
		"blackman": window.TypeBlackman,
	}
	pads := map[string]Padding{"reflect": PadReflect, "constant": PadConstant}
	norms := map[string]Normalization{
		"librosa":       NormalizationLibrosa,
		"convolutional": NormalizationConvolutional,
		"wrap":          NormalizationWrap,
	}
	outputs := map[string]Output{"magnitude": OutputMagnitude, "complex": OutputComplex}

	w, ok1 := windows[c.Window]
	p, ok2 := pads[c.PadMode]
	n, ok3 := norms[c.Normalization]
	o, ok4 := outputs[c.Output]

	if !ok1 || !ok2 || !ok3 || !ok4 {
		tb.Fatalf("unsupported fixture config %+v", c)
	}

	return []Option{
		WithHopLength(c.Hop),
		WithFMin(c.FMin),
		WithBins(c.NBins),
		WithBinsPerOctave(c.BinsPerOctave),
		WithFilterScale(c.FilterScale),
		WithBasisNorm(Norm(c.BasisNorm)),
		WithWindow(w),
		WithPadding(p),
		WithEarlyDownsampling(c.EarlyDownsampl),
		WithNormalization(n),
		WithOutput(o),
	}
}

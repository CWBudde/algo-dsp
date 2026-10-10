package design_test

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

// firwin2GoldenTol is the absolute tolerance against scipy.signal.firwin2.
// The measured error is at the double-precision rounding level (<= 2.2e-16).
const firwin2GoldenTol = 1e-14

// firwin2KaiserTol is the looser tolerance for the Kaiser-windowed case.
// dsp/window evaluates the Kaiser window with the Abramowitz-Stegun polynomial
// approximation of I0 (about 1e-7 relative accuracy), whereas scipy uses a
// full-precision Bessel function; the taps differ by about 1.5e-10. This is a
// limitation of dsp/window, not of Firwin2.
const firwin2KaiserTol = 1e-9

type firwin2Fixture struct {
	Versions map[string]string `json:"versions"`
	Cases    []firwin2Case     `json:"cases"`
}

type firwin2Case struct {
	Name        string    `json:"name"`
	NumTaps     int       `json:"numtaps"`
	Freq        []float64 `json:"freq"`
	Gain        []float64 `json:"gain"`
	Taps        []float64 `json:"taps"`
	NFreqs      *int      `json:"nfreqs"`
	FS          *float64  `json:"fs"`
	Window      *string   `json:"window"`
	WindowParam *float64  `json:"window_param"`
}

func loadFirwin2Fixture(t *testing.T) firwin2Fixture {
	t.Helper()

	fh, err := os.Open(filepath.Join("testdata", "firwin2.json.gz"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}

	defer func() { _ = fh.Close() }()

	gz, err := gzip.NewReader(fh)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}

	defer func() { _ = gz.Close() }()

	var fx firwin2Fixture

	err = json.NewDecoder(gz).Decode(&fx)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	if len(fx.Cases) == 0 {
		t.Fatal("fixture has no cases")
	}

	return fx
}

// options translates the scipy keyword arguments of a fixture case.
func (c firwin2Case) options(t *testing.T) []design.Option {
	t.Helper()

	var opts []design.Option

	if c.NFreqs != nil {
		opts = append(opts, design.WithNFreqs(*c.NFreqs))
	}

	if c.FS != nil {
		opts = append(opts, design.WithSampleRate(*c.FS))
	}

	if c.Window != nil {
		switch *c.Window {
		case "none":
			opts = append(opts, design.WithoutWindow())
		case "hann":
			opts = append(opts, design.WithWindow(window.TypeHann))
		case "blackman":
			opts = append(opts, design.WithWindow(window.TypeBlackman))
		case "kaiser":
			if c.WindowParam == nil {
				t.Fatalf("case %s: kaiser window without beta", c.Name)
			}

			opts = append(opts, design.WithWindow(window.TypeKaiser, window.WithAlpha(*c.WindowParam)))
		default:
			t.Fatalf("case %s: unknown window %q", c.Name, *c.Window)
		}
	}

	return opts
}

func maxAbsDiff(a, b []float64) float64 {
	m := 0.0

	for i := range a {
		m = max(m, math.Abs(a[i]-b[i]))
	}

	return m
}

func TestFirwin2Golden(t *testing.T) {
	fx := loadFirwin2Fixture(t)
	t.Logf("fixture versions: %v", fx.Versions)

	for _, tc := range fx.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := design.Firwin2(tc.NumTaps, tc.Freq, tc.Gain, tc.options(t)...)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			if len(got) != len(tc.Taps) || len(got) != tc.NumTaps {
				t.Fatalf("len = %d, want %d (numtaps %d)", len(got), len(tc.Taps), tc.NumTaps)
			}

			tol := firwin2GoldenTol
			if tc.Window != nil && *tc.Window == "kaiser" {
				tol = firwin2KaiserTol
			}

			diff := maxAbsDiff(got, tc.Taps)
			t.Logf("max abs error %.3g (tol %.0e)", diff, tol)

			if diff > tol {
				for i := range got {
					if math.Abs(got[i]-tc.Taps[i]) > tol {
						t.Errorf("tap %d = %.17g, want %.17g", i, got[i], tc.Taps[i])
					}
				}
			}
		})
	}
}

func TestFirwin2Validation(t *testing.T) {
	lpFreq := []float64{0, 0.3, 0.4, 1}
	lpGain := []float64{1, 1, 0, 0}

	tests := []struct {
		name    string
		numtaps int
		freq    []float64
		gain    []float64
		opts    []design.Option
	}{
		{name: "numtaps zero", numtaps: 0, freq: lpFreq, gain: lpGain},
		{name: "numtaps negative", numtaps: -3, freq: lpFreq, gain: lpGain},
		{name: "length mismatch", numtaps: 11, freq: lpFreq, gain: []float64{1, 1, 0}},
		{name: "single frequency", numtaps: 11, freq: []float64{0}, gain: []float64{1}},
		{name: "empty", numtaps: 11, freq: nil, gain: nil},
		{
			name: "nfreqs equal numtaps", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithNFreqs(11)},
		},
		{
			name: "nfreqs below numtaps", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithNFreqs(5)},
		},
		{
			name: "nfreqs one", numtaps: 1, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithNFreqs(1)},
		},
		{
			name: "nfreqs zero", numtaps: 1, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithNFreqs(0)},
		},
		{name: "NaN frequency", numtaps: 11, freq: []float64{0, math.NaN(), 1}, gain: []float64{1, 1, 0}},
		{name: "Inf frequency", numtaps: 11, freq: []float64{0, math.Inf(1), 1}, gain: []float64{1, 1, 0}},
		{name: "NaN gain", numtaps: 11, freq: []float64{0, 0.5, 1}, gain: []float64{1, math.NaN(), 0}},
		{name: "Inf gain", numtaps: 11, freq: []float64{0, 0.5, 1}, gain: []float64{math.Inf(-1), 1, 0}},
		{name: "does not start at 0", numtaps: 11, freq: []float64{0.1, 0.5, 1}, gain: []float64{1, 1, 0}},
		{name: "does not end at Nyquist", numtaps: 11, freq: []float64{0, 0.5, 0.9}, gain: []float64{1, 1, 0}},
		{
			name: "normalized Nyquist with sample rate", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(8000)},
		},
		{name: "0 repeated", numtaps: 11, freq: []float64{0, 0, 1}, gain: []float64{1, 0, 0}},
		{name: "Nyquist repeated", numtaps: 11, freq: []float64{0, 1, 1}, gain: []float64{1, 1, 0}},
		{name: "decreasing", numtaps: 11, freq: []float64{0, 0.6, 0.4, 1}, gain: lpGain},
		{
			name: "value three times", numtaps: 11,
			freq: []float64{0, 0.5, 0.5, 0.5, 1}, gain: []float64{1, 1, 0.5, 0, 0},
		},
		{
			// The repeated 0.5 is separated by eps*nyq = 2^-52 on each side,
			// which pushes it past its neighbour one ulp above 0.5.
			name: "too close to a repeated value", numtaps: 11,
			freq: []float64{0, 0.5, 0.5, math.Nextafter(0.5, 1), 1},
			gain: []float64{1, 1, 0, 0, 0},
		},
		{name: "type II nonzero Nyquist gain", numtaps: 10, freq: []float64{0, 0.5, 1}, gain: []float64{1, 1, 0.5}},
		{name: "nil option", numtaps: 11, freq: lpFreq, gain: lpGain, opts: []design.Option{nil}},
		{
			name: "unknown window type", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithWindow(window.TypeFreeCosine + 1)},
		},
		{
			name: "negative window type", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithWindow(window.Type(-1))},
		},
		{
			name: "sample rate zero", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(0)},
		},
		{
			name: "sample rate negative", numtaps: 11, freq: []float64{0, 0.5, -1}, gain: []float64{1, 1, 0},
			opts: []design.Option{design.WithSampleRate(-2)},
		},
		{
			name: "sample rate NaN", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(math.NaN())},
		},
		{
			name: "sample rate +Inf", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(math.Inf(1))},
		},
		{
			name: "sample rate -Inf", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(math.Inf(-1))},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taps, err := design.Firwin2(tc.numtaps, tc.freq, tc.gain, tc.opts...)
			if err == nil {
				t.Fatalf("Firwin2 succeeded with %d taps, want error", len(taps))
			}

			if !errors.Is(err, design.ErrInvalidFirwin2) {
				t.Fatalf("error %q does not wrap ErrInvalidFirwin2", err)
			}

			if taps != nil {
				t.Errorf("taps = %v, want nil on error", taps)
			}
		})
	}
}

func TestFirwin2ValidBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		numtaps int
		freq    []float64
		gain    []float64
		opts    []design.Option
	}{
		{
			name: "nfreqs just above numtaps", numtaps: 11, freq: []float64{0, 1}, gain: []float64{1, 1},
			opts: []design.Option{design.WithNFreqs(12)},
		},
		{
			name: "minimal mesh", numtaps: 1, freq: []float64{0, 1}, gain: []float64{1, 1},
			opts: []design.Option{design.WithNFreqs(2)},
		},
		{name: "type I nonzero Nyquist gain", numtaps: 11, freq: []float64{0, 1}, gain: []float64{0, 1}},
		{name: "repeated interior frequency", numtaps: 11, freq: []float64{0, 0.5, 0.5, 1}, gain: []float64{1, 1, 0, 0}},
		{
			name: "two adjacent steps", numtaps: 11,
			freq: []float64{0, 0.3, 0.3, 0.6, 0.6, 1}, gain: []float64{1, 1, 0, 0, 1, 1},
		},
		{
			name: "sample rate in Hz", numtaps: 11, freq: []float64{0, 100, 24000}, gain: []float64{1, 1, 0},
			opts: []design.Option{design.WithSampleRate(48000)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taps, err := design.Firwin2(tc.numtaps, tc.freq, tc.gain, tc.opts...)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			if len(taps) != tc.numtaps {
				t.Fatalf("len = %d, want %d", len(taps), tc.numtaps)
			}

			for i, v := range taps {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("tap %d = %v", i, v)
				}
			}
		})
	}
}

func TestFirwin2Symmetric(t *testing.T) {
	fx := loadFirwin2Fixture(t)

	for _, tc := range fx.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			taps, err := design.Firwin2(tc.NumTaps, tc.Freq, tc.Gain, tc.options(t)...)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			// The taps come from a DFT, so mirrored taps agree only to
			// rounding (a few 1e-15 for 256 taps; scipy's agree no better).
			n := len(taps)
			for i := range n / 2 {
				if d := math.Abs(taps[i] - taps[n-1-i]); d > 1e-14 {
					t.Errorf("taps[%d] = %.17g, taps[%d] = %.17g, |diff| = %.3g", i, taps[i], n-1-i, taps[n-1-i], d)
				}
			}
		})
	}
}

func TestFirwin2DoesNotModifyInputs(t *testing.T) {
	tests := []struct {
		name    string
		numtaps int
		freq    []float64
		gain    []float64
	}{
		{name: "plain", numtaps: 31, freq: []float64{0, 0.3, 0.4, 1}, gain: []float64{1, 1, 0, 0}},
		{name: "repeated frequency", numtaps: 31, freq: []float64{0, 0.5, 0.5, 1}, gain: []float64{1, 1, 0, 0}},
		{
			name: "two repeated frequencies", numtaps: 32,
			freq: []float64{0, 0.25, 0.25, 0.75, 0.75, 1}, gain: []float64{0, 0, 1, 1, 0, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			freq := slices.Clone(tc.freq)
			gain := slices.Clone(tc.gain)

			_, err := design.Firwin2(tc.numtaps, freq, gain)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			if !slices.Equal(freq, tc.freq) {
				t.Errorf("freq modified: %v, want %v", freq, tc.freq)
			}

			if !slices.Equal(gain, tc.gain) {
				t.Errorf("gain modified: %v, want %v", gain, tc.gain)
			}
		})
	}
}

// responseAt evaluates |H(e^{j*pi*w})| for w normalized to Nyquist.
func responseAt(taps []float64, w float64) float64 {
	var re, im float64

	for n, h := range taps {
		re += h * math.Cos(math.Pi*w*float64(n))
		im -= h * math.Sin(math.Pi*w*float64(n))
	}

	return math.Hypot(re, im)
}

func TestFirwin2Response(t *testing.T) {
	tests := []struct {
		name    string
		numtaps int
		freq    []float64
		gain    []float64
		check   []float64 // normalized frequencies to probe
		want    []float64 // desired magnitude there
		tol     float64
	}{
		{
			name: "lowpass", numtaps: 101,
			freq: []float64{0, 0.2, 0.3, 1}, gain: []float64{1, 1, 0, 0},
			check: []float64{0, 0.1, 0.5, 1}, want: []float64{1, 1, 0, 0}, tol: 5e-3,
		},
		{
			name: "highpass", numtaps: 101,
			freq: []float64{0, 0.4, 0.5, 1}, gain: []float64{0, 0, 1, 1},
			check: []float64{0, 0.2, 0.7, 1}, want: []float64{0, 0, 1, 1}, tol: 5e-3,
		},
		{
			name: "scaled lowpass", numtaps: 101,
			freq: []float64{0, 0.2, 0.3, 1}, gain: []float64{0.5, 0.5, 0, 0},
			check: []float64{0, 0.1, 0.5}, want: []float64{0.5, 0.5, 0}, tol: 5e-3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taps, err := design.Firwin2(tc.numtaps, tc.freq, tc.gain)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			// The DC gain is the plain sum of the taps.
			var sum float64
			for _, h := range taps {
				sum += h
			}

			if math.Abs(sum-tc.gain[0]) > tc.tol {
				t.Errorf("DC gain (sum of taps) = %.6g, want %.6g", sum, tc.gain[0])
			}

			for i, w := range tc.check {
				got := responseAt(taps, w)
				if math.Abs(got-tc.want[i]) > tc.tol {
					t.Errorf("|H(%g)| = %.6g, want %.6g", w, got, tc.want[i])
				}
			}
		})
	}
}

func TestFirwin2Options(t *testing.T) {
	freq := []float64{0, 0.3, 0.4, 1}
	gain := []float64{1, 1, 0, 0}

	design41 := func(t *testing.T, opts ...design.Option) []float64 {
		t.Helper()

		taps, err := design.Firwin2(41, freq, gain, opts...)
		if err != nil {
			t.Fatalf("Firwin2: %v", err)
		}

		return taps
	}

	hann := design41(t, design.WithWindow(window.TypeHann))
	none := design41(t, design.WithoutWindow())
	hamming := design41(t)

	tests := []struct {
		name string
		got  []float64
		want []float64
	}{
		{"default is Hamming", design41(t, design.WithWindow(window.TypeHamming)), hamming},
		{"WithWindow after WithoutWindow", design41(t, design.WithoutWindow(), design.WithWindow(window.TypeHann)), hann},
		{"WithoutWindow after WithWindow", design41(t, design.WithWindow(window.TypeHann), design.WithoutWindow()), none},
		{"later WithWindow wins", design41(t, design.WithWindow(window.TypeBlackman), design.WithWindow(window.TypeHann)), hann},
		{"default nfreqs", design41(t, design.WithNFreqs(65)), hamming},
		{"later WithNFreqs wins", design41(t, design.WithNFreqs(100), design.WithNFreqs(65)), hamming},
		{"default sample rate is 2", design41(t, design.WithSampleRate(2)), hamming},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !slices.Equal(tc.got, tc.want) {
				t.Errorf("taps differ, max abs diff %.3g", maxAbsDiff(tc.got, tc.want))
			}
		})
	}

	if slices.Equal(hann, none) || slices.Equal(hann, hamming) {
		t.Fatal("window options had no effect")
	}

	t.Run("window options are copied", func(t *testing.T) {
		wopts := []window.Option{window.WithAlpha(8.6)}
		opt := design.WithWindow(window.TypeKaiser, wopts...)
		wopts[0] = window.WithAlpha(1)

		got := design41(t, opt)
		want := design41(t, design.WithWindow(window.TypeKaiser, window.WithAlpha(8.6)))

		if !slices.Equal(got, want) {
			t.Errorf("WithWindow aliases its options slice, max abs diff %.3g", maxAbsDiff(got, want))
		}
	})
}

func TestFirwin2SampleRateEquivalence(t *testing.T) {
	norm, err := design.Firwin2(51, []float64{0, 0.25, 0.375, 1}, []float64{1, 1, 0, 0})
	if err != nil {
		t.Fatalf("Firwin2: %v", err)
	}

	hz, err := design.Firwin2(51, []float64{0, 1000, 1500, 4000}, []float64{1, 1, 0, 0}, design.WithSampleRate(8000))
	if err != nil {
		t.Fatalf("Firwin2 with sample rate: %v", err)
	}

	if d := maxAbsDiff(norm, hz); d > 1e-15 {
		t.Errorf("Hz design differs from normalized design by %.3g", d)
	}
}

func BenchmarkFirwin2(b *testing.B) {
	freq := []float64{0, 0.5 / 1.001, 0.5 * 1.001, 1}
	gain := []float64{1, 1, 0, 0}

	b.ReportAllocs()

	for b.Loop() {
		_, err := design.Firwin2(256, freq, gain)
		if err != nil {
			b.Fatal(err)
		}
	}
}

package graphiceq

import (
	"errors"
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/conv"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

const testSampleRate = 48000

// testGains is a deliberately uneven octave setting: neighbouring bands differ
// by up to 10 dB, so the shelf cascade is not given an easy target.
var testGains = []float64{6, -3, 0, 4, -6, 2, 0, -2, 5, 0}

func octaveBands(gains []float64) []Band {
	bands := make([]Band, len(gains))
	frequency := 31.25

	for i, gain := range gains {
		bands[i] = Band{Frequency: frequency, GainDB: gain}
		frequency *= 2
	}

	return bands
}

func designOrFail(t *testing.T, cfg Config) Result {
	t.Helper()

	result, err := Design(cfg)
	if err != nil {
		t.Fatalf("Design: %v", err)
	}

	return result
}

// TestDefaultLengthHalvesPerOffloadedBand pins the mechanism the whole package
// exists for: octave centres double, so each band moved into the IIR part halves
// the FIR the remaining response needs.
func TestDefaultLengthHalvesPerOffloadedBand(t *testing.T) {
	bands := octaveBands(testGains)

	previous := 0

	for split := range 5 {
		length := DefaultLength(Config{
			SampleRate: testSampleRate,
			Bands:      bands,
			IIRBands:   split,
		})

		if length%2 == 0 {
			t.Fatalf("split %d: length %d is even", split, length)
		}

		if split > 0 {
			ratio := float64(previous) / float64(length)
			if math.Abs(ratio-2) > 0.05 {
				t.Errorf(
					"split %d: length %d follows %d, ratio %.3f, want 2",
					split, length, previous, ratio,
				)
			}
		}

		previous = length
	}
}

func TestDefaultLengthWithoutFIRBand(t *testing.T) {
	bands := octaveBands(testGains)

	cases := map[string]Config{
		"no bands":       {SampleRate: testSampleRate},
		"no sample rate": {Bands: bands},
		"all offloaded": {
			SampleRate: testSampleRate,
			Bands:      bands,
			IIRBands:   len(bands),
		},
		"negative split": {
			SampleRate: testSampleRate,
			Bands:      bands,
			IIRBands:   -1,
		},
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if got := DefaultLength(cfg); got != 0 {
				t.Errorf("DefaultLength = %d, want 0", got)
			}
		})
	}
}

// TestOneOffloadedBandHalvesLatencyForFree reproduces the claim the DAFx 2022
// paper makes for its shelving band: half the latency at practically unchanged
// accuracy.
func TestOneOffloadedBandHalvesLatencyForFree(t *testing.T) {
	bands := octaveBands(testGains)

	allFIR := designOrFail(t, Config{
		SampleRate: testSampleRate,
		Bands:      bands,
	})

	hybrid := designOrFail(t, Config{
		SampleRate: testSampleRate,
		Bands:      bands,
		IIRBands:   1,
	})

	if hybrid.Latency*2 != allFIR.Latency {
		t.Errorf(
			"latency %d, want half of %d",
			hybrid.Latency,
			allFIR.Latency,
		)
	}

	if hybrid.Metrics.MaxErrorDB > 1.2*allFIR.Metrics.MaxErrorDB {
		t.Errorf(
			"max error %.3f dB at half the latency, all-FIR reaches %.3f dB",
			hybrid.Metrics.MaxErrorDB,
			allFIR.Metrics.MaxErrorDB,
		)
	}
}

// TestHybridBeatsEqualLatencyFIR is the comparison that decides whether the
// split is worth having: at the same tap count, does moving bands into the IIR
// part buy accuracy?
func TestHybridBeatsEqualLatencyFIR(t *testing.T) {
	bands := octaveBands(testGains)

	for split := 1; split < 5; split++ {
		hybrid := designOrFail(t, Config{
			SampleRate: testSampleRate,
			Bands:      bands,
			IIRBands:   split,
		})

		allFIR := designOrFail(t, Config{
			SampleRate: testSampleRate,
			Bands:      bands,
			Length:     len(hybrid.Taps),
		})

		if hybrid.Metrics.RMSErrorDB >= allFIR.Metrics.RMSErrorDB {
			t.Errorf(
				"split %d: hybrid RMS %.4f dB does not beat all-FIR %.4f dB",
				split,
				hybrid.Metrics.RMSErrorDB,
				allFIR.Metrics.RMSErrorDB,
			)
		}

		if hybrid.Metrics.MaxErrorDB >= allFIR.Metrics.MaxErrorDB {
			t.Errorf(
				"split %d: hybrid peak %.4f dB does not beat all-FIR %.4f dB",
				split,
				hybrid.Metrics.MaxErrorDB,
				allFIR.Metrics.MaxErrorDB,
			)
		}
	}
}

// TestSplitDegradesGradually records that the accuracy paid for the latency
// grows slowly, so the trade-off stays usable for several bands.
func TestSplitDegradesGradually(t *testing.T) {
	bands := octaveBands(testGains)

	for split := range 5 {
		result := designOrFail(t, Config{
			SampleRate: testSampleRate,
			Bands:      bands,
			IIRBands:   split,
		})

		if result.Metrics.MaxErrorDB > 1 {
			t.Errorf(
				"split %d: peak error %.3f dB exceeds 1 dB",
				split,
				result.Metrics.MaxErrorDB,
			)
		}
	}
}

// TestZigzagTargetDefeatsTheSplit documents the failure mode. Shelves can only
// step monotonically between neighbouring band gains, so a target that reverses
// direction at every octave is beyond them and the error stays in the response.
func TestZigzagTargetDefeatsTheSplit(t *testing.T) {
	gains := make([]float64, len(testGains))
	for i := range gains {
		gains[i] = 12
		if i%2 == 1 {
			gains[i] = -12
		}
	}

	bands := octaveBands(gains)

	allFIR := designOrFail(t, Config{
		SampleRate: testSampleRate,
		Bands:      bands,
	})

	hybrid := designOrFail(t, Config{
		SampleRate: testSampleRate,
		Bands:      bands,
		IIRBands:   3,
	})

	if hybrid.Metrics.MaxErrorDB < 2*allFIR.Metrics.MaxErrorDB {
		t.Errorf(
			"peak error %.3f dB against the all-FIR %.3f dB; the zigzag "+
				"limitation may no longer hold",
			hybrid.Metrics.MaxErrorDB,
			allFIR.Metrics.MaxErrorDB,
		)
	}
}

func TestAllFIRDesignHasNoSections(t *testing.T) {
	result := designOrFail(t, Config{
		SampleRate: testSampleRate,
		Bands:      octaveBands(testGains),
	})

	if result.Sections != nil {
		t.Errorf("got %d sections, want none", len(result.Sections))
	}

	if want := (len(result.Taps) - 1) / 2; result.Latency != want {
		t.Errorf("latency %d, want %d", result.Latency, want)
	}
}

func TestFlatTargetIsTransparent(t *testing.T) {
	bands := octaveBands(make([]float64, len(testGains)))

	result := designOrFail(t, Config{
		SampleRate: testSampleRate,
		Bands:      bands,
		IIRBands:   3,
		Length:     129,
	})

	if result.Metrics.MaxErrorDB > 1e-9 {
		t.Errorf("peak error %.3g dB, want a transparent design",
			result.Metrics.MaxErrorDB)
	}

	centre := (len(result.Taps) - 1) / 2
	if math.Abs(result.Taps[centre]-1) > 1e-9 {
		t.Errorf("centre tap %.9f, want 1", result.Taps[centre])
	}
}

// TestImpulseResponseMatchesMetrics checks the reported error against the
// response of the structure that would actually run, rather than against the
// design grid the metrics were computed on.
func TestImpulseResponseMatchesMetrics(t *testing.T) {
	bands := octaveBands(testGains)

	result := designOrFail(t, Config{
		SampleRate: testSampleRate,
		Bands:      bands,
		IIRBands:   2,
	})

	const tail = 32768

	chain := biquad.NewChain(result.Sections)

	combined, err := conv.Direct(chain.ImpulseResponse(tail), result.Taps)
	if err != nil {
		t.Fatalf("convolve: %v", err)
	}

	for i, band := range bands {
		got := responseDB(combined, band.Frequency, testSampleRate)
		want := band.GainDB + result.Metrics.BandErrorDB[i]

		if math.Abs(got-want) > 0.05 {
			t.Errorf(
				"band %d at %.2f Hz: measured %.4f dB, metrics claim %.4f dB",
				i, band.Frequency, got, want,
			)
		}
	}
}

// responseDB evaluates the DTFT of taps at one frequency.
func responseDB(taps []float64, frequency, sampleRate float64) float64 {
	omega := 2 * math.Pi * frequency / sampleRate

	sum := complex(0, 0)
	for n, tap := range taps {
		sum += complex(tap, 0) * cmplx.Exp(complex(0, -omega*float64(n)))
	}

	return 20 * math.Log10(cmplx.Abs(sum))
}

func TestWindowTapersTheTaps(t *testing.T) {
	bands := octaveBands(testGains)

	cfg := Config{
		SampleRate: testSampleRate,
		Bands:      bands,
		IIRBands:   2,
		Length:     257,
	}

	plain := designOrFail(t, cfg)

	cfg.Window = window.TypeKaiser
	cfg.WindowAlpha = 6
	windowed := designOrFail(t, cfg)

	if math.Abs(windowed.Taps[0]) >= math.Abs(plain.Taps[0]) {
		t.Errorf(
			"first tap %.3g is not tapered against %.3g",
			windowed.Taps[0],
			plain.Taps[0],
		)
	}
}

func TestExplicitFFTSize(t *testing.T) {
	bands := octaveBands(testGains)

	cfg := Config{
		SampleRate: testSampleRate,
		Bands:      bands,
		IIRBands:   2,
		Length:     257,
	}

	coarse := designOrFail(t, cfg)

	cfg.FFTSize = 8192
	fine := designOrFail(t, cfg)

	if len(coarse.Taps) != len(fine.Taps) {
		t.Fatalf("lengths differ: %d and %d", len(coarse.Taps), len(fine.Taps))
	}

	// A denser grid changes the sampled target only marginally, so the designs
	// must stay close; a large deviation would mean the grid, not the
	// specification, decides the result.
	for i := range coarse.Taps {
		if math.Abs(coarse.Taps[i]-fine.Taps[i]) > 1e-3 {
			t.Fatalf(
				"tap %d differs: %.6f and %.6f",
				i, coarse.Taps[i], fine.Taps[i],
			)
		}
	}
}

func TestValidation(t *testing.T) {
	bands := octaveBands(testGains)

	cases := []struct {
		name string
		cfg  Config
		want error
	}{
		{
			name: "no bands",
			cfg:  Config{SampleRate: testSampleRate},
			want: ErrNoBands,
		},
		{
			name: "zero sample rate",
			cfg:  Config{Bands: bands},
			want: ErrInvalidSampleRate,
		},
		{
			name: "infinite sample rate",
			cfg:  Config{SampleRate: math.Inf(1), Bands: bands},
			want: ErrInvalidSampleRate,
		},
		{
			name: "band above nyquist",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      []Band{{Frequency: 30000}},
			},
			want: ErrInvalidBand,
		},
		{
			name: "band at zero",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      []Band{{Frequency: 0}},
			},
			want: ErrInvalidBand,
		},
		{
			name: "descending bands",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands: []Band{
					{Frequency: 1000},
					{Frequency: 500},
				},
			},
			want: ErrInvalidBand,
		},
		{
			name: "non-finite gain",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      []Band{{Frequency: 1000, GainDB: math.NaN()}},
			},
			want: ErrInvalidBand,
		},
		{
			name: "split leaves no FIR band",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      bands,
				IIRBands:   len(bands),
			},
			want: ErrInvalidSplit,
		},
		{
			name: "negative split",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      bands,
				IIRBands:   -1,
			},
			want: ErrInvalidSplit,
		},
		{
			name: "negative Q",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      bands,
				Q:          -1,
			},
			want: ErrInvalidQ,
		},
		{
			name: "negative window alpha",
			cfg: Config{
				SampleRate:  testSampleRate,
				Bands:       bands,
				WindowAlpha: -1,
			},
			want: ErrInvalidWindowAlpha,
		},
		{
			name: "negative length",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      bands,
				Length:     -3,
			},
			want: ErrInvalidLength,
		},
		{
			name: "even length",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      bands,
				Length:     128,
			},
			want: ErrInvalidLength,
		},
		{
			name: "odd FFT size",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      bands,
				Length:     127,
				FFTSize:    511,
			},
			want: ErrInvalidLength,
		},
		{
			name: "FFT size below length",
			cfg: Config{
				SampleRate: testSampleRate,
				Bands:      bands,
				Length:     1025,
				FFTSize:    512,
			},
			want: ErrInvalidLength,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Design(testCase.cfg)
			if !errors.Is(err, testCase.want) {
				t.Errorf("got %v, want %v", err, testCase.want)
			}
		})
	}
}

func BenchmarkDesign(b *testing.B) {
	cfg := Config{
		SampleRate: testSampleRate,
		Bands:      octaveBands(testGains),
		IIRBands:   2,
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, err := Design(cfg); err != nil {
			b.Fatal(err)
		}
	}
}

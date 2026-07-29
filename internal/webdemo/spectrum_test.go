package webdemo

import (
	"math"
	"testing"
)

const testSampleRate = 48000.0

// feedSine pushes n samples of a unit-amplitude sine at freqHz through the
// analyzer.
func feedSine(e *Engine, freqHz float64, n int) {
	for i := range n {
		e.pushSpectrumSample(math.Sin(2 * math.Pi * freqHz * float64(i) / testSampleRate))
	}
}

// TestSpectrumPeaksAtInputFrequency is the cheap, high-value analyzer test: a
// tone at a bin centre must show up at that bin, at roughly the right level.
func TestSpectrumPeaksAtInputFrequency(t *testing.T) {
	t.Parallel()

	const fftSize = 2048

	e := newTestEngine(t)

	err := e.SetSpectrum(SpectrumParams{
		FFTSize:   fftSize,
		Overlap:   0.75,
		Window:    "hann",
		Smoothing: 0, // no smoothing: assert on a single frame
	})
	if err != nil {
		t.Fatalf("SetSpectrum: %v", err)
	}

	// Put the tone exactly on bin 64 so there is no scalloping loss.
	const bin = 64

	binHz := testSampleRate / fftSize
	freq := bin * binHz

	feedSine(e, freq, fftSize*4)

	if !e.spectrumReady {
		t.Fatal("analyzer never produced a frame")
	}

	peakBin := 0
	for k, db := range e.spectrumDB {
		if db > e.spectrumDB[peakBin] {
			peakBin = k
		}
	}

	if peakBin != bin {
		t.Errorf("peak at bin %d (%.1f Hz), want bin %d (%.1f Hz)",
			peakBin, float64(peakBin)*binHz, bin, freq)
	}

	// A full-scale sine is 0 dBFS by this analyzer's normalisation (the
	// one-sided doubling compensates for the split between +/- frequencies).
	if got := e.spectrumDB[bin]; math.Abs(got) > 1.0 {
		t.Errorf("peak level = %.2f dBFS, want approximately 0 dBFS", got)
	}

	// Well away from the tone the response must be far down.
	for _, k := range []int{bin - 20, bin + 20, len(e.spectrumDB) / 2} {
		if e.spectrumDB[k] > -60 {
			t.Errorf("bin %d = %.1f dBFS, want below -60 dBFS", k, e.spectrumDB[k])
		}
	}
}

func TestSpectrumSilenceReadsFloor(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	err := e.SetSpectrum(SpectrumParams{FFTSize: 512, Overlap: 0.5, Window: "hann"})
	if err != nil {
		t.Fatalf("SetSpectrum: %v", err)
	}

	for range 4096 {
		e.pushSpectrumSample(0)
	}

	for k, db := range e.spectrumDB {
		if db > -120 {
			t.Fatalf("silent input gave %.1f dBFS at bin %d, want the floor", db, k)
		}
	}
}

// TestSpectrumCurveDBBeforeReady guards the path the UI hits on first paint,
// before any audio has been rendered.
func TestSpectrumCurveDBBeforeReady(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	got := e.SpectrumCurveDB([]float64{20, 1000, 20000})
	if len(got) != 3 {
		t.Fatalf("got %d values, want 3", len(got))
	}

	for i, db := range got {
		if db != -130 {
			t.Errorf("value %d = %v, want the -130 floor", i, db)
		}
	}
}

func TestSpectrumCurveDBClampsOutOfRangeFrequencies(t *testing.T) {
	t.Parallel()

	const fftSize = 1024

	e := newTestEngine(t)

	err := e.SetSpectrum(SpectrumParams{FFTSize: fftSize, Overlap: 0.5, Window: "hann"})
	if err != nil {
		t.Fatalf("SetSpectrum: %v", err)
	}

	feedSine(e, 1000, fftSize*4)

	nyquist := testSampleRate / 2

	got := e.SpectrumCurveDB([]float64{-100, 0, nyquist, nyquist * 4})
	for i, db := range got {
		if math.IsNaN(db) || math.IsInf(db, 0) {
			t.Fatalf("value %d is not finite: %v", i, db)
		}
	}

	// Below and above the representable range the curve saturates at the
	// first and last bin rather than indexing out of bounds.
	if got[0] != got[1] {
		t.Errorf("negative frequency gave %v, want the same as DC (%v)", got[0], got[1])
	}

	if got[2] != got[3] {
		t.Errorf("above Nyquist gave %v, want the same as Nyquist (%v)", got[3], got[2])
	}
}

func TestSanitizeSpectrumParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   SpectrumParams
		want SpectrumParams
	}{
		{
			name: "zero value gets defaults",
			in:   SpectrumParams{},
			want: SpectrumParams{FFTSize: 2048, Overlap: 0.25, Smoothing: 0, Window: "blackmanharris"},
		},
		{
			name: "non-power-of-two FFT size falls back",
			in:   SpectrumParams{FFTSize: 1000, Overlap: 0.5, Smoothing: 0.5, Window: "hann"},
			want: SpectrumParams{FFTSize: 2048, Overlap: 0.5, Smoothing: 0.5, Window: "hann"},
		},
		{
			name: "overlap clamped low",
			in:   SpectrumParams{FFTSize: 512, Overlap: -1, Window: "hann"},
			want: SpectrumParams{FFTSize: 512, Overlap: 0.25, Window: "hann"},
		},
		{
			name: "overlap clamped high",
			in:   SpectrumParams{FFTSize: 512, Overlap: 5, Window: "hann"},
			want: SpectrumParams{FFTSize: 512, Overlap: 0.95, Window: "hann"},
		},
		{
			name: "smoothing clamped",
			in:   SpectrumParams{FFTSize: 512, Overlap: 0.5, Smoothing: 2, Window: "hann"},
			want: SpectrumParams{FFTSize: 512, Overlap: 0.5, Smoothing: 0.95, Window: "hann"},
		},
		{
			name: "window name normalised",
			in:   SpectrumParams{FFTSize: 512, Overlap: 0.5, Window: "  FlatTop "},
			want: SpectrumParams{FFTSize: 512, Overlap: 0.5, Window: "flattop"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := sanitizeSpectrumParams(tc.in); got != tc.want {
				t.Errorf("sanitizeSpectrumParams(%+v) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSetSpectrumRejectsUnknownWindow(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	err := e.SetSpectrum(SpectrumParams{FFTSize: 512, Overlap: 0.5, Window: "gaussian"})
	if err == nil {
		t.Fatal("expected an error for an unsupported window")
	}
}

func TestSpectrumWindowTypeAcceptsEveryUIOption(t *testing.T) {
	t.Parallel()

	// These are the values the analyser <select> in web/index.html can emit.
	for _, name := range []string{"hann", "hamming", "blackman", "blackmanharris", "flattop"} {
		if _, err := spectrumWindowType(name); err != nil {
			t.Errorf("spectrumWindowType(%q): %v", name, err)
		}
	}
}

// TestSpectrumHopSize checks the overlap-to-hop conversion, which decides how
// often a frame is produced.
func TestSpectrumHopSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		overlap float64
		fftSize int
		wantHop int
	}{
		{overlap: 0.75, fftSize: 2048, wantHop: 512},
		{overlap: 0.5, fftSize: 2048, wantHop: 1024},
		{overlap: 0.25, fftSize: 1024, wantHop: 768},
		// Clamped to 0.95, so the hop is 5% of the frame but never below 1.
		{overlap: 0.99, fftSize: 256, wantHop: 13},
	}

	for _, tc := range tests {
		e := newTestEngine(t)

		err := e.SetSpectrum(SpectrumParams{
			FFTSize: tc.fftSize,
			Overlap: tc.overlap,
			Window:  "hann",
		})
		if err != nil {
			t.Fatalf("SetSpectrum: %v", err)
		}

		if e.spectrumHopSize != tc.wantHop {
			t.Errorf("overlap %.2f, size %d: hop = %d, want %d",
				tc.overlap, tc.fftSize, e.spectrumHopSize, tc.wantHop)
		}

		if e.spectrumHopSize < 1 {
			t.Errorf("hop size must never be below 1, got %d", e.spectrumHopSize)
		}
	}
}

// TestSpectrumSmoothingConverges verifies the exponential smoothing actually
// averages towards the true level rather than sticking at the first frame.
func TestSpectrumSmoothingConverges(t *testing.T) {
	t.Parallel()

	const fftSize = 1024

	e := newTestEngine(t)

	err := e.SetSpectrum(SpectrumParams{
		FFTSize:   fftSize,
		Overlap:   0.5,
		Window:    "hann",
		Smoothing: 0.9,
	})
	if err != nil {
		t.Fatalf("SetSpectrum: %v", err)
	}

	const bin = 32

	freq := bin * testSampleRate / fftSize

	feedSine(e, freq, fftSize*40)

	if got := e.spectrumDB[bin]; math.Abs(got) > 2.0 {
		t.Errorf("smoothed peak = %.2f dBFS, want it to converge near 0 dBFS", got)
	}
}

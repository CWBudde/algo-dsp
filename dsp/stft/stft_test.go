package stft_test

import (
	"errors"
	"math"
	"math/cmplx"
	"sync"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/stft"
	"github.com/cwbudde/algo-dsp/dsp/window"
	"github.com/cwbudde/algo-dsp/internal/testutil"
)

func maxAbsDiff(t *testing.T, got, want []float64) float64 {
	t.Helper()

	d, err := testutil.MaxAbsDiff(got, want)
	if err != nil {
		t.Fatal(err)
	}

	return d
}

func roundTrip(t *testing.T, s *stft.STFT, x []float64) []float64 {
	t.Helper()

	spec, err := s.Forward(x)
	if err != nil {
		t.Fatal(err)
	}

	if len(spec) != s.FrameCount(len(x)) {
		t.Fatalf("got %d frames, want %d", len(spec), s.FrameCount(len(x)))
	}

	y, err := s.Inverse(spec, len(x))
	if err != nil {
		t.Fatal(err)
	}

	return y
}

func TestPerfectReconstruction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nfft, hop int
		n         int
		tol       float64
		opts      []stft.Option
	}{
		{"zero 256/64", 256, 64, 10007, 1e-12, nil},
		{"zero 2048/512", 2048, 512, 24001, 1e-12, nil},
		{"zero normalized", 1024, 256, 9000, 1e-12, []stft.Option{stft.WithNormalized()}},
		{"reflect 256/64", 256, 64, 10007, 1e-12, []stft.Option{stft.WithCenter(stft.PadReflect)}},
		{"reflect normalized 4096/1024", 4096, 1024, 30000, 1e-12, []stft.Option{
			stft.WithCenter(stft.PadReflect), stft.WithNormalized(),
		}},
		{"reflect odd nfft", 255, 60, 5000, 1e-12, []stft.Option{stft.WithCenter(stft.PadReflect)}},
		// Periodic Hann starts with a zero, which PadNone cannot recover at
		// sample 0; Hamming has no zeros.
		{"none hamming 512/128", 512, 128, 512 + 37*128, 1e-12, []stft.Option{
			stft.WithCenter(stft.PadNone), stft.WithWindow(window.TypeHamming),
		}},
		{"non-COLA hann 1024/300", 1024, 300, 20000, 1e-10, nil},
		{"non-COLA blackman 1000/333", 1000, 333, 12345, 1e-10, []stft.Option{
			stft.WithWindow(window.TypeBlackman), stft.WithCenter(stft.PadReflect),
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, err := stft.New(tc.nfft, tc.hop, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			x := testutil.DeterministicNoise(42, 1, tc.n)
			y := roundTrip(t, s, x)

			if d := maxAbsDiff(t, y, x); d > tc.tol {
				t.Fatalf("max reconstruction error %g > %g", d, tc.tol)
			}
		})
	}
}

func TestReconstructionFloat32(t *testing.T) {
	t.Parallel()

	for _, p := range []stft.Padding{stft.PadZero, stft.PadReflect} {
		s, err := stft.New32(1024, 256, stft.WithCenter(p), stft.WithNormalized())
		if err != nil {
			t.Fatal(err)
		}

		x64 := testutil.DeterministicNoise(3, 1, 8000)

		x := make([]float32, len(x64))
		for i, v := range x64 {
			x[i] = float32(v)
		}

		spec, err := s.Forward(x)
		if err != nil {
			t.Fatal(err)
		}

		y, err := s.Inverse(spec, len(x))
		if err != nil {
			t.Fatal(err)
		}

		for i := range x {
			if d := math.Abs(float64(y[i] - x[i])); d > 1e-5 {
				t.Fatalf("%v: sample %d: error %g", p, i, d)
			}
		}
	}
}

func TestFloat32MatchesFloat64(t *testing.T) {
	t.Parallel()

	s64, err := stft.New(512, 128)
	if err != nil {
		t.Fatal(err)
	}

	s32, err := stft.New32(512, 128)
	if err != nil {
		t.Fatal(err)
	}

	x64 := testutil.DeterministicNoise(5, 1, 3000)

	x32 := make([]float32, len(x64))
	for i, v := range x64 {
		x32[i] = float32(v)
	}

	d64 := make([]complex128, s64.Bins())
	d32 := make([]complex64, s32.Bins())

	for frame := range s64.FrameCount(len(x64)) {
		if err := s64.FrameInto(d64, x64, frame); err != nil {
			t.Fatal(err)
		}

		if err := s32.FrameInto(d32, x32, frame); err != nil {
			t.Fatal(err)
		}

		for k := range d64 {
			if d := cmplx.Abs(d64[k] - complex128(d32[k])); d > 1e-4 {
				t.Fatalf("frame %d bin %d: |diff| %g", frame, k, d)
			}
		}
	}
}

// TestPaddingMatchesExplicitPadding checks centred framing against PadNone
// framing of a signal padded by hand with nfft/2 samples on both sides.
func TestPaddingMatchesExplicitPadding(t *testing.T) {
	t.Parallel()

	const (
		nfft = 64
		hop  = 16
		n    = 200
	)

	x := testutil.DeterministicNoise(9, 1, n)
	pad := nfft / 2

	reflected := make([]float64, n+2*pad)
	zeroed := make([]float64, n+2*pad)
	copy(reflected[pad:], x)
	copy(zeroed[pad:], x)

	for j := 1; j <= pad; j++ {
		reflected[pad-j] = x[j]
		reflected[pad+n-1+j] = x[n-1-j]
	}

	tests := []struct {
		name   string
		mode   stft.Padding
		padded []float64
	}{
		{"zero", stft.PadZero, zeroed},
		{"reflect", stft.PadReflect, reflected},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			centred, err := stft.New(nfft, hop, stft.WithCenter(tc.mode))
			if err != nil {
				t.Fatal(err)
			}

			plain, err := stft.New(nfft, hop, stft.WithCenter(stft.PadNone))
			if err != nil {
				t.Fatal(err)
			}

			// torch.stft(center=True) yields 1 + n/hop frames.
			frames := 1 + n/hop
			if got := plain.FrameCount(len(tc.padded)); got != frames {
				t.Fatalf("PadNone frame count %d, want %d", got, frames)
			}

			a := make([]complex128, centred.Bins())
			b := make([]complex128, plain.Bins())

			for frame := range frames {
				if err := centred.FrameInto(a, x, frame); err != nil {
					t.Fatal(err)
				}

				if err := plain.FrameInto(b, tc.padded, frame); err != nil {
					t.Fatal(err)
				}

				for k := range a {
					if a[k] != b[k] {
						t.Fatalf("frame %d bin %d: %v != %v", frame, k, a[k], b[k])
					}
				}
			}
		})
	}
}

func TestUnitaryParseval(t *testing.T) {
	t.Parallel()

	for _, nfft := range []int{512, 511} {
		s, err := stft.New(nfft, nfft/4, stft.WithNormalized(), stft.WithCenter(stft.PadNone))
		if err != nil {
			t.Fatal(err)
		}

		x := testutil.DeterministicNoise(11, 1, 4*nfft)
		w := s.Window()

		dst := make([]complex128, s.Bins())

		const frame = 3
		if err := s.FrameInto(dst, x, frame); err != nil {
			t.Fatal(err)
		}

		timeEnergy := 0.0

		for k := range nfft {
			v := x[frame*s.Hop()+k] * w[k]
			timeEnergy += v * v
		}

		// One-sided spectrum: DC (and Nyquist for even nfft) count once,
		// every other bin stands for itself and its mirror image.
		freqEnergy := 0.0

		for k, v := range dst {
			p := real(v)*real(v) + imag(v)*imag(v)
			if k == 0 || (nfft%2 == 0 && k == len(dst)-1) {
				freqEnergy += p
			} else {
				freqEnergy += 2 * p
			}
		}

		if rel := math.Abs(freqEnergy-timeEnergy) / timeEnergy; rel > 1e-12 {
			t.Fatalf("nfft %d: time %g, freq %g (rel %g)", nfft, timeEnergy, freqEnergy, rel)
		}
	}
}

func TestSineAtBinCentre(t *testing.T) {
	t.Parallel()

	const (
		nfft = 1024
		bin  = 37
		amp  = 0.7
	)

	x := testutil.DeterministicSine(bin, nfft, amp, 8*nfft)

	s, err := stft.New(nfft, 256)
	if err != nil {
		t.Fatal(err)
	}

	dst := make([]complex128, s.Bins())
	if err := s.FrameInto(dst, x, 10); err != nil {
		t.Fatal(err)
	}

	peak := 0

	for k := range dst {
		if cmplx.Abs(dst[k]) > cmplx.Abs(dst[peak]) {
			peak = k
		}
	}

	if peak != bin {
		t.Fatalf("peak at bin %d, want %d", peak, bin)
	}

	coherentGain := window.Info(window.TypeHann).CoherentGain
	want := amp * nfft / 2 * coherentGain

	if got := cmplx.Abs(dst[bin]); math.Abs(got-want) > 1e-9*want {
		t.Fatalf("peak magnitude %v, want %v", got, want)
	}
}

func TestInverseWindowSumZero(t *testing.T) {
	t.Parallel()

	x := testutil.DeterministicNoise(1, 1, 4096)

	tests := []struct {
		name   string
		nfft   int
		hop    int
		opts   []stft.Option
		frames int // -1: all frames from Forward
		length int
	}{
		// Hann starts with a zero; with hop == nfft the frame boundaries
		// are never covered by a non-zero window value.
		{"hann none hop=nfft", 512, 512, []stft.Option{stft.WithCenter(stft.PadNone)}, -1, 4096},
		// PadNone never recovers sample 0 under a periodic Hann window.
		{"hann none sample 0", 512, 128, []stft.Option{stft.WithCenter(stft.PadNone)}, -1, 4096},
		// Too few frames for the requested length.
		{"uncovered tail", 512, 128, nil, 4, 4096},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, err := stft.New(tc.nfft, tc.hop, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			spec, err := s.Forward(x)
			if err != nil {
				t.Fatal(err)
			}

			if tc.frames >= 0 {
				spec = spec[:tc.frames]
			}

			_, err = s.Inverse(spec, tc.length)
			if !errors.Is(err, stft.ErrWindowSumZero) {
				t.Fatalf("got %v, want ErrWindowSumZero", err)
			}
		})
	}
}

func TestInverseIgnoresEdgeImaginaryParts(t *testing.T) {
	t.Parallel()

	for _, nfft := range []int{256, 255} {
		s, err := stft.New(nfft, nfft/4)
		if err != nil {
			t.Fatal(err)
		}

		x := testutil.DeterministicNoise(2, 1, 2000)

		spec, err := s.Forward(x)
		if err != nil {
			t.Fatal(err)
		}

		before := spec[0][0]

		for _, frame := range spec {
			frame[0] += 0.5i
			if nfft%2 == 0 {
				frame[len(frame)-1] += 0.25i
			}
		}

		y, err := s.Inverse(spec, len(x))
		if err != nil {
			t.Fatal(err)
		}

		if d := maxAbsDiff(t, y, x); d > 1e-12 {
			t.Fatalf("nfft %d: error %g", nfft, d)
		}

		// The caller's spectrum is left untouched.
		if spec[0][0] != before+0.5i {
			t.Fatalf("nfft %d: Inverse modified its input", nfft)
		}
	}
}

func TestNewErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		nfft    int
		hop     int
		opts    []stft.Option
		wantErr error
	}{
		{"nfft zero", 0, 1, nil, stft.ErrInvalidSize},
		{"nfft negative", -4, 1, nil, stft.ErrInvalidSize},
		{"nfft one", 1, 1, nil, stft.ErrInvalidSize},
		{"hop zero", 256, 0, nil, stft.ErrInvalidSize},
		{"hop negative", 256, -1, nil, stft.ErrInvalidSize},
		{"nil option", 256, 64, []stft.Option{nil}, stft.ErrNilOption},
		{"bad padding", 256, 64, []stft.Option{stft.WithCenter(stft.Padding(7))}, stft.ErrInvalidPadding},
		{"custom window length", 256, 64, []stft.Option{stft.WithCustomWindow(make([]float64, 255))}, stft.ErrInvalidWindow},
		{"custom window empty", 256, 64, []stft.Option{stft.WithCustomWindow(nil)}, stft.ErrInvalidWindow},
		{"custom window NaN", 4, 1, []stft.Option{stft.WithCustomWindow([]float64{1, math.NaN(), 1, 1})}, stft.ErrInvalidWindow},
		{"custom window zeros", 4, 1, []stft.Option{stft.WithCustomWindow(make([]float64, 4))}, stft.ErrInvalidWindow},
		{"window all zero", 64, 16, []stft.Option{stft.WithWindow(window.TypeRectangular, window.WithInvert())}, stft.ErrInvalidWindow},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, err := stft.New(tc.nfft, tc.hop, tc.opts...)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}

			if s != nil {
				t.Fatal("non-nil STFT on error")
			}

			s32, err := stft.New32(tc.nfft, tc.hop, tc.opts...)
			if !errors.Is(err, tc.wantErr) || s32 != nil {
				t.Fatalf("New32: got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestMethodErrors(t *testing.T) {
	t.Parallel()

	s, err := stft.New(256, 64)
	if err != nil {
		t.Fatal(err)
	}

	reflect, err := stft.New(256, 64, stft.WithCenter(stft.PadReflect))
	if err != nil {
		t.Fatal(err)
	}

	x := testutil.DeterministicNoise(1, 1, 1000)
	short := make([]complex128, s.Bins()-1)

	tests := []struct {
		name    string
		call    func() error
		wantErr error
	}{
		{"short dst", func() error { return s.FrameInto(short, x, 0) }, stft.ErrShortDst},
		{"frame range", func() error { return s.FrameInto(make([]complex128, s.Bins()), x, 100) }, stft.ErrFrameRange},
		{"reflect short frame", func() error {
			return reflect.FrameInto(make([]complex128, s.Bins()), x[:128], 0)
		}, stft.ErrSignalTooShort},
		{"reflect short forward", func() error {
			_, err := reflect.Forward(x[:100])

			return err
		}, stft.ErrSignalTooShort},
		{"inverse bins", func() error {
			_, err := s.Inverse([][]complex128{make([]complex128, 3)}, 10)

			return err
		}, stft.ErrInvalidSpectrum},
		{"inverse negative length", func() error {
			_, err := s.Inverse(nil, -1)

			return err
		}, stft.ErrInvalidSpectrum},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.call(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestAccessorsAndFrameCount(t *testing.T) {
	t.Parallel()

	s, err := stft.New(2048, 240, stft.WithNormalized())
	if err != nil {
		t.Fatal(err)
	}

	if s.NFFT() != 2048 || s.Hop() != 240 || s.Bins() != 1025 {
		t.Fatalf("NFFT/Hop/Bins = %d/%d/%d", s.NFFT(), s.Hop(), s.Bins())
	}

	if s.Padding() != stft.PadZero || !s.Normalized() {
		t.Fatalf("Padding/Normalized = %v/%v", s.Padding(), s.Normalized())
	}

	w := s.Window()
	want := window.Generate(window.TypeHann, 2048, window.WithPeriodic())

	if d := maxAbsDiff(t, w, want); d != 0 {
		t.Fatalf("default window differs from periodic Hann by %g", d)
	}

	w[1] = 42

	if s.Window()[1] == 42 {
		t.Fatal("Window returned internal storage")
	}

	counts := []struct {
		n, want int
	}{
		{-1, 0}, {0, 0}, {1, 1}, {240, 1}, {241, 2}, {24000, 100}, {24001, 101},
	}
	for _, c := range counts {
		if got := s.FrameCount(c.n); got != c.want {
			t.Fatalf("FrameCount(%d) = %d, want %d", c.n, got, c.want)
		}
	}

	none, err := stft.New(2048, 240, stft.WithCenter(stft.PadNone))
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct{ n, want int }{{2047, 0}, {2048, 1}, {2287, 1}, {2288, 2}} {
		if got := none.FrameCount(c.n); got != c.want {
			t.Fatalf("PadNone FrameCount(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestPaddingString(t *testing.T) {
	t.Parallel()

	for p, want := range map[stft.Padding]string{
		stft.PadNone: "none", stft.PadZero: "zero", stft.PadReflect: "reflect", stft.Padding(9): "Padding(9)",
	} {
		if got := p.String(); got != want {
			t.Fatalf("%d: got %q, want %q", int(p), got, want)
		}
	}
}

func TestCustomWindowAndEmptySignal(t *testing.T) {
	t.Parallel()

	w := window.Generate(window.TypeHamming, 128, window.WithPeriodic())

	s, err := stft.New(128, 32, stft.WithCustomWindow(w))
	if err != nil {
		t.Fatal(err)
	}

	w[0] = 99 // the option copied the slice

	if s.Window()[0] == 99 {
		t.Fatal("WithCustomWindow did not copy its input")
	}

	spec, err := s.Forward(nil)
	if err != nil || len(spec) != 0 {
		t.Fatalf("Forward(nil) = %d frames, %v", len(spec), err)
	}

	y, err := s.Inverse(nil, 0)
	if err != nil || len(y) != 0 {
		t.Fatalf("Inverse(nil, 0) = %d samples, %v", len(y), err)
	}

	x := testutil.DeterministicNoise(4, 1, 1000)
	if d := maxAbsDiff(t, roundTrip(t, s, x), x); d > 1e-12 {
		t.Fatalf("custom window reconstruction error %g", d)
	}
}

func TestCloneConcurrentDeterministic(t *testing.T) {
	t.Parallel()

	s, err := stft.New(1024, 256, stft.WithCenter(stft.PadReflect))
	if err != nil {
		t.Fatal(err)
	}

	x := testutil.DeterministicNoise(8, 1, 20000)

	want, err := s.Forward(x)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 4

	results := make([][][]complex128, workers)

	var wg sync.WaitGroup

	for i := range workers {
		c := s.Clone()

		wg.Go(func() {
			spec, err := c.Forward(x)
			if err != nil {
				t.Error(err)

				return
			}

			results[i] = spec
		})
	}

	wg.Wait()

	for i, got := range results {
		if len(got) != len(want) {
			t.Fatalf("worker %d: %d frames, want %d", i, len(got), len(want))
		}

		for f := range want {
			for k := range want[f] {
				if got[f][k] != want[f][k] {
					t.Fatalf("worker %d frame %d bin %d differs", i, f, k)
				}
			}
		}
	}
}

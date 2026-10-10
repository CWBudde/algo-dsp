package cqt

import (
	"errors"
	"math"
	"slices"
	"sync"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// basicPitchOptions is the configuration of basic-pitch's CQT front end.
func basicPitchOptions() []Option {
	return []Option{WithHopLength(256), WithFMin(27.5), WithBins(309), WithBinsPerOctave(36)}
}

func newBasicPitch(tb testing.TB) *Transform {
	tb.Helper()

	tr, err := New(22050, basicPitchOptions()...)
	if err != nil {
		tb.Fatal(err)
	}

	return tr
}

func sine(n int, freq, sr float64) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = math.Sin(2 * math.Pi * freq * float64(i) / sr)
	}

	return x
}

func TestNewErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sr   float64
		opts []Option
		want error
	}{
		{"zero sample rate", 0, nil, ErrInvalidOption},
		{"negative sample rate", -1, nil, ErrInvalidOption},
		{"NaN sample rate", math.NaN(), nil, ErrInvalidOption},
		{"infinite sample rate", math.Inf(1), nil, ErrInvalidOption},
		{"nil option", 22050, []Option{WithBins(12), nil}, ErrNilOption},
		{"hop 0", 22050, []Option{WithHopLength(0)}, ErrInvalidOption},
		{"fmin 0", 22050, []Option{WithFMin(0)}, ErrInvalidOption},
		{"fmin NaN", 22050, []Option{WithFMin(math.NaN())}, ErrInvalidOption},
		{"fmin Inf", 22050, []Option{WithFMin(math.Inf(1))}, ErrInvalidOption},
		{"bins 0", 22050, []Option{WithBins(0)}, ErrInvalidOption},
		{"bins per octave 0", 22050, []Option{WithBinsPerOctave(0)}, ErrInvalidOption},
		{"filter scale 0", 22050, []Option{WithFilterScale(0)}, ErrInvalidOption},
		{"filter scale Inf", 22050, []Option{WithFilterScale(math.Inf(1))}, ErrInvalidOption},
		{"window type negative", 22050, []Option{WithWindow(window.Type(-1))}, ErrInvalidOption},
		{"window type unknown", 22050, []Option{WithWindow(window.TypeFreeCosine + 1)}, ErrInvalidOption},
		{
			"window all zero", 22050,
			[]Option{WithWindow(window.TypeFreeCosine, window.WithCustomCoeffs([]float64{0}))},
			ErrInvalidOption,
		},
		{
			"window non-finite", 22050,
			[]Option{WithWindow(window.TypeFreeCosine, window.WithCustomCoeffs([]float64{math.Inf(1)}))},
			ErrInvalidOption,
		},
		{"basis norm", 22050, []Option{WithBasisNorm(Norm(3))}, ErrInvalidOption},
		{"padding", 22050, []Option{WithPadding(Padding(2))}, ErrInvalidOption},
		{"normalization", 22050, []Option{WithNormalization(Normalization(3))}, ErrInvalidOption},
		{"output", 22050, []Option{WithOutput(Output(2))}, ErrInvalidOption},
		{"kernel too long", 22050, []Option{WithFilterScale(1e9)}, ErrInvalidOption},
		{"single-sample kernels", 22050, []Option{WithFilterScale(1e-6)}, ErrInvalidOption},
		{"top bin above Nyquist", 22050, []Option{WithFMin(10000), WithBins(24)}, ErrNyquist},
		{"too many octaves for any hop", 22050, []Option{WithFMin(1e-300), WithBins(12 * 900)}, ErrHop},
		// nnAudio defaults have 7 octaves, so the hop must be divisible by 64.
		{"hop not divisible", 22050, []Option{WithHopLength(100)}, ErrHop},
		// Early downsampling by 4 turns hop 3 into 0.
		{"hop downsampled to 0", 22050, []Option{WithHopLength(3), WithFMin(30), WithBins(12)}, ErrHop},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr, err := New(tc.sr, tc.opts...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("New error = %v, want %v", err, tc.want)
			}

			if tr != nil {
				t.Fatal("New returned a Transform along with an error")
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	t.Parallel()

	tr, err := New(22050)
	if err != nil {
		t.Fatal(err)
	}

	if tr.NumBins() != 84 || tr.Octaves() != 7 || tr.Hop() != 512 || tr.DownsampleFactor() != 1 {
		t.Errorf("defaults: bins %d octaves %d hop %d factor %d, want 84 7 512 1",
			tr.NumBins(), tr.Octaves(), tr.Hop(), tr.DownsampleFactor())
	}

	if f := tr.Frequencies(); f[0] != 32.70 {
		t.Errorf("Frequencies()[0] = %g, want 32.70", f[0])
	}
}

func TestEnumStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		got, want string
	}{
		{NormNone.String(), "none"},
		{NormL1.String(), "l1"},
		{NormL2.String(), "l2"},
		{Norm(7).String(), "Norm(7)"},
		{PadReflect.String(), "reflect"},
		{PadConstant.String(), "constant"},
		{Padding(7).String(), "Padding(7)"},
		{NormalizationLibrosa.String(), "librosa"},
		{NormalizationConvolutional.String(), "convolutional"},
		{NormalizationWrap.String(), "wrap"},
		{Normalization(7).String(), "Normalization(7)"},
		{OutputMagnitude.String(), "magnitude"},
		{OutputComplex.String(), "complex"},
		{Output(7).String(), "Output(7)"},
	}

	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("String() = %q, want %q", tc.got, tc.want)
		}
	}
}

// octaveLengths returns the signal length of every octave, top first, for n
// input samples.
func octaveLengths(tr *Transform, n int) []int {
	if tr.factor > 1 {
		n = decimatedLength(n, lowpassTaps, tr.factor)
	}

	out := []int{n}
	for range tr.nOctaves - 1 {
		n = decimatedLength(n, lowpassTaps, 2)
		out = append(out, n)
	}

	return out
}

// TestReflectFallback checks that the fixtures flagged reflect_fallback are
// exactly those where an octave is not longer than nfft/2 samples, the case
// in which torch's reflection pad raises and nnAudio zero pads instead.
func TestReflectFallback(t *testing.T) {
	t.Parallel()

	for _, fx := range loadFixtures(t) {
		tr, err := New(fx.Config.SR, fx.Config.options(t)...)
		if err != nil {
			t.Fatal(err)
		}

		fallback := false

		if tr.padding == PadReflect {
			for _, n := range octaveLengths(tr, fx.Length) {
				fallback = fallback || n <= tr.NFFT()/2
			}
		}

		if fallback != fx.ReflectFallback {
			t.Errorf("%s: fallback = %v, fixture says %v", fx.Name, fallback, fx.ReflectFallback)
		}
	}
}

func TestFrameCountsAgree(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)

	for _, n := range []int{2048, 4000, 22050, 43844, 44100, 100000} {
		frames := tr.NumFrames(n)
		hop := tr.Hop()

		for o, l := range octaveLengths(tr, n) {
			if got := (l+2*(tr.NFFT()/2)-tr.NFFT())/hop + 1; got != frames {
				t.Errorf("n=%d octave %d: %d frames, NumFrames %d", n, o, got, frames)
			}

			hop /= 2
		}
	}
}

func TestSignalTooShort(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)

	// Find the shortest length that can be processed.
	minLen := 1
	for tr.NumFrames(minLen) == 0 {
		minLen++
	}

	for _, n := range []int{0, 1, minLen - 1} {
		if tr.NumFrames(n) != 0 || tr.OutputLen(n) != 0 {
			t.Errorf("n=%d: NumFrames %d OutputLen %d, want 0", n, tr.NumFrames(n), tr.OutputLen(n))
		}

		x := make([]float64, n)

		_, err := tr.Process(x)
		if !errors.Is(err, ErrSignalTooShort) {
			t.Errorf("Process(%d samples) error = %v, want ErrSignalTooShort", n, err)
		}

		err = tr.ProcessInto(make([]float64, 1000), x)
		if !errors.Is(err, ErrSignalTooShort) {
			t.Errorf("ProcessInto(%d samples) error = %v, want ErrSignalTooShort", n, err)
		}

		err = tr.ProcessInto32(make([]float32, 1000), make([]float32, n))
		if !errors.Is(err, ErrSignalTooShort) {
			t.Errorf("ProcessInto32(%d samples) error = %v, want ErrSignalTooShort", n, err)
		}
	}

	y, err := tr.Process(make([]float64, minLen))
	if err != nil || len(y) != tr.OutputLen(minLen) {
		t.Errorf("Process(%d samples) = %d values, %v", minLen, len(y), err)
	}
}

// TestSingleSample runs a single-octave transform without early downsampling
// on one sample: reflect padding falls back to zeros.
func TestSingleSample(t *testing.T) {
	t.Parallel()

	tr, err := New(8000, WithFMin(1000), WithBins(12), WithHopLength(1), WithEarlyDownsampling(false))
	if err != nil {
		t.Fatal(err)
	}

	got, err := tr.Process([]float64{1})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2*12 {
		t.Fatalf("got %d values, want 24", len(got))
	}

	c, err := New(8000, WithFMin(1000), WithBins(12), WithHopLength(1), WithEarlyDownsampling(false),
		WithPadding(PadConstant))
	if err != nil {
		t.Fatal(err)
	}

	want, err := c.Process([]float64{1})
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(got, want) {
		t.Errorf("reflect fallback %v differs from constant padding %v", got, want)
	}
}

func TestShortDst(t *testing.T) {
	t.Parallel()

	for _, out := range []Output{OutputMagnitude, OutputComplex} {
		tr, err := New(22050, append(basicPitchOptions(), WithOutput(out))...)
		if err != nil {
			t.Fatal(err)
		}

		x := sine(8192, 440, 22050)
		need := tr.OutputLen(len(x))

		err = tr.ProcessInto(make([]float64, need-1), x)
		if !errors.Is(err, ErrShortDst) {
			t.Errorf("%v: ProcessInto error = %v, want ErrShortDst", out, err)
		}

		x32 := make([]float32, len(x))
		for i, v := range x {
			x32[i] = float32(v)
		}

		err = tr.ProcessInto32(make([]float32, need-1), x32)
		if !errors.Is(err, ErrShortDst) {
			t.Errorf("%v: ProcessInto32 error = %v, want ErrShortDst", out, err)
		}

		// Capacity beyond OutputLen is left untouched.
		dst := make([]float64, need+3)
		for i := range dst {
			dst[i] = -7
		}

		err = tr.ProcessInto(dst, x)
		if err != nil {
			t.Fatal(err)
		}

		if !slices.Equal(dst[need:], []float64{-7, -7, -7}) {
			t.Errorf("%v: tail of dst modified: %v", out, dst[need:])
		}
	}
}

func TestAccessorsReturnCopies(t *testing.T) {
	t.Parallel()

	// The early_ds8_complex configuration downsamples early by 8.
	tr, err := New(22050, WithHopLength(1024), WithFMin(30), WithBins(48))
	if err != nil {
		t.Fatal(err)
	}

	tr.Frequencies()[0] = -1
	tr.Lengths()[0] = -1
	tr.Lowpass()[0] = -1
	tr.EarlyLowpass()[0] = -1
	tr.Kernels()[0][tr.kernels[0].start] = -1

	if tr.Frequencies()[0] != 30 || tr.Lengths()[0] == -1 || tr.Lowpass()[0] == -1 ||
		tr.EarlyLowpass()[0] == -1 || tr.Kernels()[0][tr.kernels[0].start] == -1 {
		t.Error("accessor exposed internal state")
	}

	if tr.DownsampleFactor() != 8 || tr.SampleRate() != 22050.0/8 {
		t.Errorf("SampleRate = %g", tr.SampleRate())
	}
}

func TestZeroAllocs(t *testing.T) {
	tr := newBasicPitch(t)
	x := testSignal(t)
	dst := make([]float64, tr.OutputLen(len(x)))

	x32 := make([]float32, len(x))
	for i, v := range x {
		x32[i] = float32(v)
	}

	dst32 := make([]float32, len(dst))

	// Warm up the scratch buffers.
	if err := tr.ProcessInto(dst, x); err != nil {
		t.Fatal(err)
	}

	if err := tr.ProcessInto32(dst32, x32); err != nil {
		t.Fatal(err)
	}

	if a := testing.AllocsPerRun(3, func() { _ = tr.ProcessInto(dst, x) }); a != 0 {
		t.Errorf("ProcessInto: %v allocs/op, want 0", a)
	}

	if a := testing.AllocsPerRun(3, func() { _ = tr.ProcessInto32(dst32, x32) }); a != 0 {
		t.Errorf("ProcessInto32: %v allocs/op, want 0", a)
	}

	// Shorter inputs reuse the grown buffers.
	if a := testing.AllocsPerRun(3, func() { _ = tr.ProcessInto(dst, x[:20000]) }); a != 0 {
		t.Errorf("ProcessInto (shorter input): %v allocs/op, want 0", a)
	}
}

func TestCloneConcurrent(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)
	x := testSignal(t)

	want, err := tr.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 4

	results := make([][]float64, workers)
	errs := make([]error, workers)

	// Clone reads the receiver, so every clone is made before any worker
	// starts using the original.
	transforms := make([]*Transform, workers)
	transforms[0] = tr

	for w := 1; w < workers; w++ {
		transforms[w] = tr.Clone()
	}

	var wg sync.WaitGroup

	for w, c := range transforms {
		wg.Go(func() {
			dst := make([]float64, c.OutputLen(len(x)))
			for range 2 {
				errs[w] = c.ProcessInto(dst, x)
			}

			results[w] = dst
		})
	}

	wg.Wait()

	for w := range workers {
		if errs[w] != nil {
			t.Fatalf("worker %d: %v", w, errs[w])
		}

		if !slices.Equal(results[w], want) {
			t.Errorf("worker %d: result differs from sequential run", w)
		}
	}
}

// TestSinePeak checks that a sine at a bin's centre frequency peaks at that
// bin in the middle frame.
func TestSinePeak(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)
	freqs := tr.Frequencies()
	bins := tr.NumBins()

	for _, b := range []int{40, 75, 120, 160, 200, 250, 300} {
		x := sine(43844, freqs[b], 22050)

		got, err := tr.Process(x)
		if err != nil {
			t.Fatal(err)
		}

		mid := tr.NumFrames(len(x)) / 2
		row := got[mid*bins : (mid+1)*bins]

		peak := 0
		for i, v := range row {
			if v > row[peak] {
				peak = i
			}
		}

		if peak != b {
			t.Errorf("sine at %.2f Hz (bin %d) peaks at bin %d", freqs[b], b, peak)
		}
	}
}

// TestLinearity checks that complex output is linear in the input.
func TestLinearity(t *testing.T) {
	t.Parallel()

	tr, err := New(22050, append(basicPitchOptions(), WithOutput(OutputComplex))...)
	if err != nil {
		t.Fatal(err)
	}

	x := testSignal(t)[:20000]
	y := sine(len(x), 1234.5, 22050)
	z := make([]float64, len(x))

	for i := range z {
		z[i] = 2*x[i] - 0.5*y[i]
	}

	tx, _ := tr.Process(x)
	ty, _ := tr.Process(y)

	tz, err := tr.Process(z)
	if err != nil {
		t.Fatal(err)
	}

	peak := 0.0
	for _, v := range tz {
		peak = max(peak, math.Abs(v))
	}

	for i := range tz {
		if d := math.Abs(tz[i] - (2*tx[i] - 0.5*ty[i])); d > 1e-12*peak {
			t.Fatalf("index %d: T(2x-y/2) = %g, 2T(x)-T(y)/2 = %g", i, tz[i], 2*tx[i]-0.5*ty[i])
		}
	}
}

func TestNaNPropagates(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)
	x := sine(8192, 440, 22050)
	x[4000] = math.NaN()

	got, err := tr.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	nan := 0

	for _, v := range got {
		if math.IsNaN(v) {
			nan++
		}
	}

	if nan == 0 {
		t.Error("NaN input did not propagate")
	}
}

func BenchmarkNewBasicPitch(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_, err := New(22050, basicPitchOptions()...)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessIntoBasicPitch(b *testing.B) {
	tr := newBasicPitch(b)
	x := testSignal(b)
	dst := make([]float64, tr.OutputLen(len(x)))

	// Grow the scratch buffers before measuring the steady state.
	err := tr.ProcessInto(dst, x)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(8 * len(x)))

	for b.Loop() {
		err := tr.ProcessInto(dst, x)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessInto32BasicPitch(b *testing.B) {
	tr := newBasicPitch(b)
	x64 := testSignal(b)

	x := make([]float32, len(x64))
	for i, v := range x64 {
		x[i] = float32(v)
	}

	dst := make([]float32, tr.OutputLen(len(x)))

	// Grow the scratch buffers before measuring the steady state.
	err := tr.ProcessInto32(dst, x)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(4 * len(x)))

	for b.Loop() {
		err := tr.ProcessInto32(dst, x)
		if err != nil {
			b.Fatal(err)
		}
	}
}

package stft

import (
	"errors"
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/window"
	algofft "github.com/cwbudde/algo-fft"
)

func streamRoundTrip[F algofft.Float, C algofft.Complex](t *testing.T, padding Padding, normalized bool, tolerance float64) {
	t.Helper()

	opts := []Option{WithWindow(window.TypeHamming), WithCenter(padding)}
	if normalized {
		opts = append(opts, WithNormalized())
	}

	transform, err := newTransform[F, C](32, 8, opts)
	if err != nil {
		t.Fatal(err)
	}

	length := 3107
	if padding == PadNone {
		length = 3104
	}

	x := make([]F, length)
	for i := range x {
		x[i] = F(.3*math.Sin(float64(i)*.119) + .07*math.Cos(float64(i)*.47))
	}

	frames, err := transform.Forward(x)
	if err != nil {
		t.Fatal(err)
	}

	want, err := transform.Inverse(frames, len(x))
	if err != nil {
		t.Fatal(err)
	}

	analysis, err := transform.AnalysisStream()
	if err != nil {
		t.Fatal(err)
	}

	synthesis, err := transform.InverseStream(int64(len(x)))
	if err != nil {
		t.Fatal(err)
	}

	for _, partition := range []int{1, 13, 128, 3107} {
		analysis.Reset()
		synthesis.Reset()

		seen := 0
		output := make([]F, len(x))
		emitOutput := func(offset int64, samples []F) error { copy(output[int(offset):], samples); return nil }

		emitFrame := func(index int64, bins []C) error {
			if index != int64(seen) || seen >= len(frames) {
				t.Fatalf("unexpected frame%d", index)
			}

			for i, bin := range bins {
				if cmplx.Abs(complex128(bin)-complex128(frames[seen][i])) > tolerance {
					t.Fatalf("frame%d bin%d", seen, i)
				}
			}

			seen++

			return synthesis.ProcessFrame(bins, emitOutput)
		}
		for offset := 0; offset < len(x); offset += partition {
			if err := analysis.Process(x[offset:min(offset+partition, len(x))], emitFrame); err != nil {
				t.Fatal(err)
			}
		}

		for {
			done, err := analysis.FinishStep(1, emitFrame)
			if err != nil {
				t.Fatal(err)
			}

			if done {
				break
			}
		}

		for {
			done, err := synthesis.FinishStep(17, emitOutput)
			if err != nil {
				t.Fatal(err)
			}

			if done {
				break
			}
		}

		if seen != len(frames) {
			t.Fatalf("frames%d want%d", seen, len(frames))
		}

		for i, value := range output {
			if math.Abs(float64(value-want[i])) > tolerance {
				t.Fatalf("sample%d got%g want%g", i, value, want[i])
			}
		}
	}
}

func TestStreamingSTFTPartitionInverseReset(t *testing.T) {
	for _, padding := range []Padding{PadZero, PadReflect, PadNone} {
		for _, normalized := range []bool{false, true} {
			streamRoundTrip[float64, complex128](t, padding, normalized, 1e-12)
			streamRoundTrip[float32, complex64](t, padding, normalized, 2e-5)
		}
	}
}

func TestStreamingSTFTIndependentDFT(t *testing.T) {
	x := []float64{1, -2, .5, 3, -1, 4, .25, -.75, 2}

	s, err := NewStream(8, 3, WithWindow(window.TypeRectangular))
	if err != nil {
		t.Fatal(err)
	}

	emit := func(frame int64, bins []complex128) error {
		for k, got := range bins {
			want := complex(0, 0)

			for j := 0; j < 8; j++ {
				at := int(frame)*3 - 4 + j
				if at >= 0 && at < len(x) {
					want += complex(x[at], 0) * cmplx.Exp(complex(0, -2*math.Pi*float64(k*j)/8))
				}
			}

			if cmplx.Abs(got-want) > 1e-12 {
				t.Fatalf("frame%d bin%d got%v want%v", frame, k, got, want)
			}
		}

		return nil
	}
	if err := s.Process(x, emit); err != nil {
		t.Fatal(err)
	}

	if _, err := s.FinishStep(8, emit); err != nil {
		t.Fatal(err)
	}
}

func TestStreamingSTFTPreparedAllocations(t *testing.T) {
	tf, _ := New(256, 64)
	a, _ := tf.AnalysisStream()
	s, _ := tf.InverseStream(4096)
	input := make([]float64, 128)
	input[0] = .2
	output := func(int64, []float64) error { return nil }
	frame := func(_ int64, bins []complex128) error { return s.ProcessFrame(bins, output) }

	if n := testing.AllocsPerRun(5, func() {
		a.Reset()
		s.Reset()

		for range 32 {
			if err := a.Process(input, frame); err != nil {
				panic(err)
			}
		}

		for {
			done, err := a.FinishStep(16, frame)
			if err != nil {
				panic(err)
			}

			if done {
				break
			}
		}

		for {
			done, err := s.FinishStep(256, output)
			if err != nil {
				panic(err)
			}

			if done {
				break
			}
		}
	}); n != 0 {
		t.Fatalf("allocations%g", n)
	}
}

func TestStreamingSTFTRejectsInvalidWithoutAdvancing(t *testing.T) {
	if _, err := NewStream(1<<20, 256); err == nil {
		t.Fatal("oversized FFT accepted")
	}

	s, _ := NewStream(16, 4)

	emit := func(int64, []complex128) error { return nil }
	if err := s.Process([]float64{1, math.NaN()}, emit); err == nil || s.count != 0 {
		t.Fatal("nonfinite input changed state")
	}

	if _, err := s.FinishStep(0, emit); err == nil {
		t.Fatal("zero budget accepted")
	}

	s.Reset()

	sentinel := errors.New("cancel")
	if err := s.Process(make([]float64, 32), func(int64, []complex128) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}

	if err := s.Process(nil, emit); err == nil {
		t.Fatal("callback failure not terminal")
	}

	s.Reset()

	if err := s.Process(make([]float64, 32), emit); err != nil {
		t.Fatal(err)
	}

	reflect, _ := NewStream(16, 4, WithCenter(PadReflect))
	if _, err := reflect.FinishStep(1, emit); !errors.Is(err, ErrSignalTooShort) {
		t.Fatal(err)
	}

	tf, _ := New(16, 4)

	inverse, _ := tf.InverseStream(20)
	if _, err := inverse.FinishStep(20, func(int64, []float64) error { return nil }); !errors.Is(err, ErrWindowSumZero) {
		t.Fatal(err)
	}
}

func TestStreamingSTFTEmptyOddAndWindowConfigurations(t *testing.T) {
	for _, kind := range []window.Type{window.TypeHann, window.TypeHamming, window.TypeBlackman, window.TypeRectangular} {
		for _, padding := range []Padding{PadZero, PadReflect, PadNone} {
			tf, err := New32(5, 2, WithWindow(kind), WithCenter(padding))
			if err != nil {
				t.Fatal(err)
			}

			x := []float32{.1, .2, .3, .4, .5, .6, .7, .8, .9}

			want, err := tf.Forward(x)
			if err != nil {
				t.Fatal(err)
			}

			stream, err := NewStream32(5, 2, WithWindow(kind), WithCenter(padding))
			if err != nil {
				t.Fatal(err)
			}

			count := 0

			emit := func(frame int64, bins []complex64) error {
				for i, bin := range bins {
					if bin != want[frame][i] {
						t.Fatalf("window%v padding%s frame%d", kind, padding, frame)
					}
				}

				count++

				return nil
			}
			for _, sample := range x {
				if err := stream.Process([]float32{sample}, emit); err != nil {
					t.Fatal(err)
				}
			}

			for {
				done, err := stream.FinishStep(1, emit)
				if err != nil {
					t.Fatal(err)
				}

				if done {
					break
				}
			}

			if count != len(want) {
				t.Fatal(count, len(want))
			}
		}
	}

	emit := func(int64, []complex128) error { return nil }

	zero, _ := NewStream(8, 2)
	if done, err := zero.FinishStep(1, emit); err != nil || !done {
		t.Fatal(done, err)
	}

	if done, err := zero.FinishStep(1, emit); err != nil || !done {
		t.Fatal(done, err)
	}

	if err := zero.Process(nil, emit); err == nil {
		t.Fatal("EOF input accepted")
	}

	if _, err := NewStream32(8, 9); err == nil {
		t.Fatal("oversized hop accepted")
	}

	var unconfigured STFT
	if _, err := unconfigured.AnalysisStream(); err == nil {
		t.Fatal("unconfigured analysis accepted")
	}

	if _, err := unconfigured.InverseStream(1); err == nil {
		t.Fatal("unconfigured synthesis accepted")
	}
}

func TestStreamingInverseValidationOwnershipAndCallbackFailure(t *testing.T) {
	tf, _ := New(16, 4)
	inverse, _ := tf.InverseStream(20)
	bins := make([]complex128, 9)
	bins[0] = 16

	emit := func(int64, []float64) error { return nil }
	if err := inverse.ProcessFrame(bins[:8], emit); err == nil {
		t.Fatal("invalid shape")
	}

	bins[8] = complex(math.NaN(), 0)
	if err := inverse.ProcessFrame(bins, emit); err == nil || inverse.frame != 0 {
		t.Fatal("unsafe frame changed state")
	}

	bins[8] = 0
	for range 3 {
		if err := inverse.ProcessFrame(bins, emit); err != nil {
			t.Fatal(err)
		}
	}

	if bins[0] != 16 {
		t.Fatal("caller bins modified")
	}

	sentinel := errors.New("cancel")
	if err := inverse.ProcessFrame(bins, func(int64, []float64) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}

	if _, err := inverse.FinishStep(1, emit); err == nil {
		t.Fatal("terminal callback failure ignored")
	}

	inverse.Reset()

	if inverse.frame != 0 || inverse.written != 0 || inverse.failure != nil {
		t.Fatal("reset failed")
	}

	if _, err := inverse.FinishStep(0, emit); err == nil {
		t.Fatal("invalid budget")
	}

	if _, err := tf.InverseStream(-1); err == nil {
		t.Fatal("negative length")
	}

	empty, _ := tf.InverseStream(0)
	if done, err := empty.FinishStep(1, emit); err != nil || !done {
		t.Fatal(done, err)
	}

	if done, err := empty.FinishStep(1, emit); err != nil || !done {
		t.Fatal(done, err)
	}
}

func BenchmarkStreamingSTFT(b *testing.B) {
	s, _ := NewStream(1024, 256)
	input := make([]float64, 128)
	input[0] = .2
	emit := func(int64, []complex128) error { return nil }

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := s.Process(input, emit); err != nil {
			b.Fatal(err)
		}
	}
}

func TestInverseStreamRejectsFramesDuringPartialFinish(t *testing.T) {
	tf, _ := New(16, 4)
	s, _ := tf.InverseStream(20)
	bins := make([]complex128, 9)
	bins[0] = 16

	emit := func(int64, []float64) error { return nil }
	for range 5 {
		if err := s.ProcessFrame(bins, emit); err != nil {
			t.Fatal(err)
		}
	}

	done, err := s.FinishStep(1, emit)
	if err != nil || done {
		t.Fatal(done, err)
	}

	frame := s.frame
	if err := s.ProcessFrame(bins, emit); err == nil || s.frame != frame {
		t.Fatal("frame accepted after finish began")
	}

	for !done {
		done, err = s.FinishStep(1, emit)
		if err != nil {
			t.Fatal(err)
		}
	}

	s.Reset()

	if err := s.ProcessFrame(bins, emit); err != nil {
		t.Fatal(err)
	}
}

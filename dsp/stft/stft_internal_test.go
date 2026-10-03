package stft

import (
	"errors"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/window"
	"github.com/cwbudde/algo-dsp/internal/testutil"
	algofft "github.com/cwbudde/algo-fft"
)

// referenceFrame is the straightforward framing loop used by the
// AudioVisualizer analysis: zero outside the signal, sample*window, then a
// forward real FFT.
func referenceFrame(t *testing.T, x []float64, nfft, hop, frame int) []complex128 {
	t.Helper()

	plan, err := algofft.NewPlanReal64(nfft)
	if err != nil {
		t.Fatal(err)
	}

	w, err := window.Hann(nfft, window.WithPeriodic())
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]float64, nfft)
	center := frame * hop

	for i := range buf {
		j := center + i - nfft/2

		buf[i] = 0
		if j >= 0 && j < len(x) {
			buf[i] = x[j] * w[i]
		}
	}

	sp := make([]complex128, nfft/2+1)

	err = plan.Forward(sp, buf)
	if err != nil {
		t.Fatal(err)
	}

	return sp
}

func TestFrameIntoMatchesReferenceBitExact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nfft, hop int
		n         int
	}{
		{"features 2048/240", 2048, 240, 24000},
		{"melody 4096/240", 4096, 240, 12000},
		{"short signal", 512, 128, 300},
		{"odd nfft", 255, 64, 1000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			x := testutil.DeterministicNoise(7, 0.8, tc.n)

			s, err := New(tc.nfft, tc.hop)
			if err != nil {
				t.Fatal(err)
			}

			dst := make([]complex128, s.Bins())

			for frame := range s.FrameCount(tc.n) {
				err := s.FrameInto(dst, x, frame)
				if err != nil {
					t.Fatal(err)
				}

				want := referenceFrame(t, x, tc.nfft, tc.hop, frame)
				for k := range want {
					if dst[k] != want[k] {
						t.Fatalf("frame %d bin %d: got %v want %v", frame, k, dst[k], want[k])
					}
				}
			}
		})
	}
}

// TestFrameIntoZeroAlloc is not parallel: testing.AllocsPerRun panics in
// parallel tests.
func TestFrameIntoZeroAlloc(t *testing.T) {
	x := testutil.DeterministicNoise(1, 1, 24000)

	for _, p := range []Padding{PadZero, PadReflect, PadNone} {
		s, err := New(2048, 240, WithCenter(p), WithNormalized())
		if err != nil {
			t.Fatal(err)
		}

		dst := make([]complex128, s.Bins())

		allocs := testing.AllocsPerRun(50, func() {
			_ = s.FrameInto(dst, x, 3)
		})
		if allocs != 0 {
			t.Fatalf("%v: FrameInto allocates %.1f times per call", p, allocs)
		}
	}

	s32, err := New32(2048, 240)
	if err != nil {
		t.Fatal(err)
	}

	x32 := make([]float32, len(x))
	for i, v := range x {
		x32[i] = float32(v)
	}

	dst32 := make([]complex64, s32.Bins())

	allocs := testing.AllocsPerRun(50, func() {
		_ = s32.FrameInto(dst32, x32, 3)
	})
	if allocs != 0 {
		t.Fatalf("float32 FrameInto allocates %.1f times per call", allocs)
	}
}

func TestDropEdgeImag(t *testing.T) {
	t.Parallel()

	s64 := []complex128{1 + 2i, 3 + 4i, 5 + 6i}
	dropEdgeImag(s64, true)

	if s64[0] != 1 || s64[1] != 3+4i || s64[2] != 5 {
		t.Fatalf("complex128: %v", s64)
	}

	s32 := []complex64{1 + 2i, 3 + 4i, 5 + 6i}
	dropEdgeImag(s32, false)

	if s32[0] != 1 || s32[1] != 3+4i || s32[2] != 5+6i {
		t.Fatalf("complex64: %v", s32)
	}

	s32b := []complex64{1 + 2i, 5 + 6i}
	dropEdgeImag(s32b, true)

	if s32b[1] != 5 {
		t.Fatalf("complex64 nyquist: %v", s32b)
	}
}

func TestCheckFrame(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		padding Padding
		n       int
		frame   int
		wantErr error
	}{
		{"zero first", PadZero, 1000, 0, nil},
		{"zero last ceil", PadZero, 1050, 10, nil},
		{"zero beyond ceil", PadZero, 1050, 11, ErrFrameRange},
		{"zero torch extra frame", PadZero, 1000, 10, nil},
		{"zero beyond torch", PadZero, 1000, 11, ErrFrameRange},
		{"zero negative", PadZero, 1000, -1, ErrFrameRange},
		{"zero empty", PadZero, 0, 0, ErrFrameRange},
		{"reflect short", PadReflect, 128, 0, ErrSignalTooShort},
		{"reflect min", PadReflect, 129, 1, nil},
		{"none last", PadNone, 1000, 7, nil},
		{"none beyond", PadNone, 1000, 8, ErrFrameRange},
		{"none short", PadNone, 255, 0, ErrFrameRange},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, err := New(256, 100, WithCenter(tc.padding))
			if err != nil {
				t.Fatal(err)
			}

			err = s.checkFrame(tc.n, tc.frame)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

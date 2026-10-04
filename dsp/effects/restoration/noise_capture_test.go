package restoration

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/stft"
)

func captureReader(input []float64) Reader {
	return func(dst []float64, start int64) int { return copy(dst, input[start:]) }
}

func TestNoiseCaptureUnbiasedConstant(t *testing.T) {
	transform, _ := stft.New(256, 64, stft.WithCenter(stft.PadNone))

	full := make([]float64, 256)
	for i := range full {
		full[i] = .125
	}

	bins := make([]complex128, 129)
	if err := transform.FrameInto(bins, full, 0); err != nil {
		t.Fatal(err)
	}

	for _, length := range []int{128, 129, 255, 256, 257, 1024, 1093} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			input := make([]float64, length)
			for i := range input {
				input[i] = .125
			}

			c, err := NewNoiseCapture(int64(length), captureReader(input), 256, 48000)
			if err != nil {
				t.Fatal(err)
			}

			if _, err = c.Profile(); err == nil {
				t.Fatal("incomplete profile")
			}

			for steps := 0; ; steps++ {
				done, err := c.Step(context.Background())
				if err != nil {
					t.Fatal(err)
				}

				if steps > length {
					t.Fatal("unbounded capture")
				}

				if done {
					break
				}
			}

			p, err := c.Profile()
			if err != nil || c.Progress() != int64(length) || p.Frames() < 1 {
				t.Fatalf("profile: %v", err)
			}

			for i, power := range p.Powers() {
				want := real(bins[i])*real(bins[i]) + imag(bins[i])*imag(bins[i])
				if math.Abs(power-want) > 1e-12 {
					t.Fatalf("bin %d: power %g != %g", i, power, want)
				}
			}

			if _, err = NewNoiseReducer(p, 24, "wiener"); err != nil {
				t.Fatal(err)
			}

			if done, err := c.Step(context.Background()); !done || err != nil {
				t.Fatal("completed capture")
			}
		})
	}
}

func TestNoiseCaptureFailures(t *testing.T) {
	input := make([]float64, 512)
	for _, tc := range []struct {
		length int64
		reader Reader
		size   int
		rate   float64
	}{
		{127, captureReader(input), 256, 48000}, {512, nil, 256, 48000}, {512, captureReader(input), 255, 48000}, {512, captureReader(input), 256, math.NaN()},
	} {
		if _, err := NewNoiseCapture(tc.length, tc.reader, tc.size, tc.rate); err == nil {
			t.Fatal("invalid capture")
		}
	}

	for _, reader := range []Reader{
		func(dst []float64, _ int64) int { return len(dst) - 1 },
		func(dst []float64, _ int64) int { dst[0] = math.Inf(1); return len(dst) },
	} {
		c, _ := NewNoiseCapture(512, reader, 256, 48000)
		if _, err := c.Step(context.Background()); err == nil {
			t.Fatal("bad reader")
		}

		if _, err := c.Profile(); err == nil {
			t.Fatal("failed profile exposed")
		}

		if _, err := c.Step(context.Background()); err == nil {
			t.Fatal("failure not terminal")
		}
	}

	c, _ := NewNoiseCapture(512, captureReader(input), 256, 48000)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Step(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestNoiseCaptureCompleteWindowReference(t *testing.T) {
	input := make([]float64, 1024)
	for i := range input {
		input[i] = math.Sin(float64(i) * .25)
	}

	c, _ := NewNoiseCapture(1024, captureReader(input), 256, 48000)
	reference, _ := NewNoiseProfile(256, 48000)
	transform, _ := stft.New(256, 64, stft.WithCenter(stft.PadNone))

	bins := make([]complex128, 129)
	for start := 0; start+256 <= len(input); start += 64 {
		if err := transform.FrameInto(bins, input[start:start+256], 0); err != nil {
			t.Fatal(err)
		}

		if err := reference.AddSpectrum(bins); err != nil {
			t.Fatal(err)
		}

		if _, err := c.Step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	profile, err := c.Profile()
	if err != nil || profile.Frames() != 13 {
		t.Fatalf("profile: %v", err)
	}

	for i, power := range profile.Powers() {
		if power != reference.power[i] {
			t.Fatalf("bin %d: %g != %g", i, power, reference.power[i])
		}
	}
}

func ExampleNoiseCapture() {
	input := make([]float64, 256)
	for i := range input {
		input[i] = .125
	}

	capture, _ := NewNoiseCapture(256, func(dst []float64, start int64) int { return copy(dst, input[start:]) }, 256, 48000)
	done, _ := capture.Step(context.Background())
	profile, _ := capture.Profile()
	fmt.Println(done, profile.Frames())
	// Output: true 1
}

func BenchmarkNoiseCaptureStep(b *testing.B) {
	input := make([]float64, 1024)
	capture, _ := NewNoiseCapture(1024, captureReader(input), 256, 48000)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		capture.start, capture.progress, capture.done = 0, 0, false
		if _, err := capture.Step(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

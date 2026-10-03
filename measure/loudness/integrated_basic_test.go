package loudness

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

func basicIntegratedTone(frames int) []float64 {
	signal := make([]float64, frames)
	for i := range signal {
		signal[i] = 0.5 * math.Sin(2*math.Pi*1000*float64(i)/48000)
	}

	return signal
}

func basicIntegratedFinish(t *testing.T, a *IntegratedAnalyzer) IntegratedResult {
	t.Helper()

	for {
		done, err := a.FinishStep(1)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			result, err := a.Result()
			if err != nil {
				t.Fatal(err)
			}

			return result
		}
	}
}

func TestIntegratedBasicCompleteWindowsAndReset(t *testing.T) {
	for _, frames := range []int{19200, 19201, 23999, 24000, 48000} {
		t.Run(stringBasicFrames(frames), func(t *testing.T) {
			a, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: int64(frames)})
			if err != nil {
				t.Fatal(err)
			}

			input := basicIntegratedTone(frames)
			if err := a.ProcessPlanar([][]float64{input}); err != nil {
				t.Fatal(err)
			}

			wantBlocks := 1 + (frames-19200)/4800
			if len(a.blocks) != wantBlocks {
				t.Fatalf("blocks=%d, want %d (no startup or EOF padding)", len(a.blocks), wantBlocks)
			}

			first := basicIntegratedFinish(t, a)
			if first.Frames != int64(frames) || first.SamplePeak != 0.5 || !integratedFinite(first.LUFS) {
				t.Fatalf("invalid completed result: %+v", first)
			}

			a.Reset()

			for _, sample := range input {
				if err := a.ProcessPlanar([][]float64{{sample}}); err != nil {
					t.Fatal(err)
				}
			}

			if got := basicIntegratedFinish(t, a); got != first {
				t.Fatalf("sample chunks/reset changed result: %+v != %+v", got, first)
			}
		})
	}
}

func stringBasicFrames(frames int) string {
	// A numerical label without taking a dependency on production formatting.
	return fmt.Sprintf("frames-%d", frames)
}

func TestIntegratedBasicNilAndEmptyCalls(t *testing.T) {
	var nilAnalyzer *IntegratedAnalyzer
	if err := nilAnalyzer.ProcessPlanar(nil); !errors.Is(err, ErrState) {
		t.Fatalf("nil process=%v", err)
	}

	if _, err := nilAnalyzer.FinishStep(1); !errors.Is(err, ErrState) {
		t.Fatalf("nil finish=%v", err)
	}

	if _, err := nilAnalyzer.Result(); !errors.Is(err, ErrState) {
		t.Fatalf("nil result=%v", err)
	}

	nilAnalyzer.Reset()

	var zero IntegratedAnalyzer
	if _, err := zero.FinishStep(1); !errors.Is(err, ErrState) {
		t.Fatalf("unconfigured finish=%v, want ErrState", err)
	}

	a, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 19200})
	if err != nil {
		t.Fatal(err)
	}

	if err := a.ProcessPlanar([][]float64{nil}); err != nil {
		t.Fatal(err)
	}

	if a.frames != 0 {
		t.Fatal("empty block advanced input")
	}
}

func TestIntegratedBasicFractionalHopHasNoCadenceDrift(t *testing.T) {
	const rate = 8005

	input := make([]float64, rate)
	for i := range input {
		input[i] = 0.5 * math.Sin(2*math.Pi*1000*float64(i)/rate)
	}

	var results [2]IntegratedResult

	for variant, chunk := range []int{rate, 127} {
		a, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: rate, Channels: 1, MaxFrames: rate})
		if err != nil {
			t.Fatal(err)
		}

		for start := 0; start < len(input); {
			end := start + min(chunk, len(input)-start)
			if err := a.ProcessPlanar([][]float64{input[start:end]}); err != nil {
				t.Fatal(err)
			}

			start = end
		}

		if a.gateIndex != 7 || len(a.blocks) != 7 || cap(a.blocks) != 7 {
			t.Fatalf("8005 Hz must have seven complete nominal windows: gates=%d blocks=%d capacity=%d", a.gateIndex, len(a.blocks), cap(a.blocks))
		}

		results[variant] = basicIntegratedFinish(t, a)
	}

	if results[0] != results[1] {
		t.Fatalf("fractional-hop partition changed result: %+v != %+v", results[0], results[1])
	}
}

func TestIntegratedBasicNoFalseResidualAfterHugeInput(t *testing.T) {
	a, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 48000 * 20})
	if err != nil {
		t.Fatal(err)
	}

	input := make([]float64, 4800)

	input[0] = 1e100
	if err := a.ProcessPlanar([][]float64{input}); err != nil {
		t.Fatal(err)
	}

	clear(input)

	for range 199 {
		if err := a.ProcessPlanar([][]float64{input}); err != nil {
			t.Fatal(err)
		}
	}
	// Filter tails have decayed far below the absolute gate by 20 seconds.
	energy := 0.0
	for _, value := range a.ring {
		energy += value
	}

	if energy/float64(len(a.ring)) > integratedAbsoluteEnergy() {
		t.Fatalf("false residual after huge transient: energy=%g", energy)
	}
}

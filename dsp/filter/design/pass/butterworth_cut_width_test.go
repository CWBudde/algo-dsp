package pass

import (
	"math"
	"testing"
)

func TestButterworthCutFloat64Cascade(t *testing.T) {
	// Seven-pole low pass derived from the bilinear transform at 5333 Hz
	// on the 48 kHz grid, with gain before the reverse-order pole pairs.
	input := []float64{1 + 0x1p-34, -0.419 + 0x1p-37, 0.75031, 0.03125, -0.2, 1e-11, 0.20147, 0.45}
	want := []float64{
		0.00016887531542143241, 0.0017650559632234777, 0.0088515698078186471, 0.02866531620520003,
		0.068044805297672428, 0.12699132208673911, 0.19401267126627481, 0.24772736058202161,
	}
	var cut ButterworthCut
	cut.Configure(5333, 48000, 7, false)

	for i, sample := range input {
		if got := cut.ProcessSample64(sample); math.Abs(got-want[i]) > 2e-16 {
			t.Fatalf("sample %d: got %.17g, want %.17g", i, got, want[i])
		}
	}
}

func TestButterworthCutWidthsShareState(t *testing.T) {
	var mixed, wide ButterworthCut
	mixed.Configure(8333, 48000, 5, true)
	wide.Configure(8333, 48000, 5, true)

	for i, sample := range []float64{0.75, -0.5, -0.25, 0.375, 0.625, 1, -1} {
		want := wide.ProcessSample64(sample)
		if i%2 == 0 {
			if got := mixed.ProcessSample(float32(sample)); got != float32(want) {
				t.Fatalf("float32 sample %d: got %g, want %g", i, got, float32(want))
			}
		} else if got := mixed.ProcessSample64(sample); got != want {
			t.Fatalf("float64 sample %d: got %g, want %g", i, got, want)
		}
	}
}

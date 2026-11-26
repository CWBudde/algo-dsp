package modulation

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/hilbert"
)

func TestDAVBode32FixedClockAndFirstOrder(t *testing.T) {
	var shifter DAVBode32
	if err := shifter.Configure(13, 1, .1); err != nil {
		t.Fatal(err)
	}
	if shifter.stepSine != float32(math.Sin(2*math.Pi*13/44100)) {
		t.Fatalf("LFO step %g", shifter.stepSine)
	}
	coeffs, err := hilbert.DesignCoefficients(1, .1)
	if err != nil {
		t.Fatal(err)
	}
	want := float32(-float64(float32(coeffs[0])) * float64(float32(math.Sqrt(.5))))
	up, down := shifter.ProcessSample(1)
	if up != want || down != want {
		t.Fatalf("first pair %g, %g; want %g", up, down, want)
	}
	if _, down := shifter.ProcessSample(0); down == 0 {
		t.Fatal("the delayed quadrature branch must contribute on the second sample")
	}
}

func TestDAVBode32RetuneResetAndAllocations(t *testing.T) {
	var shifter DAVBode32
	if err := shifter.Configure(22, 12, .1); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		shifter.ProcessSample(.5)
	}
	previous := shifter
	if err := shifter.Configure(22, 33, .1); err == nil || previous != shifter {
		t.Fatal("invalid order mutated the state")
	}
	next := previous
	if err := shifter.Configure(22, 12, .07); err != nil {
		t.Fatal(err)
	}
	if shifter.sine != previous.sine || shifter.history != previous.history {
		t.Fatal("bandwidth edit changed the phase or history")
	}
	if got := testing.AllocsPerRun(100, func() {
		if err := next.Configure(32, 32, .1); err != nil {
			t.Fatal(err)
		}
		next.ProcessSample(.1)
		if err := next.Configure(33, 12, .1); err != nil {
			t.Fatal(err)
		}
	}); got != 0 {
		t.Fatalf("processing and retuning allocated %g times", got)
	}
	shifter.Reset()
	if up, down := shifter.ProcessSample(0); up != 0 || down != 0 {
		t.Fatalf("nonzero reset samples %g, %g", up, down)
	}
}

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

// TestDAVBode32LargeOrderCapture pins the impulse response on both sides of
// the order-4 boundary, where the legacy implementation swaps the two
// quadrature branches, and for a typical large order.
func TestDAVBode32LargeOrderCapture(t *testing.T) {
	captures := map[int][8][2]float32{
		4: {
			{-0.030796656, -0.030796656},
			{-0.023779007, 0.023779007},
			{0.40577215, 0.40577215},
			{0.25014892, -0.25014892},
			{-0.37165588, -0.37165588},
			{0.004808728, -0.004808728},
			{-0.19321455, -0.19321455},
			{-0.11314197, 0.11314197},
		},
		5: {
			{-0.013196298, -0.013196298},
			{-0.012453528, 0.012453528},
			{0.26329002, 0.26329002},
			{0.21074897, -0.21074897},
			{-0.47489172, -0.47489172},
			{-0.18232068, 0.18232068},
			{-0.09750748, -0.09750748},
			{-0.24529247, 0.24529247},
		},
		12: {
			{0, 0},
			{-0.0005202791, -0.0005202791},
			{0.0010868821, -0.0010868821},
			{0.016640846, 0.016640846},
			{-0.033345178, 0.033345178},
			{-0.1185881, -0.1185881},
			{0.2288817, -0.2288817},
			{0.24092482, 0.24092482},
		},
	}

	const tolerance = 1e-6

	for order, capture := range captures {
		var shifter DAVBode32
		if err := shifter.Configure(1000, order, .1); err != nil {
			t.Fatal(err)
		}

		for i, want := range capture {
			input := float32(0)
			if i == 0 {
				input = 1
			}

			up, down := shifter.ProcessSample(input)
			if math.Abs(float64(up-want[0])) > tolerance ||
				math.Abs(float64(down-want[1])) > tolerance {
				t.Fatalf("order %d sample %d: got %g, %g; want %g, %g",
					order, i, up, down, want[0], want[1])
			}
		}
	}
}

// TestDAVBode32QuadratureBranchSelection checks for every order around the
// boundary that the outputs come from the last two filter stages, swapped
// only above order 4.
func TestDAVBode32QuadratureBranchSelection(t *testing.T) {
	const sqrtHalf32 = float64(float32(.7071067811865475244))

	for order := 2; order <= 8; order++ {
		var shifter DAVBode32
		if err := shifter.Configure(1000, order, .1); err != nil {
			t.Fatal(err)
		}

		for i := range 32 {
			sine, cosine := shifter.sine, shifter.cosine
			state := &shifter.history[shifter.phase]

			up, down := shifter.ProcessSample(float32(math.Sin(float64(i) * .3)))

			re, im := state.output[order-2], state.output[order-1]
			if order > 4 {
				re, im = im, re
			}

			re *= cosine
			im *= sine

			wantUp := float32((float64(re) - float64(im)) * sqrtHalf32)
			wantDown := float32((float64(re) + float64(im)) * sqrtHalf32)

			if math.Abs(float64(up-wantUp)) > 1e-6 || math.Abs(float64(down-wantDown)) > 1e-6 {
				t.Fatalf("order %d sample %d: got %g, %g; want %g, %g",
					order, i, up, down, wantUp, wantDown)
			}
		}
	}
}

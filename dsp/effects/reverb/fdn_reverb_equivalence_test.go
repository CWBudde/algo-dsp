package reverb

import (
	"math"
	"testing"
)

// referenceProcessSample is FDNReverb.ProcessSample as it stood before the
// inner loop was rewritten: a sine per delay line per sample for the modulator,
// and the feedback matrix multiplied out.
//
// It is kept here, rather than in the git history, because it is the only thing
// that can say whether the rewrite changed the sound. `phase` carries the
// modulator angle the old implementation held in a struct field.
func referenceProcessSample(r *FDNReverb, phase *float64, input float64) float64 {
	in := input
	if r.preDelaySamples > 0 {
		r.preDelayLine.writeSample(input)
		in = r.preDelayLine.sampleFractionalDelay(r.preDelaySamples)
	}

	var delays [fdnSize]float64

	for i := 0; i < fdnSize; i++ {
		phaseOffset := (2 * math.Pi * float64(i)) / float64(fdnSize)
		mod := 0.5 * (1 + math.Sin(*phase+phaseOffset))
		delay := r.baseDelaySamples[i]*r.lineDelayScale + r.modDepthSamples*mod
		delays[i] = r.lines[i].sampleFractionalDelay(delay)
	}

	*phase += 2 * math.Pi * r.modRateHz / r.sampleRate
	if *phase >= 2*math.Pi {
		*phase -= 2 * math.Pi
	}

	for i := 0; i < fdnSize; i++ {
		feedback := 0.0
		for j := 0; j < fdnSize; j++ {
			feedback += fdnHadamard[i][j] * delays[j]
		}

		feedback *= r.matrixScale
		filtered := feedback*(1-r.damp) + r.filterState[i]*r.damp
		r.filterState[i] = filtered
		writeSample := in*r.inputGain + filtered*r.feedbackGain[i]
		r.lines[i].writeSample(writeSample)
	}

	out := 0.0
	for i := 0; i < fdnSize; i++ {
		out += delays[i]
	}

	out *= r.outputGain

	return input*r.dry + out*r.wet
}

// TestHadamardInPlaceMatchesMatrix checks the butterfly against the matrix it
// stands in for, which is the whole claim the factorisation makes.
func TestHadamardInPlaceMatchesMatrix(t *testing.T) {
	inputs := [][fdnSize]float64{
		{1, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 1, 0, 0, 0, 0},
		{1, 2, 3, 4, 5, 6, 7, 8},
		{-0.5, 0.25, 1e-8, -3, 7.5, 0, 1, -1},
	}

	for n, in := range inputs {
		var want [fdnSize]float64
		for i := 0; i < fdnSize; i++ {
			for j := 0; j < fdnSize; j++ {
				want[i] += fdnHadamard[i][j] * in[j]
			}
		}

		got := in
		hadamardInPlace(&got)

		for i := range want {
			if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
				t.Fatalf("input %d row %d: got=%g want=%g diff=%g", n, i, got[i], want[i], diff)
			}
		}
	}
}

// TestFDNReverbMatchesReference is the one that decides whether the rewrite is
// allowed to ship: same input, same room, both implementations, over long
// enough for the modulator to go round several times and for the rotor to drift
// if it were going to.
func TestFDNReverbMatchesReference(t *testing.T) {
	const (
		sampleRate = 48000.0
		samples    = 480000 // 10 s, and the 0.1 Hz modulator's whole cycle.
	)

	fast, err := NewFDNReverb(sampleRate)
	if err != nil {
		t.Fatalf("NewFDNReverb: %v", err)
	}

	slow, err := NewFDNReverb(sampleRate)
	if err != nil {
		t.Fatalf("NewFDNReverb: %v", err)
	}

	phase := 0.0
	worst := 0.0

	for n := 0; n < samples; n++ {
		// A struck-bar-ish excitation: an impulse every second into an
		// otherwise silent input, so the tail is exercised both while it is
		// loud and while it has decayed a long way.
		x := 0.0
		if n%int(sampleRate) == 0 {
			x = 1
		}

		got := fast.ProcessSample(x)
		want := referenceProcessSample(slow, &phase, x)

		if diff := math.Abs(got - want); diff > worst {
			worst = diff
		}
	}

	// The two differ only by rounding: the rotor against a sine, and pairwise
	// against left-to-right summation. The bound is far below anything audible
	// -- a 24-bit LSB at full scale is about 1.2e-7 -- and far above the noise
	// the two roundings actually produce, so it fails on a behaviour change
	// rather than on a compiler's choice of instruction order.
	if worst > 1e-9 {
		t.Fatalf("worst deviation from the reference implementation: %g", worst)
	}

	t.Logf("worst deviation over %d samples: %g", samples, worst)
}

// TestFDNReverbLFOStaysOnTheUnitCircle guards the rotor's magnitude directly,
// because a drift there is a modulation depth that changes over hours and would
// otherwise only show up as a room that slowly stops sounding like itself.
func TestFDNReverbLFOStaysOnTheUnitCircle(t *testing.T) {
	r, err := NewFDNReverb(48000)
	if err != nil {
		t.Fatalf("NewFDNReverb: %v", err)
	}

	for n := 0; n < 4800000; n++ { // 100 s
		r.advanceLFO()
	}

	magnitude := math.Hypot(r.lfoRe, r.lfoIm)
	if diff := math.Abs(magnitude - 1); diff > 1e-12 {
		t.Fatalf("rotor magnitude drifted to %.17g (off by %g)", magnitude, diff)
	}
}

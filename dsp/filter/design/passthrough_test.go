package design

import (
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// assertPassesImpulseThrough checks that a section built from c reproduces its
// input exactly, which is the observable meaning of "transparent".
func assertPassesImpulseThrough(t *testing.T, name string, c biquad.Coefficients) {
	t.Helper()

	if c.IsZero() {
		t.Fatalf("%s: returned the zero (muting) section", name)
	}

	in := []float64{1, 0, 0, 0, 0.5, -0.25, 0, 0}
	buf := append([]float64(nil), in...)

	s := biquad.NewSection(c)
	s.ProcessBlock(buf)

	for i := range in {
		if buf[i] != in[i] {
			t.Fatalf("%s: sample %d = %v, want %v (filter is not transparent)", name, i, buf[i], in[i])
		}
	}
}

// TestUndesignableFiltersArePassThrough pins the package contract: a designer
// that cannot honour its parameters returns biquad.Identity, not the zero
// value, so the stage passes audio through instead of muting it.
func TestUndesignableFiltersArePassThrough(t *testing.T) {
	designers := map[string]func(freq, sampleRate float64) biquad.Coefficients{
		"Lowpass":   func(f, sr float64) biquad.Coefficients { return Lowpass(f, 0.707, sr) },
		"Highpass":  func(f, sr float64) biquad.Coefficients { return Highpass(f, 0.707, sr) },
		"Bandpass":  func(f, sr float64) biquad.Coefficients { return Bandpass(f, 0.707, sr) },
		"Notch":     func(f, sr float64) biquad.Coefficients { return Notch(f, 0.707, sr) },
		"Allpass":   func(f, sr float64) biquad.Coefficients { return Allpass(f, 0.707, sr) },
		"Peak":      func(f, sr float64) biquad.Coefficients { return Peak(f, 6, 0.707, sr) },
		"LowShelf":  func(f, sr float64) biquad.Coefficients { return LowShelf(f, 6, 0.707, sr) },
		"HighShelf": func(f, sr float64) biquad.Coefficients { return HighShelf(f, 6, 0.707, sr) },
	}

	params := []struct {
		name             string
		freq, sampleRate float64
	}{
		// The downstream case that motivated this contract: a 20 kHz cutoff
		// requested at a 32 kHz sample rate.
		{"freq above Nyquist", 20000, 32000},
		{"freq at Nyquist", 24000, 48000},
		{"zero freq", 0, 48000},
		{"negative freq", -100, 48000},
		{"zero sample rate", 1000, 0},
		{"negative sample rate", 1000, -48000},
	}

	for name, design := range designers {
		for _, p := range params {
			t.Run(name+"/"+p.name, func(t *testing.T) {
				got := design(p.freq, p.sampleRate)
				if got != biquad.Identity() {
					t.Fatalf("got %#v, want biquad.Identity()", got)
				}

				assertPassesImpulseThrough(t, name, got)
			})
		}
	}
}

// TestPeakOrfanidisUndesignableIsPassThrough covers the Orfanidis option path,
// whose RBJ fallback is also undesignable at these parameters.
func TestPeakOrfanidisUndesignableIsPassThrough(t *testing.T) {
	got := Peak(20000, 6, 0.707, 32000, WithDCGain(1.0), WithNyquistGain(1.0))
	if got != biquad.Identity() {
		t.Fatalf("got %#v, want biquad.Identity()", got)
	}

	assertPassesImpulseThrough(t, "Peak+Orfanidis", got)
}

// TestButterworthCascadeUndesignableIsPassThrough checks that a cascade whose
// order is honoured but whose frequency is not stays transparent. A single
// muting section would silence the whole cascade.
func TestButterworthCascadeUndesignableIsPassThrough(t *testing.T) {
	for _, order := range []int{1, 2, 3, 4, 5} {
		for _, design := range []struct {
			name string
			fn   func(freq float64, order int, sampleRate float64) []biquad.Coefficients
		}{
			{"ButterworthLP", ButterworthLP},
			{"ButterworthHP", ButterworthHP},
		} {
			t.Run(design.name, func(t *testing.T) {
				sections := design.fn(20000, order, 32000)
				if len(sections) != (order+1)/2 {
					t.Fatalf("order %d: got %d sections, want %d", order, len(sections), (order+1)/2)
				}

				for i, c := range sections {
					if c != biquad.Identity() {
						t.Fatalf("order %d section %d: got %#v, want biquad.Identity()", order, i, c)
					}
				}

				in := []float64{1, 0, 0, 0, 0.5, -0.25, 0, 0}
				buf := append([]float64(nil), in...)

				biquad.NewChain(sections).ProcessBlock(buf)

				for i := range in {
					if buf[i] != in[i] {
						t.Fatalf("order %d: sample %d = %v, want %v (cascade is not transparent)", order, i, buf[i], in[i])
					}
				}
			})
		}
	}
}

// TestValidParametersStillDesign guards against the pass-through contract
// swallowing legitimate designs.
func TestValidParametersStillDesign(t *testing.T) {
	c := Lowpass(1000, 0.707, 48000)
	if c == biquad.Identity() || c.IsZero() {
		t.Fatalf("valid lowpass degenerated to %#v", c)
	}
}

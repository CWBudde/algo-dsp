package testutil

import (
	"math"
	"testing"
)

func TestDeterministicSine(t *testing.T) {
	s := DeterministicSine(1000, 48000, 1.0, 48)
	if len(s) != 48 {
		t.Fatalf("len = %d, want 48", len(s))
	}
	// First sample of a sine at phase 0 should be 0.
	if math.Abs(s[0]) > 1e-15 {
		t.Fatalf("s[0] = %v, want 0", s[0])
	}
	// All values in [-1, 1].
	for i, v := range s {
		if v < -1 || v > 1 {
			t.Fatalf("s[%d] = %v out of range", i, v)
		}
	}
}

func TestDeterministicSineReproducible(t *testing.T) {
	a := DeterministicSine(440, 44100, 0.5, 100)
	b := DeterministicSine(440, 44100, 0.5, 100)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at index %d", i)
		}
	}
}

func TestDeterministicNoise(t *testing.T) {
	a := DeterministicNoise(42, 1.0, 64)
	b := DeterministicNoise(42, 1.0, 64)
	if len(a) != 64 {
		t.Fatalf("len = %d, want 64", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("noise not deterministic at index %d", i)
		}
	}
}

func TestDeterministicNoiseDifferentSeeds(t *testing.T) {
	a := DeterministicNoise(1, 1.0, 16)
	b := DeterministicNoise(2, 1.0, 16)
	same := true
	for i := range a {
		if a[i] != b[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced identical noise")
	}
}

func TestImpulse(t *testing.T) {
	imp := Impulse(8, 3)
	if len(imp) != 8 {
		t.Fatalf("len = %d, want 8", len(imp))
	}
	for i, v := range imp {
		if i == 3 {
			if v != 1 {
				t.Fatalf("imp[3] = %v, want 1", v)
			}
		} else if v != 0 {
			t.Fatalf("imp[%d] = %v, want 0", i, v)
		}
	}
}

func TestImpulseOutOfBounds(t *testing.T) {
	imp := Impulse(4, 10)
	for i, v := range imp {
		if v != 0 {
			t.Fatalf("imp[%d] = %v, want all zeros for out-of-bounds pos", i, v)
		}
	}
}

func TestDC(t *testing.T) {
	d := DC(0.5, 4)
	for i, v := range d {
		if v != 0.5 {
			t.Fatalf("DC[%d] = %v, want 0.5", i, v)
		}
	}
}

func TestOnes(t *testing.T) {
	o := Ones(3)
	if len(o) != 3 {
		t.Fatalf("len = %d, want 3", len(o))
	}
	for i, v := range o {
		if v != 1 {
			t.Fatalf("Ones[%d] = %v, want 1", i, v)
		}
	}
}

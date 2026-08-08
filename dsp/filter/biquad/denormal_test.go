package biquad

import (
	"math"
	"testing"
)

func TestIdentityIsPassThrough(t *testing.T) {
	c := Identity()
	if c != (Coefficients{B0: 1}) {
		t.Fatalf("Identity() = %#v, want B0=1 and zero elsewhere", c)
	}

	if c.IsZero() {
		t.Fatal("Identity() reported IsZero")
	}

	in := []float64{1, 0, -0.5, 0.25, 0, 0.75}
	buf := append([]float64(nil), in...)

	NewSection(c).ProcessBlock(buf)

	for i := range in {
		if buf[i] != in[i] {
			t.Fatalf("sample %d = %v, want %v", i, buf[i], in[i])
		}
	}
}

func TestCoefficientsIsZero(t *testing.T) {
	if !(Coefficients{}).IsZero() {
		t.Fatal("zero value not reported as zero")
	}

	if (Coefficients{A2: 1e-300}).IsZero() {
		t.Fatal("non-zero coefficients reported as zero")
	}
}

func TestSectionFlushDenormals(t *testing.T) {
	s := NewSection(Identity())

	s.SetState([2]float64{1e-40, -1e-45})
	s.FlushDenormals()

	if got := s.State(); got != [2]float64{0, 0} {
		t.Fatalf("state = %v, want zeroed", got)
	}
}

func TestSectionFlushDenormalsKeepsNormalState(t *testing.T) {
	s := NewSection(Identity())

	want := [2]float64{0.25, -1e-20}
	s.SetState(want)
	s.FlushDenormals()

	if got := s.State(); got != want {
		t.Fatalf("state = %v, want %v unchanged", got, want)
	}
}

func TestSectionFlushDenormalsAfterDecay(t *testing.T) {
	// A resonant lowpass fed an impulse then silence decays into the denormal
	// range; FlushDenormals must clear whatever is left.
	s := NewSection(Coefficients{B0: 0.5, B1: 1, B2: 0.5, A1: -1.9, A2: 0.9025})

	buf := make([]float64, 4096)
	buf[0] = 1
	s.ProcessBlock(buf)

	for range 4 {
		for i := range buf {
			buf[i] = 0
		}

		s.ProcessBlock(buf)
	}

	// How far the state has decayed depends on the platform's floating-point
	// details, so assert the contract itself: entries below the threshold are
	// zeroed, entries above it are left untouched.
	before := s.State()

	s.FlushDenormals()

	after := s.State()

	for i := range before {
		want := before[i]
		if math.Abs(before[i]) < denormalThreshold {
			want = 0
		}

		if after[i] != want {
			t.Fatalf("state[%d] = %v, want %v (before flush: %v)", i, after[i], want, before[i])
		}
	}
}

func TestChainFlushDenormals(t *testing.T) {
	c := NewChain([]Coefficients{Identity(), Identity(), Identity()})

	for i := range c.NumSections() {
		c.Section(i).SetState([2]float64{1e-40, 1e-41})
	}

	c.FlushDenormals()

	for i := range c.NumSections() {
		if got := c.Section(i).State(); got != [2]float64{0, 0} {
			t.Fatalf("section %d state = %v, want zeroed", i, got)
		}
	}
}

func TestFlushDenormalsIsZeroAlloc(t *testing.T) {
	s := NewSection(Identity())
	if allocs := testing.AllocsPerRun(100, s.FlushDenormals); allocs != 0 {
		t.Fatalf("Section.FlushDenormals allocated %v times per run", allocs)
	}

	c := NewChain([]Coefficients{Identity(), Identity(), Identity(), Identity()})
	if allocs := testing.AllocsPerRun(100, c.FlushDenormals); allocs != 0 {
		t.Fatalf("Chain.FlushDenormals allocated %v times per run", allocs)
	}
}

func BenchmarkSectionFlushDenormals(b *testing.B) {
	s := NewSection(Identity())
	s.SetState([2]float64{1e-40, 1e-41})

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		s.FlushDenormals()
	}
}

func BenchmarkChainFlushDenormals(b *testing.B) {
	c := NewChain([]Coefficients{Identity(), Identity(), Identity(), Identity()})

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		c.FlushDenormals()
	}
}

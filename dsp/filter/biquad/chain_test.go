package biquad

import (
	"fmt"
	"math"
	"testing"
)

// twoSectionCoeffs returns two biquad sections for a 4th-order-like cascade.
func twoSectionCoeffs() []Coefficients {
	return []Coefficients{
		{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04},
		{B0: 0.1, B1: 0.2, B2: 0.1, A1: -0.5, A2: 0.1},
	}
}

func TestNewChain(t *testing.T) {
	coeffs := twoSectionCoeffs()

	c := NewChain(coeffs)
	if c.NumSections() != 2 {
		t.Fatalf("NumSections: got %d, want 2", c.NumSections())
	}

	if c.Order() != 4 {
		t.Fatalf("Order: got %d, want 4", c.Order())
	}

	if c.gain != 1 {
		t.Fatalf("default gain: got %v, want 1", c.gain)
	}
}

func TestNewChain_WithGain(t *testing.T) {
	coeffs := twoSectionCoeffs()

	c := NewChain(coeffs, WithGain(0.5))
	if c.gain != 0.5 {
		t.Fatalf("gain: got %v, want 0.5", c.gain)
	}
}

func TestChain_ProcessSample_MatchesManualCascade(t *testing.T) {
	coeffs := twoSectionCoeffs()

	// Reference: manual two-section cascade.
	section1 := NewSection(coeffs[0])
	section2 := NewSection(coeffs[1])

	chain := NewChain(coeffs)

	input := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8}
	for i, x := range input {
		ref := section2.ProcessSample(section1.ProcessSample(x))

		got := chain.ProcessSample(x)
		if !almostEqual(got, ref, eps) {
			t.Errorf("sample %d: chain=%.15f, ref=%.15f", i, got, ref)
		}
	}
}

func TestChain_ProcessSample_WithGain(t *testing.T) {
	coeffs := twoSectionCoeffs()
	gain := 2.0

	// Reference: scale input then cascade.
	section1 := NewSection(coeffs[0])
	section2 := NewSection(coeffs[1])

	chain := NewChain(coeffs, WithGain(gain))

	input := []float64{1, 0.5, -0.3, 0.7}
	for i, x := range input {
		ref := section2.ProcessSample(section1.ProcessSample(x * gain))

		got := chain.ProcessSample(x)
		if !almostEqual(got, ref, eps) {
			t.Errorf("sample %d: chain=%.15f, ref=%.15f", i, got, ref)
		}
	}
}

func TestChain_ProcessBlock_MatchesSample(t *testing.T) {
	coeffs := twoSectionCoeffs()

	// Reference via ProcessSample.
	c1 := NewChain(coeffs)
	input := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8}

	ref := make([]float64, len(input))
	for i, x := range input {
		ref[i] = c1.ProcessSample(x)
	}

	// ProcessBlock.
	c2 := NewChain(coeffs)
	block := make([]float64, len(input))
	copy(block, input)
	c2.ProcessBlock(block)

	for i := range block {
		if !almostEqual(block[i], ref[i], eps) {
			t.Errorf("sample %d: block=%.15f, ref=%.15f", i, block[i], ref[i])
		}
	}
}

func TestChain_ProcessBlock_WithGain(t *testing.T) {
	coeffs := twoSectionCoeffs()
	gain := 0.5

	c1 := NewChain(coeffs, WithGain(gain))
	input := []float64{1, 0.5, -0.3, 0.7}

	ref := make([]float64, len(input))
	for i, x := range input {
		ref[i] = c1.ProcessSample(x)
	}

	c2 := NewChain(coeffs, WithGain(gain))
	block := make([]float64, len(input))
	copy(block, input)
	c2.ProcessBlock(block)

	for i := range block {
		if !almostEqual(block[i], ref[i], eps) {
			t.Errorf("sample %d: block=%.15f, ref=%.15f", i, block[i], ref[i])
		}
	}
}

func TestChain_SingleSection(t *testing.T) {
	// A single-section chain should match a standalone Section.
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	s := NewSection(c)
	chain := NewChain([]Coefficients{c})

	input := []float64{1, 0.5, -0.3, 0.7, 0}
	for i, x := range input {
		ref := s.ProcessSample(x)

		got := chain.ProcessSample(x)
		if !almostEqual(got, ref, eps) {
			t.Errorf("sample %d: chain=%.15f, section=%.15f", i, got, ref)
		}
	}
}

func TestChain_ThreeSections(t *testing.T) {
	// 6th-order cascade.
	coeffs := []Coefficients{
		{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04},
		{B0: 0.1, B1: 0.2, B2: 0.1, A1: -0.5, A2: 0.1},
		{B0: 0.3, B1: 0.3, B2: 0.3, A1: -0.1, A2: 0.02},
	}
	section1 := NewSection(coeffs[0])
	section2 := NewSection(coeffs[1])
	section3 := NewSection(coeffs[2])
	chain := NewChain(coeffs)

	if chain.Order() != 6 {
		t.Fatalf("Order: got %d, want 6", chain.Order())
	}

	input := []float64{1, 0, 0, 0, 0, 0, 0, 0}
	for i, x := range input {
		ref := section3.ProcessSample(section2.ProcessSample(section1.ProcessSample(x)))

		got := chain.ProcessSample(x)
		if !almostEqual(got, ref, eps) {
			t.Errorf("sample %d: chain=%.15f, ref=%.15f", i, got, ref)
		}
	}
}

func TestChain_Reset(t *testing.T) {
	chain := NewChain(twoSectionCoeffs())
	chain.ProcessSample(1)
	chain.ProcessSample(0.5)

	chain.Reset()

	for i := range chain.sections {
		st := chain.sections[i].State()
		if st != [2]float64{0, 0} {
			t.Errorf("section %d state not zero after reset: %v", i, st)
		}
	}
}

func TestChain_State_SaveRestore(t *testing.T) {
	chain := NewChain(twoSectionCoeffs())
	chain.ProcessSample(1)
	chain.ProcessSample(0.5)
	saved := chain.State()

	y3 := chain.ProcessSample(-0.3)
	y4 := chain.ProcessSample(0.7)

	chain.SetState(saved)
	y3b := chain.ProcessSample(-0.3)
	y4b := chain.ProcessSample(0.7)

	if !almostEqual(y3, y3b, eps) {
		t.Errorf("sample 3: got %v after restore, want %v", y3b, y3)
	}

	if !almostEqual(y4, y4b, eps) {
		t.Errorf("sample 4: got %v after restore, want %v", y4b, y4)
	}
}

func TestChain_Section_Access(t *testing.T) {
	coeffs := twoSectionCoeffs()

	chain := NewChain(coeffs)
	for i, c := range coeffs {
		s := chain.Section(i)
		if s.Coefficients != c {
			t.Errorf("section %d coefficients mismatch", i)
		}
	}
}

func TestChain_OddOrder_FirstOrderSection(t *testing.T) {
	// Simulate an odd-order filter by having a "first-order" section
	// where B2=0, A2=0 (effectively a 1st-order IIR).
	// This is how legacy Butterworth handles odd orders.
	firstOrder := Coefficients{B0: 0.3, B1: 0.3, A1: -0.4} // B2=0, A2=0
	secondOrder := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	chain := NewChain([]Coefficients{secondOrder, firstOrder})

	s1 := NewSection(secondOrder)
	s2 := NewSection(firstOrder)

	input := []float64{1, 0, 0, 0, 0.5, -0.5, 0, 0}
	for i, x := range input {
		ref := s2.ProcessSample(s1.ProcessSample(x))

		got := chain.ProcessSample(x)
		if !almostEqual(got, ref, eps) {
			t.Errorf("sample %d: chain=%.15f, ref=%.15f", i, got, ref)
		}
	}
}

func TestChain_StabilityLongRun(t *testing.T) {
	chain := NewChain(twoSectionCoeffs())
	chain.ProcessSample(1)

	for range 10000 {
		chain.ProcessSample(0)
	}

	states := chain.State()
	for i, st := range states {
		if math.Abs(st[0]) > 1e-100 || math.Abs(st[1]) > 1e-100 {
			t.Errorf("section %d state did not decay: %v", i, st)
		}
	}
}

func TestChain_UpdateCoefficients_PreservesStateWhenSectionCountMatches(t *testing.T) {
	// Warm the filter state with some samples.
	c := NewChain(twoSectionCoeffs())
	c.ProcessSample(1)
	c.ProcessSample(0.5)
	c.ProcessSample(-0.3)
	savedState := c.State()

	// Update to different coefficients with the same number of sections.
	newCoeffs := []Coefficients{
		{B0: 0.3, B1: 0.4, B2: 0.3, A1: -0.3, A2: 0.05},
		{B0: 0.2, B1: 0.1, B2: 0.2, A1: -0.4, A2: 0.08},
	}
	c.UpdateCoefficients(newCoeffs, 1.0)

	// State must be unchanged after the coefficient update.
	afterState := c.State()
	for i, s := range afterState {
		if s != savedState[i] {
			t.Errorf("section %d state changed: before=%v, after=%v", i, savedState[i], s)
		}
	}
}

func TestChain_UpdateCoefficients_AppliesNewCoefficients(t *testing.T) {
	// Build two chains, update one to the same coefficients as the other.
	origCoeffs := twoSectionCoeffs()
	c := NewChain(origCoeffs)

	newCoeffs := []Coefficients{
		{B0: 0.3, B1: 0.4, B2: 0.3, A1: -0.3, A2: 0.05},
		{B0: 0.2, B1: 0.1, B2: 0.2, A1: -0.4, A2: 0.08},
	}
	ref := NewChain(newCoeffs)

	c.UpdateCoefficients(newCoeffs, 1.0)

	// Both chains start from zero state; their output must be identical.
	input := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8}
	for i, x := range input {
		want := ref.ProcessSample(x)

		got := c.ProcessSample(x)
		if !almostEqual(got, want, eps) {
			t.Errorf("sample %d: got %.15f, want %.15f", i, got, want)
		}
	}
}

func TestChain_UpdateCoefficients_UpdatesGain(t *testing.T) {
	c := NewChain(twoSectionCoeffs(), WithGain(1.0))
	c.UpdateCoefficients(twoSectionCoeffs(), 0.5)

	if c.Gain() != 0.5 {
		t.Errorf("gain: got %v, want 0.5", c.Gain())
	}
}

func TestChain_UpdateCoefficients_DifferentSectionCountResetsState(t *testing.T) {
	// Warm a 2-section chain.
	c := NewChain(twoSectionCoeffs())
	c.ProcessSample(1)
	c.ProcessSample(0.5)

	// Change to a 1-section chain — state must be reset.
	oneSection := []Coefficients{
		{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04},
	}
	c.UpdateCoefficients(oneSection, 1.0)

	if c.NumSections() != 1 {
		t.Fatalf("NumSections: got %d, want 1", c.NumSections())
	}

	for i := range c.sections {
		st := c.sections[i].State()
		if st != [2]float64{0, 0} {
			t.Errorf("section %d state not zero after section-count change: %v", i, st)
		}
	}
}

// Benchmarks

func BenchmarkChain_ProcessSample(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprintf("sections=%d", n), func(b *testing.B) {
			coeffs := make([]Coefficients, n)
			for i := range coeffs {
				coeffs[i] = benchCoeffs
			}

			c := NewChain(coeffs)

			x := 1.0
			for b.Loop() {
				x = c.ProcessSample(x)
			}

			_ = x
		})
	}
}

func BenchmarkChain_ProcessBlock(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprintf("sections=%d", n), func(b *testing.B) {
			coeffs := make([]Coefficients, n)
			for i := range coeffs {
				coeffs[i] = benchCoeffs
			}

			c := NewChain(coeffs)

			buf := make([]float64, 1024)
			for i := range buf {
				buf[i] = float64(i) * 0.001
			}

			b.SetBytes(1024 * 8)
			b.ResetTimer()

			for range b.N {
				c.ProcessBlock(buf)
			}
		})
	}
}

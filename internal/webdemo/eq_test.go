package webdemo

import (
	"fmt"
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()

	e, err := NewEngine(48000)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	return e
}

// TestSetEQ_PreservesFilterStateOnSameOrder verifies that updating an EQ
// band's frequency or gain while keeping the same filter family, type, and
// order does not reset the biquad delay-line state.  Discarding the state
// causes an audible click because the filter output jumps discontinuously.
func TestSetEQ_PreservesFilterStateOnSameOrder(t *testing.T) {
	e := newTestEngine(t)

	// Warm up the HP filter state with a burst of samples.
	block := make([]float64, 256)
	block[0] = 1.0 // impulse
	e.hp.ProcessBlock(block)

	stateBefore := e.hp.State()
	if stateBefore == nil {
		t.Fatal("filter state is nil before update")
	}

	// Change HP frequency only — same family (rbj), type (highpass), order.
	eq := e.eq

	eq.HPFreq = 80 // was 40 Hz

	err := e.SetEQ(eq)
	if err != nil {
		t.Fatalf("SetEQ: %v", err)
	}

	stateAfter := e.hp.State()

	for i, s := range stateAfter {
		if s != stateBefore[i] {
			t.Errorf("HP section %d state changed after freq-only update: before=%v after=%v", i, stateBefore[i], s)
		}
	}
}

// TestSetEQ_ResetsStateOnSectionCountChange verifies that a change altering the
// number of biquad sections (here switching family and order) resets the
// delay-line state cleanly.
func TestSetEQ_ResetsStateOnSectionCountChange(t *testing.T) {
	e := newTestEngine(t)

	// Warm up state.
	block := make([]float64, 256)
	block[0] = 1.0
	e.hp.ProcessBlock(block)

	// Switch from 2nd-order RBJ to 4th-order Butterworth.
	eq := e.eq
	eq.HPFamily = "butterworth"

	eq.HPOrder = 4

	err := e.SetEQ(eq)
	if err != nil {
		t.Fatalf("SetEQ: %v", err)
	}

	// New filter has more sections; state should be zero (clean start).
	for i, s := range e.hp.State() {
		if s != [2]float64{0, 0} {
			t.Errorf("HP section %d state not zero after order change: %v", i, s)
		}
	}
}

// TestBuildEQChain_EllipticShelvesAreWired checks that the elliptic family
// routes shelf nodes to the high-order shelving designers rather than silently
// falling back to the single-section RBJ shelf.
func TestBuildEQChain_EllipticShelvesAreWired(t *testing.T) {
	const (
		sampleRate = 48000.0
		gainDB     = 12.0
		stopbandDB = 0.5
		order      = 6
	)

	tests := []struct {
		kind   string
		freq   float64
		probe  float64 // frequency inside the shelf region
		wantDB float64
	}{
		{eqKindLowShelf, 1000, 20, gainDB},
		{eqKindHighShelf, 5000, 23000, gainDB},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if !supportsEQFamily(tt.kind, eqFamilyElliptic) {
				t.Fatal("elliptic family should support shelving nodes")
			}

			if got := eqShapeMode(tt.kind, eqFamilyElliptic); got != eqShapeModeRipple {
				t.Errorf("shape mode = %q, want %q", got, eqShapeModeRipple)
			}

			chain := buildEQChain(eqFamilyElliptic, tt.kind, order, tt.freq, gainDB, stopbandDB, sampleRate)

			// The RBJ fallback would yield a single section.
			if got := chain.NumSections(); got != (order+1)/2 {
				t.Fatalf("sections = %d, want %d (fell back to RBJ?)", got, (order+1)/2)
			}

			// Within the 0.05 dB shelf-side ripple of the nominal gain.
			if got := chainMagnitudeDB(chain, tt.probe, sampleRate); math.Abs(got-tt.wantDB) > 0.05 {
				t.Errorf("|H(%.0f Hz)| = %.4f dB, want %.2f dB", tt.probe, got, tt.wantDB)
			}
		})
	}
}

func chainMagnitudeDB(chain *biquad.Chain, freqHz, sampleRate float64) float64 {
	h := complex(chain.Gain(), 0)
	for i := range chain.NumSections() {
		h *= chain.Section(i).Response(freqHz, sampleRate)
	}

	return 20 * math.Log10(cmplx.Abs(h))
}

// TestBuildEQChain_EquirippleShelvesSurviveSmallGains covers the case reported
// in review: the shape control is clamped independently of gain, so dragging a
// freshly selected equiripple shelf through small gains used to make the
// designer reject the ripple and silently substitute a one-section RBJ shelf
// while the UI still reported the chosen family and order.
func TestBuildEQChain_EquirippleShelvesSurviveSmallGains(t *testing.T) {
	const (
		sampleRate = 48000.0
		order      = 6
		// The node's default shape, which the ripple control reuses verbatim.
		ripple = 0.707
	)

	families := []string{eqFamilyChebyshev2, eqFamilyElliptic}
	kinds := []string{eqKindLowShelf, eqKindHighShelf}

	// Sweep the whole gain slider in 0.1 dB steps, skipping only the exact 0 dB
	// setting, which legitimately designs a single passthrough section.
	for _, family := range families {
		for _, kind := range kinds {
			for step := -240; step <= 240; step++ {
				if step == 0 {
					continue
				}

				gainDB := float64(step) / 10

				name := fmt.Sprintf("%s/%s/%+.1fdB", family, kind, gainDB)
				t.Run(name, func(t *testing.T) {
					chain := buildEQChain(family, kind, order, 1000, gainDB, ripple, sampleRate)
					if got := chain.NumSections(); got != (order+1)/2 {
						t.Fatalf("sections = %d, want %d (fell back to RBJ)", got, (order+1)/2)
					}
				})
			}
		}
	}
}

// TestEquirippleShelfRipple_Bounds pins the bound itself.
func TestEquirippleShelfRipple_Bounds(t *testing.T) {
	// A ripple that already fits the gain is passed through unchanged.
	if got, ok := equirippleShelfRipple(eqFamilyElliptic, 12, 0.5); !ok || got != 0.5 {
		t.Errorf("ripple = %v, ok = %v; want 0.5, true", got, ok)
	}

	// A ripple wider than the gain is pulled inside the admissible range.
	got, ok := equirippleShelfRipple(eqFamilyElliptic, 0.5, 12)
	if !ok {
		t.Fatal("0.5 dB gain should still admit a stopband")
	}

	if got >= 0.5-0.05 {
		t.Errorf("ripple = %v, want < %v", got, 0.5-0.05)
	}

	// Below the fixed shelf-side ripple the elliptic design is impossible.
	if _, ok := equirippleShelfRipple(eqFamilyElliptic, 0.04, 0.5); ok {
		t.Error("0.04 dB gain should not admit a stopband")
	}

	// Chebyshev II has no shelf-side reservation, so it needs only a nonzero gain.
	if _, ok := equirippleShelfRipple(eqFamilyChebyshev2, 0.04, 0.5); !ok {
		t.Error("chebyshev2 should admit a stopband at 0.04 dB gain")
	}
}

// TestRBJFallbackQ_IgnoresRippleShapes checks that a shape value carrying a dB
// ripple bound is not handed to the RBJ fallback as a Q, which would turn a
// 12 dB ripple into a Q of 12 and produce a wildly resonant filter.
func TestRBJFallbackQ_IgnoresRippleShapes(t *testing.T) {
	// Ripple mode: the shape value is a dB bound and carries no Q information.
	if got := rbjFallbackQ(eqKindLowShelf, eqFamilyElliptic, 1000, 12); got != rbjDefaultQ {
		t.Errorf("elliptic shelf fallback Q = %v, want %v", got, rbjDefaultQ)
	}

	if got := rbjFallbackQ(eqKindHighpass, eqFamilyChebyshev1, 1000, 9); got != rbjDefaultQ {
		t.Errorf("chebyshev1 highpass fallback Q = %v, want %v", got, rbjDefaultQ)
	}

	// Q mode: the RBJ families keep using the shape control as a Q.
	if got := rbjFallbackQ(eqKindLowShelf, eqFamilyRBJ, 1000, 2.5); got != 2.5 {
		t.Errorf("rbj shelf fallback Q = %v, want 2.5", got)
	}
}

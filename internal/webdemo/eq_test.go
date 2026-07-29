package webdemo

import (
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

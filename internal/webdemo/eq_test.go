package webdemo

import (
	"testing"
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
	if err := e.SetEQ(eq); err != nil {
		t.Fatalf("SetEQ: %v", err)
	}

	stateAfter := e.hp.State()

	for i, s := range stateAfter {
		if s != stateBefore[i] {
			t.Errorf("HP section %d state changed after freq-only update: before=%v after=%v", i, stateBefore[i], s)
		}
	}
}

// TestSetEQ_ResetsStateOnFilterTypeChange verifies that switching to a
// different filter order (which changes section count) resets state cleanly.
func TestSetEQ_ResetsStateOnFilterTypeChange(t *testing.T) {
	e := newTestEngine(t)

	// Warm up state.
	block := make([]float64, 256)
	block[0] = 1.0
	e.hp.ProcessBlock(block)

	// Switch from 2nd-order RBJ to 4th-order Butterworth.
	eq := e.eq
	eq.HPFamily = "butterworth"
	eq.HPOrder = 4
	if err := e.SetEQ(eq); err != nil {
		t.Fatalf("SetEQ: %v", err)
	}

	// New filter has more sections; state should be zero (clean start).
	for i, s := range e.hp.State() {
		if s != [2]float64{0, 0} {
			t.Errorf("HP section %d state not zero after order change: %v", i, s)
		}
	}
}

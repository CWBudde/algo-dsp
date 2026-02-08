package pass

import (
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ---------------------------------------------------------------------------
// Chebyshev Type II tests
// ---------------------------------------------------------------------------

func TestChebyshev2_SectionCount(t *testing.T) {
	sr := 48000.0
	ripple := 2.0
	for order := 1; order <= 8; order++ {
		want := (order + 1) / 2
		gotLP := Chebyshev2LP(1000, order, ripple, sr)
		gotHP := Chebyshev2HP(1000, order, ripple, sr)
		if len(gotLP) != want {
			t.Fatalf("LP order %d: sections=%d, want %d", order, len(gotLP), want)
		}

func TestChebyshev2_InvalidInputs(t *testing.T) {
	if got := Chebyshev2LP(1000, 0, 2, 48000); got != nil {
		t.Fatal("expected nil for order <= 0")
	}

func TestChebyshev2_AllSectionsFinite(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000}

func TestChebyshev2HP_ResponseShaped(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{4, 6}

func TestChebyshev2_DefaultRipple(t *testing.T) {
	lp := Chebyshev2LP(1000, 4, 0, 48000)
	lpRef := Chebyshev2LP(1000, 4, 1, 48000)
	if !coeffSliceEqual(lp, lpRef) {
		t.Fatal("ripple=0 should produce same result as ripple=1")
	}


package pass

import (
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ---------------------------------------------------------------------------
// Chebyshev Type I tests
// ---------------------------------------------------------------------------

func TestChebyshev1_SectionCount(t *testing.T) {
	sr := 48000.0
	ripple := 1.0
	for order := 1; order <= 8; order++ {
		want := (order + 1) / 2
		gotLP := Chebyshev1LP(1000, order, ripple, sr)
		gotHP := Chebyshev1HP(1000, order, ripple, sr)
		if len(gotLP) != want {
			t.Fatalf("LP order %d: sections=%d, want %d", order, len(gotLP), want)
		}

func TestChebyshev1_InvalidInputs(t *testing.T) {
	if got := Chebyshev1LP(1000, 0, 1, 48000); got != nil {
		t.Fatal("expected nil for order <= 0")
	}

func TestChebyshev1_AllSectionsFinite(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000}

func TestChebyshev1_ResponseFiniteAndShaped(t *testing.T) {
	sr := 48000.0
	// Order >= 4 with ripple=2 matches existing TestChebyshevResponseShape.
	for _, order := range []int{4, 6, 8}

func TestChebyshev1_DefaultRipple(t *testing.T) {
	// rippleDB <= 0 should use default of 1
	lp := Chebyshev1LP(1000, 4, 0, 48000)
	lpRef := Chebyshev1LP(1000, 4, 1, 48000)
	if !coeffSliceEqual(lp, lpRef) {
		t.Fatal("ripple=0 should produce same result as ripple=1")
	}

func TestChebyshev1ResponseShape_MultiOrder(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{3, 4, 5, 6}

func TestChebyshev_OddOrder_HasFirstOrderSection(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{3, 5, 7}


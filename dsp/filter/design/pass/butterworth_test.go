package pass

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ---------------------------------------------------------------------------
// Butterworth tests
// ---------------------------------------------------------------------------

func TestButterworthLP_SectionCount(t *testing.T) {
	sr := 48000.0
	for order := 1; order <= 8; order++ {
		want := (order + 1) / 2
		got := ButterworthLP(1000, order, sr)
		if len(got) != want {
			t.Fatalf("order %d: sections=%d, want %d", order, len(got), want)
		}

func TestButterworthHP_SectionCount(t *testing.T) {
	sr := 48000.0
	for order := 1; order <= 8; order++ {
		want := (order + 1) / 2
		got := ButterworthHP(1000, order, sr)
		if len(got) != want {
			t.Fatalf("order %d: sections=%d, want %d", order, len(got), want)
		}

func TestButterworth_EvenOrder_NoFirstOrderSection(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{2, 4, 6, 8}

func TestButterworth_OddOrder_HasFirstOrderSection(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{1, 3, 5, 7}

func TestButterworthLP_Minus3dBAtCutoff(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{1, 2, 3, 4, 5, 6, 8}

func TestButterworthHP_Minus3dBAtCutoff(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{1, 2, 3, 4, 5, 6, 8}

func TestButterworthLP_HigherOrderSteeperRolloff(t *testing.T) {
	sr := 48000.0
	prevAtten := 0.0
	for _, order := range []int{1, 2, 4, 6, 8}

func TestButterworthHP_HigherOrderSteeperRolloff(t *testing.T) {
	sr := 48000.0
	prevAtten := 0.0
	for _, order := range []int{1, 2, 4, 6, 8}

func TestButterworth_AllSectionsStable(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000, 192000}

func TestButterworth_InvalidInputs(t *testing.T) {
	if got := ButterworthLP(1000, -1, 48000); got != nil {
		t.Fatal("expected nil for negative order")
	}

func TestButterworthQ_KnownValues(t *testing.T) {
	// Order 2, index 0: Q = 1/(2*sin(pi/4)) = 1/sqrt(2)
	got := butterworthQ(2, 0)
	want := 1 / math.Sqrt2
	if !almostEqual(got, want, 1e-12) {
		t.Fatalf("order=2 index=0: Q=%.10f, want %.10f", got, want)
	}

func TestBilinearK_ValidAndInvalid(t *testing.T) {
	k, ok := bilinearK(1000, 48000)
	if !ok || k <= 0 {
		t.Fatalf("expected valid k>0, got k=%v ok=%v", k, ok)
	}

func TestButterworthFirstOrder_Passthrough(t *testing.T) {
	sr := 48000.0
	lp := butterworthFirstOrderLP(1000, sr)
	hp := butterworthFirstOrderHP(1000, sr)

	// Both should be first-order (B2=A2=0)
	if lp.B2 != 0 || lp.A2 != 0 {
		t.Fatalf("LP not first-order: %+v", lp)
	}

func TestButterworthFirstOrder_InvalidInputs(t *testing.T) {
	zero := biquad.Coefficients{}

func TestButterworth_LPHPSymmetry(t *testing.T) {
	sr := 48000.0
	order := 4
	freq := 2000.0

	lp := biquad.NewChain(ButterworthLP(freq, order, sr))
	hp := biquad.NewChain(ButterworthHP(freq, order, sr))

	// At cutoff, both should be ~-3 dB
	lpCutoff := lp.MagnitudeDB(freq, sr)
	hpCutoff := hp.MagnitudeDB(freq, sr)
	if !almostEqual(lpCutoff, hpCutoff, 0.1) {
		t.Fatalf("LP cutoff=%.2f dB, HP cutoff=%.2f dB, expected similar", lpCutoff, hpCutoff)
	}


package shelving

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ============================================================
// Chebyshev Type II shelving filter tests
// ============================================================

func chebyshev2Designers() []shelfDesigner {
	return []shelfDesigner{
		{"low", Chebyshev2LowShelf},
		{"high", Chebyshev2HighShelf},
	}
}

// ==== validation ====

func TestChebyshev2LowShelf_InvalidParams(t *testing.T) {
	tests := []struct {
		name       string
		sr, freq   float64
		gainDB     float64
		stopbandDB float64
		order      int
	}{
		{"zero sample rate", 0, 1000, 12, 0.5, 4},
		{"negative sample rate", -48000, 1000, 12, 0.5, 4},
		{"zero frequency", testSR, 0, 12, 0.5, 4},
		{"negative frequency", testSR, -100, 12, 0.5, 4},
		{"frequency at Nyquist", testSR, testSR / 2, 12, 0.5, 4},
		{"frequency above Nyquist", testSR, testSR, 12, 0.5, 4},
		{"zero order", testSR, 1000, 12, 0.5, 0},
		{"negative order", testSR, 1000, 12, 0.5, -2},
		{"zero stopband", testSR, 1000, 12, 0, 4},
		{"negative stopband", testSR, 1000, 12, -0.5, 4},
		{"stopband equal to gain", testSR, 1000, 6, 6, 4},
		{"stopband above gain", testSR, 1000, 6, 12, 4},
		{"stopband above magnitude of cut", testSR, 1000, -6, 12, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Chebyshev2LowShelf(tt.sr, tt.freq, tt.gainDB, tt.stopbandDB, tt.order); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestChebyshev2HighShelf_InvalidParams(t *testing.T) {
	if _, err := Chebyshev2HighShelf(0, 1000, 12, 0.5, 4); err == nil {
		t.Error("zero sample rate should error")
	}

	if _, err := Chebyshev2HighShelf(testSR, 1000, 12, 0, 4); err == nil {
		t.Error("zero stopband should error")
	}

	if _, err := Chebyshev2HighShelf(testSR, 1000, 6, 6, 4); err == nil {
		t.Error("stopband equal to gain should error")
	}
}

func TestChebyshev2Shelf_ZeroGain(t *testing.T) {
	for _, design := range chebyshev2Designers() {
		t.Run(design.name, func(t *testing.T) {
			sections, err := design.fn(testSR, 1000, 0, 0.5, 4)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(sections) != 1 {
				t.Fatalf("section count = %d, want 1 passthrough section", len(sections))
			}

			if got := cascadeMagnitudeDB(sections, 1000, testSR); !almostEqual(got, 0, 1e-12) {
				t.Errorf("magnitude = %.6f dB, want 0", got)
			}
		})
	}
}

// ==== structure ====

func TestChebyshev2Shelf_SectionCount(t *testing.T) {
	for _, design := range chebyshev2Designers() {
		for order := 1; order <= 8; order++ {
			t.Run(design.name+"/"+orderName(order), func(t *testing.T) {
				sections, err := design.fn(testSR, 1000, 12, 0.5, order)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if want := (order + 1) / 2; len(sections) != want {
					t.Errorf("section count = %d, want %d", len(sections), want)
				}
			})
		}
	}
}

func TestChebyshev2Shelf_Stability(t *testing.T) {
	orders := []int{1, 2, 3, 4, 5, 6, 8, 10, 12}
	gains := []float64{-30, -12, -3, 3, 12, 30}

	for _, design := range chebyshev2Designers() {
		for _, order := range orders {
			for _, gain := range gains {
				sections, err := design.fn(testSR, 1000, gain, 0.5, order)
				if err != nil {
					t.Fatalf("%s %s %s: %v", design.name, orderName(order), gainName(gain), err)
				}

				allPolesStable(t, sections)
			}
		}
	}
}

func TestChebyshev2Shelf_PoleZeroPairs(t *testing.T) {
	for _, design := range chebyshev2Designers() {
		for _, order := range []int{2, 4, 6} {
			sections, err := design.fn(testSR, 1000, 12, 0.5, order)
			if err != nil {
				t.Fatalf("%s %s: %v", design.name, orderName(order), err)
			}

			for i, s := range sections {
				if s.A2 == 0 || s.B2 == 0 {
					t.Errorf("%s %s: section %d is not a full biquad (A2=%v B2=%v)",
						design.name, orderName(order), i, s.A2, s.B2)
				}
			}
		}
	}
}

// ==== anchors ====

// TestChebyshev2Shelf_CutoffAnchor pins freqHz to the package-wide convention
// |H(f_c)|² = (G² + 1)/2, shared with Butterworth and Chebyshev I.
func TestChebyshev2Shelf_CutoffAnchor(t *testing.T) {
	gains := []float64{-24, -12, -6, -1, 1, 6, 12, 24}
	freqs := []float64{100, 300, 1000, 5000, 10000}

	for _, design := range chebyshev2Designers() {
		for order := 1; order <= 8; order++ {
			for _, gain := range gains {
				for _, freq := range freqs {
					sections, err := design.fn(testSR, freq, gain, 0.5, order)
					if err != nil {
						t.Fatalf("%s %s %s %s: %v",
							design.name, orderName(order), gainName(gain), freqName(freq), err)
					}

					got := cascadeMagnitudeDB(sections, freq, testSR)
					if want := cutoffTargetDB(gain); math.Abs(got-want) > 1e-6 {
						t.Errorf("%s %s %s %s: |H(fc)| = %.9f dB, want %.9f",
							design.name, orderName(order), gainName(gain), freqName(freq), got, want)
					}
				}
			}
		}
	}
}

// TestChebyshev2Shelf_EndpointAnchors records the semantics of the rebuilt
// designer: the shelf side is monotonic and reaches gainDB exactly, while the
// flat side lands on the ripple bound for even orders and on 0 dB for odd ones.
func TestChebyshev2Shelf_EndpointAnchors(t *testing.T) {
	const stopbandDB = 0.5

	gains := []float64{-24, -12, -6, 6, 12, 24}

	for order := 1; order <= 8; order++ {
		for _, gain := range gains {
			flatTol := 1e-6
			if order%2 == 0 {
				flatTol = stopbandDB + 1e-6
			}

			low, err := Chebyshev2LowShelf(testSR, 1000, gain, stopbandDB, order)
			if err != nil {
				t.Fatalf("low %s %s: %v", orderName(order), gainName(gain), err)
			}

			if got := cascadeMagnitudeDB(low, 0, testSR); math.Abs(got-gain) > 1e-6 {
				t.Errorf("low %s %s: DC = %.9f dB, want %.9f",
					orderName(order), gainName(gain), got, gain)
			}

			if got := cascadeMagnitudeDB(low, testSR/2, testSR); math.Abs(got) > flatTol {
				t.Errorf("low %s %s: Nyquist = %.6f dB, want 0 (tol %.3f)",
					orderName(order), gainName(gain), got, flatTol)
			}

			high, err := Chebyshev2HighShelf(testSR, 1000, gain, stopbandDB, order)
			if err != nil {
				t.Fatalf("high %s %s: %v", orderName(order), gainName(gain), err)
			}

			if got := cascadeMagnitudeDB(high, testSR/2, testSR); math.Abs(got-gain) > 1e-6 {
				t.Errorf("high %s %s: Nyquist = %.9f dB, want %.9f",
					orderName(order), gainName(gain), got, gain)
			}

			if got := cascadeMagnitudeDB(high, 0, testSR); math.Abs(got) > flatTol {
				t.Errorf("high %s %s: DC = %.6f dB, want 0 (tol %.3f)",
					orderName(order), gainName(gain), got, flatTol)
			}
		}
	}
}

// ==== equiripple stopband, monotonic shelf ====

// TestChebyshev2LowShelf_FlatBandEquiripple is what distinguishes a real
// Type II design from the Butterworth-with-shifted-gain approximation this
// designer used to be: once inside the reference band the response never leaves
// the stopbandDB envelope, oscillating with exactly M-1 extrema.
func TestChebyshev2LowShelf_FlatBandEquiripple(t *testing.T) {
	const (
		steps      = 60000
		stopbandDB = 0.5
	)

	gains := []float64{-24, -12, -3, 3, 12, 24}

	for order := 1; order <= 8; order++ {
		for _, gain := range gains {
			sections, err := Chebyshev2LowShelf(testSR, 1000, gain, stopbandDB, order)
			if err != nil {
				t.Fatalf("%s %s: %v", orderName(order), gainName(gain), err)
			}

			entered := false
			extrema := 0
			worst := 0.0

			var prev, prev2 float64

			for i := range steps {
				freq := 1 + (testSR/2-2)*float64(i)/float64(steps-1)
				mag := cascadeMagnitudeDB(sections, freq, testSR)

				if !entered && math.Abs(mag) <= stopbandDB+1e-9 {
					entered = true
				}

				if entered {
					if dev := math.Abs(mag) - stopbandDB; dev > worst {
						worst = dev
					}

					if i >= 2 && ((prev > prev2 && prev > mag) || (prev < prev2 && prev < mag)) {
						extrema++
					}
				}

				prev2, prev = prev, mag
			}

			if !entered {
				t.Errorf("%s %s: response never reached the %.2f dB envelope",
					orderName(order), gainName(gain), stopbandDB)

				continue
			}

			if worst > 1e-6 {
				t.Errorf("%s %s: flat band exceeds the %.2f dB envelope by %.3e dB",
					orderName(order), gainName(gain), stopbandDB, worst)
			}

			if extrema != order-1 {
				t.Errorf("%s %s: %d flat-band extrema, want %d",
					orderName(order), gainName(gain), extrema, order-1)
			}
		}
	}
}

// TestChebyshev2LowShelf_MonotonicShelfRegion checks the property Type II has
// and elliptic does not: no ripple on the shelf side.
func TestChebyshev2LowShelf_MonotonicShelfRegion(t *testing.T) {
	orders := []int{1, 2, 3, 4, 6, 8}
	gains := []float64{3, 6, 12, 24}

	for _, order := range orders {
		for _, gain := range gains {
			sections, err := Chebyshev2LowShelf(testSR, 1000, gain, 0.5, order)
			if err != nil {
				t.Fatalf("%s %s: %v", orderName(order), gainName(gain), err)
			}

			prev := cascadeMagnitudeDB(sections, 1, testSR)

			for freq := 11.0; freq <= 1000; freq += 10 {
				mag := cascadeMagnitudeDB(sections, freq, testSR)
				if mag > prev+1e-9 {
					t.Errorf("%s %s: boost shelf rises at %.0f Hz (%.6f -> %.6f dB)",
						orderName(order), gainName(gain), freq, prev, mag)

					break
				}

				prev = mag
			}
		}
	}
}

// ==== sweeps ====

func TestChebyshev2Shelf_VariousFrequencies(t *testing.T) {
	freqs := []float64{100, 300, 500, 1000, 2000, 5000, 10000}

	for _, design := range chebyshev2Designers() {
		for _, freq := range freqs {
			sections, err := design.fn(testSR, freq, 12, 0.5, 6)
			if err != nil {
				t.Fatalf("%s %s: %v", design.name, freqName(freq), err)
			}

			allPolesStable(t, sections)
		}
	}
}

func TestChebyshev2Shelf_ExtremeGains(t *testing.T) {
	gains := []float64{-30, -20, -6, -1, 1, 6, 20, 30}

	for _, design := range chebyshev2Designers() {
		for _, gain := range gains {
			sections, err := design.fn(testSR, 1000, gain, 0.5, 6)
			if err != nil {
				t.Fatalf("%s %s: %v", design.name, gainName(gain), err)
			}

			allPolesStable(t, sections)

			if got, want := cascadeMagnitudeDB(sections, 1000, testSR), cutoffTargetDB(gain); math.Abs(got-want) > 1e-6 {
				t.Errorf("%s %s: |H(fc)| = %.9f dB, want %.9f", design.name, gainName(gain), got, want)
			}
		}
	}
}

func TestChebyshev2Shelf_VariousStopband(t *testing.T) {
	stopbands := []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.0}

	for _, design := range chebyshev2Designers() {
		for _, stopband := range stopbands {
			sections, err := design.fn(testSR, 1000, 18, stopband, 6)
			if err != nil {
				t.Fatalf("%s stopband %.2f: %v", design.name, stopband, err)
			}

			allPolesStable(t, sections)
		}
	}
}

// TestChebyshev2Shelf_BoostCutInversion mirrors the Butterworth expectation:
// the package cutoff convention is asymmetric between boost and cut, so exact
// reciprocity holds only at the band edges. See TestLowShelf_BoostCutInversion.
func TestChebyshev2Shelf_BoostCutInversion(t *testing.T) {
	for _, design := range chebyshev2Designers() {
		for order := 1; order <= 6; order++ {
			boost, err := design.fn(testSR, 1000, 12, 0.5, order)
			if err != nil {
				t.Fatalf("boost %s: %v", orderName(order), err)
			}

			cut, err := design.fn(testSR, 1000, -12, 0.5, order)
			if err != nil {
				t.Fatalf("cut %s: %v", orderName(order), err)
			}

			combined := func(freq float64) float64 {
				return cascadeMagnitudeDB(boost, freq, testSR) + cascadeMagnitudeDB(cut, freq, testSR)
			}

			for _, freq := range []float64{1, testSR/2 - 1} {
				if sum := combined(freq); math.Abs(sum) > 0.1 {
					t.Errorf("%s %s %s: boost+cut = %.6f dB, want ~0",
						design.name, orderName(order), freqName(freq), sum)
				}
			}

			for _, freq := range []float64{50, 15000} {
				if sum := combined(freq); math.Abs(sum) > 0.5 {
					t.Errorf("%s %s %s: boost+cut = %.6f dB, want ~0",
						design.name, orderName(order), freqName(freq), sum)
				}
			}
		}
	}
}

// ==== grids ====

const cheby2GridMaxExamples = 8

// cheby2GridCaseName formats a deterministic identifier for grid sweeps.
func cheby2GridCaseName(gainDB, stopbandDB float64, order int, cutoffHz float64) string {
	return fmt.Sprintf("G%+.1f_SB%.2f_M%d_F%.0f", gainDB, stopbandDB, order, cutoffHz)
}

func cheby2AppendFailure(examples *[]string, msg string) {
	if len(*examples) < cheby2GridMaxExamples {
		*examples = append(*examples, msg)
	}
}

func TestChebyshev2Shelf_CutoffAnchorGrid(t *testing.T) {
	orders := []int{1, 2, 3, 4, 6, 8}
	cutoffs := []float64{300, 1000, 3000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{-24, -12, -6, -3, 3, 6, 12, 24}

	var examples []string

	total, failed := 0, 0

	for _, design := range chebyshev2Designers() {
		for _, order := range orders {
			for _, cutoff := range cutoffs {
				for _, stopband := range stopbands {
					for _, gain := range gains {
						if stopband >= math.Abs(gain) {
							continue
						}

						total++

						sections, err := design.fn(testSR, cutoff, gain, stopband, order)
						if err != nil {
							failed++

							cheby2AppendFailure(&examples, fmt.Sprintf("%s %s: %v",
								design.name, cheby2GridCaseName(gain, stopband, order, cutoff), err))

							continue
						}

						got := cascadeMagnitudeDB(sections, cutoff, testSR)
						if want := cutoffTargetDB(gain); math.Abs(got-want) > 1e-6 {
							failed++

							cheby2AppendFailure(&examples, fmt.Sprintf("%s %s: |H(fc)| = %.9f dB, want %.9f",
								design.name, cheby2GridCaseName(gain, stopband, order, cutoff), got, want))
						}
					}
				}
			}
		}
	}

	if failed > 0 {
		t.Fatalf("cutoff-anchor grid failures: %d/%d cases failed. First %d:\n%s",
			failed, total, len(examples), strings.Join(examples, "\n"))
	}
}

func TestChebyshev2LowShelf_MonotonicGrid_Boost(t *testing.T) {
	orders := []int{2, 3, 4, 6, 8}
	cutoffs := []float64{300, 1000, 3000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{3, 6, 12, 24}

	var examples []string

	total, failed := 0, 0

	for _, order := range orders {
		for _, cutoff := range cutoffs {
			for _, stopband := range stopbands {
				for _, gain := range gains {
					total++

					sections, err := Chebyshev2LowShelf(testSR, cutoff, gain, stopband, order)
					if err != nil {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: %v",
							cheby2GridCaseName(gain, stopband, order, cutoff), err))

						continue
					}

					upper := math.Min(cutoff*0.8, 800)
					step := math.Max(5, upper/80)
					prev := cascadeMagnitudeDB(sections, 1, testSR)
					bad := false

					for freq := step; freq <= upper; freq += step {
						mag := cascadeMagnitudeDB(sections, freq, testSR)
						if mag > prev+0.12 {
							bad = true

							cheby2AppendFailure(&examples, fmt.Sprintf("%s: rises at %.0f Hz (%.4f -> %.4f dB)",
								cheby2GridCaseName(gain, stopband, order, cutoff), freq, prev, mag))

							break
						}

						prev = mag
					}

					if bad {
						failed++
					}
				}
			}
		}
	}

	if failed > 0 {
		t.Fatalf("monotonic-shelf grid failures: %d/%d cases failed. First %d:\n%s",
			failed, total, len(examples), strings.Join(examples, "\n"))
	}
}

func TestChebyshev2Shelf_StabilityGrid(t *testing.T) {
	orders := []int{1, 2, 3, 4, 5, 6, 8, 10}
	cutoffs := []float64{50, 300, 1000, 3000, 12000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{-30, -12, -3, 3, 12, 30}

	var examples []string

	total, failed := 0, 0

	for _, design := range chebyshev2Designers() {
		for _, order := range orders {
			for _, cutoff := range cutoffs {
				for _, stopband := range stopbands {
					for _, gain := range gains {
						if stopband >= math.Abs(gain) {
							continue
						}

						total++

						sections, err := design.fn(testSR, cutoff, gain, stopband, order)
						if err != nil {
							failed++

							cheby2AppendFailure(&examples, fmt.Sprintf("%s %s: %v",
								design.name, cheby2GridCaseName(gain, stopband, order, cutoff), err))

							continue
						}

						if !sectionsStable(sections) {
							failed++

							cheby2AppendFailure(&examples, fmt.Sprintf("%s %s: unstable poles",
								design.name, cheby2GridCaseName(gain, stopband, order, cutoff)))
						}
					}
				}
			}
		}
	}

	if failed > 0 {
		t.Fatalf("stability grid failures: %d/%d cases failed. First %d:\n%s",
			failed, total, len(examples), strings.Join(examples, "\n"))
	}
}

// sectionsStable reports whether every pole of the cascade lies inside the unit
// circle, without failing a test (for use inside grid counters).
func sectionsStable(sections []biquad.Coefficients) bool {
	for _, s := range sections {
		if math.Abs(s.A2) >= 1.0 {
			return false
		}

		if s.A2 != 0 && math.Abs(s.A1) >= 1.0+s.A2 {
			return false
		}
	}

	return true
}

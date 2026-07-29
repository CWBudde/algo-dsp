package shelving

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// shelfDesigner pairs a designer with a label so the low- and high-shelf
// variants can share one test body.
type shelfDesigner struct {
	name string
	fn   func(sampleRate, freqHz, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error)
}

func shelfDesigners() []shelfDesigner {
	return []shelfDesigner{
		{"low", EllipticLowShelf},
		{"high", EllipticHighShelf},
	}
}

// cutoffTargetDB is the magnitude the package's shelving designers must reach at
// the cutoff frequency: |H(f_c)|² = (G² + G0²)/2 (Holters & Zölzer, eq. 5).
func cutoffTargetDB(gainDB float64) float64 {
	g := db2Lin(gainDB)
	return 10 * math.Log10((g*g+1)*0.5)
}

// ==== validation ====

func TestEllipticLowShelf_InvalidParams(t *testing.T) {
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
		{"stopband above gain", testSR, 1000, 6, 12, 4},
		{"stopband equal to gain", testSR, 1000, 6, 6, 4},
		{"stopband leaves no room below shelf ripple", testSR, 1000, 0.04, 0.01, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := EllipticLowShelf(tt.sr, tt.freq, tt.gainDB, tt.stopbandDB, tt.order); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestEllipticHighShelf_InvalidParams(t *testing.T) {
	if _, err := EllipticHighShelf(0, 1000, 12, 0.5, 4); err == nil {
		t.Error("zero sample rate should error")
	}

	if _, err := EllipticHighShelf(testSR, 1000, 12, 0, 4); err == nil {
		t.Error("zero stopband should error")
	}

	if _, err := EllipticHighShelf(testSR, 1000, 12, 0.5, 0); err == nil {
		t.Error("zero order should error")
	}
}

func TestEllipticShelf_ZeroGain(t *testing.T) {
	for _, design := range shelfDesigners() {
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

func TestEllipticShelf_SectionCount(t *testing.T) {
	for _, design := range shelfDesigners() {
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

func TestEllipticShelf_Stability(t *testing.T) {
	orders := []int{1, 2, 3, 4, 5, 6, 8, 10, 12}
	gains := []float64{-24, -12, -3, 3, 12, 24}

	for _, design := range shelfDesigners() {
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

// ==== the defining anchors ====

// TestEllipticShelf_CutoffAnchor is the test that pins freqHz to the same
// meaning the Butterworth and Chebyshev I designers give it.
func TestEllipticShelf_CutoffAnchor(t *testing.T) {
	gains := []float64{-24, -12, -6, -1, 1, 6, 12, 24}
	freqs := []float64{100, 300, 1000, 5000, 10000}

	for _, design := range shelfDesigners() {
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

func TestEllipticShelf_EndpointAnchors(t *testing.T) {
	const stopbandDB = 0.5

	gains := []float64{-24, -12, -6, 6, 12, 24}

	for order := 1; order <= 8; order++ {
		for _, gain := range gains {
			// Even orders land on the ripple bound at the band edges, odd
			// orders land on the nominal values.
			shelfTol, flatTol := 1e-6, 1e-6
			if order%2 == 0 {
				shelfTol, flatTol = shelfRippleDB+1e-6, stopbandDB+1e-6
			}

			low, err := EllipticLowShelf(testSR, 1000, gain, stopbandDB, order)
			if err != nil {
				t.Fatalf("low %s %s: %v", orderName(order), gainName(gain), err)
			}

			if got := cascadeMagnitudeDB(low, 0, testSR); math.Abs(got-gain) > shelfTol {
				t.Errorf("low %s %s: DC = %.6f dB, want %.6f (tol %.3f)",
					orderName(order), gainName(gain), got, gain, shelfTol)
			}

			if got := cascadeMagnitudeDB(low, testSR/2, testSR); math.Abs(got) > flatTol {
				t.Errorf("low %s %s: Nyquist = %.6f dB, want 0 (tol %.3f)",
					orderName(order), gainName(gain), got, flatTol)
			}

			high, err := EllipticHighShelf(testSR, 1000, gain, stopbandDB, order)
			if err != nil {
				t.Fatalf("high %s %s: %v", orderName(order), gainName(gain), err)
			}

			if got := cascadeMagnitudeDB(high, testSR/2, testSR); math.Abs(got-gain) > shelfTol {
				t.Errorf("high %s %s: Nyquist = %.6f dB, want %.6f (tol %.3f)",
					orderName(order), gainName(gain), got, gain, shelfTol)
			}

			if got := cascadeMagnitudeDB(high, 0, testSR); math.Abs(got) > flatTol {
				t.Errorf("high %s %s: DC = %.6f dB, want 0 (tol %.3f)",
					orderName(order), gainName(gain), got, flatTol)
			}
		}
	}
}

// ==== equiripple behaviour ====

// TestEllipticLowShelf_FlatBandEquiripple checks the property that separates a
// genuine elliptic design from a Butterworth one: once the response has entered
// the reference band it never leaves the stopbandDB envelope again, and it
// oscillates there with exactly M-1 local extrema.
func TestEllipticLowShelf_FlatBandEquiripple(t *testing.T) {
	const (
		steps      = 60000
		stopbandDB = 0.5
	)

	gains := []float64{-24, -12, -3, 3, 12, 24}

	for order := 1; order <= 8; order++ {
		for _, gain := range gains {
			sections, err := EllipticLowShelf(testSR, 1000, gain, stopbandDB, order)
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

// TestEllipticLowShelf_ShelfBandRipple checks the mirrored bound on the shelf
// side, which this library fixes at shelfRippleDB.
func TestEllipticLowShelf_ShelfBandRipple(t *testing.T) {
	const steps = 20000

	gains := []float64{-24, -12, 12, 24}

	for order := 1; order <= 8; order++ {
		for _, gain := range gains {
			sections, err := EllipticLowShelf(testSR, 1000, gain, 0.5, order)
			if err != nil {
				t.Fatalf("%s %s: %v", orderName(order), gainName(gain), err)
			}

			entered := false
			worst := 0.0

			// Walk down from the cutoff towards DC; once inside the shelf
			// envelope the response must stay there.
			for i := range steps {
				freq := 1000 * (1 - float64(i)/float64(steps))
				dev := math.Abs(cascadeMagnitudeDB(sections, freq, testSR) - gain)

				if !entered && dev <= shelfRippleDB+1e-9 {
					entered = true
				}

				if entered && dev-shelfRippleDB > worst {
					worst = dev - shelfRippleDB
				}
			}

			if !entered {
				t.Errorf("%s %s: response never reached the shelf envelope",
					orderName(order), gainName(gain))

				continue
			}

			if worst > 1e-6 {
				t.Errorf("%s %s: shelf band exceeds the %.2f dB envelope by %.3e dB",
					orderName(order), gainName(gain), shelfRippleDB, worst)
			}
		}
	}
}

// ==== reciprocity and sweeps ====

// TestEllipticShelf_BoostCutInversion mirrors the Butterworth expectation.
//
// Cascading a boost with the matching cut does NOT give unity here: the
// package-wide cutoff convention |H(f_c)|² = (G² + G0²)/2 is inherently
// asymmetric between amplification and attenuation (Holters & Zölzer §2.3,
// Fig. 3). Exact reciprocity would require the sqrt(G) cutoff of their eq. (11)
// instead. So inversion is only asserted at the band edges and well away from
// the transition, exactly as TestLowShelf_BoostCutInversion does.
func TestEllipticShelf_BoostCutInversion(t *testing.T) {
	for _, design := range shelfDesigners() {
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

			// Band edges: inversion is exact up to the ripple bounds.
			for _, freq := range []float64{1, testSR/2 - 1} {
				if sum := combined(freq); math.Abs(sum) > 0.1 {
					t.Errorf("%s %s %s: boost+cut = %.6f dB, want ~0",
						design.name, orderName(order), freqName(freq), sum)
				}
			}

			// Well away from the transition: inversion is close.
			for _, freq := range []float64{50, 15000} {
				if sum := combined(freq); math.Abs(sum) > 0.5 {
					t.Errorf("%s %s %s: boost+cut = %.6f dB, want ~0",
						design.name, orderName(order), freqName(freq), sum)
				}
			}
		}
	}
}

func TestEllipticShelf_VariousFrequencies(t *testing.T) {
	freqs := []float64{100, 300, 500, 1000, 2000, 5000, 10000}

	for _, design := range shelfDesigners() {
		for _, freq := range freqs {
			sections, err := design.fn(testSR, freq, 12, 0.5, 6)
			if err != nil {
				t.Fatalf("%s %s: %v", design.name, freqName(freq), err)
			}

			allPolesStable(t, sections)
		}
	}
}

func TestEllipticShelf_VariousStopband(t *testing.T) {
	stopbands := []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.0}

	for _, design := range shelfDesigners() {
		for _, stopband := range stopbands {
			sections, err := design.fn(testSR, 1000, 18, stopband, 6)
			if err != nil {
				t.Fatalf("%s stopband %.2f: %v", design.name, stopband, err)
			}

			allPolesStable(t, sections)

			if got, want := cascadeMagnitudeDB(sections, 1000, testSR), cutoffTargetDB(18); math.Abs(got-want) > 1e-6 {
				t.Errorf("%s stopband %.2f: |H(fc)| = %.9f dB, want %.9f", design.name, stopband, got, want)
			}
		}
	}
}

// ==== grids ====

const ellipticGridMaxExamples = 8

func ellipticGridCaseName(gainDB, stopbandDB float64, order int, cutoffHz float64) string {
	return fmt.Sprintf("G%+.1f_SB%.2f_M%d_F%.0f", gainDB, stopbandDB, order, cutoffHz)
}

func ellipticAppendFailure(examples *[]string, msg string) {
	if len(*examples) < ellipticGridMaxExamples {
		*examples = append(*examples, msg)
	}
}

func TestEllipticShelf_CutoffAnchorGrid(t *testing.T) {
	orders := []int{1, 2, 3, 4, 6, 8}
	cutoffs := []float64{300, 1000, 3000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{-24, -12, -6, -3, 3, 6, 12, 24}

	var examples []string

	total, failed := 0, 0

	for _, design := range shelfDesigners() {
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

							ellipticAppendFailure(&examples, fmt.Sprintf("%s %s: %v",
								design.name, ellipticGridCaseName(gain, stopband, order, cutoff), err))

							continue
						}

						got := cascadeMagnitudeDB(sections, cutoff, testSR)
						if want := cutoffTargetDB(gain); math.Abs(got-want) > 1e-6 {
							failed++

							ellipticAppendFailure(&examples, fmt.Sprintf("%s %s: |H(fc)| = %.9f dB, want %.9f",
								design.name, ellipticGridCaseName(gain, stopband, order, cutoff), got, want))
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

func TestEllipticShelf_StabilityGrid(t *testing.T) {
	orders := []int{1, 2, 3, 4, 5, 6, 8, 10}
	cutoffs := []float64{50, 300, 1000, 3000, 12000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{-30, -12, -3, 3, 12, 30}

	var examples []string

	total, failed := 0, 0

	for _, design := range shelfDesigners() {
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

							ellipticAppendFailure(&examples, fmt.Sprintf("%s %s: %v",
								design.name, ellipticGridCaseName(gain, stopband, order, cutoff), err))

							continue
						}

						for i, s := range sections {
							unstable := math.Abs(s.A2) >= 1.0 ||
								(s.A2 != 0 && math.Abs(s.A1) >= 1.0+s.A2)
							if unstable {
								failed++

								ellipticAppendFailure(&examples, fmt.Sprintf("%s %s: section %d unstable (A1=%.6f A2=%.6f)",
									design.name, ellipticGridCaseName(gain, stopband, order, cutoff), i, s.A1, s.A2))

								break
							}
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

package shelving

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// chebyshev2SectionsNoDCCorrection mirrors chebyshev2Sections but intentionally
// skips the final DC gain correction so we can test where endpoint behavior
// diverges.
func chebyshev2SectionsNoDCCorrection(K float64, gainDB, stopbandDB float64, order int) []biquad.Coefficients {
	G0 := 1.0
	G := db2Lin(gainDB)
	Gb := db2Lin(stopbandDB)
	g := math.Pow(G, 1.0/float64(order))

	e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	eu := math.Pow(e+math.Sqrt(1+e*e), 1.0/float64(order))
	ew := math.Pow(G0*e+Gb*math.Sqrt(1.0+e*e), 1.0/float64(order))
	A := (eu - 1.0/eu) * 0.5
	B := (ew - g*g/ew) * 0.5

	L := order / 2
	hasFirstOrder := order%2 == 1
	n := L
	if hasFirstOrder {
		n++
	}
	sections := make([]biquad.Coefficients, 0, n)

	for m := 1; m <= L; m++ {
		theta := float64(2*m-1) / float64(2*order) * math.Pi
		si := math.Sin(theta)
		ci := math.Cos(theta)
		sp := sosParams{
			den: poleParams{sigma: A * si, r2: A*A + ci*ci},
			num: poleParams{sigma: B * si, r2: B*B + g*g*ci*ci},
		}
		sections = append(sections, bilinearSOS(K, sp))
	}

	if hasFirstOrder {
		sections = append(sections, bilinearFOS(K, fosParams{denSigma: A, numSigma: B}))
	}

	return sections
}

func TestChebyshev2Math_DCCorrectionScalesAllFrequencies(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 6
	gainDB := 12.0
	rippleDB := 0.5
	K := math.Tan(math.Pi * fc / sr)

	stopbandDB := rippleDB
	raw := chebyshev2SectionsNoDCCorrection(K, gainDB, stopbandDB, order)
	corrected := chebyshev2Sections(K, gainDB, stopbandDB, order)

	rawDC := cmplx.Abs(cascadeResponse(raw, 1, sr))
	corrDC := cmplx.Abs(cascadeResponse(corrected, 1, sr))
	rawNyq := cmplx.Abs(cascadeResponse(raw, sr/2-1, sr))
	corrNyq := cmplx.Abs(cascadeResponse(corrected, sr/2-1, sr))

	dcScale := corrDC / rawDC
	nyqScale := corrNyq / rawNyq

	if !almostEqual(dcScale, nyqScale, 1e-6) {
		t.Fatalf("expected uniform scale from correction: dcScale=%.8f nyqScale=%.8f", dcScale, nyqScale)
	}
}

func TestChebyshev2Math_RawAndCorrectedEndpointAnchors(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 6
	gainDB := 12.0
	rippleDB := 0.5
	K := math.Tan(math.Pi * fc / sr)
	stopbandDB := rippleDB

	raw := chebyshev2SectionsNoDCCorrection(K, gainDB, stopbandDB, order)
	corrected := chebyshev2Sections(K, gainDB, stopbandDB, order)

	rawDC := cascadeMagnitudeDB(raw, 1, sr)
	rawNyq := cascadeMagnitudeDB(raw, sr/2-1, sr)
	corrDC := cascadeMagnitudeDB(corrected, 1, sr)
	corrNyq := cascadeMagnitudeDB(corrected, sr/2-1, sr)

	// Diagnostic expectations for current implementation behavior:
	// 1) Raw sections are anchored near stopbandDB at DC.
	// 2) Raw sections are anchored near 0 dB at Nyquist.
	// 3) After correction, DC is moved to gainDB and Nyquist is shifted by
	//    roughly gainDB-stopbandDB.
	if !almostEqual(rawDC, stopbandDB, 0.2) {
		t.Fatalf("raw DC = %.4f dB, expected near stopband %.4f dB", rawDC, stopbandDB)
	}
	if math.Abs(rawNyq) > 0.2 {
		t.Fatalf("raw Nyquist = %.4f dB, expected near 0 dB", rawNyq)
	}
	if !almostEqual(corrDC, gainDB, 0.2) {
		t.Fatalf("corrected DC = %.4f dB, expected near gain %.4f dB", corrDC, gainDB)
	}
	if !almostEqual(corrNyq, gainDB-stopbandDB, 0.2) {
		t.Fatalf("corrected Nyquist = %.4f dB, expected near gain-stopband %.4f dB", corrNyq, gainDB-stopbandDB)
	}
}

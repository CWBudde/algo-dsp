package shelving

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

const testSR = 48000.0

func almostEqual(a, b, tol float64) bool {
	if a == b {
		return true
	}

	diff := math.Abs(a - b)
	if tol > 0 && tol < 1 {
		mag := math.Max(math.Abs(a), math.Abs(b))
		if mag > 1 {
			return diff/mag < tol
		}
	}

	return diff < tol
}

func cascadeResponse(sections []biquad.Coefficients, freqHz, sampleRate float64) complex128 {
	h := complex(1, 0)
	for i := range sections {
		h *= sections[i].Response(freqHz, sampleRate)
	}

	return h
}

func cascadeMagnitudeDB(sections []biquad.Coefficients, freqHz, sampleRate float64) float64 {
	h := cascadeResponse(sections, freqHz, sampleRate)
	return 20 * math.Log10(cmplx.Abs(h))
}

func allPolesStable(t *testing.T, sections []biquad.Coefficients) {
	t.Helper()

	for i, s := range sections {
		if math.Abs(s.A2) >= 1.0 {
			t.Errorf("section %d: |A2|=%.6f >= 1, poles outside unit circle", i, math.Abs(s.A2))
		}

		if s.A2 != 0 && math.Abs(s.A1) >= 1.0+s.A2 {
			t.Errorf("section %d: |A1|=%.6f >= 1+A2=%.6f, poles outside unit circle", i, math.Abs(s.A1), 1.0+s.A2)
		}
	}
}

func TestCascadeMagnitudeDB_NonDefaultSampleRate(t *testing.T) {
	mag := cascadeMagnitudeDB([]biquad.Coefficients{{B0: 1}}, 1000, 44100)
	if !almostEqual(mag, 0, 1e-12) {
		t.Fatalf("unity section magnitude = %v dB, want 0 dB", mag)
	}
}

func orderName(order int) string {
	return "M" + itoa(order)
}

func gainName(gain float64) string {
	if gain >= 0 {
		return "+" + ftoa(gain) + "dB"
	}

	return ftoa(gain) + "dB"
}

func freqName(freq float64) string {
	return ftoa(freq) + "Hz"
}

func itoa(n int) string {
	return ftoa(float64(n))
}

func ftoa(f float64) string {
	s := ""
	if f < 0 {
		s = "-"
		f = -f
	}

	whole := int(f)
	frac := f - float64(whole)

	result := s + intToStr(whole)
	if frac > 0.001 {
		result += "." + intToStr(int(frac*10))
	}

	return result
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}

	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}

	return digits
}

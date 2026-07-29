package orfanidis

import (
	"errors"
	"math"
	"testing"
)

func TestIsZero(t *testing.T) {
	if !IsZero(0) || !IsZero(1e-13) || !IsZero(-1e-13) {
		t.Error("negligible values should be reported as zero")
	}

	if IsZero(1e-11) || IsZero(-1e-11) {
		t.Error("1e-11 should not be reported as zero")
	}
}

func TestSectionOrder(t *testing.T) {
	tests := []struct {
		name string
		sec  Section
		want int
	}{
		{"gain only", Section{B0: 2, A0: 1}, 0},
		{"first order", Section{B0: 1, B1: 2, A0: 1, A1: 3}, 1},
		{"first order from denominator", Section{B0: 1, A0: 1, A1: 3}, 1},
		{"second order", Section{B0: 1, B2: 1, A0: 1, A2: 1}, 2},
		{"second order from denominator", Section{B0: 1, A0: 1, A2: 1}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sectionOrder(tt.sec); got != tt.want {
				t.Errorf("sectionOrder = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestLowpassBLT_MatchesDirectEvaluation checks the bilinear transform by
// comparing the digital response at several frequencies against the analog
// prototype evaluated at the prewarped frequency, which is the defining
// property of the transform.
func TestLowpassBLT_MatchesDirectEvaluation(t *testing.T) {
	secs := []Section{
		{B0: 4, A0: 1},
		{B0: 2, B1: 0.5, A0: 1, A1: 0.75},
		{B0: 1, B1: 0.3, B2: 0.2, A0: 1, A1: 0.9, A2: 0.4},
	}

	sections, err := LowpassBLT(secs)
	if err != nil {
		t.Fatalf("LowpassBLT: %v", err)
	}

	if len(sections) != len(secs) {
		t.Fatalf("section count = %d, want %d", len(sections), len(secs))
	}

	const sampleRate = 48000.0

	for _, freq := range []float64{10, 250, 1000, 6000, 20000} {
		omega := math.Tan(math.Pi * freq / sampleRate)
		want := AnalogMagnitude(secs, omega)

		got := 1.0

		for i := range sections {
			h := sections[i].Response(freq, sampleRate)
			got *= math.Hypot(real(h), imag(h))
		}

		if math.Abs(got-want) > 1e-9*math.Max(1, want) {
			t.Errorf("freq %.0f Hz: digital |H| = %.12f, analog |H| = %.12f", freq, got, want)
		}
	}
}

func TestLowpassBLT_Errors(t *testing.T) {
	if _, err := LowpassBLT(nil); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("empty input: err = %v, want ErrInvalidParams", err)
	}

	// A denominator that sums to zero cannot be normalized.
	if _, err := LowpassBLT([]Section{{B0: 1, A0: 1, A1: -2, A2: 1}}); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("singular denominator: err = %v, want ErrInvalidParams", err)
	}

	if _, err := LowpassBLT([]Section{{B0: 1, A0: 0}}); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("zero gain denominator: err = %v, want ErrInvalidParams", err)
	}

	if _, err := LowpassBLT([]Section{{B0: 1, B1: 1, A0: 1, A1: -1}}); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("singular first-order denominator: err = %v, want ErrInvalidParams", err)
	}
}

func TestBandpassBLT_Errors(t *testing.T) {
	if _, err := BandpassBLT([]Section{{B0: 1, A0: 0}}, 0.5); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("err = %v, want ErrInvalidParams", err)
	}
}

// TestBandpassBLT_DegenerateMatchesLowpass checks that ω0 = 0 reproduces the
// plain lowpass transform and ω0 = π negates the odd-power coefficients, which
// is what makes the shelving designers a special case of the band ones.
func TestBandpassBLT_DegenerateMatchesLowpass(t *testing.T) {
	secs := []Section{
		{B0: 2, A0: 1},
		{B0: 1, B1: 0.4, B2: 0.25, A0: 1, A1: 0.8, A2: 0.3},
	}

	low, err := LowpassBLT(secs)
	if err != nil {
		t.Fatalf("LowpassBLT: %v", err)
	}

	atDC, err := BandpassBLT(secs, 0)
	if err != nil {
		t.Fatalf("BandpassBLT(0): %v", err)
	}

	atNyquist, err := BandpassBLT(secs, math.Pi)
	if err != nil {
		t.Fatalf("BandpassBLT(pi): %v", err)
	}

	for i := range low {
		if math.Abs(atDC[i].B[1]-low[i].B1) > 1e-12 || math.Abs(atDC[i].A[1]-low[i].A1) > 1e-12 {
			t.Errorf("section %d: w0=0 does not match the lowpass transform", i)
		}

		if math.Abs(atNyquist[i].B[1]+low[i].B1) > 1e-12 || math.Abs(atNyquist[i].A[1]+low[i].A1) > 1e-12 {
			t.Errorf("section %d: w0=pi does not negate the odd-power coefficients", i)
		}
	}
}

func TestAnalogMagnitudeAndLimit(t *testing.T) {
	// A first-order shelf from 4 down to 2.
	secs := []Section{{B0: 4, B1: 2, A0: 1, A1: 1}}

	if got := AnalogMagnitude(secs, 0); math.Abs(got-4) > 1e-12 {
		t.Errorf("DC magnitude = %v, want 4", got)
	}

	if got := analogLimit(secs); math.Abs(got-2) > 1e-12 {
		t.Errorf("limit magnitude = %v, want 2", got)
	}

	if got := AnalogMagnitude([]Section{{B0: 1}}, 1); !math.IsInf(got, 1) {
		t.Errorf("zero denominator magnitude = %v, want +Inf", got)
	}

	if got := analogLimit([]Section{{B0: 1}}); !math.IsInf(got, 1) {
		t.Errorf("zero denominator limit = %v, want +Inf", got)
	}
}

// TestEdgeOmega_PlacesTheRequestedLevel is the property the shelving cutoff
// convention depends on.
func TestEdgeOmega_PlacesTheRequestedLevel(t *testing.T) {
	gains := []float64{-24, -12, -3, 3, 12, 24}
	orders := []int{1, 2, 3, 4, 6, 8}

	for _, gainDB := range gains {
		for _, order := range orders {
			g := math.Pow(10, gainDB/20)

			spec := Spec{
				Order: order,
				G0:    1,
				G:     g,
				Gb:    math.Pow(10, (gainDB-math.Copysign(0.05, gainDB))/20),
				Gs:    math.Pow(10, math.Copysign(0.5, gainDB)/20),
			}

			build := func(wb float64) ([]Section, error) {
				scaled := spec
				scaled.WB = wb

				return EllipticPrototype(scaled)
			}

			target := math.Sqrt((g*g + 1) * 0.5)

			omega, err := EdgeOmega(build, target)
			if err != nil {
				t.Fatalf("gain %v order %d: %v", gainDB, order, err)
			}

			secs, err := build(1.0)
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			if got := AnalogMagnitude(secs, omega); math.Abs(got-target) > 1e-9*target {
				t.Errorf("gain %v order %d: |H(omega)| = %.12f, want %.12f", gainDB, order, got, target)
			}
		}
	}
}

func TestEdgeOmega_Errors(t *testing.T) {
	ok := func(float64) ([]Section, error) {
		return []Section{{B0: 4, B1: 2, A0: 1, A1: 1}}, nil
	}

	if _, err := EdgeOmega(ok, 0); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("zero target: err = %v, want ErrInvalidParams", err)
	}

	if _, err := EdgeOmega(ok, math.NaN()); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("NaN target: err = %v, want ErrInvalidParams", err)
	}

	boom := errors.New("boom")
	if _, err := EdgeOmega(func(float64) ([]Section, error) { return nil, boom }, 2); !errors.Is(err, boom) {
		t.Errorf("build error: err = %v, want boom", err)
	}

	// A target outside the reachable range never brackets a crossing.
	if _, err := EdgeOmega(ok, 100); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("unreachable target: err = %v, want ErrInvalidParams", err)
	}
}

func TestPrototypes_RejectInvalidSpecs(t *testing.T) {
	builders := map[string]func(Spec) ([]Section, error){
		"elliptic":   EllipticPrototype,
		"chebyshev2": Chebyshev2Prototype,
	}

	base := Spec{Order: 4, G0: 1, G: 4, Gb: 3.9, Gs: 1.05, WB: 1}

	for name, build := range builders {
		t.Run(name, func(t *testing.T) {
			bad := base
			bad.Order = 0

			if _, err := build(bad); !errors.Is(err, ErrInvalidParams) {
				t.Errorf("zero order: err = %v, want ErrInvalidParams", err)
			}

			bad = base
			bad.WB = 0

			if _, err := build(bad); !errors.Is(err, ErrInvalidParams) {
				t.Errorf("zero WB: err = %v, want ErrInvalidParams", err)
			}

			// Gs == G0 makes the ripple ratio singular.
			bad = base
			bad.Gs = 1

			if _, err := build(bad); !errors.Is(err, ErrInvalidParams) {
				t.Errorf("Gs == G0: err = %v, want ErrInvalidParams", err)
			}
		})
	}
}

package design

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestDesignerValidation(t *testing.T) {
	tests := []struct {
		name string
		sr   float64
		opts []DesignerOption
	}{
		{"zero sr", 0, nil},
		{"negative sr", -44100, nil},
		{"NaN sr", math.NaN(), nil},
		{"Inf sr", math.Inf(1), nil},
		{"order zero", 44100, []DesignerOption{WithOrder(0)}},
		{"order negative", 44100, []DesignerOption{WithOrder(-1)}},
		{"iterations zero", 44100, []DesignerOption{WithIterations(0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDesigner(tt.sr, tt.opts...)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestDesignerConverges(t *testing.T) {
	designer, err := NewDesigner(44100,
		WithOrder(5),
		WithIterations(500),
		WithSeed(42),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	coeffs, err := designer.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(coeffs) != 5 {
		t.Fatalf("got %d coefficients, want 5", len(coeffs))
	}

	// Coefficients should not all be zero (optimizer should have found something).
	allZero := true
	for _, coeff := range coeffs {
		if coeff != 0 {
			allZero = false

			break
		}
	}

	if allZero {
		t.Error("all coefficients are zero — optimizer did not converge")
	}
}

func TestDesignerCancellation(t *testing.T) {
	designer, err := NewDesigner(44100,
		WithOrder(8),
		WithIterations(1000000),
		WithSeed(42),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	coeffs, err := designer.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Should return whatever best was found before cancellation.
	if len(coeffs) != 8 {
		t.Errorf("got %d coefficients, want 8", len(coeffs))
	}
}

func TestDesignerProgressCallback(t *testing.T) {
	var callCount int

	designer, err := NewDesigner(44100,
		WithOrder(3),
		WithIterations(200),
		WithSeed(42),
		WithOnProgress(func(coeffs []float64, score float64) {
			callCount++

			if len(coeffs) != 3 {
				t.Errorf("callback got %d coefficients, want 3", len(coeffs))
			}

			if math.IsNaN(score) || math.IsInf(score, 0) {
				t.Errorf("callback got invalid score: %v", score)
			}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	designer.Run(ctx)

	if callCount == 0 {
		t.Error("progress callback was never called")
	}
}

func TestDesignerDeterministic(t *testing.T) {
	makeCoeffs := func() []float64 {
		designer, err := NewDesigner(44100,
			WithOrder(3),
			WithIterations(100),
			WithSeed(42),
		)
		if err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		coeffs, err := designer.Run(ctx)
		if err != nil {
			t.Fatal(err)
		}

		return coeffs
	}

	coeffs1 := makeCoeffs()
	coeffs2 := makeCoeffs()

	for idx := range coeffs1 {
		if coeffs1[idx] != coeffs2[idx] {
			t.Fatalf("coeff[%d]: %v != %v", idx, coeffs1[idx], coeffs2[idx])
		}
	}
}

func TestDesignerImprovesFitness(t *testing.T) {
	// Verify that optimized coefficients produce lower peak weighted energy
	// than zero coefficients (flat response).
	designer, err := NewDesigner(44100,
		WithOrder(5),
		WithIterations(500),
		WithSeed(42),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	coeffs, err := designer.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Evaluate flat (zero coefficients) vs optimized.
	flatScore := designer.evaluate(make([]float64, 5))
	optimizedScore := designer.evaluate(coeffs)

	t.Logf("flat score=%g, optimized score=%g, improvement=%gx",
		flatScore, optimizedScore, flatScore/optimizedScore)

	if optimizedScore >= flatScore {
		t.Errorf("optimized score (%g) should be < flat score (%g)", optimizedScore, flatScore)
	}
}

func TestDesignerNilOption(t *testing.T) {
	_, err := NewDesigner(44100, nil, WithOrder(5), nil)
	if err != nil {
		t.Fatal(err)
	}
}

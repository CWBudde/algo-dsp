package hilbert

import (
	"math"
	"testing"
)

func TestSetCoefficientsValidation32(t *testing.T) {
	p, err := New32Default()
	if err != nil {
		t.Fatalf("New32Default() error = %v", err)
	}

	if err := p.SetCoefficients(nil); err == nil {
		t.Fatal("expected error for empty coefficients")
	}

	if err := p.SetCoefficients([]float32{-1.2}); err == nil {
		t.Fatal("expected error for unstable coefficient")
	}
}

func TestProcessBlockMatchesSample32(t *testing.T) {
	pBlock, err := New32Default()
	if err != nil {
		t.Fatalf("New32Default() error = %v", err)
	}

	pSample, err := New32Default()
	if err != nil {
		t.Fatalf("New32Default() error = %v", err)
	}

	const n = 1024

	input := make([]float32, n)
	for i := range input {
		input[i] = float32(0.61*math.Sin(2*math.Pi*float64(i)/29.0) + 0.18*math.Sin(2*math.Pi*float64(i)/11.0))
	}

	gotA := make([]float32, n)

	gotB := make([]float32, n)
	if err := pBlock.ProcessBlock(input, gotA, gotB); err != nil {
		t.Fatalf("ProcessBlock() error = %v", err)
	}

	for i, x := range input {
		wantA, wantB := pSample.ProcessSample(x)
		if d := math.Abs(float64(gotA[i] - wantA)); d > 1e-6 {
			t.Fatalf("A[%d] mismatch: got=%g want=%g", i, gotA[i], wantA)
		}

		if d := math.Abs(float64(gotB[i] - wantB)); d > 1e-6 {
			t.Fatalf("B[%d] mismatch: got=%g want=%g", i, gotB[i], wantB)
		}
	}
}

func TestLegacyImpulseParity32(t *testing.T) {
	p, err := New32Default()
	if err != nil {
		t.Fatalf("New32Default() error = %v", err)
	}

	wantA := []float32{
		0.0014655919,
		0,
		-0.074359804,
		0,
		0.48791626,
		0,
		-0.6765628,
		0,
		-0.24756388,
		0,
		0.090437435,
		0,
		0.22179672,
		0,
		0.24060044,
		0,
	}
	wantB := []float32{
		0,
		0.015053541,
		0,
		-0.23033148,
		0,
		0.715875,
		0,
		-0.26744366,
		0,
		-0.4087834,
		0,
		-0.27647215,
		0,
		-0.12759046,
		0,
		-0.020833775,
	}

	for i := range wantA {
		in := float32(0)
		if i == 0 {
			in = 1
		}

		a, b := p.ProcessSample(in)
		if math.Abs(float64(a-wantA[i])) > 2e-6 {
			t.Fatalf("A[%d] = %.9f, want %.9f", i, a, wantA[i])
		}

		if math.Abs(float64(b-wantB[i])) > 2e-6 {
			t.Fatalf("B[%d] = %.9f, want %.9f", i, b, wantB[i])
		}
	}
}

func TestParity32Vs64(t *testing.T) {
	p64, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	p32, err := New32Default()
	if err != nil {
		t.Fatalf("New32Default() error = %v", err)
	}

	for i := range 2048 {
		x64 := 0.74*math.Sin(2*math.Pi*float64(i)/53.0) + 0.11*math.Sin(2*math.Pi*float64(i)/17.0)
		a64, b64 := p64.ProcessSample(x64)
		a32, b32 := p32.ProcessSample(float32(x64))

		if d := math.Abs(a64 - float64(a32)); d > 3e-5 {
			t.Fatalf("sample %d: A parity mismatch %g", i, d)
		}

		if d := math.Abs(b64 - float64(b32)); d > 3e-5 {
			t.Fatalf("sample %d: B parity mismatch %g", i, d)
		}
	}
}

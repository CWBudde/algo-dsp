package effects

import (
	"fmt"
	"math"
	"slices"
	"testing"
)

func TestChebyshevPowerCoefficients(t *testing.T) {
	cases := []struct {
		name          string
		weights, want []float64
	}{
		{"constant", []float64{-2}, []float64{-2}},
		{"negative third", []float64{0, 0, 0, -1}, []float64{0, 3, 0, -4}},
		{"positive fourth", []float64{0, 0, 0, 0, 2}, []float64{2, 0, -16, 0, 16}},
		{"mixed", []float64{.25, -.625, .5, -.25}, []float64{-.25, .125, 1, -1}},
		{"zero", []float64{0, 0, 0}, []float64{0, 0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dst := make([]float64, len(tc.weights)+1)
			dst[len(tc.weights)] = 17

			if err := ChebyshevPowerCoefficients(dst, tc.weights); err != nil {
				t.Fatal(err)
			}

			if !slices.Equal(dst[:len(tc.want)], tc.want) || dst[len(tc.want)] != 17 {
				t.Fatalf("coefficients = %v, want %v followed by 17", dst, tc.want)
			}
		})
	}

	weights, dst := make([]float64, 25), make([]float64, 25)
	for n := 1; n < len(weights); n++ {
		weights[n] = -1
		if err := ChebyshevPowerCoefficients(dst[:n+1], weights[:n+1]); err != nil {
			t.Fatal(err)
		}

		for _, x := range []float64{-1.25, -1, 0, 1, 1.25} {
			a, b := 1.0, x
			for i := 1; i < n; i++ {
				a, b = b, 2*x*b-a
			}

			got := EvaluatePowerPolynomial(dst[:n+1], x)
			if math.Abs(got+b) > 1e-12 {
				t.Errorf("-T%d(%g) = %g, want %g", n, x, got, -b)
			}
		}

		weights[n] = 0
	}

	if got := EvaluatePowerPolynomial(nil, 17); got != 0 {
		t.Fatalf("empty polynomial = %g, want 0", got)
	}
}

func TestChebyshevPowerOverlapAndInvalidWrites(t *testing.T) {
	backing := []float64{.25, -.625, .5, -.25, 19}
	if err := ChebyshevPowerCoefficients(backing[1:], backing[:4]); err != nil {
		t.Fatal(err)
	}

	if want := []float64{.25, -.25, .125, 1, -1}; !slices.Equal(backing, want) {
		t.Fatalf("overlapping transform = %v, want %v", backing, want)
	}

	dst := make([]float64, 25)
	for i := range dst {
		dst[i] = float64(i + 1)
	}

	before := slices.Clone(dst)
	overflow := make([]float64, 25)
	overflow[24] = math.MaxFloat64

	for _, w := range [][]float64{nil, make([]float64, 26), {1, math.NaN()}, {math.Inf(-1)}, overflow} {
		if err := ChebyshevPowerCoefficients(dst, w); err == nil || !slices.Equal(dst, before) {
			t.Fatalf("invalid weights %v wrote to destination or returned no error", w)
		}
	}

	if err := ChebyshevPowerCoefficients(dst[:1], []float64{1, 0}); err == nil || !slices.Equal(dst, before) {
		t.Fatal("short destination changed or returned no error")
	}
}

func TestChebyshevPowerNoAllocations(t *testing.T) {
	weights, dst := [25]float64{0, .7, .25, -.2}, [25]float64{}
	weights[24] = -.1

	allocs := testing.AllocsPerRun(10, func() {
		if err := ChebyshevPowerCoefficients(dst[:], weights[:]); err != nil {
			t.Fatal(err)
		}

		if EvaluatePowerPolynomial(dst[:], 1) == 0 {
			t.Fatal("expected a nonzero endpoint")
		}
	})
	if allocs != 0 {
		t.Fatalf("allocations = %g, want zero", allocs)
	}
}

func ExampleChebyshevPowerCoefficients() {
	weights := []float64{0, 0.5, -0.25}
	coefficients := make([]float64, len(weights))

	if err := ChebyshevPowerCoefficients(coefficients, weights); err != nil {
		panic(err)
	}

	fmt.Println(coefficients)
	// Output: [0.25 0.5 -0.5]
}

func ExampleEvaluatePowerPolynomial() {
	fmt.Println(EvaluatePowerPolynomial([]float64{0.25, 0.5, -0.5}, 1.5))
	// Output: -0.125
}

func BenchmarkChebyshevPowerCoefficients(b *testing.B) {
	var weights, dst [25]float64
	for i := range weights {
		weights[i] = 1 / float64(i+1)
	}

	b.ReportAllocs()

	for b.Loop() {
		if err := ChebyshevPowerCoefficients(dst[:], weights[:]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluatePowerPolynomial(b *testing.B) {
	var coefficients [25]float64
	for i := range coefficients {
		coefficients[i] = 1 / float64(i+1)
	}

	b.ReportAllocs()

	for b.Loop() {
		EvaluatePowerPolynomial(coefficients[:], .75)
	}
}

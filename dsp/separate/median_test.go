package separate_test

import (
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/separate"
)

// naiveMedian is the reference: for every sample, gather the window with
// scipy "reflect" indexing, sort it and take the middle element.
func naiveMedian(src []float64, size int) []float64 {
	n := len(src)
	out := make([]float64, n)
	win := make([]float64, size)

	reflect := func(j int) int {
		// Walk the mirror step by step (independent of the modulo form
		// used by the implementation).
		for j < 0 || j >= n {
			if j < 0 {
				j = -j - 1
			} else {
				j = 2*n - 1 - j
			}
		}

		return j
	}

	for i := range src {
		for w := range size {
			win[w] = src[reflect(i-size/2+w)]
		}

		slices.Sort(win)
		out[i] = win[size/2]
	}

	return out
}

func TestMedianFilterMatchesNaive(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(1, 2))

	for _, size := range []int{1, 3, 5, 17, 31} {
		for _, n := range []int{0, 1, 2, 3, 8, 16, 17, 40, 257} {
			for _, quantized := range []bool{false, true} {
				src := make([]float64, n)
				for i := range src {
					v := rng.NormFloat64()
					if quantized {
						// Many duplicates, signed zeros and infinities.
						v = math.Round(v * 2)

						switch rng.IntN(10) {
						case 0:
							v = math.Inf(1)
						case 1:
							v = math.Inf(-1)
						case 2:
							v = math.Copysign(0, -1)
						}
					}

					src[i] = v
				}

				m, err := separate.NewMedianFilter(size)
				if err != nil {
					t.Fatal(err)
				}

				dst := make([]float64, n)

				err = m.Filter(dst, src)
				if err != nil {
					t.Fatal(err)
				}

				want := naiveMedian(src, size)
				for i := range want {
					if dst[i] != want[i] {
						t.Fatalf("size %d n %d quantized %v: dst[%d] = %v, want %v", size, n, quantized, i, dst[i], want[i])
					}
				}

				// Reuse must give the same result.
				err = m.Filter(dst, src)
				if err != nil {
					t.Fatal(err)
				}

				for i := range want {
					if dst[i] != want[i] {
						t.Fatalf("reuse: size %d n %d: dst[%d] = %v, want %v", size, n, i, dst[i], want[i])
					}
				}
			}
		}
	}
}

func TestMedianFilterEdges(t *testing.T) {
	t.Parallel()

	m, err := separate.NewMedianFilter(3)
	if err != nil {
		t.Fatal(err)
	}

	if m.Size() != 3 {
		t.Fatalf("Size() = %d", m.Size())
	}

	// Reflect padding repeats the edge sample: the first window is
	// {x0, x0, x1}, so a step at the edge survives.
	src := []float64{5, 0, 0, 0, 9}
	dst := make([]float64, len(src))

	err = m.Filter(dst, src)
	if err != nil {
		t.Fatal(err)
	}

	want := []float64{5, 0, 0, 0, 9}
	if !slices.Equal(dst, want) {
		t.Fatalf("got %v, want %v", dst, want)
	}

	// A steady value keeps its level up to the edges.
	src = []float64{2, 2, 2, 2}
	m5, _ := separate.NewMedianFilter(5)

	err = m5.Filter(dst[:4], src)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(dst[:4], src) {
		t.Fatalf("got %v, want %v", dst[:4], src)
	}
}

func TestMedianFilterErrors(t *testing.T) {
	t.Parallel()

	for _, size := range []int{0, -1, 2, 16} {
		_, err := separate.NewMedianFilter(size)
		if !errors.Is(err, separate.ErrInvalidKernel) {
			t.Errorf("size %d: got %v, want ErrInvalidKernel", size, err)
		}
	}

	m, err := separate.NewMedianFilter(3)
	if err != nil {
		t.Fatal(err)
	}

	err = m.Filter(make([]float64, 2), make([]float64, 3))
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("short dst: got %v", err)
	}

	err = m.Filter(make([]float64, 3), []float64{1, math.NaN(), 2})
	if !errors.Is(err, separate.ErrInvalidValue) {
		t.Errorf("NaN: got %v", err)
	}
}

func TestMedianFilterNoAllocs(t *testing.T) {
	m, err := separate.NewMedianFilter(17)
	if err != nil {
		t.Fatal(err)
	}

	src := make([]float64, 1025)
	for i := range src {
		src[i] = math.Sin(float64(i))
	}

	dst := make([]float64, len(src))

	allocs := testing.AllocsPerRun(10, func() {
		_ = m.Filter(dst, src)
	})
	if allocs != 0 {
		t.Fatalf("Filter allocates %v times per run", allocs)
	}
}

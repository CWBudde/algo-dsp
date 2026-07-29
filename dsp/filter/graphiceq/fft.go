package graphiceq

import (
	"fmt"

	algofft "github.com/cwbudde/algo-fft"
)

type fftWorkspace struct {
	size int
	plan *algofft.Plan[complex128]
}

func newFFTWorkspace(size int) (*fftWorkspace, error) {
	plan, err := algofft.NewPlan64(size)
	if err != nil {
		return nil, fmt.Errorf("graphiceq: create FFT plan: %w", err)
	}

	return &fftWorkspace{size: size, plan: plan}, nil
}

func (w *fftWorkspace) forwardReal(input []float64) ([]complex128, error) {
	src := make([]complex128, w.size)
	for i := 0; i < len(input) && i < w.size; i++ {
		src[i] = complex(input[i], 0)
	}

	dst := make([]complex128, w.size)
	if err := w.plan.Forward(dst, src); err != nil {
		return nil, fmt.Errorf("graphiceq: forward FFT: %w", err)
	}

	return dst, nil
}

func (w *fftWorkspace) inverseReal(input []complex128) ([]float64, error) {
	src := make([]complex128, w.size)
	copy(src, input)

	dst := make([]complex128, w.size)
	if err := w.plan.Inverse(dst, src); err != nil {
		return nil, fmt.Errorf("graphiceq: inverse FFT: %w", err)
	}

	out := make([]float64, w.size)
	for i := range out {
		out[i] = real(dst[i])
	}

	return out, nil
}

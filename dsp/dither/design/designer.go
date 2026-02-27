package design

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
)

const (
	defaultOrder      = 8
	defaultIterations = 10000
	fftSize           = 64
	fftSizeHalf       = fftSize / 2
	ntryMax           = 20
)

// ProgressFunc is called when the optimizer finds a new best coefficient set.
// The coeffs slice is a copy safe to retain. The score is log10 of the
// ATH-weighted peak magnitude (lower is better).
type ProgressFunc func(coeffs []float64, score float64)

// DesignerOption configures a [Designer].
type DesignerOption func(*designerConfig) error

type designerConfig struct {
	order      int
	iterations int
	seed       uint64
	hasSeed    bool
	onProgress ProgressFunc
}

// WithOrder sets the number of FIR coefficients to optimize (default 8).
func WithOrder(order int) DesignerOption {
	return func(cfg *designerConfig) error {
		if order < 1 {
			return fmt.Errorf("design: order must be >= 1: %d", order)
		}

		cfg.order = order

		return nil
	}
}

// WithIterations sets the number of perturbation steps per outer loop (default 10000).
func WithIterations(iterations int) DesignerOption {
	return func(cfg *designerConfig) error {
		if iterations < 1 {
			return fmt.Errorf("design: iterations must be >= 1: %d", iterations)
		}

		cfg.iterations = iterations

		return nil
	}
}

// WithOnProgress sets a callback invoked when a new best coefficient set is found.
func WithOnProgress(fn ProgressFunc) DesignerOption {
	return func(cfg *designerConfig) error {
		cfg.onProgress = fn
		return nil
	}
}

// WithSeed sets a fixed RNG seed for deterministic results.
func WithSeed(seed uint64) DesignerOption {
	return func(cfg *designerConfig) error {
		cfg.seed = seed
		cfg.hasSeed = true

		return nil
	}
}

// Designer finds optimal FIR noise-shaping coefficients using a stochastic
// search weighted by the absolute threshold of hearing.
type Designer struct {
	sampleRate float64
	order      int
	iterations int
	rng        *rand.Rand
	onProgress ProgressFunc
	iath       [fftSizeHalf]float64
}

// NewDesigner creates a new coefficient optimizer for the given sample rate.
func NewDesigner(sampleRate float64, opts ...DesignerOption) (*Designer, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("design: sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := designerConfig{
		order:      defaultOrder,
		iterations: defaultIterations,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	var rng *rand.Rand
	if cfg.hasSeed {
		rng = rand.New(rand.NewPCG(cfg.seed, 0))
	} else {
		rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}

	designer := &Designer{
		sampleRate: sampleRate,
		order:      cfg.order,
		iterations: cfg.iterations,
		rng:        rng,
		onProgress: cfg.onProgress,
	}

	// Build inverse ATH weight table.
	for bin := range fftSizeHalf {
		freq := sampleRate * float64(bin) / float64(fftSize)
		athLinear := math.Pow(10, ATH(freq)*0.1) / CriticalBandwidth(freq)
		designer.iath[bin] = 1.0 / athLinear
	}

	return designer, nil
}

// Run executes the optimizer until ctx is cancelled or the search converges.
// Returns the best coefficient set found.
func (d *Designer) Run(ctx context.Context) ([]float64, error) {
	best := make([]float64, d.order)
	candidate := make([]float64, d.order)
	bestGlobal := make([]float64, d.order)
	bestGlobalScore := math.MaxFloat64

	iterRecip := 1.0 / float64(d.iterations)

	for {
		// Check for cancellation at the top of each outer loop.
		select {
		case <-ctx.Done():
			return d.copyCoeffs(bestGlobal), nil
		default:
		}

		retryCount := 0
		lastBestScore := math.MaxFloat64
		bestScore := math.MaxFloat64

		for retryCount == 0 || (lastBestScore < bestScore && retryCount < ntryMax) {
			for step := range d.iterations {
				// Check cancellation periodically.
				if step%1000 == 0 {
					select {
					case <-ctx.Done():
						return d.copyCoeffs(bestGlobal), nil
					default:
					}
				}

				// Perturb best coefficients with simulated annealing.
				anneal := float64(d.iterations-step) * iterRecip

				for coeff := range d.order {
					candidate[coeff] = best[coeff]
					if d.rng.IntN(2) == 0 {
						candidate[coeff] += (d.rng.Float64() - 0.5) * anneal
					}
				}

				// Evaluate fitness (ATH-weighted peak spectral energy).
				score := d.evaluate(candidate)
				if score < bestScore {
					bestScore = score

					copy(best, candidate)
				}
			}

			retryCount++
		}

		// Update global best.
		if bestScore < bestGlobalScore {
			bestGlobalScore = bestScore

			copy(bestGlobal, best)

			if d.onProgress != nil {
				d.onProgress(d.copyCoeffs(bestGlobal), math.Log10(bestGlobalScore))
			}
		}

		// Reset if retries exhausted without improvement.
		if retryCount >= ntryMax {
			lastBestScore = math.MaxFloat64

			continue
		}

		lastBestScore = bestScore
	}
}

// evaluate computes the maximum ATH-weighted spectral energy for the given
// coefficient set. Uses a direct DFT since the size (64) is small enough.
func (d *Designer) evaluate(coeffs []float64) float64 {
	var peak float64

	for bin := range fftSizeHalf {
		// Compute DFT bin. Impulse response is [1, coeffs[0], coeffs[1], ...].
		omega := 2 * math.Pi * float64(bin) / float64(fftSize)

		re := 1.0

		var im float64

		for idx, coeff := range coeffs {
			angle := omega * float64(idx+1)
			re += coeff * math.Cos(angle)
			im -= coeff * math.Sin(angle)
		}

		// Squared magnitude weighted by inverse ATH.
		weighted := (re*re + im*im) * d.iath[bin]
		if weighted > peak {
			peak = weighted
		}
	}

	return peak
}

func (d *Designer) copyCoeffs(src []float64) []float64 {
	out := make([]float64, len(src))
	copy(out, src)

	return out
}

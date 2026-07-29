package mixedphase

import "fmt"

// MinimumPhase reconstructs a causal minimum-phase FIR with the prototype's
// sampled magnitude response using a real-cepstrum lifter.
//
// fftSize may be zero to select a dense power-of-two grid automatically. The
// returned FIR has len(prototype) taps; truncating the dense reconstruction can
// introduce a small magnitude error.
//
// Use [MinimumPhaseWith] to select a different reconstruction method.
func MinimumPhase(prototype []float64, fftSize int) ([]float64, error) {
	return MinimumPhaseWith(prototype, MinimumPhaseConfig{FFTSize: fftSize})
}

// MinimumPhaseWith reconstructs a causal minimum-phase FIR with the prototype's
// sampled magnitude response using the configured method.
//
// The returned FIR has len(prototype) taps; truncating the dense
// reconstruction can introduce a small magnitude error.
func MinimumPhaseWith(
	prototype []float64,
	cfg MinimumPhaseConfig,
) ([]float64, error) {
	if len(prototype) == 0 {
		return nil, ErrEmptyPrototype
	}

	if cfg.Epsilon < 0 {
		return nil, fmt.Errorf("%w: got %g", ErrInvalidEpsilon, cfg.Epsilon)
	}

	if !cfg.Method.valid() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidMethod, int(cfg.Method))
	}

	size, err := nextDesignFFTSize(len(prototype), cfg.FFTSize)
	if err != nil {
		return nil, err
	}

	w, err := newFFTWorkspace(size)
	if err != nil {
		return nil, err
	}

	targetSpectrum, err := w.forwardReal(prototype)
	if err != nil {
		return nil, err
	}

	targetMagnitude := magnitude(targetSpectrum)
	epsilon := defaultEpsilon(targetMagnitude, cfg.Epsilon)

	minimumSpectrum, err := minimumPhaseSpectrum(
		w,
		targetMagnitude,
		epsilon,
		cfg.Method,
	)
	if err != nil {
		return nil, fmt.Errorf("mixedphase: reconstruct minimum phase: %w", err)
	}

	impulse, err := w.inverseReal(minimumSpectrum)
	if err != nil {
		return nil, err
	}

	return append([]float64(nil), impulse[:len(prototype)]...), nil
}

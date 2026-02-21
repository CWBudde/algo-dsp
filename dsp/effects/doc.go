// Package effects provides reusable non-I/O DSP effect kernels.
//
// Subpackages:
//   - github.com/cwbudde/algo-dsp/dsp/effects/dynamics
//   - github.com/cwbudde/algo-dsp/dsp/effects/modulation
//   - github.com/cwbudde/algo-dsp/dsp/effects/pitch
//   - github.com/cwbudde/algo-dsp/dsp/effects/reverb
//
// Effects remaining in this package:
//   - Delay: Feedback delay with dry/wet mix.
//   - HarmonicBass: Psychoacoustic bass enhancer with harmonic generation.
//
// All effects are designed for real-time processing with zero-allocation
// hot paths and support both single-sample and buffer-based processing.
package effects

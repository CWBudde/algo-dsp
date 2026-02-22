// Package effects provides reusable non-I/O DSP effect kernels.
//
// Subpackages:
//   - github.com/cwbudde/algo-dsp/dsp/effects/dynamics
//   - github.com/cwbudde/algo-dsp/dsp/effects/modulation
//   - github.com/cwbudde/algo-dsp/dsp/effects/pitch
//   - github.com/cwbudde/algo-dsp/dsp/effects/reverb
//
// Effects remaining in this package:
//   - BitCrusher: Sample rate and bit-depth reduction for lo-fi aesthetics.
//   - Delay: Feedback delay with dry/wet mix.
//   - Distortion: Configurable clipping/waveshaping and Chebyshev harmonics.
//   - HarmonicBass: Psychoacoustic bass enhancer with harmonic generation.
//   - TransformerSimulation: Transformer-style saturation with HQ/light modes.
//   - SpectralFreeze: STFT-based magnitude hold with selectable phase strategy.
//
// All effects are designed for real-time processing with zero-allocation
// hot paths and support both single-sample and buffer-based processing.
package effects

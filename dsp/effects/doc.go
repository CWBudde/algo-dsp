// Package effects provides reusable non-I/O DSP effect kernels.
//
// Dynamics processors are provided in the subpackage:
//   - github.com/cwbudde/algo-dsp/dsp/effects/dynamics
//
// Time-based effects:
//   - Chorus: Modulated delay effect for ensemble sounds.
//   - Delay: Feedback delay with dry/wet mix.
//   - Flanger: Short modulated delay with feedback for comb-filter sweeps.
//   - Phaser: Allpass-cascade modulation effect with feedback.
//   - Tremolo: LFO amplitude modulation with optional smoothing.
//   - PitchShifter: Time-domain WSOLA-style pitch shifting.
//   - Reverb: Algorithmic reverb using Schroeder/Freeverb architecture.
//   - FDNReverb: Feedback delay network reverb with modulation and damping.
//
// Spectral/psychoacoustic effects:
//   - HarmonicBass: Psychoacoustic bass enhancer with harmonic generation.
//   - SpectralPitchShifter: Frequency-domain phase-vocoder pitch shifter.
//
// Shared abstractions:
//   - PitchProcessor: Common interface for interchangeable pitch shifters.
//
// All effects are designed for real-time processing with zero-allocation
// hot paths and support both single-sample and buffer-based processing.
package effects

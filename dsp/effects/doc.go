// Package effects provides reusable non-I/O DSP effect kernels,
// including chorus, reverb, and dynamics processors.
//
// Dynamics processors:
//   - Compressor: Soft-knee compressor with log2-domain gain calculation
//     for smooth compression curves and transparent dynamic range control.
//
// Time-based effects:
//   - Chorus: Modulated delay effect for ensemble sounds.
//   - Reverb: Algorithmic reverb using Schroeder/Freeverb architecture.
//
// All effects are designed for real-time processing with zero-allocation
// hot paths and support both single-sample and buffer-based processing.
package effects

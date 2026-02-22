// Package dynamics provides reusable non-I/O dynamics processors.
//
// Included processors:
//   - Compressor: Soft-knee compressor with log2-domain gain computation.
//   - Expander: Downward expander with soft-knee and range control.
//   - MultibandCompressor: Multiband compressor using Linkwitz-Riley crossovers
//     with adjustable order and per-band soft-knee compression.
//   - DeEsser: Split-band sibilance detector and reducer.
//   - Gate: Soft-knee noise gate with hold support.
//   - Limiter: Peak limiter built on a high-ratio compressor.
//   - LookaheadLimiter: Limiter with delayed program path and optional
//     sidechain detector input.
//   - TransientShaper: Attack/release transient splitting with independent
//     attack and sustain shaping controls.
package dynamics

// Package dynamics provides reusable non-I/O dynamics processors.
//
// Included processors:
//   - Compressor: Soft-knee compressor with log2-domain gain computation.
//   - MultibandCompressor: Multiband compressor using Linkwitz-Riley crossovers
//     with adjustable order and per-band soft-knee compression.
//   - DeEsser: Split-band sibilance detector and reducer.
//   - Gate: Soft-knee noise gate with hold support.
//   - Limiter: Peak limiter built on a high-ratio compressor.
package dynamics

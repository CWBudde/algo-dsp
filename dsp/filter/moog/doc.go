// Package moog provides nonlinear Moog ladder low-pass filters with
// legacy-faithful, Huovilainen-style, and Zero-Delay Feedback variants.
//
// Supported variants:
//   - VariantClassic / VariantImprovedClassic:
//     Legacy DAV_DspFilterMoog-inspired topology with exact tanh.
//   - VariantClassicLightweight / VariantImprovedClassicLightweight:
//     Same topology with polynomial tanh approximation.
//   - VariantHuovilainen:
//     Huovilainen-style tuning/resonance compensation with half-sample
//     feedback estimate for robust high-resonance behavior.
//   - VariantZDF:
//     Zero-Delay Feedback topology (Zavalishin 2012) with Newton-Raphson
//     iteration (D'Angelo & Välimäki 2014). Provides the most accurate
//     cutoff tuning and self-oscillation behavior at higher CPU cost.
//     Newton iterations are configurable via WithNewtonIterations (default 4).
//
// All variants are stateful, deterministic, and support:
//   - Per-sample and in-place block processing
//   - Explicit state save/restore via State
//   - Optional oversampled anti-alias processing for nonlinear drive
//   - Stereo helper with per-channel independent state
package moog

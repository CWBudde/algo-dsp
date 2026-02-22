// Package moog provides nonlinear Moog ladder low-pass filters with
// legacy-faithful and Huovilainen-style variants.
//
// Supported variants:
//   - VariantClassic / VariantImprovedClassic:
//     Legacy DAV_DspFilterMoog-inspired topology with exact tanh.
//   - VariantClassicLightweight / VariantImprovedClassicLightweight:
//     Same topology with polynomial tanh approximation.
//   - VariantHuovilainen:
//     Huovilainen-style tuning/resonance compensation with half-sample
//     feedback estimate for robust high-resonance behavior.
//
// All variants are stateful, deterministic, and support:
//   - Per-sample and in-place block processing
//   - Explicit state save/restore via State
//   - Optional oversampled anti-alias processing for nonlinear drive
//   - Stereo helper with per-channel independent state
package moog

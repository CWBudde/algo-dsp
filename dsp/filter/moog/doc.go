// Package moog provides nonlinear Moog ladder filter runtime implementations.
//
// A [Filter] is a classic resonant lowpass: four cascaded one-pole sections
// (24 dB/oct) with a tanh nonlinearity per stage and a negative feedback path
// from the fourth stage output that produces the characteristic resonant peak
// and self-oscillation. The implementation is a faithful Go port of the legacy
// Pascal reference (DAV_DspFilterMoog.pas) and follows the Huovilainen model:
//
//	A. Huovilainen, "Non-Linear Digital Implementation of the Moog Ladder
//	Filter", Proc. Int. Conf. on Digital Audio Effects (DAFx-04), 2004.
//
// # Variants
//
// Two ladder topologies are available via [WithVariant]:
//
//	SimpleClassic   - per-stage update scaled by the base coefficient g.
//	ImprovedClassic - per-stage update additionally scaled by 2*thermalVoltage,
//	                  giving a stronger drive into the saturating stages.
//
// Either variant can use the exact math.Tanh or a lower-latency polynomial-2^x
// approximation ([WithFastTanh]); the approximation reduces the per-sample cost
// of the tanh nonlinearity in the feedback-bound processing path at a small
// accuracy cost.
//
// # Parameters
//
// The core exposes the legacy parameters directly: cutoff (Hz), a raw
// resonance feedback amount, a thermal-voltage shaping control, and an output
// gain. For musical use, [Filter.SetResonanceNormalized] maps a [0, 1] control
// onto the feedback amount, where 1.0 corresponds to the classic
// self-oscillation onset.
//
// Coefficients are recomputed at construction and whenever cutoff, resonance,
// thermal voltage, or sample rate change.
//
// Example:
//
//	f, _ := moog.New(1000, 48000, moog.WithResonance(2.5)) // 1 kHz, resonant
//	y := f.ProcessSample(x)
package moog

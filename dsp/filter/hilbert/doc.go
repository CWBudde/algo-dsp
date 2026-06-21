// Package hilbert provides a polyphase half-pi Hilbert transformer based on
// the HIIR-style allpass/polyphase structure used in DAV_DspPolyphaseHilbert.
//
// The package exposes stateful 64-bit and 32-bit processors for streaming and
// block processing, plus helpers for analytic-signal envelope extraction.
//
// Coefficients can be designed with [DesignCoefficients] or supplied directly.
// Designed defaults match the legacy workflow: 8 coefficients and 0.1
// normalized transition bandwidth.
// Presets [PresetFast], [PresetBalanced], and [PresetLowFrequency] provide
// quick trade-offs between CPU cost and low-frequency quadrature accuracy.
package hilbert

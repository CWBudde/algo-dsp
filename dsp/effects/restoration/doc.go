// Package restoration provides bounded spectral restoration, noise profiles,
// short-gap interpolation and harmonic hum filtering. Audio and transport are
// supplied by the caller; no complete document is retained.
//
// Profile capture and per-bin gating follow the Noise Reduction and Spectral
// Noise Gate Delphi examples (DAV_DspSpectralNoiseReduction.pas): averaged bin
// powers, separate DC/Nyquist handling and smoothed gain. The legacy DC capture
// typo (filter power instead of input power) is deliberately corrected. Wiener
// mode adds decision-directed SNR estimation; frequency and temporal smoothing
// reduce isolated musical-noise peaks. These are deterministic algorithms, not
// a guarantee of perceptual transparency on arbitrary recordings.
package restoration

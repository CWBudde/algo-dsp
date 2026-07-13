// Package thd provides total harmonic distortion calculation kernels.
//
// All aggregate metrics (THD, THD+N, noise, odd/even harmonic distortion,
// rub & buzz) follow the IEEE root-sum-square convention: components are
// summed in the power domain and reported as amplitude ratios relative to
// the fundamental, i.e. THD = sqrt(sum |H_k|^2) / |H_1|. SINAD is the
// power ratio of the fundamental to everything else in the analysis range,
// expressed in dB (SINAD = -THDN_dB).
package thd

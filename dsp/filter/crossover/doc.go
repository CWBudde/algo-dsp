// Package crossover provides Linkwitz-Riley crossover networks for splitting
// an audio signal into frequency bands.
//
// A crossover network divides a signal into complementary lowpass and highpass
// outputs (or multiple bands for higher-order networks) such that their sum
// reconstructs the original signal with flat magnitude response.
//
// The [Crossover] type implements a two-way (LP + HP) Linkwitz-Riley crossover
// of arbitrary even order. The [MultiBand] type chains multiple two-way
// crossovers to split a signal into three or more bands.
//
// Linkwitz-Riley filters are constructed by cascading two identical Butterworth
// filters. An LR-2N crossover uses order-N Butterworth prototypes, producing
// -6.02 dB at the crossover frequency and allpass summation.
//
// Example:
//
//	xo, _ := crossover.New(1000, 4, 48000) // LR4 at 1 kHz
//	lo, hi := xo.ProcessSample(inputSample)
//	sum := lo + hi // ≈ allpass-filtered input
package crossover

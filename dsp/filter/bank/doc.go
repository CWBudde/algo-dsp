// Package bank provides octave and fractional-octave filter bank builders.
//
// A filter bank is a collection of bandpass filters that partition the
// audio spectrum into frequency bands. Each band is implemented as a
// cascade of a Butterworth lowpass and highpass filter, forming a bandpass
// response around its center frequency.
//
// The package supports two construction modes:
//
//   - [Octave] builds standard octave or fractional-octave (1/3, 1/6, etc.)
//     filter banks with center frequencies per IEC 61260 (base-10 system).
//   - [Custom] builds a bank from arbitrary center frequencies and a
//     specified bandwidth in octaves.
//   - [NewOctaveAnalyzer] builds a streaming analyzer that applies the
//     same band definitions with envelope smoothing and optional downsampling.
//
// Band edge frequencies follow the IEC 61260 standard:
//
//	G = 10^(3/10)              (octave ratio)
//	f_center = 1000 * G^(k/N)  (for 1/N-octave, integer k)
//	f_upper  = f_center * G^(1/(2*N))
//	f_lower  = f_center * G^(-1/(2*N))
//
// Each band's lowpass filter is set at f_upper and the highpass filter
// at f_lower, both using Butterworth topology for maximally flat passband.
//
// Basic usage:
//
//	b := bank.Octave(1, 48000)  // full-octave bank, 48 kHz sample rate
//	outputs := b.ProcessSample(sample)
//	for i, band := range b.Bands() {
//	    fmt.Printf("%.0f Hz: %f\n", band.CenterFreq, outputs[i])
//	}
package bank

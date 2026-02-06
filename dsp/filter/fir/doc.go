// Package fir provides a direct-form FIR filter runtime.
//
// A [Filter] applies a set of pre-computed coefficients to an input stream
// using a circular-buffer delay line. It is suitable for short filters
// (order < ~256). For long FIR filters, consider using FFT-based partitioned
// convolution (dsp/conv, Phase 7).
//
// This package provides the processing runtime only. Coefficient design
// (windowed-sinc, Parks-McClellan, etc.) is a separate concern.
package fir

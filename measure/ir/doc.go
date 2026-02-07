// Package ir provides impulse response analysis metrics for room acoustics
// and system characterization.
//
// The package implements standard ISO 3382 room acoustic parameters
// derived from the Schroeder backward integration of squared impulse responses:
//
//   - RT60: Reverberation time (time for -60 dB decay)
//   - EDT: Early Decay Time (extrapolated from 0 to -10 dB)
//   - T20, T30: Reverberation time from -5 to -25 dB and -5 to -35 dB
//   - C50, C80: Clarity (early-to-late energy ratio at 50ms and 80ms)
//   - D50, D80: Definition (early energy fraction at 50ms and 80ms)
//   - Center Time: Temporal energy centroid
//
// # Usage
//
//	analyzer := ir.NewAnalyzer(48000) // sample rate
//	metrics, err := analyzer.Analyze(impulseResponse)
//	fmt.Printf("RT60 = %.2f s, C80 = %.1f dB\n", metrics.RT60, metrics.C80)
package ir

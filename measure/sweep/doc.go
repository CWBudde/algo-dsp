// Package sweep provides logarithmic and linear sweep generation
// with inverse filter construction and deconvolution for impulse
// response measurement.
//
// A logarithmic sweep is the preferred excitation signal for measuring
// impulse responses of acoustic systems. Its key properties:
//
//   - Each octave takes equal time, giving uniform SNR across frequency
//   - Harmonic distortion products separate cleanly in time after deconvolution
//   - The inverse filter is analytically known (time-reversed + amplitude compensation)
//
// # Usage
//
// Generate a sweep, record the system response, and deconvolve:
//
//	s := &sweep.LogSweep{
//	    StartFreq: 20, EndFreq: 20000,
//	    Duration: 5, SampleRate: 48000,
//	}
//	excitation, _ := s.Generate()
//	// ... play excitation through system, record response ...
//	ir, _ := s.Deconvolve(response)
//
// For nonlinear system analysis, extract harmonic IRs:
//
//	harmonicIRs, _ := s.ExtractHarmonicIRs(response, 5)
//	// harmonicIRs[0] = linear IR, [1] = H2, [2] = H3, etc.
package sweep

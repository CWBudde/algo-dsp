// Package resample provides rational sample-rate conversion using polyphase FIR
// filtering with anti-aliasing defaults.
//
// Quality modes:
//   - QualityFast: lower CPU, lower attenuation
//   - QualityBalanced: default mode
//   - QualityBest: higher attenuation and flatter passband
//
// Default quality/performance matrix:
//
//	mode            taps/phase   nominal stopband
//	QualityFast     16           ~55 dB
//	QualityBalanced 32           ~75 dB
//	QualityBest     64           ~90 dB
//
// Common workflows:
//   - NewRational(up, down, opts...)
//   - NewForRates(inRate, outRate, opts...)
//   - Resample(input, up, down, opts...)
//   - ResampleAligned / Resampler.ProcessAligned for whole signals with the
//     filter group delay removed (input time t maps to output time t)
//   - Upsample2x / Downsample2x convenience wrappers
package resample

package design

import "math"

// ATH returns the absolute threshold of hearing in dB SPL at the given
// frequency in Hz. Based on Painter & Spanias (1997), modified by
// Gabriel Bouvigne to better fit empirical measurements.
func ATH(freqHz float64) float64 {
	// Convert to kHz with a minimum clamp to avoid singularity at 0.
	freq := math.Max(0.01, freqHz*0.001)

	return 3.640*math.Pow(freq, -0.8) -
		6.800*math.Exp(-0.6*(freq-3.4)*(freq-3.4)) +
		6.000*math.Exp(-0.15*(freq-8.7)*(freq-8.7)) +
		0.0006*freq*freq*freq*freq
}

// CriticalBandwidth returns the critical bandwidth in Hz at the given
// frequency in Hz, per Zwicker (Psychoakustik, 1982; ISBN 3-540-11401-7).
func CriticalBandwidth(freqHz float64) float64 {
	freq := freqHz * 0.001 // convert to kHz

	return 25 + 75*math.Pow(1+1.4*freq*freq, 0.69)
}

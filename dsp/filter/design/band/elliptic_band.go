package band

import (
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// EllipticBand designs a high-order Elliptic band filter for graphic EQ.
//
// gainDB is the desired center gain in dB. bandwidthHz is the band width in Hz.
// order must be an even integer greater than 2.
func EllipticBand(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	if gainDB == 0 {
		return passthroughSections(), nil
	}

	w0, wb, err := bandParams(sampleRate, f0Hz, bandwidthHz, order)
	if err != nil {
		return nil, err
	}

	gb := ellipticBWGainDB(gainDB)

	return ellipticBandRad(w0, wb, gainDB, gb, order)
}

// ellipticBWGainDB computes the bandwidth gain for Elliptic band filters.
func ellipticBWGainDB(gainDB float64) float64 {
	if gainDB < 0 {
		return gainDB + 0.05
	}

	return gainDB - 0.05
}

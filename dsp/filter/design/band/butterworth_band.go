package band

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ButterworthBand designs a high-order Butterworth band filter for graphic EQ.
//
// gainDB is the desired center gain in dB. bandwidthHz is the band width in Hz.
// order must be an even integer greater than 2.
func ButterworthBand(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	if gainDB == 0 {
		return passthroughSections(), nil
	}

	w0, wb, err := bandParams(sampleRate, f0Hz, bandwidthHz, order)
	if err != nil {
		return nil, err
	}

	gb := butterworthBWGainDB(gainDB)

	return butterworthBandRad(w0, wb, gainDB, gb, order)
}

// butterworthBWGainDB computes the bandwidth gain for Butterworth band filters.
func butterworthBWGainDB(gainDB float64) float64 {
	if gainDB < -3 {
		return gainDB + 3
	}

	if gainDB < 3 {
		return gainDB / math.Sqrt2
	}

	return gainDB - 3
}

// butterworthBandRad designs a Butterworth band filter using rad/sample parameters.
func butterworthBandRad(w0, wb, gainDB, gbDB float64, order int) ([]biquad.Coefficients, error) {
	G0 := 1.0 // db2Lin(0) is always exactly 1
	G := db2Lin(gainDB)
	Gb := db2Lin(gbDB)

	if G == 0 || Gb == 0 || G0 == 0 {
		return nil, ErrInvalidParams
	}

	if Gb*Gb == G0*G0 {
		return nil, ErrInvalidParams
	}

	e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	g := math.Pow(G, 1.0/float64(order))
	g0 := math.Pow(G0, 1.0/float64(order))
	beta := math.Pow(e, -1.0/float64(order)) * math.Tan(wb/2)
	c0 := math.Cos(w0)

	sections := make([]biquad.Coefficients, 0, order)

	L := order / 2
	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1) / float64(order)
		si := math.Sin(math.Pi * ui * 0.5)

		Di := beta*beta + 2*si*beta + 1
		if Di == 0 {
			return nil, ErrInvalidParams
		}

		B := [5]float64{
			(g*g*beta*beta + 2*g*g0*si*beta + g0*g0) / Di,
			-4 * c0 * (g0*g0 + g*g0*si*beta) / Di,
			2 * (g0*g0*(1+2*c0*c0) - g*g*beta*beta) / Di,
			-4 * c0 * (g0*g0 - g*g0*si*beta) / Di,
			(g*g*beta*beta - 2*g*g0*si*beta + g0*g0) / Di,
		}

		A := [5]float64{
			1,
			-4 * c0 * (1 + si*beta) / Di,
			2 * (1 + 2*c0*c0 - beta*beta) / Di,
			-4 * c0 * (1 - si*beta) / Di,
			(beta*beta - 2*si*beta + 1) / Di,
		}

		biquads, err := splitFOSection(B, A)
		if err != nil {
			return nil, err
		}

		sections = append(sections, biquads...)
	}

	return sections, nil
}

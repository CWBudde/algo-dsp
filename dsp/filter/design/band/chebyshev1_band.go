package band

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// Chebyshev1Band designs a high-order Chebyshev Type I band filter for graphic EQ.
//
// gainDB is the desired center gain in dB. bandwidthHz is the band width in Hz.
// order must be an even integer greater than 2.
func Chebyshev1Band(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	if gainDB == 0 {
		return passthroughSections(), nil
	}

	w0, wb, err := bandParams(sampleRate, f0Hz, bandwidthHz, order)
	if err != nil {
		return nil, err
	}

	gb := chebyshev1BWGainDB(gainDB)

	return chebyshev1BandRad(w0, wb, gainDB, gb, order)
}

// chebyshev1BWGainDB computes the bandwidth gain for Chebyshev Type I band filters.
func chebyshev1BWGainDB(gainDB float64) float64 {
	if gainDB < 0 {
		return gainDB + 0.1
	}

	return gainDB - 0.1
}

// chebyshev1BandRad designs a Chebyshev Type I band filter using rad/sample parameters.
func chebyshev1BandRad(w0, wb, gainDB, gbDB float64, order int) ([]biquad.Coefficients, error) {
	G0 := 1.0 // db2Lin(0) is always exactly 1
	G := db2Lin(gainDB)
	Gb := db2Lin(gbDB)

	if Gb*Gb == G0*G0 {
		return nil, ErrInvalidParams
	}

	e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	g0 := math.Pow(G0, 1.0/float64(order))
	alfa := math.Pow(1.0/e+math.Sqrt(1+math.Pow(e, -2.0)), 1.0/float64(order))
	beta := math.Pow(G/e+Gb*math.Sqrt(1+math.Pow(e, -2.0)), 1.0/float64(order))
	A := 0.5 * (alfa - 1.0/alfa)
	B := 0.5 * (beta - g0*g0*(1/beta))
	tb := math.Tan(wb * 0.5)
	c0 := math.Cos(w0)

	sections := make([]biquad.Coefficients, 0, order)

	L := order / 2
	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1.0) / float64(order)
		ci := math.Cos(math.Pi * ui * 0.5)
		si := math.Sin(math.Pi * ui * 0.5)

		Di := (A*A+ci*ci)*tb*tb + 2.0*A*si*tb + 1
		if Di == 0 {
			return nil, ErrInvalidParams
		}

		Bv := [5]float64{
			((B*B+g0*g0*ci*ci)*tb*tb + 2*g0*B*si*tb + g0*g0) / Di,
			-4 * c0 * (g0*g0 + g0*B*si*tb) / Di,
			2 * (g0*g0*(1+2*c0*c0) - (B*B+g0*g0*ci*ci)*tb*tb) / Di,
			-4 * c0 * (g0*g0 - g0*B*si*tb) / Di,
			((B*B+g0*g0*ci*ci)*tb*tb - 2*g0*B*si*tb + g0*g0) / Di,
		}

		Av := [5]float64{
			1,
			-4 * c0 * (1 + A*si*tb) / Di,
			2 * (1 + 2*c0*c0 - (A*A+ci*ci)*tb*tb) / Di,
			-4 * c0 * (1 - A*si*tb) / Di,
			((A*A+ci*ci)*tb*tb - 2*A*si*tb + 1) / Di,
		}

		biquads, err := splitFOSection(Bv, Av)
		if err != nil {
			return nil, err
		}

		sections = append(sections, biquads...)
	}

	return sections, nil
}

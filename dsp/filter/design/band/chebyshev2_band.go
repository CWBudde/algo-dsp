package band

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// Chebyshev2Band designs a high-order Chebyshev Type II band filter for graphic EQ.
//
// gainDB is the desired center gain in dB. bandwidthHz is the band width in Hz.
// order must be an even integer greater than 2.
func Chebyshev2Band(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	if gainDB == 0 {
		return passthroughSections(), nil
	}

	w0, wb, err := bandParams(sampleRate, f0Hz, bandwidthHz, order)
	if err != nil {
		return nil, err
	}

	gb := chebyshev2BWGainDB(gainDB)

	return chebyshev2BandRad(w0, wb, gainDB, gb, order)
}

// chebyshev2BWGainDB computes the bandwidth gain for Chebyshev Type II band filters.
func chebyshev2BWGainDB(gainDB float64) float64 {
	if gainDB < 0 {
		return -0.1
	}

	return 0.1
}

// chebyshev2BandRad designs a Chebyshev Type II band filter using rad/sample parameters.
func chebyshev2BandRad(w0, wb, gainDB, gbDB float64, order int) ([]biquad.Coefficients, error) {
	G0 := 1.0 // db2Lin(0) is always exactly 1
	G := db2Lin(gainDB)
	Gb := db2Lin(gbDB)
	if Gb*Gb == G0*G0 {
		return nil, ErrInvalidParams
	}

	e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	g := math.Pow(G, 1.0/float64(order))
	eu := math.Pow(e+math.Sqrt(1+e*e), 1.0/float64(order))
	ew := math.Pow(G0*e+Gb*math.Sqrt(1.0+e*e), 1.0/float64(order))
	A := (eu - 1.0/eu) * 0.5
	B := (ew - g*g/ew) * 0.5
	tb := math.Tan(wb * 0.5)
	c0 := math.Cos(w0)

	sections := make([]biquad.Coefficients, 0, order)
	L := order / 2
	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1.0) / float64(order)
		ci := math.Cos(math.Pi * ui * 0.5)
		si := math.Sin(math.Pi * ui * 0.5)
		Di := tb*tb + 2*A*si*tb + A*A + ci*ci
		if Di == 0 {
			return nil, ErrInvalidParams
		}

		Bv := [5]float64{
			(g*g*tb*tb + 2.0*g*B*si*tb + B*B + g*g*ci*ci) / Di,
			-4 * c0 * (B*B + g*g*ci*ci + g*B*si*tb) / Di,
			2 * ((B*B+g*g*ci*ci)*(1.0+2.0*c0*c0) - g*g*tb*tb) / Di,
			-4 * c0 * (B*B + g*g*ci*ci - g*B*si*tb) / Di,
			(g*g*tb*tb - 2*g*B*si*tb + B*B + g*g*ci*ci) / Di,
		}

		Av := [5]float64{
			1,
			-4 * c0 * (A*A + ci*ci + A*si*tb) / Di,
			2 * ((A*A+ci*ci)*(1+2*c0*c0) - tb*tb) / Di,
			-4 * c0 * (A*A + ci*ci - A*si*tb) / Di,
			(tb*tb - 2*A*si*tb + A*A + ci*ci) / Di,
		}

		biquads, err := splitFOSection(Bv, Av)
		if err != nil {
			return nil, err
		}
		sections = append(sections, biquads...)
	}

	return sections, nil
}

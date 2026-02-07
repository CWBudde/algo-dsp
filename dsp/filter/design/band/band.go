package band

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

var ErrInvalidParams = errors.New("band: invalid parameters")

// ButterworthBand designs a high-order Butterworth band filter for graphic EQ.
//
// gainDB is the desired center gain in dB. bandwidthHz is the band width in Hz.
// order must be an even integer greater than 2.
func ButterworthBand(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	w0, wb, err := bandParams(sampleRate, f0Hz, bandwidthHz, order)
	if err != nil {
		return nil, err
	}
	if gainDB == 0 {
		return passthroughSections(), nil
	}
	gb := butterworthBWGainDB(gainDB)

	return butterworthBandRad(w0, wb, gainDB, gb, order)
}

// Chebyshev1Band designs a high-order Chebyshev Type I band filter for graphic EQ.
//
// gainDB is the desired center gain in dB. bandwidthHz is the band width in Hz.
// order must be an even integer greater than 2.
func Chebyshev1Band(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	w0, wb, err := bandParams(sampleRate, f0Hz, bandwidthHz, order)
	if err != nil {
		return nil, err
	}
	if gainDB == 0 {
		return passthroughSections(), nil
	}
	gb := chebyshev1BWGainDB(gainDB)

	return chebyshev1BandRad(w0, wb, gainDB, gb, order)
}

// Chebyshev2Band designs a high-order Chebyshev Type II band filter for graphic EQ.
//
// gainDB is the desired center gain in dB. bandwidthHz is the band width in Hz.
// order must be an even integer greater than 2.
func Chebyshev2Band(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	w0, wb, err := bandParams(sampleRate, f0Hz, bandwidthHz, order)
	if err != nil {
		return nil, err
	}
	if gainDB == 0 {
		return passthroughSections(), nil
	}
	gb := chebyshev2BWGainDB(gainDB)
	return chebyshev2BandRad(w0, wb, gainDB, gb, order)
}

// EllipticBand designs a high-order Elliptic band filter for graphic EQ.
//
// gainDB is the desired center gain in dB. bandwidthHz is the band width in Hz.
// order must be an even integer greater than 2.
func EllipticBand(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	w0, wb, err := bandParams(sampleRate, f0Hz, bandwidthHz, order)
	if err != nil {
		return nil, err
	}
	if gainDB == 0 {
		return passthroughSections(), nil
	}
	gb := ellipticBWGainDB(gainDB)
	return ellipticBandRad(w0, wb, gainDB, gb, order)
}

func bandParams(sampleRate, f0Hz, bandwidthHz float64, order int) (float64, float64, error) {
	if sampleRate <= 0 || f0Hz <= 0 || bandwidthHz <= 0 {
		return 0, 0, ErrInvalidParams
	}
	if f0Hz >= sampleRate*0.5 {
		return 0, 0, ErrInvalidParams
	}
	if order <= 2 || order%2 != 0 {
		return 0, 0, ErrInvalidParams
	}
	fl := f0Hz - bandwidthHz*0.5
	fh := f0Hz + bandwidthHz*0.5
	if fl <= 0 || fh >= sampleRate*0.5 {
		return 0, 0, ErrInvalidParams
	}

	w0 := 2 * math.Pi * f0Hz / sampleRate
	wb := 2 * math.Pi * bandwidthHz / sampleRate
	if !(w0 > 0 && w0 < math.Pi && wb > 0 && wb < math.Pi) {
		return 0, 0, ErrInvalidParams
	}
	return w0, wb, nil
}

func passthroughSections() []biquad.Coefficients {
	return []biquad.Coefficients{{B0: 1, B1: 0, B2: 0, A1: 0, A2: 0}}
}

func butterworthBWGainDB(gainDB float64) float64 {
	if gainDB < -3 {
		return gainDB + 3
	}
	if gainDB < 3 {
		return gainDB / math.Sqrt2
	}
	return gainDB - 3
}

func chebyshev1BWGainDB(gainDB float64) float64 {
	if gainDB < 0 {
		return gainDB + 0.1
	}
	return gainDB - 0.1
}

func chebyshev2BWGainDB(gainDB float64) float64 {
	if gainDB < 0 {
		return -0.1
	}
	return 0.1
}

func ellipticBWGainDB(gainDB float64) float64 {
	if gainDB < 0 {
		return gainDB + 0.05
	}
	return gainDB - 0.05
}

func butterworthBandRad(w0, wb, gainDB, gbDB float64, order int) ([]biquad.Coefficients, error) {
	G0 := db2Lin(0)
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

func chebyshev1BandRad(w0, wb, gainDB, gbDB float64, order int) ([]biquad.Coefficients, error) {
	G0 := db2Lin(0)
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

func chebyshev2BandRad(w0, wb, gainDB, gbDB float64, order int) ([]biquad.Coefficients, error) {
	G0 := db2Lin(0)
	G := db2Lin(gainDB)
	Gb := db2Lin(gbDB)
	if Gb*Gb == G0*G0 {
		return nil, ErrInvalidParams
	}

	e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	g := math.Pow(G, 1.0/float64(order))
	eu := math.Pow(e+math.Sqrt(1+e*e), 1.0/float64(order))
	ew := math.Pow(G0*e+Gb*math.Sqrt(1+e*e), 1.0/float64(order))
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
			(g*g*tb*tb + 2*g*B*si*tb + B*B + g*g*ci*ci) / Di,
			-4 * c0 * (B*B + g*g*ci*ci + g*B*si*tb) / Di,
			2 * ((B*B+g*g*ci*ci)*(1+2*c0*c0) - g*g*tb*tb) / Di,
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

func db2Lin(db float64) float64 {
	return math.Pow(10, db/20.0)
}

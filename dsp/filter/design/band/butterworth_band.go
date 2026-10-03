package band

import (
	"math"
	"math/cmplx"

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

	sections := make([]biquad.Coefficients, 0, order)

	L := order / 2
	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1) / float64(order)
		si := math.Sin(math.Pi * ui * 0.5)

		// Factor P(z)^2 + 2*sin(theta)*beta*P(z)*(z^2-1)
		// + beta^2*(z^2-1)^2 analytically. Generic quartic roots lose
		// precision when low-frequency roots cluster around z=1.
		ci := math.Cos(math.Pi * ui * 0.5)
		poleV := complex(si*beta, ci*beta)
		zeroV := poleV * complex(g/g0, 0)
		pole1, pole2 := bandQuadraticRoots(w0, poleV)
		zero1, zero2 := bandQuadraticRoots(w0, zeroV)
		gain := (g*g*beta*beta + 2*g*g0*si*beta + g0*g0) / (beta*beta + 2*si*beta + 1)
		sectionGain := math.Sqrt(gain)
		biquads := []biquad.Coefficients{
			{B0: sectionGain, B1: -2 * real(zero1) * sectionGain, B2: real(zero1*cmplx.Conj(zero1)) * sectionGain, A1: -2 * real(pole1), A2: real(pole1 * cmplx.Conj(pole1))},
			{B0: sectionGain, B1: -2 * real(zero2) * sectionGain, B2: real(zero2*cmplx.Conj(zero2)) * sectionGain, A1: -2 * real(pole2), A2: real(pole2 * cmplx.Conj(pole2))},
		}

		sections = append(sections, biquads...)
	}

	return sections, nil
}

// bandQuadraticRoots solves (1+v)z²-2*cos(w0)z+(1-v)=0.
// sin(w0)² replaces 1-cos(w0)² to avoid cancellation near DC.
func bandQuadraticRoots(w0 float64, v complex128) (complex128, complex128) {
	sine := math.Sin(w0)
	delta := cmplx.Sqrt(v*v - complex(sine*sine, 0))
	cosine := complex(math.Cos(w0), 0)

	return (cosine + delta) / (1 + v), (cosine - delta) / (1 + v)
}

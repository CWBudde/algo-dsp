package orfanidis

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/internal/ellipticmath"
)

// EllipticPrototype builds the analog prototype sections of an elliptic
// (Cauer) parametric equalizer of order Spec.Order.
//
// The design is equiripple on both sides of the transition: the response stays
// within Gb of G on the active side and within Gs of G0 on the reference side.
// Pole and zero locations come from the Jacobi cd function evaluated at the
// arguments derived from the elliptic degree equation, following Orfanidis.
//
// For even orders the cascade starts with a plain gain stage carrying Gb; for
// odd orders that stage is replaced by a first-order section carrying G.
func EllipticPrototype(s Spec) ([]Section, error) {
	if s.Order < 1 || s.WB <= 0 {
		return nil, ErrInvalidParams
	}

	G0, G, Gb, Gs := s.G0, s.G, s.Gb, s.Gs

	eNum, eDen := G*G-Gb*Gb, Gb*Gb-G0*G0
	esNum, esDen := G*G-Gs*Gs, Gs*Gs-G0*G0

	if IsZero(eDen) || IsZero(esDen) {
		return nil, ErrInvalidParams
	}

	eRatio, esRatio := eNum/eDen, esNum/esDen
	if eRatio <= 0 || esRatio <= 0 || !isFinite(eRatio) || !isFinite(esRatio) {
		return nil, ErrInvalidParams
	}

	e := math.Sqrt(eRatio)
	es := math.Sqrt(esRatio)

	k1 := e / es
	if !isFinite(k1) || k1 <= 0 || k1 >= 1 {
		return nil, ErrInvalidParams
	}

	k := ellipticmath.EllipDeg(s.Order, k1, Tol)

	order := complex(float64(s.Order), 0)
	ju0 := ellipticmath.ASNE(complex(0, 1)*complex(G/(e*G0), 0), k1, Tol) / order
	jv0 := ellipticmath.ASNE(complex(0, 1)/complex(e, 0), k1, Tol) / order

	r := s.Order % 2
	L := (s.Order - r) / 2

	secs := make([]Section, 0, L+1)

	if r == 0 {
		secs = append(secs, Section{B0: Gb, A0: 1})
	} else {
		z0 := real(complex(0, 1) * ellipticmath.CDE(-1.0+ju0, k, Tol))
		p0 := real(complex(0, 1) * ellipticmath.CDE(-1.0+jv0, k, Tol))

		if IsZero(z0) || IsZero(p0) {
			return nil, ErrInvalidParams
		}

		secs = append(secs, Section{
			B0: G * s.WB, B1: -G / z0,
			A0: s.WB, A1: -1 / p0,
		})
	}

	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1.0) / float64(s.Order)

		zero := complex(0, 1) * ellipticmath.CDE(complex(ui, 0)-ju0, k, Tol)
		pole := complex(0, 1) * ellipticmath.CDE(complex(ui, 0)-jv0, k, Tol)

		if zero == 0 || pole == 0 {
			return nil, ErrInvalidParams
		}

		invZero := 1.0 / zero
		invPole := 1.0 / pole

		zabs := cmplx.Abs(invZero)
		pabs := cmplx.Abs(invPole)

		secs = append(secs, Section{
			B0: s.WB * s.WB, B1: -2 * s.WB * real(invZero), B2: zabs * zabs,
			A0: s.WB * s.WB, A1: -2 * s.WB * real(invPole), A2: pabs * pabs,
		})
	}

	for _, sec := range secs {
		if !sectionIsFinite(sec) {
			return nil, ErrInvalidParams
		}
	}

	return secs, nil
}

func sectionIsFinite(s Section) bool {
	return isFinite(s.B0) && isFinite(s.B1) && isFinite(s.B2) &&
		isFinite(s.A0) && isFinite(s.A1) && isFinite(s.A2)
}

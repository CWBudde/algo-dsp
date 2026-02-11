package ellipticmath

import (
	"math"
	"math/cmplx"
)

// Landen computes the Landen sequence of descending moduli for k.
// If tol < 1 it is interpreted as a convergence threshold; otherwise
// it is interpreted as a fixed iteration count.
func Landen(k, tol float64) []float64 {
	var v []float64
	if k == 0 || k == 1.0 {
		return []float64{k}
	}
	if tol < 1 {
		for k > tol {
			t := k / (1.0 + math.Sqrt((1-k)*(1+k)))
			k = t * t
			v = append(v, k)
		}
	} else {
		M := int(tol)
		for i := 1; i <= M; i++ {
			t := k / (1.0 + math.Sqrt((1-k)*(1+k)))
			k = t * t
			v = append(v, k)
		}
	}

	return v
}

// LandenK computes K(k) from a precomputed Landen sequence using
// K(k) = (pi/2) * product(1 + v[i]).
func LandenK(v []float64) float64 {
	prod := 1.0
	for _, x := range v {
		prod *= 1.0 + x
	}
	return prod * math.Pi * 0.5
}

// EllipK computes the complete elliptic integral K(k) and K'(k).
func EllipK(k, tol float64) (float64, float64) {
	return EllipKReuse(k, tol, nil)
}

// EllipKReuse is like EllipK but accepts an optional precomputed Landen
// sequence for the K(k) half.
func EllipKReuse(k, tol float64, vk []float64) (float64, float64) {
	kmin := 1e-6
	kmax := math.Sqrt(1 - kmin*kmin)

	var K, Kp float64
	if k == 1.0 {
		K = math.Inf(1)
	} else if k > kmax {
		kp := math.Sqrt((1 - k) * (1 + k))
		L := -math.Log(kp / 4.0)
		K = L + (L-1)*kp*kp/4.0
	} else {
		if vk == nil {
			vk = Landen(k, tol)
		}
		K = LandenK(vk)
	}

	if k == 0.0 {
		Kp = math.Inf(1)
	} else if k < kmin {
		L := -math.Log(k / 4.0)
		Kp = L + (L-1.0)*k*k/4.0
	} else {
		kp := math.Sqrt((1 - k) * (1 + k))
		Kp = LandenK(Landen(kp, tol))
	}

	return K, Kp
}

// EllipDeg2 computes the nome-series approximation used by EllipDeg
// in the very small-k1 regime.
func EllipDeg2(n, k, tol float64) float64 {
	const terms = 7
	K, Kp := EllipK(k, tol)
	q := math.Exp(-math.Pi * Kp / K)
	q1 := math.Pow(q, n)

	var s1, s2 float64
	q1sq := q1
	q1pow := q1
	q1gap := q1
	q1_2 := q1 * q1
	for i := 1; i <= terms; i++ {
		s2 += q1sq
		s1 += q1sq * q1pow
		q1gap *= q1_2
		q1sq *= q1gap
		q1pow *= q1
	}

	r := (1.0 + s1) / (1.0 + 2*s2)
	return 4 * math.Sqrt(q1) * r * r
}

// SymmetricRemainder returns x modulo y mapped to approximately [-y/2, y/2].
func SymmetricRemainder(x, y float64) float64 {
	z := math.Remainder(x, y)
	correction := 0.0
	if math.Abs(z) > y/2.0 {
		correction = 1.0
	}
	return z - y*math.Copysign(correction, z)
}

// ACDE computes the inverse cd Jacobi elliptic function.
func ACDE(w complex128, k, tol float64) complex128 {
	v := Landen(k, tol)
	for i := range v {
		v1 := k
		if i > 0 {
			v1 = v[i-1]
		}
		w = w / (1.0 + cmplx.Sqrt(1.0-w*w*complex(v1*v1, 0))) * 2.0 / (1 + complex(v[i], 0))
	}

	u := 2.0 / math.Pi * cmplx.Acos(w)
	K, Kp := EllipKReuse(k, tol, v)

	return complex(SymmetricRemainder(real(u), 4), 0) + complex(0, 1)*complex(SymmetricRemainder(imag(u), 2*(Kp/K)), 0)
}

// ASNE computes the inverse sn Jacobi elliptic function.
func ASNE(w complex128, k, tol float64) complex128 {
	return 1.0 - ACDE(w, k, tol)
}

// CDE computes the cd Jacobi elliptic function.
func CDE(u complex128, k, tol float64) complex128 {
	v := Landen(k, tol)
	w := cmplx.Cos(u * math.Pi * 0.5)
	for i := len(v) - 1; i >= 0; i-- {
		w = (1 + complex(v[i], 0)) * w / (1.0 + complex(v[i], 0)*w*w)
	}

	return w
}

// SNE computes the sn Jacobi elliptic function for a vector of real arguments.
func SNE(u []float64, k, tol float64) []float64 {
	v := Landen(k, tol)
	w := make([]float64, len(u))
	for i := range u {
		w[i] = math.Sin(u[i] * math.Pi * 0.5)
	}
	for i := len(v) - 1; i >= 0; i-- {
		for j := range w {
			w[j] = ((1 + v[i]) * w[j]) / (1 + v[i]*w[j]*w[j])
		}
	}

	return w
}

// EllipDeg solves the elliptic degree equation for order N and selectivity k1.
func EllipDeg(N int, k1, tol float64) float64 {
	L := N / 2
	ui := make([]float64, 0, L)
	for i := 1; i <= L; i++ {
		ui = append(ui, (2.0*float64(i)-1.0)/float64(N))
	}
	kmin := 1e-6
	if k1 < kmin {
		return EllipDeg2(1.0/float64(N), k1, tol)
	}

	kc := math.Sqrt((1 - k1) * (1 + k1))
	w := SNE(ui, kc, tol)
	prod := 1.0
	for _, x := range w {
		prod *= x
	}
	kp := math.Pow(kc, float64(N)) * math.Pow(prod, 4)

	return math.Sqrt(1 - kp*kp)
}

package geq

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

type soSection struct {
	b0, b1, b2 float64
	a0, a1, a2 float64
}

type foSection struct {
	b [5]float64
	a [5]float64
}

func ellipticBandRad(w0, wb, gainDB, gbDB float64, order int) ([]biquad.Coefficients, error) {
	if order <= 2 || order%2 != 0 {
		return nil, ErrInvalidParams
	}

	G0 := db2Lin(0)
	G := db2Lin(gainDB)
	Gb := db2Lin(gbDB)
	Gs := db2Lin(gainDB - gbDB)

	WB := math.Tan(wb / 2.0)
	e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	es := math.Sqrt((G*G - Gs*Gs) / (Gs*Gs - G0*G0))
	k1 := e / es
	k := ellipdeg(order, k1, 2.2e-16)

	ju0 := asne(complex(0, 1)*complex(G/(e*G0), 0), k1, 2.2e-16) / complex(float64(order), 0)
	jv0 := asne(complex(0, 1)/complex(e, 0), k1, 2.2e-16) / complex(float64(order), 0)

	r := order % 2
	L := (order - r) / 2

	var aSections []soSection
	if r == 0 {
		aSections = append(aSections, soSection{b0: Gb, b1: 0, b2: 0, a0: 1, a1: 0, a2: 0})
	} else {
		z0 := real(complex(0, 1) * cde(-1.0+ju0, k, 2.2e-16))
		B00 := G * WB
		B01 := -G / z0
		A00 := WB
		A01 := -1 / real(complex(0, 1)*cde(-1.0+jv0, k, 2.2e-16))
		aSections = append(aSections, soSection{b0: B00, b1: B01, b2: 0, a0: A00, a1: A01, a2: 0})
	}

	if L > 0 {
		for i := 1; i <= L; i++ {
			ui := (2.0*float64(i) - 1.0) / float64(order)
			zeros := complex(0, 1) * cde(complex(ui, 0)-ju0, k, 2.2e-16)
			poles := complex(0, 1) * cde(complex(ui, 0)-jv0, k, 2.2e-16)

			invZero := 1.0 / zeros
			invPole := 1.0 / poles
			zre := real(invZero)
			pre := real(invPole)
			zabs := cmplx.Abs(invZero)
			pabs := cmplx.Abs(invPole)
			sa := soSection{
				b0: WB * WB,
				b1: -2 * WB * zre,
				b2: zabs * zabs,
				a0: WB * WB,
				a1: -2 * WB * pre,
				a2: pabs * pabs,
			}
			aSections = append(aSections, sa)
		}
	}

	foSections := blt(aSections, w0)
	out := make([]biquad.Coefficients, 0, len(foSections)*2)
	for _, s := range foSections {
		biquads, err := splitFOSection(s.b, s.a)
		if err != nil {
			return nil, err
		}
		out = append(out, biquads...)
	}
	return out, nil
}

func blt(aSections []soSection, w0 float64) []foSection {
	c0 := math.Cos(w0)
	K := len(aSections)

	B := make([][5]float64, K)
	A := make([][5]float64, K)
	Bhat := make([][3]float64, K)
	Ahat := make([][3]float64, K)

	B0 := make([]float64, K)
	B1 := make([]float64, K)
	B2 := make([]float64, K)
	A0 := make([]float64, K)
	A1 := make([]float64, K)
	A2 := make([]float64, K)
	for i := 0; i < K; i++ {
		B0[i] = aSections[i].b0
		B1[i] = aSections[i].b1
		B2[i] = aSections[i].b2
		A0[i] = aSections[i].a0
		A1[i] = aSections[i].a1
		A2[i] = aSections[i].a2
	}

	var zths []int
	for i := 0; i < len(B0); i++ {
		if isZero(B1[i]) && isZero(A1[i]) && isZero(B2[i]) && isZero(A2[i]) {
			zths = append(zths, i)
		}
	}
	for _, j := range zths {
		Bhat[j][0] = B0[j] / A0[j]
		Ahat[j][0] = 1
		B[j][0] = Bhat[j][0]
		A[j][0] = 1
	}

	var fths []int
	for i := 0; i < len(B0); i++ {
		if (!isZero(B1[i]) || !isZero(A1[i])) && isZero(B2[i]) && isZero(A2[i]) {
			fths = append(fths, i)
		}
	}
	for _, j := range fths {
		D := A0[j] + A1[j]
		Bhat[j][0] = (B0[j] + B1[j]) / D
		Bhat[j][1] = (B0[j] - B1[j]) / D
		Ahat[j][0] = 1
		Ahat[j][1] = (A0[j] - A1[j]) / D

		B[j][0] = Bhat[j][0]
		B[j][1] = c0 * (Bhat[j][1] - Bhat[j][0])
		B[j][2] = -Bhat[j][1]
		A[j][0] = 1
		A[j][1] = c0 * (Ahat[j][1] - 1)
		A[j][2] = -Ahat[j][1]
	}

	var sths []int
	for i := 0; i < len(B0); i++ {
		if !isZero(B2[i]) || !isZero(A2[i]) {
			sths = append(sths, i)
		}
	}
	for _, j := range sths {
		D := A0[j] + A1[j] + A2[j]
		Bhat[j][0] = (B0[j] + B1[j] + B2[j]) / D
		Bhat[j][1] = 2 * (B0[j] - B2[j]) / D
		Bhat[j][2] = (B0[j] - B1[j] + B2[j]) / D
		Ahat[j][0] = 1
		Ahat[j][1] = 2 * (A0[j] - A2[j]) / D
		Ahat[j][2] = (A0[j] - A1[j] + A2[j]) / D

		B[j][0] = Bhat[j][0]
		B[j][1] = c0 * (Bhat[j][1] - 2*Bhat[j][0])
		B[j][2] = (Bhat[j][0]-Bhat[j][1]+Bhat[j][2])*c0*c0 - Bhat[j][1]
		B[j][3] = c0 * (Bhat[j][1] - 2*Bhat[j][2])
		B[j][4] = Bhat[j][2]

		A[j][0] = 1
		A[j][1] = c0 * (Ahat[j][1] - 2)
		A[j][2] = (1-Ahat[j][1]+Ahat[j][2])*c0*c0 - Ahat[j][1]
		A[j][3] = c0 * (Ahat[j][1] - 2*Ahat[j][2])
		A[j][4] = Ahat[j][2]
	}

	if isZero(math.Abs(c0) - 1) {
		for i := 0; i < len(Bhat); i++ {
			B[i][0] = Bhat[i][0]
			B[i][1] = Bhat[i][1]
			B[i][2] = Bhat[i][2]
			A[i][0] = Ahat[i][0]
			A[i][1] = Ahat[i][1]
			A[i][2] = Ahat[i][2]
			B[i][3], B[i][4] = 0, 0
			A[i][3], A[i][4] = 0, 0
			B[i][1] *= c0
			A[i][1] *= c0
		}
	}

	out := make([]foSection, 0, len(B))
	for i := range B {
		out = append(out, foSection{b: B[i], a: A[i]})
	}
	return out
}

func isZero(v float64) bool {
	return math.Abs(v) < 1e-12
}

func landen(k, tol float64) []float64 {
	var v []float64
	if k == 0 || k == 1.0 {
		v = append(v, k)
	}
	if tol < 1 {
		for k > tol {
			k = math.Pow(k/(1.0+math.Sqrt(1.0-k*k)), 2)
			v = append(v, k)
		}
	} else {
		M := int(tol)
		for i := 1; i <= M; i++ {
			k = math.Pow(k/(1.0+math.Sqrt(1.0-k*k)), 2)
			v = append(v, k)
		}
	}
	return v
}

func ellipk(k, tol float64) (float64, float64) {
	kmin := 1e-6
	kmax := math.Sqrt(1 - kmin*kmin)

	var K, Kp float64
	if k == 1.0 {
		K = math.Inf(1)
	} else if k > kmax {
		kp := math.Sqrt(1.0 - k*k)
		L := -math.Log(kp / 4.0)
		K = L + (L-1)*kp*kp/4.0
	} else {
		v := landen(k, tol)
		for i := range v {
			v[i] += 1.0
		}
		prod := 1.0
		for _, x := range v {
			prod *= x
		}
		K = prod * math.Pi / 2.0
	}

	if k == 0.0 {
		Kp = math.Inf(1)
	} else if k < kmin {
		L := -math.Log(k / 4.0)
		Kp = L + (L-1.0)*k*k/4.0
	} else {
		kp := math.Sqrt(1.0 - k*k)
		v := landen(kp, tol)
		for i := range v {
			v[i] += 1.0
		}
		prod := 1.0
		for _, x := range v {
			prod *= x
		}
		Kp = prod * math.Pi / 2.0
	}

	return K, Kp
}

func ellipdeg2(n, k, tol float64) float64 {
	const M = 7
	K, Kp := ellipk(k, tol)
	q := math.Exp(-math.Pi * Kp / K)
	q1 := math.Pow(q, n)
	var s1, s2 float64
	for i := 1; i <= M; i++ {
		s1 += math.Pow(q1, float64(i*(i+1)))
		s2 += math.Pow(q1, float64(i*i))
	}
	return 4 * math.Sqrt(q1) * math.Pow((1.0+s1)/(1.0+2*s2), 2)
}

func srem(x, y float64) float64 {
	z := math.Remainder(x, y)
	correction := 0.0
	if math.Abs(z) > y/2.0 {
		correction = 1.0
	}
	return z - y*math.Copysign(correction, z)
}

func acde(w complex128, k, tol float64) complex128 {
	v := landen(k, tol)
	for i := 0; i < len(v); i++ {
		v1 := k
		if i > 0 {
			v1 = v[i-1]
		}
		w = w / (1.0 + cmplx.Sqrt(1.0-w*w*complex(v1*v1, 0))) * 2.0 / (1 + complex(v[i], 0))
	}

	u := 2.0 / math.Pi * cmplx.Acos(w)
	K, Kp := ellipk(k, tol)
	return complex(srem(real(u), 4), 0) + complex(0, 1)*complex(srem(imag(u), 2*(Kp/K)), 0)
}

func asne(w complex128, k, tol float64) complex128 {
	return 1.0 - acde(w, k, tol)
}

func cde(u complex128, k, tol float64) complex128 {
	v := landen(k, tol)
	w := cmplx.Cos(u * math.Pi / 2.0)
	for i := len(v) - 1; i >= 0; i-- {
		w = (1 + complex(v[i], 0)) * w / (1.0 + complex(v[i], 0)*w*w)
	}
	return w
}

func sne(u []float64, k, tol float64) []float64 {
	v := landen(k, tol)
	w := make([]float64, len(u))
	for i := range u {
		w[i] = math.Sin(u[i] * math.Pi / 2.0)
	}
	for i := len(v) - 1; i >= 0; i-- {
		for j := range w {
			w[j] = ((1 + v[i]) * w[j]) / (1 + v[i]*w[j]*w[j])
		}
	}
	return w
}

func ellipdeg(N int, k1, tol float64) float64 {
	L := N / 2
	ui := make([]float64, 0, L)
	for i := 1; i <= L; i++ {
		ui = append(ui, (2.0*float64(i)-1.0)/float64(N))
	}
	kmin := 1e-6
	if k1 < kmin {
		return ellipdeg2(1.0/float64(N), k1, tol)
	}
	kc := math.Sqrt(1 - k1*k1)
	w := sne(ui, kc, tol)
	prod := 1.0
	for _, x := range w {
		prod *= x
	}
	kp := math.Pow(kc, float64(N)) * math.Pow(prod, 4)
	return math.Sqrt(1 - kp*kp)
}

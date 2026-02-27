//nolint:funlen
package pass

import (
	"math"
	"math/cmplx"
	"sort"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/internal/ellipticmath"
)

const (
	ellipticTol       = 2.2e-16
	ellipticRootTol   = 1e-9
	ellipticEpsilon   = 2.220446049250313e-16
	arcJacSNMaxIter   = 10
	arcJacImagCheck   = 1e-7
	ellipticSeriesLen = 7
)

// EllipticLP designs a lowpass elliptic (Cauer) filter cascade.
//
// Elliptic filters provide the sharpest transition from passband to stopband
// among classical IIR filter types, at the cost of ripple in both regions.
// The rippleDB parameter controls passband ripple (in dB, typical 0.1-1.0),
// while stopbandDB controls the minimum stopband attenuation (in dB, typical 40-80).
//
// The design uses the standard analog elliptic prototype (poles and zeros
// placed via Jacobi elliptic functions) followed by bilinear transform.
func EllipticLP(freq float64, order int, rippleDB, stopbandDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}

	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}

	if rippleDB <= 0 || stopbandDB <= rippleDB {
		return nil
	}

	k, ok := bilinearK(freq, sampleRate)
	if !ok {
		return nil
	}

	az, ap, ak, ok := ellipticAnalogPrototype(order, rippleDB, stopbandDB)
	if !ok {
		return nil
	}

	dz, dp, dk, ok := bilinearZPK(az, ap, ak, k)
	if !ok {
		return nil
	}

	sections := zpkToSections(dz, dp, dk)
	if len(sections) == 0 {
		return nil
	}

	normalizeCascadeLP(sections)

	return sections
}

// EllipticHP designs a highpass elliptic (Cauer) filter cascade.
//
// Applies an LP-to-HP frequency transformation to the analog elliptic prototype
// before the bilinear transform. The passband (above freq) has controlled ripple,
// and the stopband (below freq) has controlled minimum attenuation.
func EllipticHP(freq float64, order int, rippleDB, stopbandDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}

	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}

	if rippleDB <= 0 || stopbandDB <= rippleDB {
		return nil
	}

	k, ok := bilinearK(freq, sampleRate)
	if !ok {
		return nil
	}

	az, ap, ak, ok := ellipticAnalogPrototype(order, rippleDB, stopbandDB)
	if !ok {
		return nil
	}

	hz, hp, hk, ok := lpToHPZPK(az, ap, ak)
	if !ok {
		return nil
	}

	dz, dp, dk, ok := bilinearZPK(hz, hp, hk, k)
	if !ok {
		return nil
	}

	sections := zpkToSections(dz, dp, dk)
	if len(sections) == 0 {
		return nil
	}

	normalizeCascadeHP(sections)

	return sections
}

func ellipticAnalogPrototype(order int, rippleDB, stopbandDB float64) ([]complex128, []complex128, float64, bool) {
	if order <= 0 {
		return nil, nil, 0, false
	}

	epsSq := dbToMinusOne(rippleDB)

	stopSq := dbToMinusOne(stopbandDB)
	if epsSq <= 0 || stopSq <= 0 {
		return nil, nil, 0, false
	}

	ck1Sq := epsSq / stopSq
	if !(ck1Sq > 0 && ck1Sq < 1) {
		return nil, nil, 0, false
	}

	if order == 1 {
		p := -math.Sqrt(1.0 / epsSq)
		return nil, []complex128{complex(p, 0)}, -p, true
	}

	m := ellipdegParam(order, ck1Sq, ellipticTol)
	if !(m > 0 && m < 1) {
		return nil, nil, 0, false
	}

	kmod := math.Sqrt(m)
	capk, _ := ellipticmath.EllipK(kmod, ellipticTol)
	ck1 := math.Sqrt(ck1Sq)

	val0, _ := ellipticmath.EllipK(ck1, ellipticTol)
	if capk == 0 || val0 == 0 || math.IsNaN(capk) || math.IsNaN(val0) || math.IsInf(capk, 0) || math.IsInf(val0, 0) {
		return nil, nil, 0, false
	}

	start := 1 - order%2
	svals := make([]float64, 0, (order+1)/2)
	cvals := make([]float64, 0, (order+1)/2)
	dvals := make([]float64, 0, (order+1)/2)
	zerosBase := make([]complex128, 0, order)

	for j := start; j < order; j += 2 {
		u := float64(j) * capk / float64(order)

		sn, cn, dn, ok := jacobiSCDFloat(u, kmod, ellipticTol)
		if !ok {
			return nil, nil, 0, false
		}

		svals = append(svals, sn)
		cvals = append(cvals, cn)

		dvals = append(dvals, dn)
		if math.Abs(sn) > ellipticEpsilon {
			zerosBase = append(zerosBase, complex(0, 1)/(complex(kmod*sn, 0)))
		}
	}

	eps := math.Sqrt(epsSq)

	r := arcJacSC1(1.0/eps, ck1Sq, ellipticTol)
	if !(r > 0) || math.IsNaN(r) || math.IsInf(r, 0) {
		return nil, nil, 0, false
	}

	v0 := capk * r / (float64(order) * val0)

	sv, cv, dv, ok := jacobiSCDFloat(v0, math.Sqrt(1.0-m), ellipticTol)
	if !ok {
		return nil, nil, 0, false
	}

	polesBase := make([]complex128, len(svals))
	for i := range svals {
		den := 1.0 - (dvals[i]*sv)*(dvals[i]*sv)
		if math.Abs(den) <= ellipticEpsilon {
			return nil, nil, 0, false
		}

		num := complex(cvals[i]*dvals[i]*sv*cv, svals[i]*dv)
		polesBase[i] = -num / complex(den, 0)
	}

	poles := make([]complex128, 0, order)
	if order%2 == 1 {
		norm2 := 0.0
		for _, p := range polesBase {
			norm2 += real(p * cmplx.Conj(p))
		}

		thr := ellipticEpsilon * math.Sqrt(norm2)

		poles = append(poles, polesBase...)
		for _, p := range polesBase {
			if math.Abs(imag(p)) > thr {
				poles = append(poles, cmplx.Conj(p))
			}
		}
	} else {
		poles = append(poles, polesBase...)
		for _, p := range polesBase {
			poles = append(poles, cmplx.Conj(p))
		}
	}

	zeros := make([]complex128, 0, len(zerosBase)*2)
	for _, z := range zerosBase {
		zeros = append(zeros, z, cmplx.Conj(z))
	}

	prodP := complexProductNeg(poles)

	prodZ := complex(1, 0)
	if len(zeros) > 0 {
		prodZ = complexProductNeg(zeros)
	}

	if prodZ == 0 {
		return nil, nil, 0, false
	}

	gain := real(prodP / prodZ)
	if order%2 == 0 {
		gain /= math.Sqrt(1.0 + epsSq)
	}

	if gain == 0 || math.IsNaN(gain) || math.IsInf(gain, 0) {
		return nil, nil, 0, false
	}

	return zeros, poles, gain, true
}

func lpToHPZPK(z, p []complex128, k float64) ([]complex128, []complex128, float64, bool) {
	degree := len(p) - len(z)
	if degree < 0 {
		return nil, nil, 0, false
	}

	zh := make([]complex128, 0, len(z)+degree)
	for _, zr := range z {
		if zr == 0 {
			return nil, nil, 0, false
		}

		zh = append(zh, 1.0/zr)
	}

	for range degree {
		zh = append(zh, 0)
	}

	ph := make([]complex128, 0, len(p))
	for _, pr := range p {
		if pr == 0 {
			return nil, nil, 0, false
		}

		ph = append(ph, 1.0/pr)
	}

	kh := k
	if len(z) > 0 {
		kh *= real(complexProductNeg(z))
	}

	if len(p) > 0 {
		den := real(complexProductNeg(p))
		if den == 0 || math.IsNaN(den) || math.IsInf(den, 0) {
			return nil, nil, 0, false
		}

		kh /= den
	}

	if kh == 0 || math.IsNaN(kh) || math.IsInf(kh, 0) {
		return nil, nil, 0, false
	}

	return zh, ph, kh, true
}

func bilinearZPK(z, p []complex128, kGain, k float64) ([]complex128, []complex128, float64, bool) {
	degree := len(p) - len(z)
	if degree < 0 {
		return nil, nil, 0, false
	}

	zd := make([]complex128, 0, len(z)+degree)
	for _, zr := range z {
		den := 1.0 - complex(k, 0)*zr
		if den == 0 {
			return nil, nil, 0, false
		}

		zd = append(zd, (1.0+complex(k, 0)*zr)/den)
	}

	for range degree {
		zd = append(zd, -1)
	}

	pd := make([]complex128, 0, len(p))
	for _, pr := range p {
		den := 1.0 - complex(k, 0)*pr
		if den == 0 {
			return nil, nil, 0, false
		}

		pd = append(pd, (1.0+complex(k, 0)*pr)/den)
	}

	num := complexProductOneMinusK(z, k)

	den := complexProductOneMinusK(p, k)
	if den == 0 {
		return nil, nil, 0, false
	}

	kd := kGain * real(num/den)
	if kd == 0 || math.IsNaN(kd) || math.IsInf(kd, 0) {
		return nil, nil, 0, false
	}

	return zd, pd, kd, true
}

//nolint:cyclop
func zpkToSections(z, p []complex128, gain float64) []biquad.Coefficients {
	if len(p) == 0 {
		return nil
	}

	pGroups := groupRoots(p)
	zGroups := groupRoots(z)

	if len(pGroups) == 0 {
		return nil
	}

	sort.Slice(pGroups, func(i, j int) bool {
		if len(pGroups[i]) != len(pGroups[j]) {
			return len(pGroups[i]) > len(pGroups[j])
		}

		return groupImagAbs(pGroups[i]) > groupImagAbs(pGroups[j])
	})

	var zComplex, zSingle [][]complex128

	for _, g := range zGroups {
		if len(g) == 2 {
			zComplex = append(zComplex, g)
		} else {
			zSingle = append(zSingle, g)
		}
	}

	out := make([]biquad.Coefficients, 0, len(pGroups))
	for _, pg := range pGroups {
		var zg []complex128

		if len(pg) == 2 {
			if len(zComplex) > 0 {
				zg = zComplex[0]
				zComplex = zComplex[1:]
			} else if len(zSingle) > 0 {
				zg = zSingle[0]
				zSingle = zSingle[1:]
			}
		} else {
			if len(zSingle) > 0 {
				zg = zSingle[0]
				zSingle = zSingle[1:]
			} else if len(zComplex) > 0 {
				zg = zComplex[0]
				zComplex = zComplex[1:]
			}
		}

		b1, b2 := quadFromRoots(zg)
		a1, a2 := quadFromRoots(pg)
		out = append(out, biquad.Coefficients{
			B0: 1, B1: b1, B2: b2,
			A1: a1, A2: a2,
		})
	}

	if len(out) > 0 && !math.IsNaN(gain) && !math.IsInf(gain, 0) && gain != 0 {
		out[0].B0 *= gain
		out[0].B1 *= gain
		out[0].B2 *= gain
	}

	return out
}

func groupRoots(roots []complex128) [][]complex128 {
	if len(roots) == 0 {
		return nil
	}

	sortedRoots := append([]complex128(nil), roots...)
	sort.Slice(sortedRoots, func(i, j int) bool {
		ii := imag(sortedRoots[i])

		jj := imag(sortedRoots[j])
		if ii != jj {
			return ii > jj
		}

		return real(sortedRoots[i]) < real(sortedRoots[j])
	})

	used := make([]bool, len(sortedRoots))
	groups := make([][]complex128, 0, (len(sortedRoots)+1)/2)
	reals := make([]complex128, 0, len(sortedRoots))

	for i, r := range sortedRoots {
		if used[i] {
			continue
		}

		if math.Abs(imag(r)) <= ellipticRootTol {
			used[i] = true

			reals = append(reals, complex(real(r), 0))

			continue
		}

		target := cmplx.Conj(r)
		best := -1
		bestDist := math.MaxFloat64

		for j, rr := range sortedRoots {
			if i == j || used[j] {
				continue
			}

			d := cmplx.Abs(rr - target)
			if d < bestDist {
				bestDist = d
				best = j
			}
		}

		used[i] = true
		if best != -1 && bestDist <= 1e-4 {
			used[best] = true
			groups = append(groups, []complex128{r, sortedRoots[best]})
		} else {
			groups = append(groups, []complex128{r})
		}
	}

	sort.Slice(reals, func(i, j int) bool { return real(reals[i]) < real(reals[j]) })

	for i := 0; i+1 < len(reals); i += 2 {
		groups = append(groups, []complex128{reals[i], reals[i+1]})
	}

	if len(reals)%2 == 1 {
		groups = append(groups, []complex128{reals[len(reals)-1]})
	}

	return groups
}

func groupImagAbs(g []complex128) float64 {
	if len(g) == 0 {
		return 0
	}

	maxImag := 0.0
	for _, r := range g {
		if a := math.Abs(imag(r)); a > maxImag {
			maxImag = a
		}
	}

	return maxImag
}

func quadFromRoots(group []complex128) (float64, float64) {
	switch len(group) {
	case 0:
		return 0, 0
	case 1:
		r := group[0]
		return -real(r), 0
	default:
		r1, r2 := group[0], group[1]
		return -real(r1 + r2), real(r1 * r2)
	}
}

func complexProductNeg(v []complex128) complex128 {
	out := complex(1, 0)
	for _, x := range v {
		out *= -x
	}

	return out
}

func complexProductOneMinusK(v []complex128, k float64) complex128 {
	out := complex(1, 0)
	for _, x := range v {
		out *= 1.0 - complex(k, 0)*x
	}

	return out
}

func jacobiSCDFloat(uAbs, k, tol float64) (float64, float64, float64, bool) {
	if !(k >= 0 && k < 1) {
		return 0, 0, 0, false
	}

	K, _ := ellipticmath.EllipK(k, tol)
	if K == 0 || math.IsNaN(K) || math.IsInf(K, 0) {
		return 0, 0, 0, false
	}

	uNorm := uAbs / K

	sn := ellipticmath.SNE([]float64{uNorm}, k, tol)[0]
	if math.IsNaN(sn) || math.IsInf(sn, 0) {
		return 0, 0, 0, false
	}

	dn2 := 1.0 - k*k*sn*sn
	if dn2 < -1e-12 {
		return 0, 0, 0, false
	}

	if dn2 < 0 {
		dn2 = 0
	}

	dn := math.Sqrt(dn2)
	cd := real(ellipticmath.CDE(complex(uNorm, 0), k, tol))
	cn := cd * dn

	return sn, cn, dn, true
}

func arcJacSC1(w, m, tol float64) float64 {
	z := arcJacSN(complex(0, w), m, tol)
	if math.Abs(real(z)) > arcJacImagCheck*math.Max(1.0, math.Abs(imag(z))) {
		return math.NaN()
	}

	return imag(z)
}

func jacobiComplement(k complex128) complex128 {
	return cmplx.Sqrt((1.0 - k) * (1.0 + k))
}

func arcJacSN(w complex128, m, _ float64) complex128 {
	if m < 0 || m > 1 {
		return complex(math.NaN(), math.NaN())
	}

	k := complex(math.Sqrt(m), 0)
	if real(k) == 1 {
		return cmplx.Atanh(w)
	}

	ks := []complex128{k}
	for range arcJacSNMaxIter - 1 {
		kn := ks[len(ks)-1]
		if cmplx.Abs(kn) == 0 {
			break
		}

		kp := jacobiComplement(kn)
		ks = append(ks, (1.0-kp)/(1.0+kp))
	}

	K := 1.0
	for i := 1; i < len(ks); i++ {
		K *= real(1.0 + ks[i])
	}

	K *= math.Pi * 0.5

	wn := w

	for i := range len(ks) - 1 {
		kn := ks[i]
		knext := ks[i+1]

		den := (1.0 + knext) * (1.0 + jacobiComplement(kn*wn))
		if den == 0 {
			return complex(math.NaN(), math.NaN())
		}

		wn = 2.0 * wn / den
	}

	u := (2.0 / math.Pi) * cmplx.Asin(wn)

	return complex(K, 0) * u
}

func ellipdegParam(n int, m1, tol float64) float64 {
	if n <= 0 || !(m1 > 0 && m1 < 1) {
		return math.NaN()
	}

	k1 := math.Sqrt(m1)
	K1, _ := ellipticmath.EllipK(k1, tol)

	K1p, _ := ellipticmath.EllipK(math.Sqrt(1.0-m1), tol)
	if K1 <= 0 || K1p <= 0 || math.IsNaN(K1) || math.IsNaN(K1p) || math.IsInf(K1, 0) || math.IsInf(K1p, 0) {
		return math.NaN()
	}

	q1 := math.Exp(-math.Pi * K1p / K1)
	q := math.Pow(q1, 1.0/float64(n))

	num := 0.0
	for mnum := range ellipticSeriesLen {
		num += math.Pow(q, float64(mnum*(mnum+1)))
	}

	den := 1.0
	for mnum := 1; mnum < ellipticSeriesLen; mnum++ {
		den += 2.0 * math.Pow(q, float64(mnum*mnum))
	}

	return 16.0 * q * math.Pow(num/den, 4.0)
}

func dbToMinusOne(db float64) float64 {
	return math.Expm1(math.Ln10 * db / 10.0)
}

func normalizeCascadeLP(sections []biquad.Coefficients) {
	if len(sections) == 0 {
		return
	}

	gain := 1.0

	for _, s := range sections {
		den := 1 + s.A1 + s.A2
		if den == 0 {
			return
		}

		gain *= (s.B0 + s.B1 + s.B2) / den
	}

	if gain == 0 || math.IsNaN(gain) || math.IsInf(gain, 0) {
		return
	}

	sections[0].B0 /= gain
	sections[0].B1 /= gain
	sections[0].B2 /= gain
}

func normalizeCascadeHP(sections []biquad.Coefficients) {
	if len(sections) == 0 {
		return
	}

	gain := 1.0

	for _, s := range sections {
		den := 1 - s.A1 + s.A2
		if den == 0 {
			return
		}

		gain *= (s.B0 - s.B1 + s.B2) / den
	}

	if gain == 0 || math.IsNaN(gain) || math.IsInf(gain, 0) {
		return
	}

	sections[0].B0 /= gain
	sections[0].B1 /= gain
	sections[0].B2 /= gain
}

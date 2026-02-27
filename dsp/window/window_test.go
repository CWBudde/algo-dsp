package window

import (
	"math"
	"testing"
)

func TestGenerateTier1AndTier2(t *testing.T) {
	types := []Type{
		TypeRectangular,
		TypeHann,
		TypeHamming,
		TypeBlackman,
		TypeBlackmanHarris4Term,
		TypeFlatTop,
		TypeKaiser,
		TypeTukey,
		TypeTriangle,
		TypeCosine,
		TypeWelch,
		TypeLanczos,
		TypeGauss,
		TypeExactBlackman,
		TypeBlackmanHarris3Term,
		TypeBlackmanNuttall,
		TypeNuttallCTD,
		TypeNuttallCFD,
	}

	for _, typ := range types {
		t.Run(Info(typ).Name, func(t *testing.T) {
			w := Generate(typ, 64)
			if len(w) != 64 {
				t.Fatalf("len=%d, want 64", len(w))
			}

			for i, v := range w {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("coefficient[%d] invalid: %v", i, v)
				}
			}
		})
	}
}

func TestGenerateTier3(t *testing.T) {
	types := []Type{
		TypeLawrey5Term,
		TypeLawrey6Term,
		TypeBurgessOptimized59dB,
		TypeBurgessOptimized71dB,
		TypeAlbrecht2Term,
		TypeAlbrecht3Term,
		TypeAlbrecht4Term,
		TypeAlbrecht5Term,
		TypeAlbrecht6Term,
		TypeAlbrecht7Term,
		TypeAlbrecht8Term,
		TypeAlbrecht9Term,
		TypeAlbrecht10Term,
		TypeAlbrecht11Term,
		TypeFreeCosine,
	}

	for _, typ := range types {
		w := Generate(typ, 32, WithCustomCoeffs([]float64{0.5, -0.5}))
		if len(w) != 32 {
			t.Fatalf("type=%v len=%d want 32", typ, len(w))
		}
	}
}

func TestPeriodicDiffersFromSymmetric(t *testing.T) {
	a := Generate(TypeHann, 16)

	b := Generate(TypeHann, 16, WithPeriodic())
	if len(a) != 16 || len(b) != 16 {
		t.Fatalf("unexpected lengths: %d %d", len(a), len(b))
	}

	if almostEqual(a[15], b[15], 1e-12) {
		t.Fatal("expected different end coefficient for periodic form")
	}
}

func TestAdvancedOptions(t *testing.T) {
	wLeft := Generate(TypeHann, 32, WithSlope(SlopeLeft))
	wRight := Generate(TypeHann, 32, WithSlope(SlopeRight))
	wInv := Generate(TypeHann, 32, WithInvert())
	wDC := Generate(TypeHann, 32, WithDCRemoval())
	wBart := Generate(TypeTriangle, 32, WithBartlett())

	if wLeft[31] != 1 {
		t.Fatalf("left slope expected flat right tail, got %v", wLeft[31])
	}

	if wRight[0] != 1 {
		t.Fatalf("right slope expected flat left head, got %v", wRight[0])
	}

	if !almostEqual(wInv[0], 1, 1e-12) {
		t.Fatalf("invert expected first coeff near 1, got %v", wInv[0])
	}

	mean := 0.0
	for _, v := range wDC {
		mean += v
	}

	mean /= float64(len(wDC))
	if !almostEqual(mean, 0, 1e-12) {
		t.Fatalf("dc removal mean=%v, want 0", mean)
	}

	if wBart[0] != 0 {
		t.Fatalf("bartlett expected first coeff 0, got %v", wBart[0])
	}
}

func TestApplyInPlaceByType(t *testing.T) {
	buf := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	Apply(TypeRectangular, buf)

	for i, v := range buf {
		if v != float64(i+1) {
			t.Fatalf("rectangular should be passthrough at %d: %v", i, v)
		}
	}

	Apply(TypeHann, buf)

	if buf[0] != 0 {
		t.Fatalf("hann first sample should be 0, got %v", buf[0])
	}
}

func TestMetadataAndENBW(t *testing.T) {
	m := Info(TypeHann)
	if m.Name != "Hann" {
		t.Fatalf("name=%q", m.Name)
	}

	if !almostEqual(m.ENBW, 1.5, 0.01) {
		t.Fatalf("ENBW metadata=%v", m.ENBW)
	}

	w := Generate(TypeHann, 2048)

	enbw, err := EquivalentNoiseBandwidth(w)
	if err != nil {
		t.Fatalf("EquivalentNoiseBandwidth error: %v", err)
	}

	if !almostEqual(enbw, 1.5, 0.01) {
		t.Fatalf("hann ENBW=%v, want ~1.5", enbw)
	}
}

func TestMetadataParametricDefaultsPopulated(t *testing.T) {
	types := []Type{TypeKaiser, TypeTukey, TypeLanczos, TypeGauss}
	for _, typ := range types {
		m := Info(typ)
		if math.IsNaN(m.ENBW) || math.IsNaN(m.HighestSidelobe) || math.IsNaN(m.CoherentGain) || math.IsNaN(m.CoherentGainSquared) {
			t.Fatalf("metadata should be populated for type=%v: %#v", typ, m)
		}
	}
}

func TestCompatibilityWrappers(t *testing.T) {
	_, err := Hann(64)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Hamming(64)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Blackman(64)
	if err != nil {
		t.Fatal(err)
	}

	_, err = FlatTop(64)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Kaiser(64, 8)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Tukey(64, 0.5)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Gaussian(64, 0.4)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Lanczos(64)
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyCoefficientsHelpers(t *testing.T) {
	samples := []float64{1, 2, 3}
	coeffs := []float64{0.5, 0.5, 0.5}

	out, err := ApplyCoefficients(samples, coeffs)
	if err != nil {
		t.Fatal(err)
	}

	if !almostEqual(out[2], 1.5, 1e-12) {
		t.Fatalf("out[2]=%v", out[2])
	}

	err = ApplyCoefficientsInPlace(samples, coeffs)
	if err != nil {
		t.Fatal(err)
	}

	if !almostEqual(samples[1], 1.0, 1e-12) {
		t.Fatalf("samples[1]=%v", samples[1])
	}
}

func TestGoldenVectorsTier1(t *testing.T) {
	hannExpected := []float64{
		0.0, 0.1882550990706332, 0.6112604669781572, 0.9504844339512095,
		0.9504844339512095, 0.6112604669781573, 0.1882550990706333, 0.0,
	}
	hammingExpected := []float64{
		0.08, 0.25319469114498255, 0.6423596296199047, 0.9544456792351128,
		0.9544456792351128, 0.6423596296199048, 0.25319469114498266, 0.08,
	}
	bh4Expected := []float64{
		0.00006, 0.03339172347815117, 0.332833504298565,
		0.8893697722232837, 0.8893697722232838, 0.3328335042985652,
		0.0333917234781512, 0.00006,
	}
	flattopExpected := []float64{
		-0.0004210510000000013, -0.03684077608132298, 0.01070371671636002,
		0.7808739149387524, 0.7808739149387525, 0.010703716716360296,
		-0.03684077608132292, -0.0004210510000000013,
	}
	kaiserExpected := []float64{
		0.002338830460264423, 0.1091958100155291, 0.4871186737556569, 0.9261577358777303,
		0.9261577358777303, 0.4871186737556569, 0.1091958100155291, 0.002338830460264423,
	}

	checkGolden(t, Generate(TypeHann, 8), hannExpected, 1e-10)
	checkGolden(t, Generate(TypeHamming, 8), hammingExpected, 1e-10)
	checkGolden(t, Generate(TypeBlackmanHarris4Term, 8), bh4Expected, 1e-10)
	checkGolden(t, Generate(TypeFlatTop, 8), flattopExpected, 1e-8)
	checkGolden(t, Generate(TypeKaiser, 8, WithAlpha(8)), kaiserExpected, 1e-10)
}

func TestValidationAndEdgeCases(t *testing.T) {
	if got := Generate(TypeHann, 0); got != nil {
		t.Fatalf("expected nil for zero length, got %v", got)
	}

	_, err := Hann(0)
	if err == nil {
		t.Fatal("expected size validation error")
	}

	_, err = Kaiser(16, -1)
	if err == nil {
		t.Fatal("expected beta validation error")
	}

	_, err = Tukey(16, 2)
	if err == nil {
		t.Fatal("expected alpha validation error")
	}

	_, err = Gaussian(16, 0)
	if err == nil {
		t.Fatal("expected gauss alpha validation error")
	}

	_, err = EquivalentNoiseBandwidth(nil)
	if err == nil {
		t.Fatal("expected empty coeffs error")
	}

	_, err = EquivalentNoiseBandwidth([]float64{0, 0, 0})
	if err == nil {
		t.Fatal("expected zero coherent gain error")
	}

	_, err = ApplyCoefficients([]float64{1, 2}, []float64{1})
	if err == nil {
		t.Fatal("expected mismatch error")
	}

	err = ApplyCoefficientsInPlace([]float64{1, 2}, []float64{1})
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

func checkGolden(t *testing.T, got, want []float64, tol float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len mismatch got=%d want=%d", len(got), len(want))
	}

	for i := range got {
		if !almostEqual(got[i], want[i], tol) {
			t.Fatalf("index %d: got=%.16f want=%.16f", i, got[i], want[i])
		}
	}
}

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

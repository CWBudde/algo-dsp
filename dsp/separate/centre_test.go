package separate_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/separate"
	"github.com/cwbudde/algo-dsp/dsp/stft"
	"github.com/cwbudde/algo-dsp/internal/testutil"
)

func TestMidSideRoundTrip(t *testing.T) {
	t.Parallel()

	l := testutil.DeterministicNoise(1, 1, 4096)
	r := testutil.DeterministicNoise(2, 0.7, 4096)

	mid, side, err := separate.MidSide(l, r)
	if err != nil {
		t.Fatal(err)
	}

	l2, r2, err := separate.LeftRight(mid, side)
	if err != nil {
		t.Fatal(err)
	}

	for i := range l {
		if math.Abs(l2[i]-l[i]) > 1e-15 || math.Abs(r2[i]-r[i]) > 1e-15 {
			t.Fatalf("sample %d: got %v %v, want %v %v", i, l2[i], r2[i], l[i], r[i])
		}

		if mid[i] != 0.5*(l[i]+r[i]) || side[i] != 0.5*(l[i]-r[i]) {
			t.Fatalf("sample %d: mid/side %v %v", i, mid[i], side[i])
		}
	}

	// In place: mid over l, side over r.
	lc := append([]float64(nil), l...)
	rc := append([]float64(nil), r...)

	err = separate.MidSideInto(lc, rc, lc, rc)
	if err != nil {
		t.Fatal(err)
	}

	for i := range l {
		if lc[i] != mid[i] || rc[i] != side[i] {
			t.Fatalf("in-place sample %d differs", i)
		}
	}
}

func TestMidSideErrors(t *testing.T) {
	t.Parallel()

	a, b := make([]float64, 3), make([]float64, 4)

	_, _, err := separate.MidSide(a, b)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("MidSide: %v", err)
	}

	err = separate.MidSideInto(a, a, a, b)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("MidSideInto: %v", err)
	}

	err = separate.MidSideInto(b, a, a, a)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("MidSideInto dst: %v", err)
	}

	_, _, err = separate.LeftRight(a, b)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("LeftRight: %v", err)
	}
}

// stereoScene returns a centred 440 Hz tone, a hard-left 1 kHz tone and a
// hard-right 2.5 kHz tone mixed into left and right channels.
func stereoScene(fs float64, n int) (l, r, centre, left, right []float64) {
	centre = testutil.DeterministicSine(440, fs, 0.5, n)
	left = testutil.DeterministicSine(1000, fs, 0.3, n)
	right = testutil.DeterministicSine(2500, fs, 0.3, n)

	return add(centre, left), add(centre, right), centre, left, right
}

func TestCentreExtractorSeparatesPanning(t *testing.T) {
	t.Parallel()

	l, r, centre, left, right := stereoScene(24000, 24000)

	c, err := separate.NewCentreExtractor()
	if err != nil {
		t.Fatal(err)
	}

	gotC, sideL, sideR, err := c.SeparateSignal(l, r)
	if err != nil {
		t.Fatal(err)
	}

	errC := relErrDB(gotC, centre)
	errL := relErrDB(sideL, left)
	errR := relErrDB(sideR, right)
	t.Logf("centre %.1f dB, side L %.1f dB, side R %.1f dB", errC, errL, errR)

	for name, e := range map[string]float64{"centre": errC, "left": errL, "right": errR} {
		if e > -20 {
			t.Errorf("%s error %.1f dB, want < -20 dB", name, e)
		}
	}

	for i := range l {
		if math.Abs(gotC[i]+sideL[i]-l[i]) > 1e-15 || math.Abs(gotC[i]+sideR[i]-r[i]) > 1e-15 {
			t.Fatalf("sample %d does not reconstruct", i)
		}
	}
}

func TestCentreExtractorSpectrogram(t *testing.T) {
	t.Parallel()

	c, err := separate.NewCentreExtractor(
		separate.WithCentreExponent(1),
		separate.WithCentreSTFT(512, 128, stft.WithCenter(stft.PadReflect)),
	)
	if err != nil {
		t.Fatal(err)
	}

	left := [][]complex128{{1 + 1i, 2, 0, 1, 1}}
	right := [][]complex128{{1 + 1i, -2, 0, 1i, 0.5}}

	mask, err := c.Mask(left, right)
	if err != nil {
		t.Fatal(err)
	}

	// Identical, opposite, silent, 90° apart, 2:1 level (c = 2g/(1+g²)).
	want := []float64{1, 0, 0, 0, 0.8}
	for k, w := range want {
		if math.Abs(mask[0][k]-w) > 1e-15 {
			t.Fatalf("mask[%d] = %v, want %v", k, mask[0][k], w)
		}
	}

	into := [][]float64{make([]float64, 5)}

	err = c.MaskInto(into, left, right)
	if err != nil {
		t.Fatal(err)
	}

	for k := range want {
		if into[0][k] != mask[0][k] {
			t.Fatalf("MaskInto differs at %d", k)
		}
	}

	cs, sl, sr, err := c.Separate(left, right)
	if err != nil {
		t.Fatal(err)
	}

	for k := range left[0] {
		if cs[0][k]+sl[0][k] != left[0][k] || cs[0][k]+sr[0][k] != right[0][k] {
			t.Fatalf("bin %d does not reconstruct", k)
		}
	}

	if cs[0][0] != 1+1i || math.Abs(real(cs[0][4])-0.6) > 1e-15 || imag(cs[0][4]) != 0 {
		t.Fatalf("centre %v", cs[0])
	}

	// The default exponent is 2.
	c2, err := separate.NewCentreExtractor()
	if err != nil {
		t.Fatal(err)
	}

	m2, err := c2.Mask(left, right)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(m2[0][4]-0.64) > 1e-15 {
		t.Fatalf("exponent 2 mask %v, want 0.64", m2[0][4])
	}

	// A general exponent.
	c3, err := separate.NewCentreExtractor(separate.WithCentreExponent(0.5))
	if err != nil {
		t.Fatal(err)
	}

	m3, err := c3.Mask(left, right)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(m3[0][4]-math.Sqrt(0.8)) > 1e-15 {
		t.Fatalf("exponent 0.5 mask %v", m3[0][4])
	}

	if c.STFT().NFFT() != 512 || c.Clone().STFT().Hop() != 128 {
		t.Fatal("STFT settings not applied")
	}
}

func TestCentreExtractorErrors(t *testing.T) {
	t.Parallel()

	c, err := separate.NewCentreExtractor()
	if err != nil {
		t.Fatal(err)
	}

	a := [][]complex128{{1, 2}}
	b := [][]complex128{{1}}

	_, err = c.Mask(a, b)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("Mask: %v", err)
	}

	err = c.MaskInto([][]float64{{0, 0}}, a, b)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("MaskInto: %v", err)
	}

	err = c.MaskInto([][]float64{{0}}, a, a)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("MaskInto mask: %v", err)
	}

	_, _, _, err = c.Separate(a, b)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("Separate: %v", err)
	}

	_, _, _, err = c.SeparateSignal(make([]float64, 3), make([]float64, 4))
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Errorf("SeparateSignal: %v", err)
	}

	cr, err := separate.NewCentreExtractor(separate.WithCentreSTFT(256, 64, stft.WithCenter(stft.PadReflect)))
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, err = cr.SeparateSignal(make([]float64, 10), make([]float64, 10))
	if !errors.Is(err, stft.ErrSignalTooShort) {
		t.Errorf("short reflect: %v", err)
	}

	cn, err := separate.NewCentreExtractor(separate.WithCentreSTFT(256, 64, stft.WithCenter(stft.PadNone)))
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, err = cn.SeparateSignal(make([]float64, 1000), make([]float64, 1000))
	if !errors.Is(err, stft.ErrWindowSumZero) {
		t.Errorf("PadNone: %v", err)
	}
}

package separate_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/separate"
)

// exampleMix returns a 1 s, 24 kHz mixture of a steady 440 Hz tone and two
// clicks, and the two components.
func exampleMix() (mix, tone, clicks []float64) {
	const fs = 24000

	tone = make([]float64, fs)
	clicks = make([]float64, fs)

	for i := range tone {
		tone[i] = 0.2 * math.Sin(2*math.Pi*440*float64(i)/fs)
	}

	clicks[6000] = 4
	clicks[18000] = 4

	mix = make([]float64, fs)
	for i := range mix {
		mix[i] = tone[i] + clicks[i]
	}

	return mix, tone, clicks
}

func snrDB(got, want []float64) float64 {
	sig, noise := 0.0, 0.0

	for i := range want {
		d := got[i] - want[i]
		sig += want[i] * want[i]
		noise += d * d
	}

	return 10 * math.Log10(sig/noise)
}

func ExampleNewHPSS() {
	h, err := separate.NewHPSS(
		separate.WithKernels(31, 31),
		separate.WithPower(2),
		separate.WithSTFT(4096, 1024),
	)
	if err != nil {
		panic(err)
	}

	s := h.STFT()
	fmt.Println(s.NFFT(), s.Hop(), h.HasMargin())
	// Output:
	// 4096 1024 false
}

func ExampleHPSS_SeparateSignal() {
	mix, tone, clicks := exampleMix()

	h, err := separate.NewHPSS()
	if err != nil {
		panic(err)
	}

	harm, perc, _, err := h.SeparateSignal(mix)
	if err != nil {
		panic(err)
	}

	fmt.Printf("harmonic ≈ tone: %t\n", snrDB(harm, tone) > 20)
	fmt.Printf("percussive ≈ clicks: %t\n", snrDB(perc, clicks) > 10)

	maxErr := 0.0
	for i := range mix {
		maxErr = math.Max(maxErr, math.Abs(harm[i]+perc[i]-mix[i]))
	}

	fmt.Printf("outputs sum to input: %t\n", maxErr < 1e-12)
	// Output:
	// harmonic ≈ tone: true
	// percussive ≈ clicks: true
	// outputs sum to input: true
}

func ExampleWithMargin() {
	mix, _, _ := exampleMix()

	// Binary masks with margin 2: a bin is harmonic only if H > 2·P,
	// percussive only if P > 2·H; everything else is residual.
	h, err := separate.NewHPSS(separate.WithMargin(2, 2), separate.WithPower(math.Inf(1)))
	if err != nil {
		panic(err)
	}

	harm, perc, resid, err := h.SeparateSignal(mix)
	if err != nil {
		panic(err)
	}

	maxErr := 0.0
	for i := range mix {
		maxErr = math.Max(maxErr, math.Abs(harm[i]+perc[i]+resid[i]-mix[i]))
	}

	fmt.Printf("residual present: %t, sum to input: %t\n", h.HasMargin(), maxErr < 1e-12)
	// Output:
	// residual present: true, sum to input: true
}

func ExampleHPSS_Separate() {
	mix, _, _ := exampleMix()

	h, err := separate.NewHPSS()
	if err != nil {
		panic(err)
	}

	// Work on a spectrogram computed with the separator's own settings.
	s := h.STFT()

	spec, err := s.Forward(mix)
	if err != nil {
		panic(err)
	}

	harm, perc, _, err := h.Separate(spec)
	if err != nil {
		panic(err)
	}

	tone, err := s.Inverse(harm, len(mix))
	if err != nil {
		panic(err)
	}

	fmt.Println(len(spec), len(harm), len(perc), len(tone))
	// Output:
	// 47 47 47 24000
}

func ExampleHPSS_MasksInto() {
	mix, _, _ := exampleMix()

	h, err := separate.NewHPSS()
	if err != nil {
		panic(err)
	}

	spec, err := h.STFT().Forward(mix)
	if err != nil {
		panic(err)
	}

	// Allocate once with Masks, then reuse the matrices; MasksInto does not
	// allocate for a spectrogram of the same shape.
	mh, mp, mr, err := h.Masks(spec)
	if err != nil {
		panic(err)
	}

	err = h.MasksInto(mh, mp, mr, spec)
	if err != nil {
		panic(err)
	}

	// The tone lies between bins 37 and 38 (440 Hz·2048/24000 ≈ 37.5);
	// frame 12 is centred 144 samples after the first click.
	fmt.Printf("tone bin harmonic mask: %.2f\n", mh[20][38])
	fmt.Printf("click frame, high bin, percussive mask: %.2f\n", mp[12][800])
	fmt.Printf("sum: %.2f\n", mh[12][800]+mp[12][800]+mr[12][800])
	// Output:
	// tone bin harmonic mask: 1.00
	// click frame, high bin, percussive mask: 1.00
	// sum: 1.00
}

func ExampleMedianFilter_Filter() {
	m, err := separate.NewMedianFilter(3)
	if err != nil {
		panic(err)
	}

	src := []float64{1, 9, 2, 3, 8, 4}
	dst := make([]float64, len(src))

	err = m.Filter(dst, src)
	if err != nil {
		panic(err)
	}

	fmt.Println(dst)
	// Output:
	// [1 2 3 3 4 4]
}

func ExampleSoftMasks() {
	// Magnitude estimates of a target and a noise source, one frame of
	// three bins. Power 2 gives the Wiener gain S²/(S²+N²).
	target := [][]float64{{3, 1, 0}}
	noise := [][]float64{{4, 1, 0}}

	masks := [][][]float64{{make([]float64, 3)}, {make([]float64, 3)}}

	err := separate.SoftMasks(masks, [][][]float64{target, noise}, 2)
	if err != nil {
		panic(err)
	}

	fmt.Println(masks[0][0], masks[1][0])
	// Output:
	// [0.36 0.5 0.5] [0.64 0.5 0.5]
}

func ExampleApplyMask() {
	spec := [][]complex128{{2 + 2i, 4}}
	mask := [][]float64{{0.5, 0.25}}

	err := separate.ApplyMask(spec, spec, mask)
	if err != nil {
		panic(err)
	}

	fmt.Println(spec[0])
	// Output:
	// [(1+1i) (1+0i)]
}

func ExampleMagnitudes() {
	mag := [][]float64{make([]float64, 2)}

	err := separate.Magnitudes(mag, [][]complex128{{3 + 4i, -2}})
	if err != nil {
		panic(err)
	}

	fmt.Println(mag[0])
	// Output:
	// [5 2]
}

func ExampleMidSide() {
	mid, side, err := separate.MidSide([]float64{1, 0.5}, []float64{1, -0.5})
	if err != nil {
		panic(err)
	}

	fmt.Println(mid, side)
	// Output:
	// [1 0] [0 0.5]
}

func ExampleLeftRight() {
	l, r, err := separate.LeftRight([]float64{1, 0}, []float64{0, 0.5})
	if err != nil {
		panic(err)
	}

	fmt.Println(l, r)
	// Output:
	// [1 0.5] [1 -0.5]
}

func ExampleMidSideInto() {
	l := []float64{0.25, 1}
	r := []float64{0.75, 1}

	// Convert in place: l becomes mid, r becomes side.
	err := separate.MidSideInto(l, r, l, r)
	if err != nil {
		panic(err)
	}

	fmt.Println(l, r)
	// Output:
	// [0.5 1] [-0.25 0]
}

func ExampleCentreExtractor_SeparateSignal() {
	const fs, n = 24000, 24000

	l := make([]float64, n)
	r := make([]float64, n)

	for i := range l {
		voice := 0.5 * math.Sin(2*math.Pi*300*float64(i)/fs)  // centre
		guitar := 0.3 * math.Sin(2*math.Pi*900*float64(i)/fs) // hard left
		l[i] = voice + guitar
		r[i] = voice
	}

	c, err := separate.NewCentreExtractor()
	if err != nil {
		panic(err)
	}

	centre, sideL, sideR, err := c.SeparateSignal(l, r)
	if err != nil {
		panic(err)
	}

	rms := func(x []float64) float64 {
		s := 0.0
		for _, v := range x {
			s += v * v
		}

		return math.Sqrt(s / float64(len(x)))
	}

	fmt.Printf("centre %.2f, side L %.2f, side R %.2f\n", rms(centre), rms(sideL), rms(sideR))
	// Output:
	// centre 0.35, side L 0.21, side R 0.00
}

func ExampleCentreExtractor_Mask() {
	c, err := separate.NewCentreExtractor(separate.WithCentreExponent(1))
	if err != nil {
		panic(err)
	}

	// Identical bins → 1, opposite phase → 0, 2:1 level → 0.8.
	mask, err := c.Mask([][]complex128{{1, 1, 1}}, [][]complex128{{1, -1, 0.5}})
	if err != nil {
		panic(err)
	}

	fmt.Println(mask[0])
	// Output:
	// [1 0 0.8]
}

func ExampleHPSS_SeparateInto() {
	mix, _, _ := exampleMix()

	h, err := separate.NewHPSS()
	if err != nil {
		panic(err)
	}

	spec, err := h.STFT().Forward(mix)
	if err != nil {
		panic(err)
	}

	// Allocate the outputs once (Separate), then separate into them, for
	// example for every block of a long recording with the same shape. The
	// residual may overwrite the input spectrogram.
	harm, perc, _, err := h.Separate(spec)
	if err != nil {
		panic(err)
	}

	err = h.SeparateInto(harm, perc, spec, spec)
	if err != nil {
		panic(err)
	}

	fmt.Println(spec[0][0] == 0) // no margin: the residual is zero
	// Output:
	// true
}

func ExampleHPSS_Clone() {
	mix, _, _ := exampleMix()

	h, err := separate.NewHPSS()
	if err != nil {
		panic(err)
	}

	// An HPSS must not be shared between goroutines; give each its clone.
	done := make(chan []float64)

	for range 2 {
		c := h.Clone()

		go func() {
			harm, _, _, err := c.SeparateSignal(mix)
			if err != nil {
				panic(err)
			}

			done <- harm
		}()
	}

	a, b := <-done, <-done
	fmt.Println(a[1000] == b[1000])
	// Output:
	// true
}

func ExampleCentreExtractor_Separate() {
	c, err := separate.NewCentreExtractor(separate.WithCentreExponent(1))
	if err != nil {
		panic(err)
	}

	left := [][]complex128{{2, 1}}
	right := [][]complex128{{2, 0}}

	centre, sideL, sideR, err := c.Separate(left, right)
	if err != nil {
		panic(err)
	}

	fmt.Println(centre[0], sideL[0], sideR[0])
	// Output:
	// [(2+0i) (0+0i)] [(0+0i) (1+0i)] [(0+0i) (0+0i)]
}

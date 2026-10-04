package restoration

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/stft"
)

func tone(n int) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = 0.4*math.Sin(2*math.Pi*440*float64(i)/48000) + 0.1*math.Sin(2*math.Pi*1100*float64(i)/48000)
	}

	return x
}

func process(t *testing.T, x []float64, c SpectralConfig) []float64 {
	t.Helper()

	p, err := NewSpectralProcessor(int64(len(x)), func(dst []float64, start int64) int { return copy(dst, x[start:]) }, c)
	if err != nil {
		t.Fatal(err)
	}

	out := make([]float64, len(x))
	next := int64(0)

	for steps := 0; steps < 100000; steps++ {
		done, err := p.Step(context.Background(), func(at int64, y []float64) error {
			if at != next {
				t.Fatalf("noncontiguous output %d/%d", at, next)
			}

			copy(out[at:], y)
			next += int64(len(y))

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		if done {
			if next != int64(len(x)) {
				t.Fatal("short output")
			}

			return out
		}
	}

	t.Fatal("did not finish")

	return nil
}

func energy(x []float64) float64 {
	v := 0.0
	for _, s := range x {
		v += s * s
	}

	return v / float64(len(x))
}

func TestSpectralIdentityAndSelectiveAttenuation(t *testing.T) {
	for _, n := range []int{1, 257, 10000} {
		x := tone(n)

		y := process(t, x, SpectralConfig{Mode: "attenuate", SampleRate: 48000, FFTSize: 1024, Mask: Mask{End: int64(n), HighHz: 24000}})
		for i := range x {
			if math.Abs(x[i]-y[i]) > 1e-12 {
				t.Fatalf("identity n%d sample%d", n, i)
			}
		}
	}

	x := tone(48000)
	y := process(t, x, SpectralConfig{Mode: "remove", SampleRate: 48000, FFTSize: 2048, Mask: Mask{Start: 10000, End: 38000, LowHz: 300, HighHz: 600}})

	project := func(x []float64, hz float64) float64 {
		sum := 0.0
		for i := 12000; i < 36000; i++ {
			sum += x[i] * math.Sin(2*math.Pi*hz*float64(i)/48000)
		}

		return math.Abs(sum) * 2 / 24000
	}
	if project(y, 440) > 0.002 || math.Abs(project(y, 1100)-0.1) > 0.002 {
		t.Fatalf("selectivity %g/%g", project(y, 440), project(y, 1100))
	}

	for i := 0; i < 8000; i++ {
		if math.Abs(x[i]-y[i]) > 1e-12 {
			t.Fatal("outside changed")
		}
	}
}

func TestSpectralHealReferenceClick(t *testing.T) {
	x := tone(12000)

	damaged := append([]float64(nil), x...)
	for i := 6000; i < 6004; i++ {
		damaged[i] += 0.9
	}

	y := process(t, damaged, SpectralConfig{Mode: "heal", SampleRate: 48000, FFTSize: 1024, Mask: Mask{Start: 6000, End: 6004, HighHz: 24000}})
	errEnergy, maxErr := 0.0, 0.0

	for i := range x {
		e := y[i] - x[i]
		errEnergy += e * e
		maxErr = math.Max(maxErr, math.Abs(e))
	}

	db := 10 * math.Log10(errEnergy/float64(len(x)))
	t.Logf("reference click residual %.2f dBFS, max %.9g", db, maxErr)

	if db > -80 || maxErr > 1e-4 {
		t.Fatalf("click residual %g/%g", db, maxErr)
	}
}

func TestProfileLegacyPowerCaptureAndValidation(t *testing.T) {
	p, err := NewNoiseProfile(256, 48000)
	if err != nil {
		t.Fatal(err)
	}

	bins := make([]complex128, 129)
	bins[0] = 3
	bins[12] = complex(3, 4)

	bins[128] = 5
	if err = p.AddSpectrum(bins); err != nil {
		t.Fatal(err)
	}

	bins[0] = 1
	if err = p.AddSpectrum(bins); err != nil {
		t.Fatal(err)
	}

	powers := p.Powers()
	if powers[0] != 5 || powers[12] != 25 || powers[128] != 25 || p.Frames() != 2 {
		t.Fatalf("legacy mean powers %v", powers)
	}

	powers[12] = 0

	if p.Powers()[12] != 25 {
		t.Fatal("profile aliases")
	}

	bins[0] = complex(math.NaN(), 0)
	if p.AddSpectrum(bins) == nil || p.Frames() != 2 {
		t.Fatal("invalid capture mutated")
	}

	for _, method := range []string{"wiener", "subtraction", "gate"} {
		r, err := NewNoiseReducer(p, 24, method)
		if err != nil {
			t.Fatal(err)
		}

		bins[0] = 1
		if err = r.ProcessSpectrum(bins); err != nil {
			t.Fatal(err)
		}

		for _, b := range bins {
			if !finite(real(b)) || !finite(imag(b)) {
				t.Fatal("nonfinite")
			}
		}
	}
}

func TestStationaryNoiseAndMusicalNoiseProxy(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 7))

	noise := make([]float64, 96000)
	for i := range noise {
		noise[i] = 0.02 * rng.NormFloat64()
	}

	tr, _ := stft.New(2048, 512)

	spec, err := tr.Forward(noise[:24000])
	if err != nil {
		t.Fatal(err)
	}

	profile, _ := NewNoiseProfile(2048, 48000)
	for _, frame := range spec {
		if err = profile.AddSpectrum(frame); err != nil {
			t.Fatal(err)
		}
	}

	output := process(t, noise, SpectralConfig{Mode: "noise", SampleRate: 48000, FFTSize: 2048, Profile: profile, ReductionDB: 24, Method: "wiener"})
	reduction := 10 * math.Log10(energy(noise[24000:])/energy(output[24000:]))
	t.Logf("stationary noise reduction %.2f dB", reduction)

	if reduction < 15 {
		t.Fatal("noise reduction below 15 dB")
	}
	// Musical-noise proxy: bounded variation of residual power over 50ms blocks,
	// and no isolated narrow spectral line above 12x the neighbourhood mean.
	minE, maxE := math.Inf(1), 0.0

	for start := 24000; start+2400 < len(output); start += 2400 {
		e := energy(output[start : start+2400])
		minE = math.Min(minE, e)
		maxE = math.Max(maxE, e)
	}

	if 10*math.Log10(maxE/minE) > 6 {
		t.Fatalf("modulated noise %.2f dB", 10*math.Log10(maxE/minE))
	}

	residual, _ := tr.Forward(output[24000:])
	mean := make([]float64, 1025)

	for _, frame := range residual {
		for k, b := range frame {
			mean[k] += (real(b)*real(b) + imag(b)*imag(b)) / float64(len(residual))
		}
	}

	for k := 5; k < len(mean)-5; k++ {
		local := 0.0

		for j := k - 5; j <= k+5; j++ {
			if j != k {
				local += mean[j] / 10
			}
		}

		if mean[k] > 12*local {
			t.Fatalf("isolated residual line at bin%d", k)
		}
	}
}

func TestRepairClicksAndDeclip(t *testing.T) {
	original := tone(4000)

	for _, kind := range []string{"click", "clip"} {
		t.Run(kind, func(t *testing.T) {
			x := append([]float64(nil), original...)
			before := 0.0

			if kind == "click" {
				x[2000] += 1
				before = 1

				if err := RepairClicks(x, 8, 32); err != nil {
					t.Fatal(err)
				}
			} else {
				for i := range x {
					x[i] = math.Max(-0.25, math.Min(0.25, x[i]))
					e := x[i] - original[i]
					before += e * e
				}

				if err := Declip(x, 0.25, 64); err != nil {
					t.Fatal(err)
				}
			}

			after := 0.0

			for i := range x {
				e := x[i] - original[i]
				after += e * e
			}

			t.Logf("repair energy before%g after%g", before, after)

			if after >= before*0.05 {
				t.Fatalf("repair failed %g/%g", after, before)
			}
		})
	}
}

func TestHumComb(t *testing.T) {
	h, err := NewHumRemover(48000, 50, 30, 8)
	if err != nil {
		t.Fatal(err)
	}

	x := make([]float64, 144000)
	for i := range x {
		for k := 1; k <= 8; k++ {
			x[i] += 0.03 * math.Sin(2*math.Pi*50*float64(k*i)/48000)
		}

		x[i] += 0.2 * math.Sin(2*math.Pi*1500*float64(i)/48000)
	}

	if err = h.ProcessInPlace(x); err != nil {
		t.Fatal(err)
	}

	projection := func(hz float64) float64 {
		sum := 0.0
		for i := 96000; i < len(x); i++ {
			sum += x[i] * math.Sin(2*math.Pi*hz*float64(i)/48000)
		}

		return math.Abs(sum) * 2 / 48000
	}
	for k := 1; k <= 8; k++ {
		if projection(50*float64(k)) > 0.0001 {
			t.Fatalf("hum harmonic%d: %g", k, projection(50*float64(k)))
		}
	}

	if projection(1500) < 0.19 {
		t.Fatal("wanted signal lost")
	}
}

func TestCancellationAndBadSource(t *testing.T) {
	p, err := NewSpectralProcessor(1024, func(dst []float64, _ int64) int { return len(dst) }, SpectralConfig{Mode: "remove", FFTSize: 1024, SampleRate: 48000, Mask: Mask{End: 1024, HighHz: 24000}})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = p.Step(ctx, func(int64, []float64) error { t.Fatal("cancel emitted"); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	mask := Mask{End: 100, LowHz: 100, HighHz: 1000, Points: []Point{{0, 100}, {100, 100}, {50, 1000}}}
	if mask.Validate(100, 48000) != nil || !mask.Contains(50, 500) || mask.Contains(90, 900) {
		t.Fatal("polygon mask")
	}
}

func ExampleNoiseProfile() {
	p, _ := NewNoiseProfile(256, 48000)
	bins := make([]complex128, 129)
	bins[0] = 3
	_ = p.AddSpectrum(bins)
	bins[0] = 1
	_ = p.AddSpectrum(bins)
	fmt.Println(p.Frames(), p.Powers()[0])
	// Output: 2 5
}

func TestWantedToneSurvivesReduction(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 29))

	noise := make([]float64, 48000)
	for i := range noise {
		noise[i] = 0.01 * rng.NormFloat64()
	}

	tr, _ := stft.New(2048, 512)
	spec, _ := tr.Forward(noise)

	profile, _ := NewNoiseProfile(2048, 48000)
	for _, b := range spec {
		_ = profile.AddSpectrum(b)
	}

	mixed := make([]float64, 48000)
	for i := range mixed {
		mixed[i] = noise[i] + 0.4*math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	out := process(t, mixed, SpectralConfig{Mode: "noise", FFTSize: 2048, SampleRate: 48000, Profile: profile, Method: "wiener", ReductionDB: 24})

	amplitude := 0.0
	for i := 12000; i < 36000; i++ {
		amplitude += out[i] * math.Sin(2*math.Pi*440*float64(i)/48000) * 2 / 24000
	}

	if math.Abs(amplitude-0.4) > 0.025 {
		t.Fatalf("wanted amplitude %g", amplitude)
	}
}

func BenchmarkNoiseReducer(b *testing.B) {
	p, _ := NewNoiseProfile(2048, 48000)

	bins := make([]complex128, 1025)
	for i := range bins {
		bins[i] = 1
	}

	_ = p.AddSpectrum(bins)
	_ = p.AddSpectrum(bins)
	r, _ := NewNoiseReducer(p, 24, "wiener")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for i := range bins {
			bins[i] = 1
		}

		_ = r.ProcessSpectrum(bins)
	}
}

func BenchmarkHumRemover(b *testing.B) {
	h, _ := NewHumRemover(48000, 50, 30, 8)
	x := make([]float64, 4096)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = h.ProcessInPlace(x)
	}
}

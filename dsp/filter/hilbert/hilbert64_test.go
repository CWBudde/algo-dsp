package hilbert

import (
	"math"
	"testing"
)

func TestDesignCoefficientsDefaultLegacyValues(t *testing.T) {
	coeffs, err := DesignCoefficients(8, 0.1)
	if err != nil {
		t.Fatalf("DesignCoefficients() error = %v", err)
	}

	expected := []float64{
		0.023096747350551,
		0.089078664601642,
		0.189272682580641,
		0.312728886474479,
		0.449678547451987,
		0.594162288373581,
		0.745542009557270,
		0.909477237310187,
	}
	for i := range expected {
		if d := math.Abs(coeffs[i] - expected[i]); d > 1e-12 {
			t.Fatalf("coefficient[%d] = %.15f, want %.15f", i, coeffs[i], expected[i])
		}
	}

	att, err := AttenuationFromOrderTBW(8, 0.1)
	if err != nil {
		t.Fatalf("AttenuationFromOrderTBW() error = %v", err)
	}

	if math.Abs(att-137.6567258508839) > 1e-9 {
		t.Fatalf("attenuation = %.12f, want %.12f", att, 137.6567258508839)
	}
}

func TestDesignValidation(t *testing.T) {
	if _, err := DesignCoefficients(0, 0.1); err == nil {
		t.Fatal("expected error for coefficient count < 1")
	}

	if _, err := DesignCoefficients(8, 0); err == nil {
		t.Fatal("expected error for transition=0")
	}

	if _, err := DesignCoefficients(8, 0.5); err == nil {
		t.Fatal("expected error for transition >= 0.5")
	}
}

func TestSetCoefficientsValidation64(t *testing.T) {
	p, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	if err := p.SetCoefficients(nil); err == nil {
		t.Fatal("expected error for empty coefficients")
	}

	if err := p.SetCoefficients([]float64{1.01}); err == nil {
		t.Fatal("expected error for unstable coefficient")
	}
}

func TestProcessBlockMatchesSample64(t *testing.T) {
	pBlock, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	pSample, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	const n = 1024

	input := make([]float64, n)
	for i := range input {
		input[i] = 0.71*math.Sin(2*math.Pi*float64(i)/31.0) + 0.13*math.Sin(2*math.Pi*float64(i)/7.0)
	}

	gotA := make([]float64, n)

	gotB := make([]float64, n)
	if err := pBlock.ProcessBlock(input, gotA, gotB); err != nil {
		t.Fatalf("ProcessBlock() error = %v", err)
	}

	for i, x := range input {
		wantA, wantB := pSample.ProcessSample(x)
		if d := math.Abs(gotA[i] - wantA); d > 1e-12 {
			t.Fatalf("A[%d] mismatch: got=%g want=%g", i, gotA[i], wantA)
		}

		if d := math.Abs(gotB[i] - wantB); d > 1e-12 {
			t.Fatalf("B[%d] mismatch: got=%g want=%g", i, gotB[i], wantB)
		}
	}
}

func TestProcessEnvelopeMatchesSampleHypot64(t *testing.T) {
	pEnv, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	pAB, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	for i := range 1024 {
		x := 0.8*math.Sin(2*math.Pi*float64(i)/37.0) + 0.2*math.Sin(2*math.Pi*float64(i)/13.0)
		env := pEnv.ProcessEnvelopeSample(x)
		a, b := pAB.ProcessSample(x)

		want := math.Hypot(a, b)
		if d := math.Abs(env - want); d > 1e-12 {
			t.Fatalf("sample %d: envelope mismatch got=%g want=%g", i, env, want)
		}
	}
}

func TestProcessBlockLengthMismatch64(t *testing.T) {
	p, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	if err := p.ProcessBlock(make([]float64, 4), make([]float64, 3), make([]float64, 4)); err == nil {
		t.Fatal("expected length mismatch error")
	}

	if err := p.ProcessEnvelopeBlock(make([]float64, 4), make([]float64, 3)); err == nil {
		t.Fatal("expected envelope length mismatch error")
	}
}

func TestLegacyImpulseParity64(t *testing.T) {
	p, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	wantA := []float64{
		0.0014655918815034,
		0,
		-0.0743598073184986,
		0,
		0.4879162578048107,
		0,
		-0.6765628079406600,
		0,
		-0.2475638866104890,
		0,
		0.0904374392323063,
		0,
		0.2217967641781544,
		0,
		0.2406004812300144,
		0,
	}
	wantB := []float64{
		0,
		0.0150535390574415,
		0,
		-0.2303314631594787,
		0,
		0.7158749568984888,
		0,
		-0.2674436268192149,
		0,
		-0.4087833477429624,
		0,
		-0.2764721286585438,
		0,
		-0.1275904465638322,
		0,
		-0.0208337565399074,
	}

	for i := range wantA {
		in := 0.0
		if i == 0 {
			in = 1.0
		}

		a, b := p.ProcessSample(in)
		if math.Abs(a-wantA[i]) > 1e-12 {
			t.Fatalf("A[%d] = %.16f, want %.16f", i, a, wantA[i])
		}

		if math.Abs(b-wantB[i]) > 1e-12 {
			t.Fatalf("B[%d] = %.16f, want %.16f", i, b, wantB[i])
		}
	}
}

func TestQuadraturePhase64(t *testing.T) {
	freqs := []float64{1000, 2000, 5000, 10000, 15000}
	for _, freq := range freqs {
		phaseDeg := estimatePhaseDeltaDeg(t, freq)

		errDeg := math.Abs(math.Abs(phaseDeg) - 90)
		if errDeg > 2.0 {
			t.Fatalf("freq %.0f Hz: phase delta = %.3f deg (error %.3f deg)", freq, phaseDeg, errDeg)
		}
	}
}

func TestImageRejection64(t *testing.T) {
	thresholds := map[float64]float64{
		1000:  35,
		2000:  70,
		5000:  80,
		10000: 80,
	}
	for freq, minDB := range thresholds {
		rej := estimateImageRejectionDB(t, freq)
		if rej < minDB {
			t.Fatalf("freq %.0f Hz: image rejection %.2f dB, want >= %.2f dB", freq, rej, minDB)
		}
	}
}

func TestAnalyticMagnitudeMatchesAmplitude64(t *testing.T) {
	const (
		sampleRate = 44100.0
		warmup     = 4000
		n          = 32000
		amp        = 0.8
		maxRelErr  = 0.03
	)

	freqs := []float64{1000, 2000, 5000, 10000}
	for _, freqHz := range freqs {
		p, err := New64Default()
		if err != nil {
			t.Fatalf("New64Default() error = %v", err)
		}

		w := 2 * math.Pi * freqHz / sampleRate
		sum := 0.0
		count := 0

		for i := range n {
			x := amp * math.Sin(w*float64(i))
			env := p.ProcessEnvelopeSample(x)

			if i < warmup {
				continue
			}

			sum += env
			count++
		}

		mean := sum / float64(count)

		relErr := math.Abs(mean-amp) / amp
		if relErr > maxRelErr {
			t.Fatalf("freq %.0f Hz: mean envelope %.6f, want ~%.6f (rel err %.4f > %.4f)", freqHz, mean, amp, relErr, maxRelErr)
		}
	}
}

func estimatePhaseDeltaDeg(t *testing.T, freqHz float64) float64 {
	t.Helper()

	p, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	const (
		sampleRate = 44100.0
		n          = 28000
		warmup     = 4000
	)

	w := 2 * math.Pi * freqHz / sampleRate

	var (
		aSin, aCos float64
		bSin, bCos float64
	)

	for i := range n {
		x := math.Sin(w * float64(i))
		a, b := p.ProcessSample(x)

		if i < warmup {
			continue
		}

		s := math.Sin(w * float64(i))
		c := math.Cos(w * float64(i))
		aSin += a * s
		aCos += a * c
		bSin += b * s
		bCos += b * c
	}

	phaseA := math.Atan2(aCos, aSin)
	phaseB := math.Atan2(bCos, bSin)

	delta := (phaseB - phaseA) * 180 / math.Pi
	for delta > 180 {
		delta -= 360
	}

	for delta < -180 {
		delta += 360
	}

	return delta
}

func estimateImageRejectionDB(t *testing.T, freqHz float64) float64 {
	t.Helper()

	p, err := New64Default()
	if err != nil {
		t.Fatalf("New64Default() error = %v", err)
	}

	const (
		sampleRate = 44100.0
		n          = 32000
		warmup     = 3000
	)

	w := 2 * math.Pi * freqHz / sampleRate

	var (
		wantedR, wantedI float64
		imageR, imageI   float64
	)

	for i := range n {
		x := math.Cos(w * float64(i))
		a, b := p.ProcessSample(x)

		if i < warmup {
			continue
		}

		theta := w * float64(i)
		c, s := math.Cos(theta), math.Sin(theta)

		wantedR += a*c + b*s
		wantedI += b*c - a*s

		imageR += a*c - b*s
		imageI += b*c + a*s
	}

	wanted := math.Hypot(wantedR, wantedI)
	image := math.Hypot(imageR, imageI)

	return 20 * math.Log10(wanted/image)
}

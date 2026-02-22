package effects

import (
	"math"
	"testing"
)

func TestTransformerSimulationValidation(t *testing.T) {
	if _, err := NewTransformerSimulation(0); err == nil {
		t.Fatal("expected error for invalid sample rate")
	}

	if _, err := NewTransformerSimulation(48000, WithTransformerOversampling(3)); err == nil {
		t.Fatal("expected error for invalid oversampling factor")
	}

	if _, err := NewTransformerSimulation(48000, WithTransformerHighpassHz(0)); err == nil {
		t.Fatal("expected error for invalid high-pass frequency")
	}

	if _, err := NewTransformerSimulation(48000, WithTransformerDampingHz(50)); err == nil {
		t.Fatal("expected error for invalid damping frequency")
	}
}

func TestTransformerSimulationProcessInPlaceMatchesSample(t *testing.T) {
	t1, err := NewTransformerSimulation(48000,
		WithTransformerQuality(TransformerQualityHigh),
		WithTransformerOversampling(4),
	)
	if err != nil {
		t.Fatalf("NewTransformerSimulation() error = %v", err)
	}

	t2, err := NewTransformerSimulation(48000,
		WithTransformerQuality(TransformerQualityHigh),
		WithTransformerOversampling(4),
	)
	if err != nil {
		t.Fatalf("NewTransformerSimulation() error = %v", err)
	}

	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = math.Sin(2 * math.Pi * float64(i) / 53)
	}

	want := make([]float64, len(buf))
	for i := range buf {
		want[i] = t1.ProcessSample(buf[i])
	}

	got := append([]float64(nil), buf...)
	t2.ProcessInPlace(got)

	for i := range got {
		if d := math.Abs(got[i] - want[i]); d > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, got[i], want[i])
		}
	}
}

func TestTransformerSimulationResetDeterministic(t *testing.T) {
	ts, err := NewTransformerSimulation(48000,
		WithTransformerQuality(TransformerQualityHigh),
		WithTransformerOversampling(4),
	)
	if err != nil {
		t.Fatalf("NewTransformerSimulation() error = %v", err)
	}

	in := make([]float64, 160)
	for i := range in {
		in[i] = math.Sin(2*math.Pi*float64(i)/31) + 0.3*math.Sin(2*math.Pi*float64(i)/11)
	}

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = ts.ProcessSample(in[i])
	}

	ts.Reset()

	for i := range in {
		out2 := ts.ProcessSample(in[i])
		if math.Abs(out2-out1[i]) > 1e-12 {
			t.Fatalf("reset mismatch at %d: got=%g want=%g", i, out2, out1[i])
		}
	}
}

func TestTransformerSimulationMixZeroTransparent(t *testing.T) {
	ts, err := NewTransformerSimulation(48000,
		WithTransformerDrive(20),
		WithTransformerMix(0),
	)
	if err != nil {
		t.Fatalf("NewTransformerSimulation() error = %v", err)
	}

	for i := 0; i < 256; i++ {
		in := math.Sin(2 * math.Pi * 440 * float64(i) / 48000)

		out := ts.ProcessSample(in)
		if math.Abs(out-in) > 1e-12 {
			t.Fatalf("mix=0 should be transparent at sample %d", i)
		}
	}
}

func TestTransformerSimulationSampleRateAwareUpdate(t *testing.T) {
	ts, err := NewTransformerSimulation(48000,
		WithTransformerHighpassHz(30),
		WithTransformerDampingHz(10000),
	)
	if err != nil {
		t.Fatalf("NewTransformerSimulation() error = %v", err)
	}

	baseline := ts.ProcessSample(0.5)
	if err := ts.SetSampleRate(96000); err != nil {
		t.Fatalf("SetSampleRate() error = %v", err)
	}

	ts.Reset()
	after := ts.ProcessSample(0.5)

	// Should remain finite and generally differ due filter coefficient update.
	if math.IsNaN(after) || math.IsInf(after, 0) {
		t.Fatalf("invalid output after sample-rate update: %g", after)
	}

	if math.Abs(after-baseline) < 1e-6 {
		t.Fatalf("expected output to change after sample-rate update: baseline=%g after=%g", baseline, after)
	}
}

func TestTransformerSimulationHighQualityReducesAliasingSpurs(t *testing.T) {
	const (
		sr = 48000.0
		n  = 2048
		k0 = 960 // 0.46875*fs near Nyquist
	)

	hq, err := NewTransformerSimulation(sr,
		WithTransformerQuality(TransformerQualityHigh),
		WithTransformerDrive(18),
		WithTransformerMix(1),
		WithTransformerOversampling(8),
	)
	if err != nil {
		t.Fatalf("NewTransformerSimulation(high) error = %v", err)
	}

	lw, err := NewTransformerSimulation(sr,
		WithTransformerQuality(TransformerQualityLightweight),
		WithTransformerDrive(18),
		WithTransformerMix(1),
	)
	if err != nil {
		t.Fatalf("NewTransformerSimulation(lightweight) error = %v", err)
	}

	in := make([]float64, n)
	outHQ := make([]float64, n)

	outLW := make([]float64, n)
	for i := 0; i < n; i++ {
		in[i] = 0.8 * math.Sin(2*math.Pi*float64(k0)*float64(i)/n)
		outHQ[i] = hq.ProcessSample(in[i])
		outLW[i] = lw.ProcessSample(in[i])
	}

	spurHQ := spurRatio(outHQ, k0)
	spurLW := spurRatio(outLW, k0)

	if spurHQ >= spurLW*0.97 {
		t.Fatalf("expected high-quality mode to reduce spurs: hq=%g lw=%g", spurHQ, spurLW)
	}
}

func spurRatio(x []float64, fundamentalBin int) float64 {
	fund := dftBinEnergy(x, fundamentalBin)
	if fund <= 0 {
		return math.Inf(1)
	}

	spur := 0.0

	for k := 1; k <= len(x)/2; k++ {
		if k == fundamentalBin {
			continue
		}

		spur += dftBinEnergy(x, k)
	}

	return spur / fund
}

func dftBinEnergy(x []float64, k int) float64 {
	n := float64(len(x))

	var re, im float64

	for i := range x {
		phase := 2 * math.Pi * float64(k) * float64(i) / n
		re += x[i] * math.Cos(phase)
		im -= x[i] * math.Sin(phase)
	}

	return re*re + im*im
}

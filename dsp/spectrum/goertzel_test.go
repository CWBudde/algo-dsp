package spectrum

import (
	"math"
	"math/cmplx"
	"testing"
)

// directDFTBin computes the DFT value X[bin] = sum x[i]*exp(-j*2*pi*bin*i/N).
func directDFTBin(samples []float64, bin int) complex128 {
	size := len(samples)

	var acc complex128

	for i, v := range samples {
		angle := -2 * math.Pi * float64(bin) * float64(i) / float64(size)
		acc += complex(v, 0) * cmplx.Exp(complex(0, angle))
	}

	return acc
}

func cosineBlock(size int, amp, cyclesPerBlock, phase float64) []float64 {
	buf := make([]float64, size)
	for i := range buf {
		buf[i] = amp * math.Cos(2*math.Pi*cyclesPerBlock*float64(i)/float64(size)+phase)
	}

	return buf
}

func TestGoertzelDFTParity(t *testing.T) {
	const (
		sampleRate = 8000.0
		blockSize  = 256
	)

	for _, bin := range []int{1, 5, 32, 100, 127} {
		freq := float64(bin) * sampleRate / blockSize
		samples := cosineBlock(blockSize, 0.8, float64(bin), 0.4)

		analyzer, err := NewGoertzel(freq, sampleRate)
		if err != nil {
			t.Fatalf("NewGoertzel(%g): %v", freq, err)
		}

		analyzer.Reset()
		analyzer.ProcessBlock(samples)

		want := cmplx.Abs(directDFTBin(samples, bin))
		got := cmplx.Abs(analyzer.Complex())

		if diff := math.Abs(got - want); diff > 1e-9 {
			t.Errorf("bin=%d: magnitude got=%g want=%g diff=%g", bin, got, want, diff)
		}

		// Magnitude() and |Complex()| must agree.
		if diff := math.Abs(analyzer.Magnitude() - got); diff > 1e-9 {
			t.Errorf("bin=%d: Magnitude=%g != |Complex|=%g", bin, analyzer.Magnitude(), got)
		}
	}
}

func TestGoertzelNormalizedMagnitude(t *testing.T) {
	const (
		sampleRate = 48000.0
		blockSize  = 480 // 10 cycles of 1000 Hz
		amp        = 0.5
	)

	freq := 1000.0
	cycles := freq * blockSize / sampleRate // = 10

	analyzer, err := NewGoertzel(freq, sampleRate)
	if err != nil {
		t.Fatalf("NewGoertzel: %v", err)
	}

	analyzer.ProcessBlock(cosineBlock(blockSize, amp, cycles, 0))

	if diff := math.Abs(analyzer.NormalizedMagnitude(blockSize) - amp); diff > 1e-9 {
		t.Errorf("NormalizedMagnitude=%g want=%g", analyzer.NormalizedMagnitude(blockSize), amp)
	}

	if analyzer.NormalizedMagnitude(0) != 0 {
		t.Errorf("NormalizedMagnitude(0)=%g want 0", analyzer.NormalizedMagnitude(0))
	}
}

func TestGoertzelOffBinDiscrimination(t *testing.T) {
	const (
		sampleRate = 8000.0
		blockSize  = 800
	)

	tone := 1000.0
	samples := cosineBlock(blockSize, 1.0, tone*blockSize/sampleRate, 0)

	matched, _ := NewGoertzel(tone, sampleRate)
	detuned, _ := NewGoertzel(tone+200, sampleRate)

	matched.ProcessBlock(samples)
	detuned.ProcessBlock(samples)

	if matched.Power() <= detuned.Power() {
		t.Errorf("matched power %g should exceed detuned power %g", matched.Power(), detuned.Power())
	}

	// Matched bin should dominate by a wide margin.
	if matched.Power() < 100*detuned.Power() {
		t.Errorf("matched/detuned ratio too small: %g vs %g", matched.Power(), detuned.Power())
	}
}

func TestGoertzelPowerFormula(t *testing.T) {
	analyzer, _ := NewGoertzel(440, 44100)
	analyzer.ProcessBlock(cosineBlock(128, 0.7, 7, 0.2))

	want := analyzer.s0*analyzer.s0 + analyzer.s1*analyzer.s1 - analyzer.coef*analyzer.s0*analyzer.s1
	if diff := math.Abs(analyzer.Power() - want); diff > 1e-12 {
		t.Errorf("Power()=%g, legacy formula=%g", analyzer.Power(), want)
	}
}

func TestGoertzelEdgeCases(t *testing.T) {
	t.Run("silence", func(t *testing.T) {
		analyzer, _ := NewGoertzel(1000, 48000)
		analyzer.ProcessBlock(make([]float64, 256))

		if analyzer.Power() != 0 {
			t.Errorf("silence power=%g want 0", analyzer.Power())
		}

		if db := analyzer.DB(); math.IsInf(db, 0) || math.IsNaN(db) {
			t.Errorf("silence DB=%g must be finite", db)
		}
	})

	t.Run("near DC and near Nyquist", func(t *testing.T) {
		sampleRate := 48000.0
		for _, freq := range []float64{1.0, sampleRate/2 - 1.0} {
			analyzer, err := NewGoertzel(freq, sampleRate)
			if err != nil {
				t.Fatalf("NewGoertzel(%g): %v", freq, err)
			}

			analyzer.ProcessBlock(cosineBlock(1024, 1.0, freq*1024/sampleRate, 0))

			if math.IsNaN(analyzer.Power()) || math.IsInf(analyzer.Power(), 0) {
				t.Errorf("freq=%g power not finite: %g", freq, analyzer.Power())
			}
		}
	})

	t.Run("very short block", func(t *testing.T) {
		analyzer, _ := NewGoertzel(1000, 8000)
		analyzer.ProcessBlock([]float64{1})

		if math.IsNaN(analyzer.Power()) {
			t.Error("single-sample power is NaN")
		}
	})

	t.Run("large amplitude", func(t *testing.T) {
		analyzer, _ := NewGoertzel(1000, 8000)
		analyzer.ProcessBlock(cosineBlock(256, 1e6, 1000*256/8000, 0))

		if math.IsInf(analyzer.Power(), 0) {
			t.Error("large-amplitude power overflowed")
		}
	})
}

func TestGoertzelValidation(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"zero sample rate", func() error { _, err := NewGoertzel(1000, 0); return err }},
		{"negative sample rate", func() error { _, err := NewGoertzel(1000, -1); return err }},
		{"NaN sample rate", func() error { _, err := NewGoertzel(1000, math.NaN()); return err }},
		{"Inf sample rate", func() error { _, err := NewGoertzel(1000, math.Inf(1)); return err }},
		{"zero frequency", func() error { _, err := NewGoertzel(0, 48000); return err }},
		{"negative frequency", func() error { _, err := NewGoertzel(-100, 48000); return err }},
		{"NaN frequency", func() error { _, err := NewGoertzel(math.NaN(), 48000); return err }},
		{"Inf frequency", func() error { _, err := NewGoertzel(math.Inf(1), 48000); return err }},
		{"frequency at Nyquist", func() error { _, err := NewGoertzel(24000, 48000); return err }},
		{"frequency above Nyquist", func() error { _, err := NewGoertzel(30000, 48000); return err }},
		{"zero power floor", func() error { _, err := NewGoertzel(1000, 48000, WithGoertzelPowerFloor(0)); return err }},
		{"negative power floor", func() error { _, err := NewGoertzel(1000, 48000, WithGoertzelPowerFloor(-1)); return err }},
		{"SetFrequency above Nyquist", func() error {
			analyzer, _ := NewGoertzel(1000, 48000)
			return analyzer.SetFrequency(30000)
		}},
		{"SetFrequency zero", func() error {
			analyzer, _ := NewGoertzel(1000, 48000)
			return analyzer.SetFrequency(0)
		}},
		{"SetSampleRate dropping below frequency", func() error {
			analyzer, _ := NewGoertzel(10000, 48000)
			return analyzer.SetSampleRate(1000) // Nyquist 500 < 10000
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestGoertzelSettersValid(t *testing.T) {
	analyzer, _ := NewGoertzel(1000, 48000)

	if err := analyzer.SetFrequency(2000); err != nil {
		t.Fatalf("SetFrequency: %v", err)
	}

	if analyzer.Frequency() != 2000 {
		t.Errorf("Frequency()=%g want 2000", analyzer.Frequency())
	}

	if err := analyzer.SetSampleRate(44100); err != nil {
		t.Fatalf("SetSampleRate: %v", err)
	}

	if analyzer.SampleRate() != 44100 {
		t.Errorf("SampleRate()=%g want 44100", analyzer.SampleRate())
	}
}

func TestGoertzelReset(t *testing.T) {
	analyzer, _ := NewGoertzel(1000, 48000)
	samples := cosineBlock(256, 0.5, 1000*256/48000, 0.1)

	analyzer.ProcessBlock(samples)
	first := analyzer.Power()

	analyzer.Reset()

	if analyzer.Power() != 0 {
		t.Errorf("after Reset power=%g want 0", analyzer.Power())
	}

	analyzer.ProcessBlock(samples)

	if diff := math.Abs(analyzer.Power() - first); diff > 1e-12 {
		t.Errorf("power after reset+reprocess=%g want=%g", analyzer.Power(), first)
	}
}

func TestGoertzelProcessSamplePassthrough(t *testing.T) {
	analyzer, _ := NewGoertzel(1000, 48000)
	for _, v := range []float64{0.1, -0.3, 0.7} {
		if out := analyzer.ProcessSample(v); out != v {
			t.Errorf("ProcessSample(%g)=%g want passthrough", v, out)
		}
	}
}

func TestGoertzelBankMatchesIndividual(t *testing.T) {
	const sampleRate = 8000.0

	freqs := []float64{697, 770, 852, 941, 1209, 1336, 1477}
	samples := cosineBlock(800, 1.0, 770*800/sampleRate, 0)

	bank, err := NewGoertzelBank(freqs, sampleRate)
	if err != nil {
		t.Fatalf("NewGoertzelBank: %v", err)
	}

	bank.ProcessBlock(samples)

	if bank.Len() != len(freqs) {
		t.Fatalf("Len()=%d want %d", bank.Len(), len(freqs))
	}

	powers := bank.Powers(nil)
	mags := bank.Magnitudes(nil)

	for i, freq := range freqs {
		analyzer, _ := NewGoertzel(freq, sampleRate)
		analyzer.ProcessBlock(samples)

		if diff := math.Abs(powers[i] - analyzer.Power()); diff > 1e-9 {
			t.Errorf("bin %d power=%g want=%g", i, powers[i], analyzer.Power())
		}

		if diff := math.Abs(mags[i] - analyzer.Magnitude()); diff > 1e-9 {
			t.Errorf("bin %d magnitude=%g want=%g", i, mags[i], analyzer.Magnitude())
		}
	}
}

func TestGoertzelBankDTMF(t *testing.T) {
	const sampleRate = 8000.0

	// DTMF "5" = 770 Hz (row) + 1336 Hz (col).
	rows := []float64{697, 770, 852, 941}
	cols := []float64{1209, 1336, 1477, 1633}
	all := append(append([]float64{}, rows...), cols...)

	blockSize := 1024
	samples := make([]float64, blockSize)

	for i := range samples {
		samples[i] = 0.5*math.Sin(2*math.Pi*770*float64(i)/sampleRate) +
			0.5*math.Sin(2*math.Pi*1336*float64(i)/sampleRate)
	}

	bank, _ := NewGoertzelBank(all, sampleRate)
	bank.ProcessBlock(samples)
	powers := bank.Powers(nil)

	loudest := func(indices []int) int {
		best := indices[0]
		for _, idx := range indices[1:] {
			if powers[idx] > powers[best] {
				best = idx
			}
		}

		return best
	}

	rowIdx := loudest([]int{0, 1, 2, 3})
	colIdx := loudest([]int{4, 5, 6, 7})

	if all[rowIdx] != 770 {
		t.Errorf("detected row %g want 770", all[rowIdx])
	}

	if all[colIdx] != 1336 {
		t.Errorf("detected col %g want 1336", all[colIdx])
	}
}

func TestGoertzelBankPowersZeroAlloc(t *testing.T) {
	bank, _ := NewGoertzelBank([]float64{697, 770, 852}, 8000)
	bank.ProcessBlock(cosineBlock(256, 1.0, 24, 0))

	dst := make([]float64, bank.Len())

	allocs := testing.AllocsPerRun(100, func() {
		bank.Powers(dst)
		bank.Magnitudes(dst)
	})

	if allocs != 0 {
		t.Errorf("Powers/Magnitudes with reusable dst allocated %g times, want 0", allocs)
	}
}

func TestGoertzelBankValidation(t *testing.T) {
	if _, err := NewGoertzelBank(nil, 8000); err == nil {
		t.Error("expected error for empty frequency list")
	}

	if _, err := NewGoertzelBank([]float64{697, 30000}, 8000); err == nil {
		t.Error("expected error for frequency above Nyquist")
	}
}

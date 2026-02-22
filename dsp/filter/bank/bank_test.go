package bank

import (
	"math"
	"testing"
)

// IEC 61260 nominal octave band center frequencies (Hz).
var octaveNominal = []float64{31.5, 63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

func TestOctave_CenterFrequencies(t *testing.T) {
	b := Octave(1, 48000)

	bands := b.Bands()
	if len(bands) != len(octaveNominal) {
		t.Fatalf("Octave(1): got %d bands, want %d", len(bands), len(octaveNominal))
	}

	for i, band := range bands {
		// IEC 61260 exact centers differ slightly from nominal;
		// accept 5% tolerance.
		ratio := band.CenterFreq / octaveNominal[i]
		if ratio < 0.95 || ratio > 1.05 {
			t.Errorf("band %d: center %.1f Hz, want ~%.0f Hz (ratio %.3f)",
				i, band.CenterFreq, octaveNominal[i], ratio)
		}
	}
}

func TestOctave_ThirdOctaveBandCount(t *testing.T) {
	oct := Octave(1, 48000)
	third := Octave(3, 48000)
	// 1/3-octave should have approximately 3x the bands of full octave.
	ratio := float64(third.NumBands()) / float64(oct.NumBands())
	if ratio < 2.5 || ratio > 3.5 {
		t.Errorf("1/3-octave bands = %d, octave = %d, ratio %.1f (want ~3.0)",
			third.NumBands(), oct.NumBands(), ratio)
	}
}

func TestOctave_BandEdges(t *testing.T) {
	b := Octave(1, 48000)
	for _, band := range b.Bands() {
		if band.LowCutoff >= band.CenterFreq {
			t.Errorf("band %.0f Hz: low cutoff %.1f >= center", band.CenterFreq, band.LowCutoff)
		}

		if band.HighCutoff <= band.CenterFreq {
			t.Errorf("band %.0f Hz: high cutoff %.1f <= center", band.CenterFreq, band.HighCutoff)
		}
		// For octave bands, the ratio high/low should be ~2.
		ratio := band.HighCutoff / band.LowCutoff
		if math.Abs(ratio-octaveRatio) > 0.1 {
			t.Errorf("band %.0f Hz: edge ratio %.3f, want ~%.3f",
				band.CenterFreq, ratio, octaveRatio)
		}
	}
}

func TestOctave_MagnitudeAtCenter(t *testing.T) {
	sr := 48000.0

	b := Octave(1, sr)
	for _, band := range b.Bands() {
		mag := band.MagnitudeDB(band.CenterFreq, sr)
		// At center frequency, the combined LP+HP should be near 0 dB.
		// Butterworth pairs have a ~3 dB dip at band edges, but the center
		// should be within the passband. Accept -6 dB to 0 dB.
		if mag < -6 || mag > 1 {
			t.Errorf("band %.0f Hz: magnitude at center = %.1f dB, want -6..0 dB",
				band.CenterFreq, mag)
		}
	}
}

func TestOctave_Rejection(t *testing.T) {
	sr := 48000.0
	b := Octave(1, sr)

	bands := b.Bands()
	if len(bands) < 3 {
		t.Skip("not enough bands for rejection test")
	}
	// Check that the 1 kHz band rejects 125 Hz (3 octaves below).
	var bandIdx int

	for i, band := range bands {
		if math.Abs(band.CenterFreq-1000) < 50 {
			bandIdx = i
			break
		}
	}

	mag := bands[bandIdx].MagnitudeDB(125, sr)
	if mag > -15 {
		t.Errorf("1 kHz band at 125 Hz: %.1f dB, want < -15 dB", mag)
	}
}

func TestOctave_ProcessSample(t *testing.T) {
	sr := 48000.0
	b := Octave(1, sr)
	// Feed a 1 kHz sine through the bank.
	nSamples := 4800
	outputs := make([]float64, b.NumBands())

	for i := range nSamples {
		x := math.Sin(2 * math.Pi * 1000 * float64(i) / sr)

		out := b.ProcessSample(x)
		for j, v := range out {
			if a := math.Abs(v); a > outputs[j] {
				outputs[j] = a
			}
		}
	}
	// The 1 kHz band should have the largest output.
	var (
		maxIdx int
		maxVal float64
	)

	for i, v := range outputs {
		if v > maxVal {
			maxVal = v
			maxIdx = i
		}
	}

	fc := b.Bands()[maxIdx].CenterFreq
	if math.Abs(fc-1000) > 100 {
		t.Errorf("1 kHz sine: loudest band is %.0f Hz, want ~1000 Hz", fc)
	}
}

func TestOctave_ProcessBlock(t *testing.T) {
	sr := 48000.0
	b := Octave(1, sr)
	n := 1024

	input := make([]float64, n)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * 1000 * float64(i) / sr)
	}

	result := b.ProcessBlock(input)
	if len(result) != b.NumBands() {
		t.Fatalf("ProcessBlock: got %d bands, want %d", len(result), b.NumBands())
	}

	for i, buf := range result {
		if len(buf) != n {
			t.Errorf("band %d: got %d samples, want %d", i, len(buf), n)
		}
	}
}

func TestOctave_ProcessBlock_ConsistentWithProcessSample(t *testing.T) {
	sr := 48000.0
	n := 256

	input := make([]float64, n)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * 500 * float64(i) / sr)
	}

	// Process via ProcessBlock.
	b1 := Octave(1, sr)
	blockResult := b1.ProcessBlock(input)

	// Process via ProcessSample.
	b2 := Octave(1, sr)

	sampleResult := make([][]float64, b2.NumBands())
	for i := range sampleResult {
		sampleResult[i] = make([]float64, n)
	}

	for i, x := range input {
		out := b2.ProcessSample(x)
		for j, v := range out {
			sampleResult[j][i] = v
		}
	}

	// Compare.
	for band := range blockResult {
		for i := range blockResult[band] {
			diff := math.Abs(blockResult[band][i] - sampleResult[band][i])
			if diff > 1e-10 {
				t.Errorf("band %d sample %d: block=%.10f sample=%.10f diff=%.2e",
					band, i, blockResult[band][i], sampleResult[band][i], diff)

				break
			}
		}
	}
}

func TestOctave_Reset(t *testing.T) {
	sr := 48000.0
	b := Octave(1, sr)
	// Process some samples.
	for range 100 {
		b.ProcessSample(1.0)
	}

	b.Reset()
	// After reset, zero input should give zero output.
	out := b.ProcessSample(0)
	for i, v := range out {
		if v != 0 {
			t.Errorf("band %d: after Reset, ProcessSample(0) = %g, want 0", i, v)
		}
	}
}

func TestOctave_WithOrder(t *testing.T) {
	b2 := Octave(1, 48000, WithOrder(2))
	b8 := Octave(1, 48000, WithOrder(8))

	if b2.Order() != 2 {
		t.Errorf("WithOrder(2): got %d", b2.Order())
	}

	if b8.Order() != 8 {
		t.Errorf("WithOrder(8): got %d", b8.Order())
	}
}

func TestOctave_WithFrequencyRange(t *testing.T) {
	b := Octave(1, 48000, WithFrequencyRange(100, 10000))

	bands := b.Bands()
	for _, band := range bands {
		if band.CenterFreq < 100 || band.CenterFreq > 10000 {
			t.Errorf("band %.0f Hz outside requested range 100-10000", band.CenterFreq)
		}
	}
	// Should have fewer bands than full range.
	full := Octave(1, 48000)
	if b.NumBands() >= full.NumBands() {
		t.Errorf("restricted range has %d bands >= full range %d", b.NumBands(), full.NumBands())
	}
}

func TestCustom_ArbitraryFrequencies(t *testing.T) {
	centers := []float64{100, 500, 2000, 8000}

	b := Custom(centers, 1.0, 48000)
	if b.NumBands() != len(centers) {
		t.Fatalf("Custom: got %d bands, want %d", b.NumBands(), len(centers))
	}

	for i, band := range b.Bands() {
		if math.Abs(band.CenterFreq-centers[i]) > 1e-10 {
			t.Errorf("band %d: center %.1f, want %.0f", i, band.CenterFreq, centers[i])
		}
	}
}

func TestCustom_SkipsInvalidFrequencies(t *testing.T) {
	// 22000 Hz with 1-octave bandwidth at 48 kHz would exceed Nyquist.
	centers := []float64{1000, 22000}

	b := Custom(centers, 1.0, 48000)
	if b.NumBands() != 1 {
		t.Errorf("Custom: got %d bands, want 1 (22 kHz should be skipped)", b.NumBands())
	}
}

func TestOctave_SortedOrder(t *testing.T) {
	b := Octave(3, 48000)

	bands := b.Bands()
	for i := 1; i < len(bands); i++ {
		if bands[i].CenterFreq <= bands[i-1].CenterFreq {
			t.Errorf("bands not sorted: band %d (%.0f Hz) <= band %d (%.0f Hz)",
				i, bands[i].CenterFreq, i-1, bands[i-1].CenterFreq)
		}
	}
}

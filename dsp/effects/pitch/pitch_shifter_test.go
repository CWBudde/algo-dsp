package pitch

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
	algofft "github.com/cwbudde/algo-fft"
)

func TestNewPitchShifter(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		wantErr    bool
	}{
		{name: "valid 44100", sampleRate: 44100, wantErr: false},
		{name: "valid 48000", sampleRate: 48000, wantErr: false},
		{name: "invalid zero", sampleRate: 0, wantErr: true},
		{name: "invalid negative", sampleRate: -1, wantErr: true},
		{name: "invalid NaN", sampleRate: math.NaN(), wantErr: true},
		{name: "invalid +Inf", sampleRate: math.Inf(1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPitchShifter(tt.sampleRate)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewPitchShifter() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && p == nil {
				t.Fatalf("NewPitchShifter() returned nil without error")
			}
		})
	}
}

func TestPitchShifterSetPitchRatio(t *testing.T) {
	p, err := NewPitchShifter(48000)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	tests := []struct {
		name    string
		ratio   float64
		wantErr bool
	}{
		{name: "valid octave down", ratio: 0.5, wantErr: false},
		{name: "valid unison", ratio: 1.0, wantErr: false},
		{name: "valid octave up", ratio: 2.0, wantErr: false},
		{name: "invalid zero", ratio: 0, wantErr: true},
		{name: "invalid low bound", ratio: 0.1, wantErr: true},
		{name: "invalid high bound", ratio: 8.0, wantErr: true},
		{name: "invalid NaN", ratio: math.NaN(), wantErr: true},
		{name: "invalid +Inf", ratio: math.Inf(1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.SetPitchRatio(tt.ratio)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SetPitchRatio(%f) error = %v, wantErr %v", tt.ratio, err, tt.wantErr)
			}

			if !tt.wantErr && p.PitchRatio() != tt.ratio {
				t.Fatalf("PitchRatio() = %f, want %f", p.PitchRatio(), tt.ratio)
			}
		})
	}
}

func TestPitchShifterSetOverlapRejectsOverlapAboveSequence(t *testing.T) {
	p, err := NewPitchShifter(48000)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	err = p.SetSequence(20)
	if err != nil {
		t.Fatalf("SetSequence() error = %v", err)
	}

	old := p.Overlap()

	err = p.SetOverlap(30)
	if err == nil {
		t.Fatalf("SetOverlap(30) should fail when sequence is 20 ms")
	}

	if p.Overlap() != old {
		t.Fatalf("overlap should remain unchanged on error: got=%f want=%f", p.Overlap(), old)
	}
}

func TestPitchShifterIdentityIsExactCopy(t *testing.T) {
	const sampleRate = 48000.0

	p, err := NewPitchShifter(sampleRate)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	input := testutil.DeterministicSine(440, sampleRate, 0.8, 4096)

	out := p.Process(input)
	if len(out) != len(input) {
		t.Fatalf("length mismatch: got=%d want=%d", len(out), len(input))
	}

	for i := range input {
		if out[i] != input[i] {
			t.Fatalf("identity mismatch at sample %d: got=%g want=%g", i, out[i], input[i])
		}
	}

	out[0] = 123

	if input[0] == 123 {
		t.Fatalf("Process should return a copy for identity ratio")
	}
}

func TestPitchShifterProcessInPlaceMatchesProcess(t *testing.T) {
	const sampleRate = 48000.0

	p1, err := NewPitchShifter(sampleRate)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	p2, err := NewPitchShifter(sampleRate)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	err = p1.SetPitchSemitones(7)
	if err != nil {
		t.Fatalf("SetPitchSemitones() error = %v", err)
	}

	err = p2.SetPitchSemitones(7)
	if err != nil {
		t.Fatalf("SetPitchSemitones() error = %v", err)
	}

	input := testutil.DeterministicSine(330, sampleRate, 0.7, 4096)

	want := p1.Process(input)
	got := make([]float64, len(input))
	copy(got, input)
	p2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestPitchShifterPitchAccuracy(t *testing.T) {
	const (
		sampleRate = 48000.0
		f0         = 220.0
		length     = 60000
		start      = 8000
		stop       = 52000
		tolUpHz    = 10.0
		tolDnHz    = 6.0
		minUpHz    = 300.0
		maxUpHz    = 600.0
		minDnHz    = 80.0
		maxDnHz    = 180.0
		shiftSem   = 12.0
	)

	input := testutil.DeterministicSine(f0, sampleRate, 0.8, length)

	up, err := NewPitchShifter(sampleRate)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	err = up.SetPitchSemitones(shiftSem)
	if err != nil {
		t.Fatalf("SetPitchSemitones() error = %v", err)
	}

	upOut := up.Process(input)
	upFreq := estimateFrequencyAutoCorrelation(upOut[start:stop], sampleRate, minUpHz, maxUpHz)

	upWant := f0 * 2.0
	if diff := math.Abs(upFreq - upWant); diff > tolUpHz {
		t.Fatalf("pitch-up frequency mismatch: got=%gHz want=%gHz diff=%gHz", upFreq, upWant, diff)
	}

	down, err := NewPitchShifter(sampleRate)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	err = down.SetPitchSemitones(-shiftSem)
	if err != nil {
		t.Fatalf("SetPitchSemitones() error = %v", err)
	}

	downOut := down.Process(input)
	downFreq := estimateFrequencyAutoCorrelation(downOut[start:stop], sampleRate, minDnHz, maxDnHz)

	downWant := f0 * 0.5
	if diff := math.Abs(downFreq - downWant); diff > tolDnHz {
		t.Fatalf("pitch-down frequency mismatch: got=%gHz want=%gHz diff=%gHz", downFreq, downWant, diff)
	}
}

func TestPitchShifterShortBufferProducesFiniteValues(t *testing.T) {
	p, err := NewPitchShifter(48000)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	err = p.SetPitchRatio(1.7)
	if err != nil {
		t.Fatalf("SetPitchRatio() error = %v", err)
	}

	input := []float64{1, -0.25, 0.1, 0, -0.1, 0.2, -0.3, 0.4}

	out := p.Process(input)
	if len(out) != len(input) {
		t.Fatalf("length mismatch: got=%d want=%d", len(out), len(input))
	}

	for i, v := range out {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("sample %d is not finite: %v", i, v)
		}
	}
}

func TestPitchShifterResetDeterministic(t *testing.T) {
	p, err := NewPitchShifter(48000)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	err = p.SetPitchRatio(0.75)
	if err != nil {
		t.Fatalf("SetPitchRatio() error = %v", err)
	}

	input := testutil.DeterministicSine(330, 48000, 0.9, 8192)

	got1 := p.Process(input)
	p.Reset()
	got2 := p.Process(input)

	for i := range got1 {
		if diff := math.Abs(got1[i] - got2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: diff=%g", i, diff)
		}
	}
}

func estimateFrequencyAutoCorrelation(x []float64, sampleRate, minHz, maxHz float64) float64 {
	if len(x) < 8 || sampleRate <= 0 || minHz <= 0 || maxHz <= minHz {
		return 0
	}

	lagMin := int(math.Floor(sampleRate / maxHz))
	lagMax := int(math.Ceil(sampleRate / minHz))

	if lagMin < 1 {
		lagMin = 1
	}

	if lagMax >= len(x)-2 {
		lagMax = len(x) - 2
	}

	if lagMax <= lagMin {
		return 0
	}

	mean := 0.0
	for _, v := range x {
		mean += v
	}

	mean /= float64(len(x))

	centered := make([]float64, len(x))
	for i, v := range x {
		centered[i] = v - mean
	}

	bestLag := lagMin
	bestScore := math.Inf(-1)

	for lag := lagMin; lag <= lagMax; lag++ {
		score := normalizedAutocorrelation(centered, lag)
		if score > bestScore {
			bestScore = score
			bestLag = lag
		}
	}

	lag := float64(bestLag)
	if bestLag > lagMin && bestLag < lagMax {
		s0 := normalizedAutocorrelation(centered, bestLag-1)
		s1 := normalizedAutocorrelation(centered, bestLag)
		s2 := normalizedAutocorrelation(centered, bestLag+1)

		den := s0 - 2*s1 + s2
		if math.Abs(den) > 1e-12 {
			lag += 0.5 * (s0 - s2) / den
		}
	}

	if lag <= 0 {
		return 0
	}

	return sampleRate / lag
}

func TestPitchShifterSignalQuality(t *testing.T) {
	const (
		sampleRate = 48000.0
		n          = 32768
		fftLen     = 16384
	)

	cases := []struct {
		name  string
		ratio float64
	}{
		{name: "down_octave", ratio: 0.5},
		{name: "down_fourth", ratio: 0.75},
		{name: "up_fifth", ratio: 1.5},
		{name: "up_near_octave", ratio: 1.9},
		{name: "up_octave", ratio: 2.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewPitchShifter(sampleRate)
			if err != nil {
				t.Fatalf("NewPitchShifter() error = %v", err)
			}

			err = p.SetPitchRatio(tc.ratio)
			if err != nil {
				t.Fatalf("SetPitchRatio() error = %v", err)
			}

			// Choose input freq so the shifted output lands on an exact FFT bin,
			// avoiding spectral leakage in the SNR measurement.
			outBin := 100
			outFreq := float64(outBin) * sampleRate / float64(fftLen)
			inFreq := outFreq / tc.ratio

			input := make([]float64, n)
			for i := range input {
				input[i] = 0.8 * math.Sin(2*math.Pi*inFreq*float64(i)/sampleRate)
			}

			out := p.Process(input)

			snr := measureTimeDomainSNR(t, out, outFreq, sampleRate, fftLen)
			t.Logf("ratio=%.2f  inFreq=%.1f Hz  outFreq=%.1f Hz  SNR=%.1f dB",
				tc.ratio, inFreq, outFreq, snr)

			if snr < 50 {
				t.Errorf("signal quality too low: SNR = %.1f dB, want >= 50 dB", snr)
			}
		})
	}
}

func TestPitchShifterSignalQualityWSSOLAParams(t *testing.T) {
	// Tests a small pitch shift (1.1x) across different WSOLA sequence/overlap
	// configurations. Small ratios stress the overlap-add algorithm most, and
	// varying these parameters changes quality/latency trade-offs.
	const (
		sampleRate = 48000.0
		n          = 32768
		fftLen     = 16384
		ratio      = 1.1
	)

	cases := []struct {
		name       string
		sequenceMs float64
		overlapMs  float64
	}{
		{name: "seq20_ovl5", sequenceMs: 20, overlapMs: 5},
		{name: "seq40_ovl10", sequenceMs: 40, overlapMs: 10},
		{name: "seq80_ovl20", sequenceMs: 80, overlapMs: 20},
	}

	outBin := 100
	outFreq := float64(outBin) * sampleRate / float64(fftLen)
	inFreq := outFreq / ratio

	input := make([]float64, n)
	for i := range input {
		input[i] = 0.8 * math.Sin(2*math.Pi*inFreq*float64(i)/sampleRate)
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			p, err := NewPitchShifter(sampleRate)
			if err != nil {
				t.Fatalf("NewPitchShifter() error = %v", err)
			}

			err = p.SetSequence(testCase.sequenceMs)
			if err != nil {
				t.Fatalf("SetSequence() error = %v", err)
			}

			err = p.SetOverlap(testCase.overlapMs)
			if err != nil {
				t.Fatalf("SetOverlap() error = %v", err)
			}

			err = p.SetPitchRatio(ratio)
			if err != nil {
				t.Fatalf("SetPitchRatio() error = %v", err)
			}

			out := p.Process(input)

			snr := measureTimeDomainSNR(t, out, outFreq, sampleRate, fftLen)
			t.Logf("seq=%.0fms ovl=%.0fms  inFreq=%.1f Hz  outFreq=%.1f Hz  SNR=%.1f dB",
				testCase.sequenceMs, testCase.overlapMs, inFreq, outFreq, snr)

			if snr < 45 {
				t.Errorf("signal quality too low: SNR = %.1f dB, want >= 45 dB", snr)
			}
		})
	}
}

func TestPitchShifterTwoToneWellSeparated(t *testing.T) {
	// Two tones separated by a factor of 2 (an octave). WSOLA handles this well
	// because the beat period is longer than the sequence window, so the
	// autocorrelation-based segment selection is not confused.
	const (
		sampleRate = 48000.0
		n          = 32768
		fftLen     = 16384
	)

	cases := []struct {
		name  string
		ratio float64
	}{
		{name: "down_fourth", ratio: 0.75},
		{name: "up_fifth", ratio: 1.5},
		{name: "up_near_octave", ratio: 1.9},
	}

	// Place output tones on exact FFT bins with 2× separation.
	outBin1 := 60
	outFreq1 := float64(outBin1) * sampleRate / float64(fftLen)
	outFreq2 := outFreq1 * 2.0 // one octave apart — no beating within the window

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inFreq1 := outFreq1 / tc.ratio
			inFreq2 := outFreq2 / tc.ratio

			p, err := NewPitchShifter(sampleRate)
			if err != nil {
				t.Fatalf("NewPitchShifter() error = %v", err)
			}

			err = p.SetPitchRatio(tc.ratio)
			if err != nil {
				t.Fatalf("SetPitchRatio() error = %v", err)
			}

			input := make([]float64, n)
			for i := range input {
				input[i] = 0.5*math.Sin(2*math.Pi*inFreq1*float64(i)/sampleRate) +
					0.5*math.Sin(2*math.Pi*inFreq2*float64(i)/sampleRate)
			}

			out := p.Process(input)

			snr := measureTimeDomainTwoToneSNR(t, out, outFreq1, outFreq2, sampleRate, fftLen)
			t.Logf("ratio=%.2f  inFreqs=%.1f+%.1f Hz  outFreqs=%.1f+%.1f Hz  SNR=%.1f dB",
				tc.ratio, inFreq1, inFreq2, outFreq1, outFreq2, snr)

			if snr < 45 {
				t.Errorf("two-tone signal quality too low: SNR = %.1f dB, want >= 45 dB", snr)
			}
		})
	}
}

func TestPitchShifterTwoToneCloselySpaced(t *testing.T) {
	// Two tones separated by only 1.2×. With the music-tuned defaults
	// (seq=82ms), several beat cycles fit within the autocorrelation window,
	// giving the segment search enough structure to find good splice points.
	const (
		sampleRate = 48000.0
		n          = 32768
		fftLen     = 16384
	)

	cases := []struct {
		name  string
		ratio float64
	}{
		{name: "down_fourth", ratio: 0.75},
		{name: "up_fifth", ratio: 1.5},
		{name: "up_near_octave", ratio: 1.9},
	}

	outBin1 := 80
	outFreq1 := float64(outBin1) * sampleRate / float64(fftLen)
	outFreq2 := outFreq1 * 1.2

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inFreq1 := outFreq1 / tc.ratio
			inFreq2 := outFreq2 / tc.ratio

			p, err := NewPitchShifter(sampleRate)
			if err != nil {
				t.Fatalf("NewPitchShifter() error = %v", err)
			}

			err = p.SetPitchRatio(tc.ratio)
			if err != nil {
				t.Fatalf("SetPitchRatio() error = %v", err)
			}

			input := make([]float64, n)
			for i := range input {
				input[i] = 0.5*math.Sin(2*math.Pi*inFreq1*float64(i)/sampleRate) +
					0.5*math.Sin(2*math.Pi*inFreq2*float64(i)/sampleRate)
			}

			out := p.Process(input)

			testutil.RequireFinite(t, out)
			snr := measureTimeDomainTwoToneSNR(t, out, outFreq1, outFreq2, sampleRate, fftLen)
			t.Logf("ratio=%.2f  inFreqs=%.1f+%.1f Hz  outFreqs=%.1f+%.1f Hz  SNR=%.1f dB",
				tc.ratio, inFreq1, inFreq2, outFreq1, outFreq2, snr)

			if snr < 45 {
				t.Errorf("two-tone (closely spaced) signal quality too low: SNR = %.1f dB, want >= 45 dB", snr)
			}
		})
	}
}

// measureTimeDomainTwoToneSNR measures SNR when two target frequencies are present.
// Signal power is summed within ±10 bins of each target; all other bins are noise.
func measureTimeDomainTwoToneSNR(t *testing.T, out []float64, freq1, freq2, sampleRate float64, fftLen int) float64 {
	t.Helper()

	mid := max(len(out)/2-fftLen/2, 0)

	chunk := out[mid : mid+fftLen]

	plan, err := algofft.NewPlan64(fftLen)
	if err != nil {
		t.Fatalf("NewPlan64 error: %v", err)
	}

	fftIn := make([]complex128, fftLen)
	fftOut := make([]complex128, fftLen)

	for i, v := range chunk {
		fftIn[i] = complex(v, 0)
	}

	err = plan.Forward(fftOut, fftIn)
	if err != nil {
		t.Fatalf("Forward FFT error: %v", err)
	}

	tb1 := int(math.Round(freq1 * float64(fftLen) / sampleRate))
	tb2 := int(math.Round(freq2 * float64(fftLen) / sampleRate))

	const sigBW = 10

	sigPower := 0.0
	noisePower := 0.0

	for k := 1; k <= fftLen/2; k++ {
		mag2 := real(fftOut[k])*real(fftOut[k]) + imag(fftOut[k])*imag(fftOut[k])
		if (k >= tb1-sigBW && k <= tb1+sigBW) || (k >= tb2-sigBW && k <= tb2+sigBW) {
			sigPower += mag2
		} else {
			noisePower += mag2
		}
	}

	if noisePower <= 1e-30 {
		return 100.0
	}

	return 10 * math.Log10(sigPower/noisePower)
}

// measureTimeDomainSNR runs a windowed FFT on the center of out and returns
// the SNR in dB relative to a ±10 bin band around targetFreq.
func measureTimeDomainSNR(t *testing.T, out []float64, targetFreq, sampleRate float64, fftLen int) float64 {
	t.Helper()

	mid := max(len(out)/2-fftLen/2, 0)

	chunk := out[mid : mid+fftLen]

	plan, err := algofft.NewPlan64(fftLen)
	if err != nil {
		t.Fatalf("NewPlan64 error: %v", err)
	}

	fftIn := make([]complex128, fftLen)
	fftOut := make([]complex128, fftLen)

	for i, v := range chunk {
		fftIn[i] = complex(v, 0)
	}

	err = plan.Forward(fftOut, fftIn)
	if err != nil {
		t.Fatalf("Forward FFT error: %v", err)
	}

	targetBin := int(math.Round(targetFreq * float64(fftLen) / sampleRate))

	const sigBW = 10

	sigPower := 0.0
	noisePower := 0.0

	for k := 1; k <= fftLen/2; k++ {
		mag2 := real(fftOut[k])*real(fftOut[k]) + imag(fftOut[k])*imag(fftOut[k])
		if k >= targetBin-sigBW && k <= targetBin+sigBW {
			sigPower += mag2
		} else {
			noisePower += mag2
		}
	}

	if noisePower <= 1e-30 {
		return 100.0
	}

	return 10 * math.Log10(sigPower/noisePower)
}

func normalizedAutocorrelation(x []float64, lag int) float64 {
	n := len(x) - lag
	if n <= 0 {
		return -1
	}

	dot := 0.0
	e0 := 0.0
	e1 := 0.0

	for i := range n {
		a := x[i]
		b := x[i+lag]
		dot += a * b
		e0 += a * a
		e1 += b * b
	}

	if e0 <= 1e-12 || e1 <= 1e-12 {
		return -1
	}

	return dot / math.Sqrt(e0*e1)
}

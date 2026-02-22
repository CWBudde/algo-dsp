package pitch

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
	algofft "github.com/cwbudde/algo-fft"
)

func TestNewSpectralPitchShifter(t *testing.T) {
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
			s, err := NewSpectralPitchShifter(tt.sampleRate)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewSpectralPitchShifter() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if s == nil {
				t.Fatal("NewSpectralPitchShifter() returned nil")
			}

			if got := s.SampleRate(); got != tt.sampleRate {
				t.Fatalf("SampleRate() = %f, want %f", got, tt.sampleRate)
			}

			if got := s.PitchRatio(); got != defaultSpectralPitchRatio {
				t.Fatalf("PitchRatio() = %f, want %f", got, defaultSpectralPitchRatio)
			}

			if got := s.FrameSize(); got != defaultSpectralFrameSize {
				t.Fatalf("FrameSize() = %d, want %d", got, defaultSpectralFrameSize)
			}

			if got := s.AnalysisHop(); got != defaultSpectralAnalysisHop {
				t.Fatalf("AnalysisHop() = %d, want %d", got, defaultSpectralAnalysisHop)
			}

			if got := s.SynthesisHop(); got != defaultSpectralAnalysisHop {
				t.Fatalf("SynthesisHop() = %d, want %d", got, defaultSpectralAnalysisHop)
			}

			if got := s.EffectivePitchRatio(); got != 1 {
				t.Fatalf("EffectivePitchRatio() = %f, want 1", got)
			}
		})
	}
}

func TestSpectralPitchShifterSettersValidate(t *testing.T) {
	s, err := NewSpectralPitchShifter(48000)
	if err != nil {
		t.Fatalf("NewSpectralPitchShifter() error = %v", err)
	}

	if err := s.SetPitchRatio(0); err == nil {
		t.Fatal("expected error for zero pitch ratio")
	}

	if err := s.SetPitchRatio(0.1); err == nil {
		t.Fatal("expected error for too-small pitch ratio")
	}

	if err := s.SetPitchRatio(6); err == nil {
		t.Fatal("expected error for too-large pitch ratio")
	}

	if err := s.SetPitchRatio(math.NaN()); err == nil {
		t.Fatal("expected error for NaN pitch ratio")
	}

	if err := s.SetPitchRatio(math.Inf(1)); err == nil {
		t.Fatal("expected error for Inf pitch ratio")
	}

	if err := s.SetPitchSemitones(math.NaN()); err == nil {
		t.Fatal("expected error for NaN semitones")
	}

	if err := s.SetPitchSemitones(7); err != nil {
		t.Fatalf("SetPitchSemitones() error = %v", err)
	}

	if err := s.SetSampleRate(0); err == nil {
		t.Fatal("expected error for invalid sample rate")
	}

	if err := s.SetSampleRate(96000); err != nil {
		t.Fatalf("SetSampleRate() error = %v", err)
	}

	if err := s.SetFrameSize(1000); err == nil {
		t.Fatal("expected error for non power-of-two frame size")
	}

	if err := s.SetFrameSize(32); err == nil {
		t.Fatal("expected error for too-small frame size")
	}

	if err := s.SetFrameSize(2048); err != nil {
		t.Fatalf("SetFrameSize() error = %v", err)
	}

	if err := s.SetAnalysisHop(0); err == nil {
		t.Fatal("expected error for zero hop")
	}

	if err := s.SetAnalysisHop(2048); err == nil {
		t.Fatal("expected error for hop >= frame size")
	}

	if err := s.SetAnalysisHop(512); err != nil {
		t.Fatalf("SetAnalysisHop() error = %v", err)
	}
}

func TestSpectralPitchShifterProcessLengthAndFinite(t *testing.T) {
	s, err := NewSpectralPitchShifter(48000)
	if err != nil {
		t.Fatalf("NewSpectralPitchShifter() error = %v", err)
	}

	if err := s.SetPitchRatio(1.25); err != nil {
		t.Fatalf("SetPitchRatio() error = %v", err)
	}

	input := testutil.DeterministicSine(220, 48000, 0.8, 4096)

	out := s.Process(input)
	if len(out) != len(input) {
		t.Fatalf("len(out) = %d, want %d", len(out), len(input))
	}

	testutil.RequireFinite(t, out)
}

func TestSpectralPitchShifterProcessInPlaceMatchesProcess(t *testing.T) {
	s, err := NewSpectralPitchShifter(48000)
	if err != nil {
		t.Fatalf("NewSpectralPitchShifter() error = %v", err)
	}

	if err := s.SetPitchRatio(0.8); err != nil {
		t.Fatalf("SetPitchRatio() error = %v", err)
	}

	input := testutil.DeterministicSine(330, 48000, 0.7, 4096)
	want := s.Process(input)

	got := append([]float64(nil), input...)
	s.ProcessInPlace(got)

	maxDiff, err := testutil.MaxAbsDiff(got, want)
	if err != nil {
		t.Fatalf("MaxAbsDiff() error = %v", err)
	}

	if maxDiff > 1e-9 {
		t.Fatalf("max diff = %g, want <= 1e-9", maxDiff)
	}
}

func TestSpectralPitchShifterIdentityKeepsDominantFrequency(t *testing.T) {
	const (
		sampleRate = 48000.0
		n          = 8192
		bin        = 48
	)

	freq := sampleRate * float64(bin) / float64(n)
	input := testutil.DeterministicSine(freq, sampleRate, 0.8, n)

	spectral, err := NewSpectralPitchShifter(sampleRate)
	if err != nil {
		t.Fatalf("NewSpectralPitchShifter() error = %v", err)
	}

	if err := spectral.SetPitchRatio(1); err != nil {
		t.Fatalf("SetPitchRatio() error = %v", err)
	}

	out := spectral.Process(input)
	gotFreq := dominantFrequencyHz(t, out[n/4:3*n/4], sampleRate)

	relErr := math.Abs(gotFreq-freq) / freq
	if relErr > 0.04 {
		t.Fatalf("dominant frequency rel err = %f (got %f Hz, want %f Hz)", relErr, gotFreq, freq)
	}
}

func TestSpectralPitchShifterMovesDominantFrequency(t *testing.T) {
	const (
		sampleRate = 48000.0
		n          = 8192
		bin        = 40
	)

	inputFreq := sampleRate * float64(bin) / float64(n)
	input := testutil.DeterministicSine(inputFreq, sampleRate, 0.8, n)

	cases := []struct {
		name  string
		ratio float64
	}{
		{name: "up", ratio: 1.5},
		{name: "down", ratio: 0.75},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			s, err := NewSpectralPitchShifter(sampleRate)
			if err != nil {
				t.Fatalf("NewSpectralPitchShifter() error = %v", err)
			}

			if err := s.SetPitchRatio(testCase.ratio); err != nil {
				t.Fatalf("SetPitchRatio() error = %v", err)
			}

			out := s.Process(input)
			gotFreq := dominantFrequencyHz(t, out[n/4:3*n/4], sampleRate)
			wantFreq := inputFreq * s.EffectivePitchRatio()

			relErr := math.Abs(gotFreq-wantFreq) / wantFreq
			if relErr > 0.10 {
				t.Fatalf(
					"dominant frequency rel err = %f (got %f Hz, want %f Hz, ratio=%f)",
					relErr,
					gotFreq,
					wantFreq,
					s.EffectivePitchRatio(),
				)
			}
		})
	}
}

func TestSpectralPitchShifterSignalQuality(t *testing.T) {
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
		{name: "up_octave", ratio: 2.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewSpectralPitchShifter(sampleRate)
			if err != nil {
				t.Fatalf("NewSpectralPitchShifter() error = %v", err)
			}

			if err := s.SetPitchRatio(tc.ratio); err != nil {
				t.Fatalf("SetPitchRatio() error = %v", err)
			}

			// Choose input freq so the output freq lands on an exact FFT bin
			// of the analysis window, avoiding spectral leakage in the measurement.
			outBin := 100
			outFreq := float64(outBin) * sampleRate / float64(fftLen)
			inFreq := outFreq / tc.ratio

			input := make([]float64, n)
			for i := range input {
				input[i] = 0.8 * math.Sin(2*math.Pi*inFreq*float64(i)/sampleRate)
			}

			out := s.Process(input)

			// Windowed FFT analysis of the output center.
			mid := len(out)/2 - fftLen/2
			if mid < 0 {
				mid = 0
			}

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

			if err := plan.Forward(fftOut, fftIn); err != nil {
				t.Fatalf("Forward FFT error: %v", err)
			}

			targetBin := int(math.Round(outFreq * float64(fftLen) / sampleRate))
			sigBW := 10
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

			snr := 100.0
			if noisePower > 1e-30 {
				snr = 10 * math.Log10(sigPower/noisePower)
			}

			t.Logf("ratio=%.2f  inFreq=%.1f Hz  outFreq=%.1f Hz  SNR=%.1f dB",
				tc.ratio, inFreq, outFreq, snr)

			if snr < 45 {
				t.Errorf("signal quality too low: SNR = %.1f dB, want >= 45 dB", snr)
			}
		})
	}
}

func TestSpectralPitchShifterSignalQualityOverlap(t *testing.T) {
	// Tests a small pitch shift (1.1x) across different overlap factors.
	// Small ratios use the bin-shifting path, which requires higher
	// overlap for quality than the time-stretch path. The per-bin phase
	// vocoder without phase locking needs >= 8x overlap for good SNR.
	const (
		sampleRate = 48000.0
		n          = 32768
		fftLen     = 16384
		ratio      = 1.1
	)

	cases := []struct {
		name        string
		frameSize   int
		analysisHop int
		minSNR      float64
	}{
		// 2x and 4x overlap are insufficient for the bin-shifting path;
		// we still test them to catch regressions but with relaxed thresholds.
		{name: "2x_overlap", frameSize: 1024, analysisHop: 512, minSNR: -5},
		{name: "4x_overlap", frameSize: 1024, analysisHop: 256, minSNR: 15},
		{name: "8x_overlap", frameSize: 1024, analysisHop: 128, minSNR: 45},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewSpectralPitchShifter(sampleRate)
			if err != nil {
				t.Fatalf("NewSpectralPitchShifter() error = %v", err)
			}

			if err := s.SetFrameSize(tc.frameSize); err != nil {
				t.Fatalf("SetFrameSize() error = %v", err)
			}

			if err := s.SetAnalysisHop(tc.analysisHop); err != nil {
				t.Fatalf("SetAnalysisHop() error = %v", err)
			}

			if err := s.SetPitchRatio(ratio); err != nil {
				t.Fatalf("SetPitchRatio() error = %v", err)
			}

			// Choose input freq so the output freq lands on an exact FFT bin.
			outBin := 100
			outFreq := float64(outBin) * sampleRate / float64(fftLen)
			inFreq := outFreq / ratio

			input := make([]float64, n)
			for i := range input {
				input[i] = 0.8 * math.Sin(2*math.Pi*inFreq*float64(i)/sampleRate)
			}

			out := s.Process(input)

			snr := measureSNR(t, out, outFreq, sampleRate, fftLen)
			t.Logf("overlap=%d/%d  inFreq=%.1f Hz  outFreq=%.1f Hz  SNR=%.1f dB",
				tc.frameSize, tc.analysisHop, inFreq, outFreq, snr)

			if snr < tc.minSNR {
				t.Errorf("signal quality too low: SNR = %.1f dB, want >= %.0f dB", snr, tc.minSNR)
			}
		})
	}
}

// measureSNR runs a windowed FFT on the center of out and returns the SNR in dB
// relative to a ±10 bin band around targetFreq.
func measureSNR(t *testing.T, out []float64, targetFreq, sampleRate float64, fftLen int) float64 {
	t.Helper()

	mid := len(out)/2 - fftLen/2
	if mid < 0 {
		mid = 0
	}

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

	if err := plan.Forward(fftOut, fftIn); err != nil {
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

func dominantFrequencyHz(t *testing.T, signal []float64, sampleRate float64) float64 {
	t.Helper()

	if len(signal) == 0 {
		return 0
	}

	plan, err := algofft.NewPlan64(len(signal))
	if err != nil {
		t.Fatalf("failed to create FFT plan: %v", err)
	}

	in := make([]complex128, len(signal))

	out := make([]complex128, len(signal))
	for i, v := range signal {
		in[i] = complex(v, 0)
	}

	if err := plan.Forward(out, in); err != nil {
		t.Fatalf("forward FFT failed: %v", err)
	}

	maxBin := 1
	maxMag := 0.0

	for k := 1; k <= len(signal)/2; k++ {
		re := real(out[k])
		im := imag(out[k])

		mag := re*re + im*im
		if mag > maxMag {
			maxMag = mag
			maxBin = k
		}
	}

	return sampleRate * float64(maxBin) / float64(len(signal))
}

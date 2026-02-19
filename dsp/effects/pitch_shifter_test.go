package effects

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
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
	if err := p.SetSequence(20); err != nil {
		t.Fatalf("SetSequence() error = %v", err)
	}
	old := p.Overlap()
	if err := p.SetOverlap(30); err == nil {
		t.Fatalf("SetOverlap(30) should fail when sequence is 20 ms")
	}
	if p.Overlap() != old {
		t.Fatalf("overlap should remain unchanged on error: got=%f want=%f", p.Overlap(), old)
	}
}

func TestPitchShifterIdentityIsExactCopy(t *testing.T) {
	const sr = 48000.0
	p, err := NewPitchShifter(sr)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	input := testutil.DeterministicSine(440, sr, 0.8, 4096)
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
	const sr = 48000.0
	p1, err := NewPitchShifter(sr)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}
	p2, err := NewPitchShifter(sr)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}

	if err := p1.SetPitchSemitones(7); err != nil {
		t.Fatalf("SetPitchSemitones() error = %v", err)
	}
	if err := p2.SetPitchSemitones(7); err != nil {
		t.Fatalf("SetPitchSemitones() error = %v", err)
	}

	input := testutil.DeterministicSine(330, sr, 0.7, 4096)

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
		sr       = 48000.0
		f0       = 220.0
		length   = 60000
		start    = 8000
		stop     = 52000
		tolUpHz  = 10.0
		tolDnHz  = 6.0
		minUpHz  = 300.0
		maxUpHz  = 600.0
		minDnHz  = 80.0
		maxDnHz  = 180.0
		shiftSem = 12.0
	)
	input := testutil.DeterministicSine(f0, sr, 0.8, length)

	up, err := NewPitchShifter(sr)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}
	if err := up.SetPitchSemitones(shiftSem); err != nil {
		t.Fatalf("SetPitchSemitones() error = %v", err)
	}
	upOut := up.Process(input)
	upFreq := estimateFrequencyAutoCorrelation(upOut[start:stop], sr, minUpHz, maxUpHz)
	upWant := f0 * 2.0
	if diff := math.Abs(upFreq - upWant); diff > tolUpHz {
		t.Fatalf("pitch-up frequency mismatch: got=%gHz want=%gHz diff=%gHz", upFreq, upWant, diff)
	}

	down, err := NewPitchShifter(sr)
	if err != nil {
		t.Fatalf("NewPitchShifter() error = %v", err)
	}
	if err := down.SetPitchSemitones(-shiftSem); err != nil {
		t.Fatalf("SetPitchSemitones() error = %v", err)
	}
	downOut := down.Process(input)
	downFreq := estimateFrequencyAutoCorrelation(downOut[start:stop], sr, minDnHz, maxDnHz)
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
	if err := p.SetPitchRatio(1.7); err != nil {
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
	if err := p.SetPitchRatio(0.75); err != nil {
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

func normalizedAutocorrelation(x []float64, lag int) float64 {
	n := len(x) - lag
	if n <= 0 {
		return -1
	}
	dot := 0.0
	e0 := 0.0
	e1 := 0.0
	for i := 0; i < n; i++ {
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

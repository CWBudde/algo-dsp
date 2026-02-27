package modulation

import (
	"math"
	"testing"
)

func TestPhaserProcessInPlaceMatchesProcess(t *testing.T) {
	phaser1, err := NewPhaser(48000)
	if err != nil {
		t.Fatalf("NewPhaser() error = %v", err)
	}

	phaser2, err := NewPhaser(48000)
	if err != nil {
		t.Fatalf("NewPhaser() error = %v", err)
	}

	input := make([]float64, 256)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 37)
	}

	want := make([]float64, len(input))
	copy(want, input)

	for i := range want {
		want[i] = phaser1.Process(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)

	err = phaser2.ProcessInPlace(got)
	if err != nil {
		t.Fatalf("ProcessInPlace() error = %v", err)
	}

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestPhaserResetRestoresState(t *testing.T) {
	phaser, err := NewPhaser(48000)
	if err != nil {
		t.Fatalf("NewPhaser() error = %v", err)
	}

	inData := make([]float64, 128)
	inData[0] = 1

	outData1 := make([]float64, len(inData))
	for i := range inData {
		outData1[i] = phaser.Process(inData[i])
	}

	phaser.Reset()

	outData2 := make([]float64, len(inData))
	for i := range inData {
		outData2[i] = phaser.Process(inData[i])
	}

	for i := range outData1 {
		if diff := math.Abs(outData1[i] - outData2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, outData2[i], outData1[i], diff)
		}
	}
}

func TestPhaserValidation(t *testing.T) {
	_, err := NewPhaser(0)
	if err == nil {
		t.Fatal("NewPhaser() expected error for invalid sample rate")
	}

	_, err = NewPhaser(48000, WithPhaserStages(0))
	if err == nil {
		t.Fatal("NewPhaser() expected error for invalid stage count")
	}

	_, err = NewPhaser(48000, WithPhaserFrequencyRangeHz(1000, 800))
	if err == nil {
		t.Fatal("NewPhaser() expected error for invalid frequency range")
	}

	_, err = NewPhaser(48000, WithPhaserFrequencyRangeHz(1000, 30000))
	if err == nil {
		t.Fatal("NewPhaser() expected error for above-nyquist frequency")
	}
}

func TestPhaserFiniteOutputUnderFeedback(t *testing.T) {
	phaser, err := NewPhaser(48000,
		WithPhaserFeedback(0.85),
		WithPhaserMix(0.8),
		WithPhaserStages(8),
	)
	if err != nil {
		t.Fatalf("NewPhaser() error = %v", err)
	}

	for i := range 12000 {
		in := 0.3 * math.Sin(2*math.Pi*440*float64(i)/48000)

		out := phaser.Process(in)
		if math.IsNaN(out) || math.IsInf(out, 0) {
			t.Fatalf("non-finite output at sample %d: %v", i, out)
		}
	}
}

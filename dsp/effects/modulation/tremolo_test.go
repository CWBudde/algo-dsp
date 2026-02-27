package modulation

import (
	"math"
	"testing"
)

func TestTremoloProcessInPlaceMatchesProcess(t *testing.T) {
	tremolo1, err := NewTremolo(48000)
	if err != nil {
		t.Fatalf("NewTremolo() error = %v", err)
	}

	tremolo2, err := NewTremolo(48000)
	if err != nil {
		t.Fatalf("NewTremolo() error = %v", err)
	}

	input := make([]float64, 128)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 31)
	}

	want := make([]float64, len(input))
	copy(want, input)

	for i := range want {
		want[i] = tremolo1.Process(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)

	err = tremolo2.ProcessInPlace(got)
	if err != nil {
		t.Fatalf("ProcessInPlace() error = %v", err)
	}

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestTremoloResetRestoresState(t *testing.T) {
	tremolo, err := NewTremolo(48000)
	if err != nil {
		t.Fatalf("NewTremolo() error = %v", err)
	}

	inData := make([]float64, 96)
	inData[0] = 1

	outData1 := make([]float64, len(inData))
	for i := range inData {
		outData1[i] = tremolo.Process(inData[i])
	}

	tremolo.Reset()

	outData2 := make([]float64, len(inData))
	for i := range inData {
		outData2[i] = tremolo.Process(inData[i])
	}

	for i := range outData1 {
		if diff := math.Abs(outData1[i] - outData2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, outData2[i], outData1[i], diff)
		}
	}
}

func TestTremoloDepthZeroIsTransparent(t *testing.T) {
	tremolo, err := NewTremolo(48000,
		WithTremoloDepth(0),
		WithTremoloMix(1),
	)
	if err != nil {
		t.Fatalf("NewTremolo() error = %v", err)
	}

	for i := range 512 {
		in := 0.5 * math.Sin(2*math.Pi*440*float64(i)/48000)

		out := tremolo.Process(in)
		if diff := math.Abs(out - in); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, out, in)
		}
	}
}

func TestTremoloValidation(t *testing.T) {
	_, err := NewTremolo(0)
	if err == nil {
		t.Fatal("NewTremolo() expected error for invalid sample rate")
	}

	_, err = NewTremolo(48000, WithTremoloDepth(1.2))
	if err == nil {
		t.Fatal("NewTremolo() expected error for invalid depth")
	}

	_, err = NewTremolo(48000, WithTremoloSmoothingMs(-1))
	if err == nil {
		t.Fatal("NewTremolo() expected error for invalid smoothing")
	}
}

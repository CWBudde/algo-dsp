package delay

import (
	"math"
	"testing"
)

func TestDAVVibrato32DelayAndClock(t *testing.T) {
	line, err := NewDAVVibrato32(24000)
	if err != nil {
		t.Fatal(err)
	}
	if err := line.Configure(48000, .05, .2); err != nil {
		t.Fatal(err)
	}
	if line.size != 30 {
		t.Fatalf("delay span %d, want 30", line.size)
	}
	if got, want := line.stepSine, math.Sin(2*math.Pi*.2/44100); math.Abs(got-want) > 1e-15 {
		t.Fatalf("step %g, want %g", got, want)
	}
	for frame := range 128 {
		var input float32
		if frame == 48 {
			input = 1
		}
		output := line.ProcessSample(input)
		if frame == 66 && !(output > .99 && output <= 1.01) {
			t.Fatalf("peak at %d = %g, want near unity", frame, output)
		}
	}
	if err := line.Configure(96000, 1, 10); err != nil {
		t.Fatal(err)
	}
	if line.size != 24000 {
		t.Fatalf("delay span %d, want 24000", line.size)
	}
}

func TestDAVVibrato32Retuning(t *testing.T) {
	line, err := NewDAVVibrato32(24000)
	if err != nil {
		t.Fatal(err)
	}
	if err := line.Configure(44100, .05, .2); err != nil {
		t.Fatal(err)
	}
	for range 10 {
		line.ProcessSample(.5)
	}
	wantPosition, wantSine := line.position, line.sine
	if err := line.Configure(48000, .2, .3); err != nil {
		t.Fatal(err)
	}
	if line.position != wantPosition || line.sine != wantSine {
		t.Fatal("retuning reset oscillator or write position")
	}
	if err := line.Configure(192000, 1, 10); err == nil || line.position != wantPosition || line.sine != wantSine {
		t.Fatal("invalid retuning mutated state")
	}
	if err := line.ReserveDelay(48000); err != nil {
		t.Fatal(err)
	}
	if line.position != wantPosition || line.sine != wantSine {
		t.Fatal("reserving storage reset the oscillator or write position")
	}
	line.ClearHistory()
	if line.position != wantPosition || line.sine != wantSine || line.ProcessSample(0) != 0 {
		t.Fatal("clearing history changed position or retained a sample")
	}
	if got := testing.AllocsPerRun(100, func() {
		if err := line.Configure(48000, .2, .3); err != nil {
			t.Fatal(err)
		}
		line.ProcessSample(.1)
	}); got != 0 {
		t.Fatalf("retuning and processing allocated %g times", got)
	}
	line.Reset()
	if line.position != 0 || line.sine != 0 || line.cosine != -1 || line.ProcessSample(0) != 0 {
		t.Fatal("reset did not restore the starting oscillator and delay")
	}
}

func TestDAVVibrato32MinimumDepth(t *testing.T) {
	line, err := NewDAVVibrato32(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := line.Configure(44100, .001, 10); err != nil {
		t.Fatal(err)
	}
	for i := range 96 {
		if value := line.ProcessSample(float32(i)); math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			t.Fatalf("zero-span sample %d is %g", i, value)
		}
	}
}

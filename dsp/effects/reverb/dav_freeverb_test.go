package reverb

import (
	"math"
	"testing"
)

func TestDAVFreeverb32ReleasedRecurrence(t *testing.T) {
	combs := [][2]int{{3, 5}}
	allpasses := [][2]int{{2, 4}}
	reverb, err := NewDAVFreeverb32(combs, allpasses)
	if err != nil {
		t.Fatal(err)
	}
	if err := reverb.SetGeometry(combs, allpasses); err != nil {
		t.Fatal(err)
	}
	if err := reverb.SetCombFeedback(.5, .25); err != nil {
		t.Fatal(err)
	}
	if err := reverb.SetAllpassFeedback([][2]float32{{.5, .75}}); err != nil {
		t.Fatal(err)
	}

	wantLeft := [...]float32{0, 0, 0, -1, 0, 1.5, -.375, -.09375}
	wantRight := [...]float32{0, 0, 0, 0, 0, -1, 0, 0, 0, 1.75}
	for frame := range wantRight {
		var input float32
		if frame == 0 {
			input = 1
		}
		left, right := reverb.ProcessSample(input)
		if frame < len(wantLeft) && left != wantLeft[frame] {
			t.Errorf("left frame %d = %g, want %g", frame, left, wantLeft[frame])
		}
		if right != wantRight[frame] {
			t.Errorf("right frame %d = %g, want %g", frame, right, wantRight[frame])
		}
	}
}

func TestDAVFreeverb32GeometryPreservesUnchangedStages(t *testing.T) {
	capacity := [][2]int{{5, 5}, {5, 5}}
	reverb, err := NewDAVFreeverb32(capacity, capacity)
	if err != nil {
		t.Fatal(err)
	}
	if err := reverb.SetGeometry([][2]int{{3, 3}}, [][2]int{{2, 2}}); err != nil {
		t.Fatal(err)
	}
	reverb.ProcessSample(1)
	reverb.ProcessSample(0)
	if err := reverb.SetGeometry([][2]int{{6, 3}}, [][2]int{{2, 2}}); err == nil {
		t.Fatal("accepted geometry outside the reserved buffer")
	}
	if err := reverb.SetGeometry([][2]int{{5, 3}}, [][2]int{{2, 2}}); err != nil {
		t.Fatal(err)
	}
	reverb.ProcessSample(0)
	if left, right := reverb.ProcessSample(0); left != 0 || right != -1 {
		t.Fatalf("geometry edit produced %g, %g, want 0, -1", left, right)
	}
	reverb.Reset()
	if left, right := reverb.ProcessSample(0); left != 0 || right != 0 {
		t.Fatalf("reset left history %g, %g", left, right)
	}
	if err := reverb.SetCombFeedback(.5, float32(math.Inf(1))); err == nil {
		t.Fatal("accepted invalid comb feedback")
	}
}

func TestDAVFreeverb32ProcessingAndGeometryAllocateNothing(t *testing.T) {
	sizes := [][2]int{{8, 10}, {6, 7}}
	reverb, err := NewDAVFreeverb32(sizes, sizes)
	if err != nil {
		t.Fatal(err)
	}
	allpass := [][2]float32{{.5, .6}, {.7, .8}}
	if allocated := testing.AllocsPerRun(100, func() {
		_ = reverb.SetGeometry(sizes[:1], sizes[:1])
		_ = reverb.SetGeometry(sizes, sizes)
		_ = reverb.SetCombFeedback(.84, .2)
		_ = reverb.SetAllpassFeedback(allpass)
		reverb.ProcessSample(.25)
	}); allocated != 0 {
		t.Fatalf("DSP allocated %g times per callback", allocated)
	}
}

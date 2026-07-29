package pitch

import (
	"math"
	"testing"
)

func TestFrequencyToMIDIReference(t *testing.T) {
	tests := []struct {
		name string
		hz   float64
		want float64
	}{
		{"A4", 440.0, 69},
		{"A3", 220.0, 57},
		{"A5", 880.0, 81},
		{"C4", 261.6255653005986, 60},
		{"A0", 27.5, 21},
		{"C8", 4186.009044809578, 108},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FrequencyToMIDI(tt.hz, DefaultReferenceHz)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("FrequencyToMIDI(%g) = %.9f, want %.9f", tt.hz, got, tt.want)
			}
		})
	}
}

func TestMIDIToFrequencyRoundTrip(t *testing.T) {
	for note := 0; note <= 127; note++ {
		hz := MIDIToFrequency(float64(note), DefaultReferenceHz)
		if hz <= 0 || math.IsNaN(hz) || math.IsInf(hz, 0) {
			t.Fatalf("MIDIToFrequency(%d) = %g, want a positive finite value", note, hz)
		}

		back := FrequencyToMIDI(hz, DefaultReferenceHz)
		if math.Abs(back-float64(note)) > 1e-12 {
			t.Errorf("round trip for note %d = %.15f", note, back)
		}
	}
}

func TestMIDIToFrequencyKnownValues(t *testing.T) {
	tests := []struct {
		name string
		midi float64
		want float64
	}{
		{"A4", 69, 440.0},
		{"C4", 60, 261.6255653005986},
		{"A0", 21, 27.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MIDIToFrequency(tt.midi, DefaultReferenceHz)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("MIDIToFrequency(%g) = %.9f, want %.9f", tt.midi, got, tt.want)
			}
		})
	}
}

func TestReferenceHzDetune(t *testing.T) {
	// A = 432 Hz is 1200*log2(432/440) = -31.766... cents below concert pitch,
	// so every frequency maps to a correspondingly higher MIDI note number.
	const wantCents = -31.7666

	got := CentsBetween(DefaultReferenceHz, 432.0)
	if math.Abs(got-wantCents) > 1e-3 {
		t.Errorf("CentsBetween(440, 432) = %.6f, want %.6f", got, wantCents)
	}

	midi := FrequencyToMIDI(440.0, 432.0)
	wantMIDI := a4MIDINote - wantCents/centsPerSemitone

	if math.Abs(midi-wantMIDI) > 1e-6 {
		t.Errorf("FrequencyToMIDI(440, ref=432) = %.6f, want %.6f", midi, wantMIDI)
	}
}

func TestSemitoneRatioRoundTrip(t *testing.T) {
	tests := []float64{-24, -12, -7, -1, -0.25, 0, 0.25, 1, 7, 12, 24}

	for _, semitones := range tests {
		ratio := SemitonesToRatio(semitones)

		back := RatioToSemitones(ratio)
		if math.Abs(back-semitones) > 1e-12 {
			t.Errorf("RatioToSemitones(SemitonesToRatio(%g)) = %.15f", semitones, back)
		}
	}

	if got := SemitonesToRatio(0); got != 1 {
		t.Errorf("SemitonesToRatio(0) = %g, want 1", got)
	}

	if got := SemitonesToRatio(12); math.Abs(got-2) > 1e-12 {
		t.Errorf("SemitonesToRatio(12) = %g, want 2", got)
	}
}

func TestCentsBetween(t *testing.T) {
	tests := []struct {
		name      string
		from, to  float64
		wantCents float64
		tolerance float64
	}{
		{"unison", 440, 440, 0, 1e-12},
		{"octave up", 440, 880, 1200, 1e-9},
		{"octave down", 440, 220, -1200, 1e-9},
		{"semitone up", 440, 466.1637615180899, 100, 1e-6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CentsBetween(tt.from, tt.to)
			if math.Abs(got-tt.wantCents) > tt.tolerance {
				t.Errorf("CentsBetween(%g, %g) = %.9f, want %.9f", tt.from, tt.to, got, tt.wantCents)
			}
		})
	}
}

func TestPitchClassString(t *testing.T) {
	if got := PitchClassCSharp.String(); got != "C#" {
		t.Errorf("PitchClassCSharp.String() = %q, want %q", got, "C#")
	}

	if got := PitchClass(12).String(); got != "PitchClass(12)" {
		t.Errorf("PitchClass(12).String() = %q, want %q", got, "PitchClass(12)")
	}
}

func TestNewScaleValidation(t *testing.T) {
	tests := []struct {
		name    string
		root    PitchClass
		degrees []int
		wantErr bool
	}{
		{"valid major", PitchClassC, []int{0, 2, 4, 5, 7, 9, 11}, false},
		{"valid root only", PitchClassA, []int{0}, false},
		{"duplicates collapse", PitchClassC, []int{0, 0, 4, 4, 7}, false},
		{"invalid root", PitchClass(12), []int{0}, true},
		{"nil degrees", PitchClassC, nil, true},
		{"empty degrees", PitchClassC, []int{}, true},
		{"degree too high", PitchClassC, []int{0, 12}, true},
		{"degree negative", PitchClassC, []int{0, -1}, true},
		{"missing root degree", PitchClassC, []int{2, 4, 7}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewScale(tt.root, tt.degrees)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewScale error = %v, wantErr = %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if !s.IsZero() {
					t.Errorf("NewScale returned a usable scale alongside an error")
				}

				return
			}

			if s.IsZero() {
				t.Errorf("NewScale returned the zero value for a valid scale")
			}

			if s.Root() != tt.root {
				t.Errorf("Root() = %v, want %v", s.Root(), tt.root)
			}
		})
	}
}

func TestScaleDuplicateDegreesCollapse(t *testing.T) {
	s, err := NewScale(PitchClassC, []int{0, 0, 4, 4, 7})
	if err != nil {
		t.Fatalf("NewScale: %v", err)
	}

	got := s.Degrees()

	want := []int{0, 4, 7}
	if len(got) != len(want) {
		t.Fatalf("Degrees() = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Degrees() = %v, want %v", got, want)
		}
	}
}

func TestScaleContains(t *testing.T) {
	major := ScaleMajor(PitchClassC)

	// C major: C D E F G A B.
	want := [pitchClassCount]bool{
		true, false, true, false, true, true, false, true, false, true, false, true,
	}

	for pc := range pitchClassCount {
		if got := major.Contains(PitchClass(pc)); got != want[pc] {
			t.Errorf("ScaleMajor(C).Contains(%v) = %v, want %v", PitchClass(pc), got, want[pc])
		}
	}

	if major.Contains(PitchClass(12)) {
		t.Errorf("Contains(out of range) = true, want false")
	}
}

func TestScaleContainsTransposed(t *testing.T) {
	// D major: D E F# G A B C#.
	dMajor := ScaleMajor(PitchClassD)

	for _, pc := range []PitchClass{
		PitchClassD, PitchClassE, PitchClassFSharp,
		PitchClassG, PitchClassA, PitchClassB, PitchClassCSharp,
	} {
		if !dMajor.Contains(pc) {
			t.Errorf("ScaleMajor(D).Contains(%v) = false, want true", pc)
		}
	}

	for _, pc := range []PitchClass{PitchClassC, PitchClassDSharp, PitchClassF} {
		if dMajor.Contains(pc) {
			t.Errorf("ScaleMajor(D).Contains(%v) = true, want false", pc)
		}
	}
}

func TestScaleSnapChromatic(t *testing.T) {
	chromatic := ScaleChromatic(PitchClassC)

	// Chromatic snapping is nearest-semitone quantisation, except that an
	// exact half-semitone tie resolves down rather than away from zero.
	tests := []struct {
		midi float64
		want float64
	}{
		{60.0, 60}, {60.4, 60}, {60.6, 61}, {59.5, 59}, {-1.2, -1}, {127.4, 127},
	}

	for _, tt := range tests {
		if got := chromatic.SnapMIDI(tt.midi); got != tt.want {
			t.Errorf("chromatic.SnapMIDI(%g) = %g, want %g", tt.midi, got, tt.want)
		}
	}
}

func TestScaleSnapMajor(t *testing.T) {
	cMajor := ScaleMajor(PitchClassC)

	tests := []struct {
		name string
		midi float64
		want float64
	}{
		{"already a degree", 60.0, 60},
		{"slightly sharp of C", 60.3, 60},
		{"just below D", 61.4, 62},
		{"exact tie resolves low", 61.0, 60},
		{"tie just above midpoint", 61.05, 62},
		{"tie just below midpoint", 60.95, 60},
		{"E is a degree", 64.2, 64},
		{"F is a degree", 65.0, 65},
		{"between A and B", 70.0, 69},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cMajor.SnapMIDI(tt.midi); got != tt.want {
				t.Errorf("SnapMIDI(%g) = %g, want %g", tt.midi, got, tt.want)
			}
		})
	}
}

func TestScaleSnapMinorPentatonic(t *testing.T) {
	// C minor pentatonic: C D# F G A#, i.e. MIDI 60, 63, 65, 67, 70 in octave 4.
	scale := ScaleMinorPentatonic(PitchClassC)

	tests := []struct {
		midi float64
		want float64
	}{
		{60.0, 60}, {61.0, 60}, {62.0, 63}, {64.0, 63}, {66.0, 65}, {68.0, 67}, {69.0, 70},
	}

	for _, tt := range tests {
		if got := scale.SnapMIDI(tt.midi); got != tt.want {
			t.Errorf("minor pentatonic SnapMIDI(%g) = %g, want %g", tt.midi, got, tt.want)
		}
	}
}

func TestScaleSnapTransposed(t *testing.T) {
	// D major contains C# but not C; MIDI 60 is C4, which must snap to C#4 (61)
	// because 61 is one semitone away while the next degree down, B3 (59), is
	// also one away and ties resolve low.
	dMajor := ScaleMajor(PitchClassD)

	if got := dMajor.SnapMIDI(60.0); got != 59 {
		t.Errorf("D major SnapMIDI(60) = %g, want 59 (tie resolves low)", got)
	}

	if got := dMajor.SnapMIDI(60.3); got != 61 {
		t.Errorf("D major SnapMIDI(60.3) = %g, want 61", got)
	}

	if got := dMajor.SnapMIDI(62.0); got != 62 {
		t.Errorf("D major SnapMIDI(62) = %g, want 62", got)
	}
}

func TestScaleSnapSingleDegree(t *testing.T) {
	// A scale with only the root has the widest possible gaps; the outward
	// search must still terminate on a member for every input.
	scale, err := NewScale(PitchClassC, []int{0})
	if err != nil {
		t.Fatalf("NewScale: %v", err)
	}

	for note := 0; note <= 127; note++ {
		got := scale.SnapMIDI(float64(note))
		if !scale.Contains(pitchClassOfMIDI(int(got))) {
			t.Fatalf("SnapMIDI(%d) = %g, which is not a scale member", note, got)
		}

		if math.Abs(got-float64(note)) > maxSnapOffset {
			t.Fatalf("SnapMIDI(%d) = %g, further than %d semitones away", note, got, maxSnapOffset)
		}
	}
}

func TestScaleSnapExtremes(t *testing.T) {
	cMajor := ScaleMajor(PitchClassC)

	// MIDI 0 is C-1 and 127 is G9; both are scale members of C major, so they
	// must be returned unchanged rather than walking off either end.
	if got := cMajor.SnapMIDI(0); got != 0 {
		t.Errorf("SnapMIDI(0) = %g, want 0", got)
	}

	if got := cMajor.SnapMIDI(127); got != 127 {
		t.Errorf("SnapMIDI(127) = %g, want 127", got)
	}

	// Negative note numbers must use a well-defined pitch class.
	if got := cMajor.SnapMIDI(-12); got != -12 {
		t.Errorf("SnapMIDI(-12) = %g, want -12", got)
	}
}

func TestScaleSnapNonFinite(t *testing.T) {
	cMajor := ScaleMajor(PitchClassC)

	if got := cMajor.SnapMIDI(math.NaN()); !math.IsNaN(got) {
		t.Errorf("SnapMIDI(NaN) = %g, want NaN", got)
	}

	if got := cMajor.SnapMIDI(math.Inf(1)); !math.IsInf(got, 1) {
		t.Errorf("SnapMIDI(+Inf) = %g, want +Inf", got)
	}
}

func TestScaleZeroValue(t *testing.T) {
	var zero Scale

	if !zero.IsZero() {
		t.Errorf("zero value IsZero() = false, want true")
	}

	if zero.Contains(PitchClassC) {
		t.Errorf("zero value Contains(C) = true, want false")
	}

	if got := zero.SnapMIDI(60.4); got != 60 {
		t.Errorf("zero value SnapMIDI(60.4) = %g, want 60 (rounding only)", got)
	}

	if got := zero.Degrees(); len(got) != 0 {
		t.Errorf("zero value Degrees() = %v, want empty", got)
	}
}

func TestPredefinedScaleDegreeCounts(t *testing.T) {
	tests := []struct {
		name  string
		scale Scale
		want  int
	}{
		{"chromatic", ScaleChromatic(PitchClassC), 12},
		{"major", ScaleMajor(PitchClassC), 7},
		{"natural minor", ScaleNaturalMinor(PitchClassC), 7},
		{"harmonic minor", ScaleHarmonicMinor(PitchClassC), 7},
		{"major pentatonic", ScaleMajorPentatonic(PitchClassC), 5},
		{"minor pentatonic", ScaleMinorPentatonic(PitchClassC), 5},
		{"blues", ScaleBlues(PitchClassC), 6},
		{"whole tone", ScaleWholeTone(PitchClassC), 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(tt.scale.Degrees()); got != tt.want {
				t.Errorf("%s has %d degrees, want %d", tt.name, got, tt.want)
			}

			if !tt.scale.Contains(PitchClassC) {
				t.Errorf("%s rooted on C does not contain C", tt.name)
			}
		})
	}
}

func TestScaleSnapZeroAlloc(t *testing.T) {
	cMajor := ScaleMajor(PitchClassC)

	allocs := testing.AllocsPerRun(50, func() {
		_ = cMajor.SnapMIDI(61.4)
	})

	if allocs != 0 {
		t.Errorf("SnapMIDI allocs = %g, want 0", allocs)
	}
}

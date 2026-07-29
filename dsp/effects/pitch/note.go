package pitch

import (
	"fmt"
	"math"
	"sort"
)

const (
	// DefaultReferenceHz is the standard concert pitch for A4 (MIDI note 69).
	DefaultReferenceHz = 440.0

	// a4MIDINote is the MIDI note number of the reference pitch A4.
	a4MIDINote = 69.0

	semitonesPerOctave = 12.0
	centsPerSemitone   = 100.0

	// pitchClassCount is the number of semitones in an octave, and therefore
	// the number of distinct pitch classes.
	pitchClassCount = 12

	// maxSnapOffset is the largest distance in semitones that [Scale.SnapMIDI]
	// searches outward from the rounded input note. Every scale contains its
	// root, so the widest possible gap between consecutive degrees is one
	// octave and no note is ever further than six semitones from a degree.
	maxSnapOffset = 6
)

// SemitonesToRatio converts a shift in semitones to a frequency ratio.
func SemitonesToRatio(semitones float64) float64 {
	return math.Pow(2, semitones/semitonesPerOctave)
}

// RatioToSemitones converts a frequency ratio to a shift in semitones. The
// ratio must be positive; other values yield NaN.
func RatioToSemitones(ratio float64) float64 {
	return semitonesPerOctave * math.Log2(ratio)
}

// FrequencyToMIDI converts a frequency in Hz to a fractional MIDI note number,
// where referenceHz is the frequency of A4 (MIDI note 69). Pass
// [DefaultReferenceHz] for standard concert pitch. Non-positive arguments
// yield NaN.
func FrequencyToMIDI(hz, referenceHz float64) float64 {
	return a4MIDINote + semitonesPerOctave*math.Log2(hz/referenceHz)
}

// MIDIToFrequency converts a fractional MIDI note number to a frequency in Hz,
// where referenceHz is the frequency of A4 (MIDI note 69). It is the inverse
// of [FrequencyToMIDI].
func MIDIToFrequency(midi, referenceHz float64) float64 {
	return referenceHz * math.Pow(2, (midi-a4MIDINote)/semitonesPerOctave)
}

// CentsBetween returns the interval from fromHz to toHz in cents, positive
// when toHz is the higher frequency. Non-positive arguments yield NaN.
func CentsBetween(fromHz, toHz float64) float64 {
	return centsPerSemitone * semitonesPerOctave * math.Log2(toHz/fromHz)
}

// PitchClass identifies one of the twelve equal-tempered pitch classes as a
// semitone offset from C.
type PitchClass uint8

// The twelve pitch classes. Enharmonic equivalents (D-flat for C-sharp, and so
// on) are the same value; only the sharp spelling is named.
const (
	PitchClassC PitchClass = iota
	PitchClassCSharp
	PitchClassD
	PitchClassDSharp
	PitchClassE
	PitchClassF
	PitchClassFSharp
	PitchClassG
	PitchClassGSharp
	PitchClassA
	PitchClassASharp
	PitchClassB
)

func (pc PitchClass) valid() bool { return pc < pitchClassCount }

// String returns the sharp spelling of the pitch class, or "PitchClass(n)" for
// an out-of-range value.
func (pc PitchClass) String() string {
	names := [pitchClassCount]string{
		"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B",
	}
	if !pc.valid() {
		return fmt.Sprintf("PitchClass(%d)", uint8(pc))
	}

	return names[pc]
}

// pitchClassOfMIDI returns the pitch class of an integer MIDI note number,
// handling negative note numbers correctly.
func pitchClassOfMIDI(note int) PitchClass {
	return PitchClass(((note % pitchClassCount) + pitchClassCount) % pitchClassCount)
}

// Scale is an immutable set of allowed pitch classes, expressed as a root plus
// the scale degrees measured in semitones above that root.
//
// Scale is a comparable value type and is safe to copy. Its zero value is not
// a usable scale: [Scale.IsZero] reports true, [Scale.Contains] always reports
// false and [Scale.SnapMIDI] passes its input through unchanged.
type Scale struct {
	root PitchClass
	// mask is a bitset over scale degrees: bit i is set when root+i semitones
	// is a member of the scale. Bit 0 (the root) is always set for any scale
	// built by [NewScale].
	mask uint16
}

// NewScale builds a scale from a root pitch class and a set of degrees in
// semitones above the root. Degrees must lie in [0, 11] and must include 0
// (the root itself); duplicates are accepted and collapse to a single member.
func NewScale(root PitchClass, degrees []int) (Scale, error) {
	if !root.valid() {
		return Scale{}, fmt.Errorf("pitch scale root must be in [0, %d): %d", pitchClassCount, root)
	}

	if len(degrees) == 0 {
		return Scale{}, fmt.Errorf("pitch scale must have at least one degree")
	}

	var mask uint16

	for _, d := range degrees {
		if d < 0 || d >= pitchClassCount {
			return Scale{}, fmt.Errorf("pitch scale degree must be in [0, %d): %d", pitchClassCount, d)
		}

		mask |= 1 << uint(d)
	}

	if mask&1 == 0 {
		return Scale{}, fmt.Errorf("pitch scale degrees must include the root (0)")
	}

	return Scale{root: root, mask: mask}, nil
}

// mustScale builds a scale from degrees known at compile time to be valid. It
// panics on invalid input, which would be a bug in this package.
func mustScale(root PitchClass, degrees []int) Scale {
	s, err := NewScale(root, degrees)
	if err != nil {
		panic(err)
	}

	return s
}

// Root returns the pitch class the scale is rooted on.
func (s Scale) Root() PitchClass { return s.root }

// IsZero reports whether s is the unusable zero value.
func (s Scale) IsZero() bool { return s.mask == 0 }

// Degrees returns the scale degrees in semitones above the root, in ascending
// order. It allocates, so it is intended for inspection and display rather
// than for processing hot paths.
func (s Scale) Degrees() []int {
	out := make([]int, 0, pitchClassCount)

	for i := range pitchClassCount {
		if s.mask&(1<<uint(i)) != 0 {
			out = append(out, i)
		}
	}

	sort.Ints(out)

	return out
}

// Contains reports whether the pitch class is a member of the scale.
func (s Scale) Contains(pc PitchClass) bool {
	if !pc.valid() {
		return false
	}

	offset := (int(pc) - int(s.root) + pitchClassCount) % pitchClassCount

	return s.mask&(1<<uint(offset)) != 0
}

// SnapMIDI returns the MIDI note number of the scale degree nearest to midi,
// measuring distance from the unrounded input so that, for example, 61.4 under
// C major snaps up to 62 (D) rather than down to 60 (C). An exact tie resolves
// to the lower note. Non-finite input passes through unchanged, and the
// zero-value scale only rounds to the nearest semitone.
func (s Scale) SnapMIDI(midi float64) float64 {
	if math.IsNaN(midi) || math.IsInf(midi, 0) {
		return midi
	}

	nearest := int(math.Round(midi))
	if s.IsZero() {
		return float64(nearest)
	}

	best := nearest
	bestDistance := math.Inf(1)

	// Candidates are visited in ascending order and only a strictly smaller
	// distance replaces the incumbent, so an exact tie keeps the lower note.
	for candidate := nearest - maxSnapOffset; candidate <= nearest+maxSnapOffset; candidate++ {
		if !s.Contains(pitchClassOfMIDI(candidate)) {
			continue
		}

		distance := math.Abs(float64(candidate) - midi)
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}

	return float64(best)
}

// ScaleChromatic returns all twelve pitch classes. Snapping to it quantises to
// the nearest semitone.
func ScaleChromatic(root PitchClass) Scale {
	return mustScale(root, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
}

// ScaleMajor returns the Ionian (major) scale.
func ScaleMajor(root PitchClass) Scale {
	return mustScale(root, []int{0, 2, 4, 5, 7, 9, 11})
}

// ScaleNaturalMinor returns the Aeolian (natural minor) scale.
func ScaleNaturalMinor(root PitchClass) Scale {
	return mustScale(root, []int{0, 2, 3, 5, 7, 8, 10})
}

// ScaleHarmonicMinor returns the harmonic minor scale (natural minor with a
// raised seventh).
func ScaleHarmonicMinor(root PitchClass) Scale {
	return mustScale(root, []int{0, 2, 3, 5, 7, 8, 11})
}

// ScaleMajorPentatonic returns the five-note major pentatonic scale.
func ScaleMajorPentatonic(root PitchClass) Scale {
	return mustScale(root, []int{0, 2, 4, 7, 9})
}

// ScaleMinorPentatonic returns the five-note minor pentatonic scale.
func ScaleMinorPentatonic(root PitchClass) Scale {
	return mustScale(root, []int{0, 3, 5, 7, 10})
}

// ScaleBlues returns the minor pentatonic scale with the added flat fifth.
func ScaleBlues(root PitchClass) Scale {
	return mustScale(root, []int{0, 3, 5, 6, 7, 10})
}

// ScaleWholeTone returns the six-note whole-tone scale.
func ScaleWholeTone(root PitchClass) Scale {
	return mustScale(root, []int{0, 2, 4, 6, 8, 10})
}

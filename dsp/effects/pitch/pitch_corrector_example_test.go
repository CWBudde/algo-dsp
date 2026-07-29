package pitch_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// exampleSawtooth builds a band-limited sawtooth, which gives the detector the
// harmonic structure a real instrument or voice would have.
func exampleSawtooth(freqHz, sampleRate, amplitude float64, n int) []float64 {
	out := make([]float64, n)

	for h := 1; float64(h)*freqHz < sampleRate/2; h++ {
		step := 2 * math.Pi * freqHz * float64(h) / sampleRate
		gain := amplitude / float64(h)

		for i := range out {
			out[i] += gain * math.Sin(step*float64(i))
		}
	}

	return out
}

func ExamplePitchCorrector() {
	corrector, err := pitch.NewPitchCorrector(48000, pitch.WithCorrectionSpeedMs(0))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// A note roughly 47 cents sharp of A4, snapped to the chromatic grid.
	input := exampleSawtooth(452, 48000, 0.4, 96000)
	corrector.ProcessInPlace(input)

	fmt.Printf("%.0f -> %.0f Hz (%.2f semitones)\n",
		corrector.DetectedFrequency(), corrector.TargetFrequency(),
		corrector.AppliedSemitones())
	// Output: 452 -> 440 Hz (-0.47 semitones)
}

// ExamplePitchCorrector_scale constrains correction to a musical scale, so
// notes outside the scale are pulled to the nearest degree rather than merely
// to the nearest semitone.
func ExamplePitchCorrector_scale() {
	corrector, err := pitch.NewPitchCorrector(48000,
		pitch.WithCorrectionScale(pitch.ScaleMajor(pitch.PitchClassC)),
		pitch.WithCorrectionSpeedMs(0))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// 460 Hz sits between A4 and A#4. A#4 is not in C major, so the correction
	// pulls the note down to A4 rather than up.
	input := exampleSawtooth(460, 48000, 0.4, 96000)
	corrector.ProcessInPlace(input)

	fmt.Printf("target %.0f Hz\n", corrector.TargetFrequency())
	// Output: target 440 Hz
}

func ExampleScale_SnapMIDI() {
	cMajor := pitch.ScaleMajor(pitch.PitchClassC)

	// MIDI 61.4 lies between C#4 and D4. C#4 is not in C major, and D4 is the
	// closer of the two scale degrees, so the note snaps up.
	fmt.Printf("%.0f\n", cMajor.SnapMIDI(61.4))
	// Output: 62
}

func ExampleFrequencyToMIDI() {
	fmt.Printf("%.0f\n", pitch.FrequencyToMIDI(440, pitch.DefaultReferenceHz))
	// Output: 69
}

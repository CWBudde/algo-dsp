package pitch

// PitchProcessor defines the shared API for interchangeable pitch shifters.
//
// Implementations include [PitchShifter] (time-domain WSOLA-style) and
// [SpectralPitchShifter] (frequency-domain phase-vocoder).
//
//nolint:revive
type PitchProcessor interface {
	SampleRate() float64
	SetSampleRate(sampleRate float64) error

	PitchRatio() float64
	PitchSemitones() float64
	SetPitchRatio(ratio float64) error
	SetPitchSemitones(semitones float64) error

	Reset()
	Process(input []float64) []float64
	ProcessInPlace(buf []float64)
}

var (
	_ PitchProcessor = (*PitchShifter)(nil)
	_ PitchProcessor = (*SpectralPitchShifter)(nil)
)

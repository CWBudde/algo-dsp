package melody

// Bass-line analysis parameters of [BassPreset].
const (
	// BassFFTSize is the FFT size of [BassPreset] (341 ms at 24 kHz, about
	// 2.9 Hz bin spacing, which resolves semitones near 40 Hz).
	BassFFTSize = 8192
	// BassMinMIDI is the lowest pitch candidate of [BassPreset] (E1,
	// 41.2 Hz, the open low string of a four-string bass).
	BassMinMIDI = 28.0
	// BassMaxMIDI is the highest pitch candidate of [BassPreset] (C4,
	// 262 Hz).
	BassMaxMIDI = 60.0
	// BassMinHz is the lower edge of the analysis band of [BassPreset].
	BassMinHz = 30.0
	// BassMaxHz is the upper edge of the analysis band of [BassPreset].
	BassMaxHz = 1200.0
	// BassHarmonics is the number of harmonics summed per candidate by
	// [BassPreset].
	BassHarmonics = 6
	// BassSmoothing is the median-filter radius in seconds of [BassPreset].
	BassSmoothing = 0.04
	// BassNoteMinDuration is the shortest note in seconds of [BassPreset].
	BassNoteMinDuration = 0.10
	// BassNoteJump is the pitch deviation in semitones that starts a new
	// note in [BassPreset].
	BassNoteJump = 0.8
	// BassNoteJumpDuration is the time in seconds the deviation must
	// persist in [BassPreset].
	BassNoteJumpDuration = 0.05
	// BassOnsetSnap is the onset snap distance in seconds of [BassPreset],
	// meant for onsets detected on a bass stem.
	BassOnsetSnap = 0.06
)

// BassPreset returns the options that retune [Analyze] for a bass line:
// FFT [BassFFTSize], pitch candidates [BassMinMIDI]..[BassMaxMIDI], band
// [BassMinHz]..[BassMaxHz], [BassHarmonics] harmonics with the default
// decay, the default voicing threshold, smoothing [BassSmoothing], notes of
// at least [BassNoteMinDuration] split by a [BassNoteJump] semitone jump
// lasting [BassNoteJumpDuration], and onset snapping within
// [BassOnsetSnap]. Options given after the preset override it.
//
// These are AudioVisualizer's BassMelodyOptions; pair the notes with
// [BassCleanOptions] in [Clean].
func BassPreset() []Option {
	return []Option{
		WithFFTSize(BassFFTSize),
		WithMIDIRange(BassMinMIDI, BassMaxMIDI),
		WithFrequencyRange(BassMinHz, BassMaxHz),
		WithHarmonics(BassHarmonics, DefaultHarmonicDecay),
		WithVoicingThreshold(DefaultVoicingThreshold),
		WithSmoothing(BassSmoothing),
		WithNoteMinDuration(BassNoteMinDuration),
		WithNoteJump(BassNoteJump, BassNoteJumpDuration),
		WithOnsetSnap(BassOnsetSnap),
	}
}

// BassCleanOptions returns the [Clean] options for a bass line: the defaults
// with the floor lowered to [BassFloorMIDI]. These are AudioVisualizer's
// BassCleanParams. Options given after them override them.
func BassCleanOptions() []CleanOption {
	return []CleanOption{WithFloorMIDI(BassFloorMIDI)}
}

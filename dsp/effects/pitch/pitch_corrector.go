package pitch

import (
	"fmt"
	"math"
)

const (
	defaultCorrectionAmount       = 1.0
	defaultMaxCorrectionSemitones = 12.0
	defaultCorrectionSpeedMs      = 20.0
	defaultCorrectionBlockSize    = 2048
	defaultCorrectionCrossfadeMs  = 5.0
	defaultCorrectionConfidence   = 0.5

	minCorrectionBlockSize   = 64
	minCorrectionReferenceHz = 400.0
	maxCorrectionReferenceHz = 480.0

	// maxCorrectionSemitonesLimit bounds the configurable correction range.
	maxCorrectionSemitonesLimit = 24.0

	// correctionShiftLimit keeps the applied shift strictly inside the pitch
	// shifters' accepted ratio range of [0.25, 4], so the corrector can never
	// hand them a value they reject.
	correctionShiftLimit = 23.9

	// correctionIdentityEps is the applied-shift magnitude below which the
	// shifter is bypassed entirely, making a zero correction bit-exact.
	correctionIdentityEps = 1e-9
)

// CorrectionMode selects how the target pitch for a detected note is chosen.
type CorrectionMode int

const (
	// CorrectionModeScale snaps the detected pitch to the nearest degree of
	// the configured scale. This is the default.
	CorrectionModeScale CorrectionMode = iota
	// CorrectionModeFixed pulls every detected pitch toward one fixed target
	// frequency, which is useful for drones and for tuning a sustained note.
	CorrectionModeFixed
)

// PitchCorrectorOption mutates pitch corrector construction parameters.
type PitchCorrectorOption func(*pitchCorrectorConfig) error

type pitchCorrectorConfig struct {
	mode          CorrectionMode
	scale         Scale
	targetHz      float64
	referenceHz   float64
	amount        float64
	maxSemitones  float64
	speedMs       float64
	blockSize     int
	crossfadeMs   float64
	minConfidence float64
	shifter       PitchProcessor
	tracker       *PitchTracker
}

func defaultPitchCorrectorConfig() pitchCorrectorConfig {
	return pitchCorrectorConfig{
		mode:          CorrectionModeScale,
		scale:         ScaleChromatic(PitchClassC),
		referenceHz:   DefaultReferenceHz,
		amount:        defaultCorrectionAmount,
		maxSemitones:  defaultMaxCorrectionSemitones,
		speedMs:       defaultCorrectionSpeedMs,
		blockSize:     defaultCorrectionBlockSize,
		crossfadeMs:   defaultCorrectionCrossfadeMs,
		minConfidence: defaultCorrectionConfidence,
	}
}

// WithCorrectionScale selects the scale that detected pitches snap to and
// switches the corrector to [CorrectionModeScale]. The default is the
// chromatic scale, which quantises to the nearest semitone.
func WithCorrectionScale(scale Scale) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if scale.IsZero() {
			return fmt.Errorf("pitch corrector scale must not be the zero value")
		}

		cfg.scale = scale
		cfg.mode = CorrectionModeScale

		return nil
	}
}

// WithCorrectionTargetHz pulls every detected pitch toward one fixed frequency
// and switches the corrector to [CorrectionModeFixed].
func WithCorrectionTargetHz(hz float64) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if !isFinitePositive(hz) {
			return fmt.Errorf("pitch corrector target frequency must be positive and finite: %f", hz)
		}

		cfg.targetHz = hz
		cfg.mode = CorrectionModeFixed

		return nil
	}
}

// WithCorrectionReferenceHz sets the frequency of A4 used to place the note
// grid. The default is [DefaultReferenceHz]; valid values are 400 to 480 Hz.
func WithCorrectionReferenceHz(hz float64) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if err := validateCorrectionReferenceHz(hz); err != nil {
			return err
		}

		cfg.referenceHz = hz

		return nil
	}
}

// WithCorrectionAmount sets how much of the detected error is corrected, from
// 0 (bypass) to 1 (fully snapped). The default is 1. Because the interpolation
// happens in the semitone domain, 0.5 halves the error in cents.
func WithCorrectionAmount(amount float64) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if err := validateCorrectionAmount(amount); err != nil {
			return err
		}

		cfg.amount = amount

		return nil
	}
}

// WithMaxCorrectionSemitones bounds how far the corrector may shift the
// signal. Corrections beyond the bound are clamped rather than abandoned, so a
// badly mistuned or misdetected note degrades smoothly instead of switching
// the effect on and off audibly. The default is 12 semitones.
func WithMaxCorrectionSemitones(semitones float64) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if err := validateMaxCorrectionSemitones(semitones); err != nil {
			return err
		}

		cfg.maxSemitones = semitones

		return nil
	}
}

// WithCorrectionSpeedMs sets the retune time constant. The applied shift
// glides toward its target with this time constant, which both softens the
// onset of a correction and keeps consecutive analysis blocks at nearly the
// same ratio, reducing the seam artifacts described on [PitchCorrector]. Zero
// retunes instantly and is audibly the roughest setting. The default is 20 ms.
func WithCorrectionSpeedMs(ms float64) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if err := validateCorrectionSpeedMs(ms); err != nil {
			return err
		}

		cfg.speedMs = ms

		return nil
	}
}

// WithCorrectionBlockSize sets how many samples are processed per ratio
// update. The default is 2048. Smaller blocks track pitch changes sooner at
// the cost of more seams and more shifter invocations.
func WithCorrectionBlockSize(samples int) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if samples < minCorrectionBlockSize {
			return fmt.Errorf("pitch corrector block size must be at least %d: %d",
				minCorrectionBlockSize, samples)
		}

		cfg.blockSize = samples

		return nil
	}
}

// WithCorrectionCrossfadeMs sets the length of the crossfade applied at each
// block seam. The default is 5 ms; zero disables the crossfade.
func WithCorrectionCrossfadeMs(ms float64) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if ms < 0 || math.IsNaN(ms) || math.IsInf(ms, 0) {
			return fmt.Errorf("pitch corrector crossfade must not be negative: %f", ms)
		}

		cfg.crossfadeMs = ms

		return nil
	}
}

// WithCorrectionConfidence sets the minimum detector confidence, in [0, 1],
// that engages a correction. Blocks below it are treated as unvoiced. The
// default is 0.5.
func WithCorrectionConfidence(minConfidence float64) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if minConfidence < 0 || minConfidence > 1 || math.IsNaN(minConfidence) {
			return fmt.Errorf("pitch corrector confidence must be in [0, 1]: %f", minConfidence)
		}

		cfg.minConfidence = minConfidence

		return nil
	}
}

// WithCorrectionShifter supplies the pitch shifter that applies the
// correction. The default is a [SpectralPitchShifter], whose bin-shifting path
// covers the small ratios correction needs. A [PitchShifter] also works but its
// music-tuned default window is longer than one correction block; shorten its
// sequence and raise the block size before using it.
func WithCorrectionShifter(shifter PitchProcessor) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if shifter == nil {
			return fmt.Errorf("pitch corrector shifter must not be nil")
		}

		cfg.shifter = shifter

		return nil
	}
}

// WithCorrectionTracker supplies a pre-configured pitch tracker.
func WithCorrectionTracker(tracker *PitchTracker) PitchCorrectorOption {
	return func(cfg *pitchCorrectorConfig) error {
		if tracker == nil {
			return fmt.Errorf("pitch corrector tracker must not be nil")
		}

		cfg.tracker = tracker

		return nil
	}
}

func validateCorrectionAmount(amount float64) error {
	if amount < 0 || amount > 1 || math.IsNaN(amount) {
		return fmt.Errorf("pitch corrector amount must be in [0, 1]: %f", amount)
	}

	return nil
}

func validateMaxCorrectionSemitones(semitones float64) error {
	if semitones <= 0 || semitones > maxCorrectionSemitonesLimit || math.IsNaN(semitones) {
		return fmt.Errorf("pitch corrector maximum correction must be in (0, %g] semitones: %f",
			maxCorrectionSemitonesLimit, semitones)
	}

	return nil
}

func validateCorrectionSpeedMs(ms float64) error {
	if ms < 0 || math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("pitch corrector speed must not be negative: %f", ms)
	}

	return nil
}

func validateCorrectionReferenceHz(hz float64) error {
	if hz < minCorrectionReferenceHz || hz > maxCorrectionReferenceHz ||
		math.IsNaN(hz) || math.IsInf(hz, 0) {
		return fmt.Errorf("pitch corrector reference frequency must be in [%g, %g] Hz: %f",
			minCorrectionReferenceHz, maxCorrectionReferenceHz, hz)
	}

	return nil
}

// PitchCorrector performs auto-tune style pitch correction: a [PitchTracker]
// estimates the fundamental of each analysis block, the estimate is snapped to
// a musical target, and a [PitchProcessor] applies the resulting shift.
//
// The signal is processed in blocks of [PitchCorrector.BlockSize] samples, one
// pitch ratio per block. Input is queued internally, so the block timing is
// independent of how much a caller passes per call: 256-sample callbacks and
// one whole-file call produce the same output. The price is a delay of one
// block plus the seam crossfade — the leading samples of the output are
// silence, and [PitchCorrector.Latency] reports that delay together with the
// tracker's analysis latency.
//
// Because the available shifters resynthesise each call from scratch,
// consecutive blocks have no phase relationship and their boundaries are
// audible if the ratio changes abruptly. Two things keep this
// under control: a short crossfade at every seam, and the retune glide set by
// [WithCorrectionSpeedMs], which is the more important of the two because it
// keeps neighbouring blocks at nearly identical ratios. A speed of zero
// retunes instantly and is correspondingly rougher.
//
// PitchCorrector deliberately does not implement [PitchProcessor]: its pitch
// ratio is derived from detection and cannot be set from outside, so
// satisfying that interface would let a caller call SetPitchSemitones and
// silently have it ignored.
//
// The frequency shifter in the modulation package is not usable here. It
// translates every partial by a constant number of hertz, which destroys the
// harmonic relationship between them; correction needs a ratio, not an offset.
//
// This processor is mono and not thread-safe. Unlike the detector and tracker
// it allocates: [PitchProcessor.Process] returns a freshly allocated slice by
// contract, so the correction path cannot be allocation-free.
type PitchCorrector struct {
	sampleRate float64
	tracker    *PitchTracker
	shifter    PitchProcessor

	mode          CorrectionMode
	scale         Scale
	targetHz      float64
	referenceHz   float64
	amount        float64
	maxSemitones  float64
	speedMs       float64
	blockSize     int
	crossfadeMs   float64
	minConfidence float64

	crossfadeLen int
	fadeIn       []float64
	glide        float64 // one-pole coefficient derived from speedMs

	overhang    []float64
	overhangLen int

	// inQueue holds input samples that have not yet formed a complete block
	// plus its lookahead; outQueue holds corrected samples waiting to be
	// returned. Together they decouple the block timing from the size of the
	// caller's buffers.
	inQueue  []float64
	outQueue []float64

	targetSemitones  float64
	appliedSemitones float64
	detectedHz       float64
	targetFreqHz     float64
	confidence       float64
	voiced           bool
}

// NewPitchCorrector constructs a pitch corrector.
func NewPitchCorrector(sampleRate float64, opts ...PitchCorrectorOption) (*PitchCorrector, error) {
	if !isFinitePositive(sampleRate) {
		return nil, fmt.Errorf("pitch corrector sample rate must be positive and finite: %f", sampleRate)
	}

	cfg := defaultPitchCorrectorConfig()

	for _, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("pitch corrector option must not be nil")
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	if cfg.mode == CorrectionModeFixed && cfg.targetHz <= 0 {
		return nil, fmt.Errorf("pitch corrector fixed mode needs a target frequency")
	}

	tracker, err := resolveCorrectionTracker(sampleRate, cfg)
	if err != nil {
		return nil, err
	}

	shifter, err := resolveCorrectionShifter(sampleRate, cfg)
	if err != nil {
		return nil, err
	}

	c := &PitchCorrector{
		sampleRate:    sampleRate,
		tracker:       tracker,
		shifter:       shifter,
		mode:          cfg.mode,
		scale:         cfg.scale,
		targetHz:      cfg.targetHz,
		referenceHz:   cfg.referenceHz,
		amount:        cfg.amount,
		maxSemitones:  cfg.maxSemitones,
		speedMs:       cfg.speedMs,
		blockSize:     cfg.blockSize,
		crossfadeMs:   cfg.crossfadeMs,
		minConfidence: cfg.minConfidence,
	}

	c.rebuild()
	c.Reset()

	return c, nil
}

func resolveCorrectionTracker(sampleRate float64, cfg pitchCorrectorConfig) (*PitchTracker, error) {
	if cfg.tracker != nil {
		if cfg.tracker.SampleRate() != sampleRate {
			return nil, fmt.Errorf("pitch corrector tracker sample rate %g does not match %g",
				cfg.tracker.SampleRate(), sampleRate)
		}

		return cfg.tracker, nil
	}

	tracker, err := NewPitchTracker(sampleRate)
	if err != nil {
		return nil, fmt.Errorf("pitch corrector tracker: %w", err)
	}

	return tracker, nil
}

func resolveCorrectionShifter(sampleRate float64, cfg pitchCorrectorConfig) (PitchProcessor, error) {
	if cfg.shifter != nil {
		if cfg.shifter.SampleRate() != sampleRate {
			return nil, fmt.Errorf("pitch corrector shifter sample rate %g does not match %g",
				cfg.shifter.SampleRate(), sampleRate)
		}

		return cfg.shifter, nil
	}

	shifter, err := NewSpectralPitchShifter(sampleRate)
	if err != nil {
		return nil, fmt.Errorf("pitch corrector shifter: %w", err)
	}

	return shifter, nil
}

// rebuild recomputes the crossfade window and the glide coefficient. Every
// input it depends on is validated before it is stored, so it cannot fail.
func (c *PitchCorrector) rebuild() {
	// The crossfade is bounded by half a block so that a seam never reaches
	// into the following one.
	crossfadeLen := int(math.Round(c.crossfadeMs * 0.001 * c.sampleRate))
	crossfadeLen = min(max(crossfadeLen, 0), c.blockSize/2)

	c.crossfadeLen = crossfadeLen

	if cap(c.overhang) < crossfadeLen {
		c.overhang = make([]float64, crossfadeLen)
	}

	c.overhang = c.overhang[:crossfadeLen]
	c.overhangLen = 0

	c.fadeIn = make([]float64, crossfadeLen)

	for i := range crossfadeLen {
		// A raised-cosine ramp reaching 1 at the last sample, so the fade joins
		// the untouched remainder of the block without a step. The fade is
		// amplitude-complementary rather than equal-power because the two sides
		// of a seam are the same material at nearly the same ratio, so they sum
		// coherently.
		t := float64(i+1) / float64(crossfadeLen)
		c.fadeIn[i] = 0.5 - 0.5*math.Cos(math.Pi*t)
	}

	c.glide = correctionGlide(c.speedMs, c.blockSize, c.sampleRate)
}

// correctionGlide converts a retune time constant into the per-block
// coefficient of a one-pole smoother.
func correctionGlide(speedMs float64, blockSize int, sampleRate float64) float64 {
	if speedMs <= 0 {
		return 1
	}

	blockSeconds := float64(blockSize) / sampleRate

	return 1 - math.Exp(-blockSeconds/(speedMs*0.001))
}

// SampleRate returns the sample rate in Hz.
func (c *PitchCorrector) SampleRate() float64 { return c.sampleRate }

// Mode returns how the target pitch is chosen.
func (c *PitchCorrector) Mode() CorrectionMode { return c.mode }

// Scale returns the scale detected pitches snap to in [CorrectionModeScale].
func (c *PitchCorrector) Scale() Scale { return c.scale }

// TargetHz returns the fixed target frequency used in [CorrectionModeFixed].
func (c *PitchCorrector) TargetHz() float64 { return c.targetHz }

// ReferenceHz returns the frequency of A4 that places the note grid.
func (c *PitchCorrector) ReferenceHz() float64 { return c.referenceHz }

// Amount returns the fraction of the detected error that is corrected.
func (c *PitchCorrector) Amount() float64 { return c.amount }

// MaxCorrectionSemitones returns the clamp on the applied shift.
func (c *PitchCorrector) MaxCorrectionSemitones() float64 { return c.maxSemitones }

// CorrectionSpeedMs returns the retune time constant in milliseconds.
func (c *PitchCorrector) CorrectionSpeedMs() float64 { return c.speedMs }

// BlockSize returns how many samples share one pitch ratio.
func (c *PitchCorrector) BlockSize() int { return c.blockSize }

// CrossfadeLen returns the seam crossfade length in samples.
func (c *PitchCorrector) CrossfadeLen() int { return c.crossfadeLen }

// MinConfidence returns the detector confidence required to correct.
func (c *PitchCorrector) MinConfidence() float64 { return c.minConfidence }

// Tracker returns the underlying pitch tracker.
func (c *PitchCorrector) Tracker() *PitchTracker { return c.tracker }

// Shifter returns the underlying pitch shifter.
func (c *PitchCorrector) Shifter() PitchProcessor { return c.shifter }

// DetectedFrequency returns the fundamental measured for the most recent
// block, or 0 when that block was unvoiced.
func (c *PitchCorrector) DetectedFrequency() float64 { return c.detectedHz }

// TargetFrequency returns the frequency the most recent block was steered
// toward, or 0 before any voiced block.
func (c *PitchCorrector) TargetFrequency() float64 { return c.targetFreqHz }

// AppliedSemitones returns the shift actually applied to the most recent
// block, after the amount scaling, the clamp and the retune glide.
func (c *PitchCorrector) AppliedSemitones() float64 { return c.appliedSemitones }

// Confidence returns the detector confidence of the most recent block.
func (c *PitchCorrector) Confidence() float64 { return c.confidence }

// Voiced reports whether the most recent block was corrected.
func (c *PitchCorrector) Voiced() bool { return c.voiced }

// Latency returns the streaming latency in samples: one detector frame of
// analysis, plus the block the ratio is computed for, plus the seam crossfade
// lookahead.
func (c *PitchCorrector) Latency() int {
	return c.tracker.Latency() + c.delay()
}

// SetCorrectionAmount updates the fraction of the detected error corrected.
func (c *PitchCorrector) SetCorrectionAmount(amount float64) error {
	if err := validateCorrectionAmount(amount); err != nil {
		return err
	}

	c.amount = amount

	return nil
}

// SetScale selects a scale and switches to [CorrectionModeScale].
func (c *PitchCorrector) SetScale(scale Scale) error {
	if scale.IsZero() {
		return fmt.Errorf("pitch corrector scale must not be the zero value")
	}

	c.scale = scale
	c.mode = CorrectionModeScale

	return nil
}

// SetTargetHz selects a fixed target and switches to [CorrectionModeFixed].
func (c *PitchCorrector) SetTargetHz(hz float64) error {
	if !isFinitePositive(hz) {
		return fmt.Errorf("pitch corrector target frequency must be positive and finite: %f", hz)
	}

	c.targetHz = hz
	c.mode = CorrectionModeFixed

	return nil
}

// SetReferenceHz updates the frequency of A4 that places the note grid.
func (c *PitchCorrector) SetReferenceHz(hz float64) error {
	if err := validateCorrectionReferenceHz(hz); err != nil {
		return err
	}

	c.referenceHz = hz

	return nil
}

// SetMaxCorrectionSemitones updates the clamp on the applied shift.
func (c *PitchCorrector) SetMaxCorrectionSemitones(semitones float64) error {
	if err := validateMaxCorrectionSemitones(semitones); err != nil {
		return err
	}

	c.maxSemitones = semitones

	return nil
}

// SetCorrectionSpeedMs updates the retune time constant.
func (c *PitchCorrector) SetCorrectionSpeedMs(ms float64) error {
	if err := validateCorrectionSpeedMs(ms); err != nil {
		return err
	}

	c.speedMs = ms
	c.glide = correctionGlide(c.speedMs, c.blockSize, c.sampleRate)

	return nil
}

// SetMinConfidence updates the detector confidence required to correct.
func (c *PitchCorrector) SetMinConfidence(minConfidence float64) error {
	if minConfidence < 0 || minConfidence > 1 || math.IsNaN(minConfidence) {
		return fmt.Errorf("pitch corrector confidence must be in [0, 1]: %f", minConfidence)
	}

	c.minConfidence = minConfidence

	return nil
}

// SetSampleRate updates the corrector, its tracker and its shifter, and clears
// all buffered state.
func (c *PitchCorrector) SetSampleRate(sampleRate float64) error {
	if !isFinitePositive(sampleRate) {
		return fmt.Errorf("pitch corrector sample rate must be positive and finite: %f", sampleRate)
	}

	if err := c.tracker.SetSampleRate(sampleRate); err != nil {
		return fmt.Errorf("pitch corrector: %w", err)
	}

	if err := c.shifter.SetSampleRate(sampleRate); err != nil {
		return fmt.Errorf("pitch corrector shifter: %w", err)
	}

	c.sampleRate = sampleRate

	c.rebuild()
	c.Reset()

	return nil
}

// Reset clears the tracker, the shifter, the sample queues and the seam state,
// returning the corrector to its freshly constructed condition.
func (c *PitchCorrector) Reset() {
	c.tracker.Reset()
	c.shifter.Reset()

	// Priming the output queue with one delay's worth of silence is what makes
	// the delay constant: the queue can then never underrun, so no call has to
	// invent samples and shift the stream against itself.
	c.inQueue = c.inQueue[:0]
	c.outQueue = c.outQueue[:0]

	for range c.delay() {
		c.outQueue = append(c.outQueue, 0)
	}

	c.overhangLen = 0
	c.targetSemitones = 0
	c.appliedSemitones = 0
	c.detectedHz = 0
	c.targetFreqHz = 0
	c.confidence = 0
	c.voiced = false

	_ = c.shifter.SetPitchSemitones(0)
}

// Process corrects input and returns a new block of the same length.
func (c *PitchCorrector) Process(input []float64) []float64 {
	if len(input) == 0 {
		return nil
	}

	out := make([]float64, len(input))
	c.processInto(out, input)

	return out
}

// ProcessInPlace corrects buf in place.
func (c *PitchCorrector) ProcessInPlace(buf []float64) {
	if len(buf) == 0 {
		return
	}

	copy(buf, c.Process(buf))
}

// delay is the number of samples the corrected signal lags the input: one
// block, because a block's ratio is only known once the block is complete,
// plus the seam lookahead read past its end.
func (c *PitchCorrector) delay() int { return c.blockSize + c.crossfadeLen }

func (c *PitchCorrector) processInto(out, input []float64) {
	c.inQueue = append(c.inQueue, input...)

	for len(c.inQueue) >= c.delay() {
		c.correctBlock()

		// Consume the block but keep the lookahead: it is the next block's
		// opening material and must be corrected again at the new ratio.
		c.inQueue = append(c.inQueue[:0], c.inQueue[c.blockSize:]...)
	}

	// The queue was primed with one delay of silence and every block appends
	// exactly as many samples as it consumes, so it always holds at least as
	// many samples as the caller has fed in.
	copy(out, c.outQueue)
	c.outQueue = append(c.outQueue[:0], c.outQueue[len(out):]...)
}

// correctBlock corrects the block at the head of the input queue and appends
// its blockSize output samples to the output queue.
func (c *PitchCorrector) correctBlock() {
	c.tracker.Write(c.inQueue[:c.blockSize])
	c.updateShift()

	// The block is processed together with a short lookahead, whose output
	// becomes the crossfade partner for the next block's opening samples.
	shifted := c.runShifter(c.inQueue[:c.delay()])

	for i := range c.blockSize {
		v := shifted[i]
		if i < c.overhangLen {
			// Written as a lerp rather than a weighted sum so that a seam
			// between two identical segments is bit-exact, which is what
			// makes a zero correction a true bypass.
			v = c.overhang[i] + (v-c.overhang[i])*c.fadeIn[i]
		}

		c.outQueue = append(c.outQueue, v)
	}

	c.overhangLen = copy(c.overhang, shifted[c.blockSize:])
}

// runShifter applies the current shift, bypassing the shifter entirely when
// there is nothing to correct so that a zero correction is bit-exact.
func (c *PitchCorrector) runShifter(src []float64) []float64 {
	if math.Abs(c.appliedSemitones) < correctionIdentityEps {
		return src
	}

	return c.shifter.Process(src)
}

// updateShift folds the tracker's latest estimate into the target shift and
// advances the retune glide by one block.
func (c *PitchCorrector) updateShift() {
	est := c.tracker.Estimate()

	c.confidence = est.Confidence
	c.voiced = est.Voiced && est.Confidence >= c.minConfidence && est.FrequencyHz > 0

	if c.voiced {
		c.detectedHz = est.FrequencyHz
		c.targetSemitones = c.shiftForFrequency(est.FrequencyHz)
	} else {
		// Consonants and note transitions are routinely unvoiced for a block or
		// two. Holding the last target avoids the swoop that relaxing to zero
		// and back would produce.
		c.detectedHz = 0
	}

	c.appliedSemitones += c.glide * (c.targetSemitones - c.appliedSemitones)
	c.appliedSemitones = clampAbs(c.appliedSemitones, correctionShiftLimit)

	if err := c.shifter.SetPitchSemitones(c.appliedSemitones); err != nil {
		// The applied shift is clamped inside every shifter's accepted range,
		// so a rejection means the shifter was reconfigured externally. Keep
		// whatever ratio it currently holds rather than failing the block.
		c.appliedSemitones = c.shifter.PitchSemitones()
	}
}

// shiftForFrequency returns the correction for one detected frequency, in
// semitones, after the amount scaling and the clamp.
func (c *PitchCorrector) shiftForFrequency(detectedHz float64) float64 {
	detectedMIDI := FrequencyToMIDI(detectedHz, c.referenceHz)

	targetMIDI := c.scale.SnapMIDI(detectedMIDI)
	if c.mode == CorrectionModeFixed {
		targetMIDI = FrequencyToMIDI(c.targetHz, c.referenceHz)
	}

	c.targetFreqHz = MIDIToFrequency(targetMIDI, c.referenceHz)

	// The interpolation is in the semitone domain, so an amount of 0.5 leaves
	// exactly half the error in cents.
	return clampAbs((targetMIDI-detectedMIDI)*c.amount, c.maxSemitones)
}

func clampAbs(v, limit float64) float64 {
	if math.IsNaN(v) {
		return 0
	}

	return min(max(v, -limit), limit)
}

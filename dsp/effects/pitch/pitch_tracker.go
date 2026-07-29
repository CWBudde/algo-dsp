package pitch

import "fmt"

const (
	// defaultTrackerMedianTaps is the length of the median filter applied to
	// successive voiced estimates. Three taps remove an isolated octave jump
	// while delaying a genuine pitch change by only one hop.
	defaultTrackerMedianTaps = 3
	maxTrackerMedianTaps     = 5

	// defaultTrackerHoldFrames is how many consecutive unvoiced frames keep
	// reporting the last voiced estimate. Note transitions and consonants are
	// routinely unvoiced for a frame or two.
	defaultTrackerHoldFrames = 2

	// trackerHopDivisor derives the default hop from the frame size, giving
	// four analyses per frame.
	trackerHopDivisor = 4
)

// PitchTrackerOption mutates pitch tracker construction parameters.
type PitchTrackerOption func(*pitchTrackerConfig) error

type pitchTrackerConfig struct {
	hop         int // 0 selects frameSize/trackerHopDivisor
	medianTaps  int
	holdFrames  int
	detector    *YINDetector
	detectorOpt []YINDetectorOption
}

func defaultPitchTrackerConfig() pitchTrackerConfig {
	return pitchTrackerConfig{
		hop:        0,
		medianTaps: defaultTrackerMedianTaps,
		holdFrames: defaultTrackerHoldFrames,
	}
}

// WithTrackerHop sets how many samples advance between analyses. The default
// is a quarter of the detector's frame size.
func WithTrackerHop(samples int) PitchTrackerOption {
	return func(cfg *pitchTrackerConfig) error {
		if samples <= 0 {
			return fmt.Errorf("pitch tracker hop must be positive: %d", samples)
		}

		cfg.hop = samples

		return nil
	}
}

// WithTrackerMedianFilter sets the number of successive estimates the median
// filter spans. Valid values are 1 (disabled), 3 and 5; the default is 3.
func WithTrackerMedianFilter(taps int) PitchTrackerOption {
	return func(cfg *pitchTrackerConfig) error {
		if taps != 1 && taps != 3 && taps != 5 {
			return fmt.Errorf("pitch tracker median filter must be 1, 3 or 5 taps: %d", taps)
		}

		cfg.medianTaps = taps

		return nil
	}
}

// WithTrackerHoldFrames sets how many consecutive unvoiced frames continue to
// report the last voiced estimate. Zero disables the hold.
func WithTrackerHoldFrames(frames int) PitchTrackerOption {
	return func(cfg *pitchTrackerConfig) error {
		if frames < 0 {
			return fmt.Errorf("pitch tracker hold frames must not be negative: %d", frames)
		}

		cfg.holdFrames = frames

		return nil
	}
}

// WithTrackerDetector supplies a pre-configured detector. Its sample rate must
// match the tracker's.
func WithTrackerDetector(d *YINDetector) PitchTrackerOption {
	return func(cfg *pitchTrackerConfig) error {
		if d == nil {
			return fmt.Errorf("pitch tracker detector must not be nil")
		}

		cfg.detector = d

		return nil
	}
}

// WithTrackerDetectorOptions forwards options to the detector the tracker
// builds for itself. It is ignored when [WithTrackerDetector] is also used.
func WithTrackerDetectorOptions(opts ...YINDetectorOption) PitchTrackerOption {
	return func(cfg *pitchTrackerConfig) error {
		cfg.detectorOpt = opts

		return nil
	}
}

// PitchTracker turns the frame-at-a-time [YINDetector] into a streaming pitch
// follower. Samples are buffered and analysed once per hop; successive voiced
// estimates pass through a median filter, and a short run of unvoiced frames
// keeps reporting the last voiced result.
//
// The median filter is the tracker's main defence against the octave errors
// that YIN makes sporadically on noisy or transitional material: a single
// stray frame cannot move the reported estimate.
//
// Reported latency is one detector frame. This processor is mono, real-time
// safe after construction, and not thread-safe.
type PitchTracker struct {
	sampleRate float64
	detector   *YINDetector
	hop        int
	autoHop    bool
	medianTaps int
	holdFrames int

	ring     []float64 // frameSize samples of history
	writePos int
	filled   int
	pending  int // samples written since the last analysis

	frame []float64 // linearised copy of ring, handed to the detector

	history    [maxTrackerMedianTaps]float64 // recent voiced frequencies
	historyLen int
	scratch    [maxTrackerMedianTaps]float64

	current    PitchEstimate
	unvoicedIn int // consecutive unvoiced frames since the last voiced one
}

// NewPitchTracker constructs a streaming pitch tracker.
func NewPitchTracker(sampleRate float64, opts ...PitchTrackerOption) (*PitchTracker, error) {
	if !isFinitePositive(sampleRate) {
		return nil, fmt.Errorf("pitch tracker sample rate must be positive and finite: %f", sampleRate)
	}

	cfg := defaultPitchTrackerConfig()

	for _, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("pitch tracker option must not be nil")
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	detector := cfg.detector
	if detector == nil {
		d, err := NewYINDetector(sampleRate, cfg.detectorOpt...)
		if err != nil {
			return nil, fmt.Errorf("pitch tracker detector: %w", err)
		}

		detector = d
	}

	if detector.SampleRate() != sampleRate {
		return nil, fmt.Errorf("pitch tracker detector sample rate %g does not match %g",
			detector.SampleRate(), sampleRate)
	}

	autoHop := cfg.hop == 0

	hop := cfg.hop
	if autoHop {
		hop = max(detector.FrameSize()/trackerHopDivisor, 1)
	}

	if hop > detector.FrameSize() {
		return nil, fmt.Errorf("pitch tracker hop %d exceeds the detector frame size %d",
			hop, detector.FrameSize())
	}

	t := &PitchTracker{
		sampleRate: sampleRate,
		detector:   detector,
		hop:        hop,
		autoHop:    autoHop,
		medianTaps: cfg.medianTaps,
		holdFrames: cfg.holdFrames,
		ring:       make([]float64, detector.FrameSize()),
		frame:      make([]float64, detector.FrameSize()),
	}

	return t, nil
}

// SampleRate returns the sample rate in Hz.
func (t *PitchTracker) SampleRate() float64 { return t.sampleRate }

// Detector returns the underlying detector, which may be reconfigured between
// blocks. Changing its frame size invalidates the tracker's buffers, so call
// [PitchTracker.Reset] afterwards.
func (t *PitchTracker) Detector() *YINDetector { return t.detector }

// Hop returns the analysis hop in samples.
func (t *PitchTracker) Hop() int { return t.hop }

// MedianTaps returns the length of the median filter.
func (t *PitchTracker) MedianTaps() int { return t.medianTaps }

// HoldFrames returns how many unvoiced frames keep the last voiced estimate.
func (t *PitchTracker) HoldFrames() int { return t.holdFrames }

// Latency returns the analysis latency in samples, which is one detector
// frame: an estimate describes the frame ending at the most recent sample.
func (t *PitchTracker) Latency() int { return t.detector.FrameSize() }

// Estimate returns the most recent estimate after median filtering and the
// unvoiced hold. Before the first full frame has arrived it reports the zero
// value, which is unvoiced.
func (t *PitchTracker) Estimate() PitchEstimate { return t.current }

// SetSampleRate updates the tracker and its detector, resizing the internal
// buffers if the detector's frame size changes. All buffered history is
// discarded, as if [PitchTracker.Reset] had been called.
func (t *PitchTracker) SetSampleRate(sampleRate float64) error {
	if !isFinitePositive(sampleRate) {
		return fmt.Errorf("pitch tracker sample rate must be positive and finite: %f", sampleRate)
	}

	if err := t.detector.SetSampleRate(sampleRate); err != nil {
		return fmt.Errorf("pitch tracker: %w", err)
	}

	t.sampleRate = sampleRate

	t.Reset()

	return nil
}

// syncToDetector matches the ring buffer, the analysis frame and an automatic
// hop to the detector's current frame size, which the caller may have changed
// through [PitchTracker.Detector]. It allocates only when the size differs.
func (t *PitchTracker) syncToDetector() {
	frameSize := t.detector.FrameSize()
	if len(t.ring) != frameSize {
		t.ring = make([]float64, frameSize)
		t.frame = make([]float64, frameSize)
	}

	if t.autoHop {
		t.hop = max(frameSize/trackerHopDivisor, 1)
	}

	t.hop = min(t.hop, frameSize)
}

// Reset clears all buffered history and the current estimate, and adopts the
// detector's current frame size.
func (t *PitchTracker) Reset() {
	t.syncToDetector()

	for i := range t.ring {
		t.ring[i] = 0
	}

	t.writePos = 0
	t.filled = 0
	t.pending = 0
	t.historyLen = 0
	t.current = PitchEstimate{}
	t.unvoicedIn = 0

	t.detector.Reset()
}

// Write appends samples to the tracker, running an analysis every time a full
// hop has accumulated. It does not allocate.
func (t *PitchTracker) Write(samples []float64) {
	for _, v := range samples {
		t.ring[t.writePos] = v
		t.writePos++

		if t.writePos == len(t.ring) {
			t.writePos = 0
		}

		if t.filled < len(t.ring) {
			t.filled++
		}

		t.pending++
		if t.pending < t.hop || t.filled < len(t.ring) {
			continue
		}

		t.pending = 0

		t.analyse()
	}
}

// analyse linearises the ring buffer oldest-sample-first and runs one
// detection, then folds the result into the smoothed estimate.
func (t *PitchTracker) analyse() {
	n := copy(t.frame, t.ring[t.writePos:])
	copy(t.frame[n:], t.ring[:t.writePos])

	est, err := t.detector.Detect(t.frame)
	if err != nil {
		// The frame is exactly the detector's frame size, so this cannot
		// happen unless the detector was reconfigured without a Reset.
		return
	}

	t.smooth(est)
}

// smooth applies the median filter to voiced estimates and the hold to
// unvoiced ones.
func (t *PitchTracker) smooth(est PitchEstimate) {
	if !est.Voiced {
		t.unvoicedIn++

		if t.unvoicedIn <= t.holdFrames && t.current.Voiced {
			// Keep reporting the held estimate, but refresh the measured
			// fields so callers still see the true level and periodicity.
			t.current.RMS = est.RMS
			t.current.Aperiodicity = est.Aperiodicity
			t.current.Confidence = est.Confidence

			return
		}

		t.historyLen = 0
		t.current = est

		return
	}

	t.unvoicedIn = 0

	t.pushHistory(est.FrequencyHz)

	est.FrequencyHz = t.medianFrequency()
	if est.FrequencyHz > 0 {
		est.Tau = t.sampleRate / est.FrequencyHz
	}

	t.current = est
}

// pushHistory appends a voiced frequency to the fixed-size history ring. The
// first estimate after a reset or an unvoiced gap seeds the whole window, so
// the median always runs over a full, odd-length set of taps: a partially
// filled window would average its two initial samples and let a single stray
// frame move the reported pitch precisely while the history warms up.
func (t *PitchTracker) pushHistory(hz float64) {
	if t.historyLen == 0 {
		for i := range t.medianTaps {
			t.history[i] = hz
		}

		t.historyLen = t.medianTaps

		return
	}

	copy(t.history[:t.medianTaps-1], t.history[1:t.medianTaps])
	t.history[t.medianTaps-1] = hz
}

// medianFrequency returns the median of the buffered voiced frequencies using
// an insertion sort over a fixed array, so it never allocates.
func (t *PitchTracker) medianFrequency() float64 {
	n := t.historyLen
	if n == 0 {
		return 0
	}

	copy(t.scratch[:n], t.history[:n])

	for i := 1; i < n; i++ {
		v := t.scratch[i]

		j := i - 1
		for j >= 0 && t.scratch[j] > v {
			t.scratch[j+1] = t.scratch[j]
			j--
		}

		t.scratch[j+1] = v
	}

	if n%2 == 1 {
		return t.scratch[n/2]
	}

	// Unreachable with the accepted tap counts (1, 3 and 5) because
	// pushHistory seeds the whole window, so n is always 0 or medianTaps.
	return 0.5 * (t.scratch[n/2-1] + t.scratch[n/2])
}

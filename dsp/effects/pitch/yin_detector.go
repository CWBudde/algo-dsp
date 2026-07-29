package pitch

import (
	"fmt"
	"math"
)

const (
	// defaultYINThreshold is the absolute threshold applied to the cumulative
	// mean normalized difference. The original paper suggests 0.1; practical
	// implementations use 0.10 to 0.20. Raising it much beyond 0.3 invites
	// octave-up errors, because a shallow early dip caused by a strong second
	// harmonic passes the threshold before the true dip at the period. Lowering
	// it below about 0.05 reports slightly aperiodic material as unvoiced.
	defaultYINThreshold = 0.15

	defaultYINMinFrequency = 60.0
	defaultYINMaxFrequency = 1600.0

	// defaultYINSilenceThresholdDB gates the analysis on frame RMS so that
	// silence and near-silence are reported as unvoiced rather than producing
	// an arbitrary estimate from numerical noise.
	defaultYINSilenceThresholdDB = -60.0

	// minYINTau is the shortest lag the search may consider. It must be at
	// least 2: the cumulative mean normalized difference is identically 1 at
	// lag 1 by construction, and parabolic interpolation needs the lag below
	// the minimum to exist.
	minYINTau = 2

	// yinFrameSizePeriods is the number of longest-lag periods a frame must
	// span. With frameSize = 2*tauMax the integration window is still tauMax
	// samples long at the longest lag, so the difference function averages over
	// at least one full period everywhere in the search range.
	yinFrameSizePeriods = 2

	yinTiny = 1e-12
)

// PitchEstimate is the result of analysing a single frame with a
// [YINDetector].
//
// When Voiced is false the detector found no lag whose normalized difference
// fell below the threshold. FrequencyHz and Tau are then zero, so a caller can
// never mistake a failed estimate for a real one, but Aperiodicity and
// Confidence still describe the best candidate that was rejected.
type PitchEstimate struct {
	// FrequencyHz is the estimated fundamental in Hz, or 0 when unvoiced.
	FrequencyHz float64
	// Tau is the estimated period as a fractional lag in samples, or 0 when
	// unvoiced.
	Tau float64
	// Aperiodicity is the interpolated normalized difference at the chosen
	// minimum, clamped to [0, 1]. Lower means more strongly periodic.
	Aperiodicity float64
	// Confidence is 1 - Aperiodicity, clamped to [0, 1].
	Confidence float64
	// RMS is the root mean square of the analysed frame, useful for gating.
	RMS float64
	// Voiced reports whether a periodic estimate was found.
	Voiced bool
}

// YINDetectorOption mutates YIN detector construction parameters.
type YINDetectorOption func(*yinDetectorConfig) error

type yinDetectorConfig struct {
	threshold          float64
	minHz              float64
	maxHz              float64
	frameSize          int // 0 selects the automatic size, 2*tauMax
	silenceThresholdDB float64
	parabolic          bool
}

func defaultYINDetectorConfig() yinDetectorConfig {
	return yinDetectorConfig{
		threshold:          defaultYINThreshold,
		minHz:              defaultYINMinFrequency,
		maxHz:              defaultYINMaxFrequency,
		frameSize:          0,
		silenceThresholdDB: defaultYINSilenceThresholdDB,
		parabolic:          true,
	}
}

// WithYINThreshold sets the absolute threshold applied to the cumulative mean
// normalized difference. Valid range is (0, 1); the default is 0.15.
func WithYINThreshold(threshold float64) YINDetectorOption {
	return func(cfg *yinDetectorConfig) error {
		if err := validateYINThreshold(threshold); err != nil {
			return err
		}

		cfg.threshold = threshold

		return nil
	}
}

// WithYINFrequencyRange sets the fundamental frequency search range in Hz. The
// default is 60 Hz to 1600 Hz. A narrower range lowers both the CPU cost and
// the latency, since the frame length is derived from the lowest frequency.
func WithYINFrequencyRange(minHz, maxHz float64) YINDetectorOption {
	return func(cfg *yinDetectorConfig) error {
		if err := validateYINFrequencyRange(minHz, maxHz); err != nil {
			return err
		}

		cfg.minHz = minHz
		cfg.maxHz = maxHz

		return nil
	}
}

// WithYINFrameSize sets the analysis frame length in samples. It must be at
// least twice the longest lag in the search range. The default, selected when
// this option is not used, is exactly that minimum.
func WithYINFrameSize(samples int) YINDetectorOption {
	return func(cfg *yinDetectorConfig) error {
		if samples <= 0 {
			return fmt.Errorf("yin detector frame size must be positive: %d", samples)
		}

		cfg.frameSize = samples

		return nil
	}
}

// WithYINSilenceThresholdDB sets the frame RMS below which the detector
// reports unvoiced without analysing. The default is -60 dBFS.
func WithYINSilenceThresholdDB(db float64) YINDetectorOption {
	return func(cfg *yinDetectorConfig) error {
		if math.IsNaN(db) || math.IsInf(db, 0) {
			return fmt.Errorf("yin detector silence threshold must be finite: %f", db)
		}

		cfg.silenceThresholdDB = db

		return nil
	}
}

// WithYINParabolicInterpolation enables or disables sub-sample refinement of
// the period estimate. It defaults to enabled; disabling it quantises the
// estimate to whole samples and is intended for testing the refinement itself.
func WithYINParabolicInterpolation(enabled bool) YINDetectorOption {
	return func(cfg *yinDetectorConfig) error {
		cfg.parabolic = enabled

		return nil
	}
}

func validateYINThreshold(threshold float64) error {
	if threshold <= 0 || threshold >= 1 || math.IsNaN(threshold) {
		return fmt.Errorf("yin detector threshold must be in (0, 1): %f", threshold)
	}

	return nil
}

func validateYINFrequencyRange(minHz, maxHz float64) error {
	if !isFinitePositive(minHz) {
		return fmt.Errorf("yin detector minimum frequency must be positive and finite: %f", minHz)
	}

	if !isFinitePositive(maxHz) {
		return fmt.Errorf("yin detector maximum frequency must be positive and finite: %f", maxHz)
	}

	if minHz >= maxHz {
		return fmt.Errorf("yin detector minimum frequency must be below the maximum: %f >= %f",
			minHz, maxHz)
	}

	return nil
}

// YINDetector estimates the fundamental frequency of a single frame using the
// YIN algorithm of de Cheveigné and Kawahara (JASA 111(4), 2002): a squared
// difference function, cumulative mean normalization, an absolute threshold
// with a local-minimum refinement, and parabolic interpolation.
//
// Unlike a spectral peak picker it estimates the period directly, so it
// recovers the fundamental of a signal whose fundamental partial is absent.
//
// The detector holds no state between frames; [PitchTracker] wraps it with a
// ring buffer, hop scheduling and smoothing for streaming use. Analysis
// latency is one frame, which at the default 60 Hz lower bound is 2*tauMax
// samples, roughly 33 ms at 48 kHz. Raise the lower frequency bound to shorten
// it.
//
// This processor is mono, real-time safe, and not thread-safe.
type YINDetector struct {
	sampleRate         float64
	threshold          float64
	minHz              float64
	maxHz              float64
	frameSize          int
	autoFrameSize      bool
	silenceThresholdDB float64
	parabolic          bool

	tauMin      int
	tauMax      int
	integration int     // difference-function window length, frameSize - tauMax
	silenceRMS  float64 // silenceThresholdDB converted to a linear amplitude

	diff []float64
	cmnd []float64
}

// NewYINDetector constructs a YIN fundamental frequency detector.
func NewYINDetector(sampleRate float64, opts ...YINDetectorOption) (*YINDetector, error) {
	if !isFinitePositive(sampleRate) {
		return nil, fmt.Errorf("yin detector sample rate must be positive and finite: %f", sampleRate)
	}

	cfg := defaultYINDetectorConfig()

	for _, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("yin detector option must not be nil")
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	d := &YINDetector{
		sampleRate:         sampleRate,
		threshold:          cfg.threshold,
		minHz:              cfg.minHz,
		maxHz:              cfg.maxHz,
		frameSize:          cfg.frameSize,
		autoFrameSize:      cfg.frameSize == 0,
		silenceThresholdDB: cfg.silenceThresholdDB,
		parabolic:          cfg.parabolic,
	}

	if err := d.rebuild(); err != nil {
		return nil, err
	}

	return d, nil
}

// SampleRate returns the sample rate in Hz.
func (d *YINDetector) SampleRate() float64 { return d.sampleRate }

// Threshold returns the absolute threshold on the normalized difference.
func (d *YINDetector) Threshold() float64 { return d.threshold }

// MinFrequency returns the lower bound of the search range in Hz.
func (d *YINDetector) MinFrequency() float64 { return d.minHz }

// MaxFrequency returns the upper bound of the search range in Hz.
func (d *YINDetector) MaxFrequency() float64 { return d.maxHz }

// FrameSize returns the analysis frame length in samples, which is also the
// detector's latency.
func (d *YINDetector) FrameSize() int { return d.frameSize }

// MinTau returns the shortest lag searched, in samples.
func (d *YINDetector) MinTau() int { return d.tauMin }

// MaxTau returns the longest lag searched, in samples.
func (d *YINDetector) MaxTau() int { return d.tauMax }

// SilenceThresholdDB returns the frame RMS gate in dBFS.
func (d *YINDetector) SilenceThresholdDB() float64 { return d.silenceThresholdDB }

// ParabolicInterpolation reports whether sub-sample refinement is enabled.
func (d *YINDetector) ParabolicInterpolation() bool { return d.parabolic }

// SetSampleRate updates the sample rate and recomputes the lag bounds.
func (d *YINDetector) SetSampleRate(sampleRate float64) error {
	if !isFinitePositive(sampleRate) {
		return fmt.Errorf("yin detector sample rate must be positive and finite: %f", sampleRate)
	}

	old := d.sampleRate

	d.sampleRate = sampleRate
	if err := d.rebuild(); err != nil {
		d.sampleRate = old
		_ = d.rebuild()

		return err
	}

	return nil
}

// SetThreshold updates the absolute threshold on the normalized difference.
func (d *YINDetector) SetThreshold(threshold float64) error {
	if err := validateYINThreshold(threshold); err != nil {
		return err
	}

	d.threshold = threshold

	return nil
}

// SetFrequencyRange updates the search range and recomputes the lag bounds.
func (d *YINDetector) SetFrequencyRange(minHz, maxHz float64) error {
	if err := validateYINFrequencyRange(minHz, maxHz); err != nil {
		return err
	}

	oldMin, oldMax := d.minHz, d.maxHz

	d.minHz, d.maxHz = minHz, maxHz
	if err := d.rebuild(); err != nil {
		d.minHz, d.maxHz = oldMin, oldMax
		_ = d.rebuild()

		return err
	}

	return nil
}

// SetFrameSize updates the analysis frame length. Pass 0 to return to the
// automatic size derived from the search range.
func (d *YINDetector) SetFrameSize(samples int) error {
	if samples < 0 {
		return fmt.Errorf("yin detector frame size must not be negative: %d", samples)
	}

	oldSize, oldAuto := d.frameSize, d.autoFrameSize

	d.frameSize, d.autoFrameSize = samples, samples == 0
	if err := d.rebuild(); err != nil {
		d.frameSize, d.autoFrameSize = oldSize, oldAuto
		_ = d.rebuild()

		return err
	}

	return nil
}

// SetSilenceThresholdDB updates the frame RMS gate in dBFS.
func (d *YINDetector) SetSilenceThresholdDB(db float64) error {
	if math.IsNaN(db) || math.IsInf(db, 0) {
		return fmt.Errorf("yin detector silence threshold must be finite: %f", db)
	}

	d.silenceThresholdDB = db
	d.silenceRMS = math.Pow(10, db/20)

	return nil
}

// Reset clears the scratch buffers. The detector carries no state between
// frames, so this only affects what a debugger would see.
func (d *YINDetector) Reset() {
	for i := range d.diff {
		d.diff[i] = 0
	}

	for i := range d.cmnd {
		d.cmnd[i] = 0
	}
}

func (d *YINDetector) rebuild() error {
	nyquist := d.sampleRate / 2
	if d.maxHz >= nyquist {
		return fmt.Errorf("yin detector maximum frequency must be below Nyquist (%g): %f",
			nyquist, d.maxHz)
	}

	tauMax := int(math.Floor(d.sampleRate / d.minHz))

	tauMin := max(int(math.Ceil(d.sampleRate/d.maxHz)), minYINTau)

	// The threshold search needs at least one lag strictly between the bounds
	// for the local-minimum descent and the parabolic fit to have room.
	if tauMin+1 >= tauMax {
		return fmt.Errorf("yin detector frequency range is too narrow for the sample rate: "+
			"lags [%d, %d]", tauMin, tauMax)
	}

	frameSize := d.frameSize
	if d.autoFrameSize {
		frameSize = yinFrameSizePeriods * tauMax
	}

	if frameSize < yinFrameSizePeriods*tauMax {
		return fmt.Errorf("yin detector frame size must be at least %d samples for a %g Hz "+
			"lower bound: %d", yinFrameSizePeriods*tauMax, d.minHz, frameSize)
	}

	d.tauMin = tauMin
	d.tauMax = tauMax
	d.frameSize = frameSize
	d.integration = frameSize - tauMax
	d.silenceRMS = math.Pow(10, d.silenceThresholdDB/20)

	if len(d.diff) != tauMax+1 {
		d.diff = make([]float64, tauMax+1)
		d.cmnd = make([]float64, tauMax+1)
	}

	return nil
}

// Detect analyses one frame and returns the estimate. Only the first
// [YINDetector.FrameSize] samples of frame are used; a shorter slice is an
// error. Detect does not allocate.
func (d *YINDetector) Detect(frame []float64) (PitchEstimate, error) {
	if len(frame) < d.frameSize {
		return PitchEstimate{}, fmt.Errorf("yin detector needs at least %d samples per frame: %d",
			d.frameSize, len(frame))
	}

	x := frame[:d.frameSize]

	est := PitchEstimate{RMS: frameRMS(x)}
	if est.RMS < d.silenceRMS {
		// Silence: no meaningful period, and the normalization below would be
		// dividing numerical noise by numerical noise.
		est.Aperiodicity = 1

		return est, nil
	}

	d.differenceFunction(x)
	d.cumulativeMeanNormalize()

	tau, crossed := d.searchThreshold()

	refinedTau, minValue := d.refine(tau)

	est.Aperiodicity = clamp01(minValue)
	est.Confidence = clamp01(1 - est.Aperiodicity)

	if !crossed {
		// No lag was periodic enough. Report the best candidate's aperiodicity
		// so callers can see how close it came, but no frequency.
		return est, nil
	}

	est.Tau = refinedTau
	est.FrequencyHz = d.sampleRate / refinedTau
	est.Voiced = true

	return est, nil
}

// differenceFunction fills diff[tau] with the squared difference of the frame
// against itself delayed by tau, summed over the integration window.
func (d *YINDetector) differenceFunction(x []float64) {
	n := d.integration

	d.diff[0] = 0

	for tau := 1; tau <= d.tauMax; tau++ {
		sum := 0.0

		for j := range n {
			delta := x[j] - x[j+tau]
			sum += delta * delta
		}

		d.diff[tau] = sum
	}
}

// cumulativeMeanNormalize converts the difference function into the cumulative
// mean normalized difference, which removes the trivial minimum at lag 0 and
// makes the absolute threshold meaningful across signal levels.
func (d *YINDetector) cumulativeMeanNormalize() {
	// d'(0) is 0/0 and is defined to be 1. A consequence is that d'(1) is also
	// identically 1, which is why the search never starts below lag 2.
	d.cmnd[0] = 1

	running := 0.0

	for tau := 1; tau <= d.tauMax; tau++ {
		running += d.diff[tau]
		if running < yinTiny {
			d.cmnd[tau] = 1
			continue
		}

		d.cmnd[tau] = d.diff[tau] * float64(tau) / running
	}
}

// searchThreshold returns the first lag whose normalized difference falls below
// the threshold, followed down to its local minimum. The second return value
// reports whether the threshold was crossed at all; if it was not, the lag of
// the global minimum over the search range is returned instead.
func (d *YINDetector) searchThreshold() (int, bool) {
	for tau := d.tauMin; tau <= d.tauMax; tau++ {
		if d.cmnd[tau] >= d.threshold {
			continue
		}

		// Descend to the bottom of this dip. Without this step the estimate
		// lands on the dip's leading edge and reads consistently sharp.
		for tau+1 <= d.tauMax && d.cmnd[tau+1] < d.cmnd[tau] {
			tau++
		}

		return tau, true
	}

	best := d.tauMin
	for tau := d.tauMin + 1; tau <= d.tauMax; tau++ {
		if d.cmnd[tau] < d.cmnd[best] {
			best = tau
		}
	}

	return best, false
}

// refine fits a parabola through the chosen minimum and its two neighbours,
// returning the sub-sample lag and the interpolated minimum value.
func (d *YINDetector) refine(tau int) (float64, float64) {
	refined := float64(tau)
	minValue := d.cmnd[tau]

	if !d.parabolic {
		return refined, minValue
	}

	below, above := tau-1, tau+1
	if below < 1 || above > d.tauMax {
		// At an edge there is no bracketing triple; keep the sampled minimum.
		return refined, minValue
	}

	s0, s1, s2 := d.cmnd[below], d.cmnd[tau], d.cmnd[above]

	// With s1 the minimum, denom is negative, so a lower right neighbour gives
	// a positive shift and moves the estimate toward it.
	denom := 2*s1 - s2 - s0
	if math.Abs(denom) <= yinTiny {
		return refined, minValue
	}

	shift := (s2 - s0) / (2 * denom)
	if math.Abs(shift) >= 1 {
		// A nearly flat triple can throw the fit outside the bracket.
		return refined, minValue
	}

	return refined + shift, s1 - 0.25*(s0-s2)*shift
}

func frameRMS(x []float64) float64 {
	sum := 0.0
	for _, v := range x {
		sum += v * v
	}

	return math.Sqrt(sum / float64(len(x)))
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 1
	}

	if v < 0 {
		return 0
	}

	if v > 1 {
		return 1
	}

	return v
}

package effects

import (
	"fmt"
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// BandLayout selects the frequency band distribution for the vocoder.
type BandLayout int

const (
	// BandLayoutThirdOctave uses 32 ISO 1/3-octave center frequencies (16 Hz–20 kHz).
	// Both analysis and synthesis use 2nd-order CPG bandpass filters at Q ≈ 4.32.
	BandLayoutThirdOctave BandLayout = iota

	// BandLayoutBark uses 24 Bark-scale center frequencies (100 Hz–15.5 kHz).
	// Analysis uses per-band Q derived from spacing between adjacent Bark
	// frequencies. Synthesis defaults to the same per-band Bark Q values unless
	// overridden globally via WithVocoderSynthesisQ.
	BandLayoutBark
)

var thirdOctaveFrequencies = [32]float64{
	16, 20, 25, 31, 40, 50, 63, 80, 100, 125, 160, 200, 250, 315, 400, 500,
	630, 800, 1000, 1250, 1600, 2000, 2500, 3150, 4000, 5000, 6300, 8000,
	10000, 12500, 16000, 20000,
}

var barkFrequencies = [24]float64{
	100, 200, 300, 400, 510, 630, 770, 920, 1080, 1270, 1480, 1720, 2000,
	2320, 2700, 3150, 3700, 4400, 5300, 6400, 7700, 9500, 12000, 15500,
}

// thirdOctaveQ is Q for a 1/3-octave bandpass: 1 / (2^(1/6) - 2^(-1/6)).
const thirdOctaveQ = 4.3184727050832485

const (
	defaultVocoderAttackMs     = 0.5
	defaultVocoderReleaseMs    = 2.0
	defaultVocoderSynthQ       = thirdOctaveQ
	defaultVocoderInputLevel   = 0.0
	defaultVocoderSynthLevel   = 0.0
	defaultVocoderVocoderLevel = 1.0

	minVocoderAttackMs  = 0.01
	maxVocoderAttackMs  = 100.0
	minVocoderReleaseMs = 0.01
	maxVocoderReleaseMs = 1000.0
	minVocoderLevel     = 0.0
	maxVocoderLevel     = 10.0
	minVocoderSynthQ    = 0.1
	maxVocoderSynthQ    = 20.0
)

// VocoderOption configures a Vocoder at construction time.
type VocoderOption func(*vocoderConfig) error

type vocoderConfig struct {
	layout       BandLayout
	synthQ       float64
	synthQSet    bool
	attackMs     float64
	releaseMs    float64
	inputLevel   float64
	synthLevel   float64
	vocoderLevel float64
	downsample   bool
}

func defaultVocoderConfig() vocoderConfig {
	return vocoderConfig{
		layout:       BandLayoutThirdOctave,
		synthQ:       defaultVocoderSynthQ,
		attackMs:     defaultVocoderAttackMs,
		releaseMs:    defaultVocoderReleaseMs,
		inputLevel:   defaultVocoderInputLevel,
		synthLevel:   defaultVocoderSynthLevel,
		vocoderLevel: defaultVocoderVocoderLevel,
	}
}

// WithDownsampling enables per-band multirate analysis. Low-frequency bands
// run analysis and envelope updates at reduced rates (power-of-2 per band)
// using decimated control streams; synthesis remains at full sample rate.
// Off by default.
func WithDownsampling(enabled bool) VocoderOption {
	return func(cfg *vocoderConfig) error {
		cfg.downsample = enabled
		return nil
	}
}

// WithBandLayout sets the frequency band distribution.
func WithBandLayout(layout BandLayout) VocoderOption {
	return func(cfg *vocoderConfig) error {
		if layout != BandLayoutThirdOctave && layout != BandLayoutBark {
			return fmt.Errorf("vocoder: invalid band layout: %d", layout)
		}

		cfg.layout = layout

		return nil
	}
}

// WithVocoderSynthesisQ sets the Q factor for synthesis bandpass filters.
func WithVocoderSynthesisQ(q float64) VocoderOption {
	return func(cfg *vocoderConfig) error {
		if q < minVocoderSynthQ || q > maxVocoderSynthQ || math.IsNaN(q) || math.IsInf(q, 0) {
			return fmt.Errorf("vocoder: synthesis Q must be in [%g, %g]: %g",
				minVocoderSynthQ, maxVocoderSynthQ, q)
		}

		cfg.synthQ = q
		cfg.synthQSet = true

		return nil
	}
}

// WithVocoderAttack sets the envelope follower attack time in milliseconds.
func WithVocoderAttack(ms float64) VocoderOption {
	return func(cfg *vocoderConfig) error {
		if ms < minVocoderAttackMs || ms > maxVocoderAttackMs || math.IsNaN(ms) || math.IsInf(ms, 0) {
			return fmt.Errorf("vocoder: attack must be in [%g, %g] ms: %g",
				minVocoderAttackMs, maxVocoderAttackMs, ms)
		}

		cfg.attackMs = ms

		return nil
	}
}

// WithVocoderRelease sets the envelope follower release time in milliseconds.
func WithVocoderRelease(ms float64) VocoderOption {
	return func(cfg *vocoderConfig) error {
		if ms < minVocoderReleaseMs || ms > maxVocoderReleaseMs || math.IsNaN(ms) || math.IsInf(ms, 0) {
			return fmt.Errorf("vocoder: release must be in [%g, %g] ms: %g",
				minVocoderReleaseMs, maxVocoderReleaseMs, ms)
		}

		cfg.releaseMs = ms

		return nil
	}
}

// WithVocoderInputLevel sets the dry modulator level (linear gain).
func WithVocoderInputLevel(level float64) VocoderOption {
	return func(cfg *vocoderConfig) error {
		if level < minVocoderLevel || level > maxVocoderLevel || math.IsNaN(level) || math.IsInf(level, 0) {
			return fmt.Errorf("vocoder: input level must be in [%g, %g]: %g",
				minVocoderLevel, maxVocoderLevel, level)
		}

		cfg.inputLevel = level

		return nil
	}
}

// WithVocoderSynthLevel sets the dry carrier level (linear gain).
func WithVocoderSynthLevel(level float64) VocoderOption {
	return func(cfg *vocoderConfig) error {
		if level < minVocoderLevel || level > maxVocoderLevel || math.IsNaN(level) || math.IsInf(level, 0) {
			return fmt.Errorf("vocoder: synth level must be in [%g, %g]: %g",
				minVocoderLevel, maxVocoderLevel, level)
		}

		cfg.synthLevel = level

		return nil
	}
}

// WithVocoderLevel sets the vocoded output level (linear gain).
func WithVocoderLevel(level float64) VocoderOption {
	return func(cfg *vocoderConfig) error {
		if level < minVocoderLevel || level > maxVocoderLevel || math.IsNaN(level) || math.IsInf(level, 0) {
			return fmt.Errorf("vocoder: vocoder level must be in [%g, %g]: %g",
				minVocoderLevel, maxVocoderLevel, level)
		}

		cfg.vocoderLevel = level

		return nil
	}
}

// Vocoder implements a channel vocoder effect that applies the spectral
// envelope of a modulator signal to a carrier signal. The modulator is
// decomposed into frequency bands, per-band amplitude envelopes are
// extracted, and those envelopes modulate matching bands of the carrier.
type Vocoder struct {
	sampleRate float64
	layout     BandLayout
	numBands   int

	// Analysis filter bank: bandpass sections for both layouts.
	analysisFilters []biquad.Section

	// Synthesis filter bank: bandpass sections for both layouts.
	synthesisFilters []biquad.Section

	// Per-band envelope followers.
	envelopes    []float64
	attackCoeff  float64
	releaseCoeff float64

	// Mix levels (linear gain).
	inputLevel   float64
	synthLevel   float64
	vocoderLevel float64

	// Downsampling state for multirate analysis.
	downsample                bool
	downsampleFactors         []int // per-band factor (power of 2)
	downsampleCount           int   // global sample counter
	downsampleMax             int   // counter wraps at this value (LCM of all factors)
	downsampleAnalysisFilters []biquad.Section
	downsampleGroupFactors    []int
	downsampleGroupMasks      []int
	downsampleGroupAAFilters  []biquad.Section
	downsampleGroupBands      [][]int
	downsampleAttackCoeffs    []float64
	downsampleReleaseCoeffs   []float64

	// Stored config for rebuilding.
	synthQ           float64
	synthQOverridden bool
	attackMs         float64
	releaseMs        float64
}

// NewVocoder creates a new Vocoder effect.
func NewVocoder(sampleRate float64, opts ...VocoderOption) (*Vocoder, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("vocoder: sample rate must be > 0: %f", sampleRate)
	}

	cfg := defaultVocoderConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	v := &Vocoder{
		sampleRate:       sampleRate,
		layout:           cfg.layout,
		synthQ:           cfg.synthQ,
		synthQOverridden: cfg.synthQSet,
		attackMs:         cfg.attackMs,
		releaseMs:        cfg.releaseMs,
		inputLevel:       cfg.inputLevel,
		synthLevel:       cfg.synthLevel,
		vocoderLevel:     cfg.vocoderLevel,
		downsample:       cfg.downsample,
	}

	v.computeEnvelopeCoeffs()

	err := v.buildFilterBanks()
	if err != nil {
		return nil, err
	}

	return v, nil
}

func (v *Vocoder) computeEnvelopeCoeffs() {
	sr := v.sampleRate
	if v.attackMs > 0 {
		v.attackCoeff = 1.0 - math.Exp(-1.0/(v.attackMs*0.001*sr))
	} else {
		v.attackCoeff = 1.0
	}

	if v.releaseMs > 0 {
		v.releaseCoeff = math.Exp(-1.0 / (v.releaseMs * 0.001 * sr))
	} else {
		v.releaseCoeff = 0.0
	}
}

func (v *Vocoder) buildFilterBanks() error {
	switch v.layout {
	case BandLayoutThirdOctave:
		return v.buildThirdOctaveBanks()
	case BandLayoutBark:
		return v.buildBarkBanks()
	default:
		return fmt.Errorf("vocoder: unsupported band layout: %d", v.layout)
	}
}

func (v *Vocoder) buildThirdOctaveBanks() error {
	nyquist := v.sampleRate / 2

	// Count usable bands (center frequency below 90% of Nyquist).
	n := 0

	for _, f := range thirdOctaveFrequencies {
		if f < nyquist*0.9 {
			n++
		}
	}

	if n == 0 {
		return fmt.Errorf("vocoder: no usable bands at sample rate %g Hz", v.sampleRate)
	}

	v.numBands = n
	v.analysisFilters = make([]biquad.Section, n)
	v.synthesisFilters = make([]biquad.Section, n)
	v.envelopes = make([]float64, n)
	analysisQ := make([]float64, n)

	for i := range n {
		freq := thirdOctaveFrequencies[i]
		analysisQ[i] = thirdOctaveQ

		// Analysis bandpass at 1/3-octave Q.
		ac := cpgBandpass(freq, thirdOctaveQ, v.sampleRate)
		v.analysisFilters[i] = *biquad.NewSection(ac)

		// Synthesis bandpass at configurable Q.
		sc := cpgBandpass(freq, v.synthQ, v.sampleRate)
		v.synthesisFilters[i] = *biquad.NewSection(sc)
	}

	if v.downsample {
		v.computeDownsampleFactors(thirdOctaveFrequencies[:n], analysisQ)
	}

	return nil
}

func (v *Vocoder) buildBarkBanks() error {
	nyquist := v.sampleRate / 2

	n := 0

	for _, f := range barkFrequencies {
		if f < nyquist*0.9 {
			n++
		}
	}

	if n == 0 {
		return fmt.Errorf("vocoder: no usable Bark bands at sample rate %g Hz", v.sampleRate)
	}

	v.numBands = n
	v.analysisFilters = make([]biquad.Section, n)
	v.synthesisFilters = make([]biquad.Section, n)
	v.envelopes = make([]float64, n)
	analysisQ := make([]float64, n)

	for i := range n {
		freq := barkFrequencies[i]
		q := barkBandQ(i)
		analysisQ[i] = q

		// Analysis bandpass at Bark center frequency.
		ac := cpgBandpass(freq, q, v.sampleRate)
		v.analysisFilters[i] = *biquad.NewSection(ac)

		// Synthesis bandpass defaults to Bark-derived per-band Q.
		synthQ := q
		if v.synthQOverridden {
			synthQ = v.synthQ
		}

		sc := cpgBandpass(freq, synthQ, v.sampleRate)
		v.synthesisFilters[i] = *biquad.NewSection(sc)
	}

	if v.downsample {
		v.computeDownsampleFactors(barkFrequencies[:n], analysisQ)
	}

	return nil
}

// computeDownsampleFactors assigns per-band power-of-2 multirate factors.
// For each band, the factor is the largest power of 2 such that
// 2*factor*centerFreq < 0.1*sampleRate. High-frequency bands get factor 1
// (analysis+envelope updated each sample); low-frequency bands get higher factors.
func (v *Vocoder) computeDownsampleFactors(freqs, analysisQ []float64) {
	v.downsampleFactors = make([]int, v.numBands)
	v.downsampleAnalysisFilters = make([]biquad.Section, v.numBands)
	v.downsampleAttackCoeffs = make([]float64, v.numBands)
	v.downsampleReleaseCoeffs = make([]float64, v.numBands)
	maxFactor := 1

	threshold := 0.1 * v.sampleRate
	for i := range v.numBands {
		factor := 1
		for float64(2*(factor<<1))*freqs[i] < threshold {
			factor <<= 1
		}

		v.downsampleFactors[i] = factor
		if factor > maxFactor {
			maxFactor = factor
		}
	}

	v.buildDownsampleGroups()

	for i := range v.numBands {
		dsRate := v.sampleRate / float64(v.downsampleFactors[i])

		ac := cpgBandpass(freqs[i], analysisQ[i], dsRate)
		if ac == (biquad.Coefficients{}) {
			// Fallback to full-rate analysis if the decimated rate is invalid.
			ac = cpgBandpass(freqs[i], analysisQ[i], v.sampleRate)
		}

		v.downsampleAnalysisFilters[i] = *biquad.NewSection(ac)
	}

	v.downsampleMax = maxFactor // counter wraps at largest factor
	v.downsampleCount = 0
	v.computeDownsampleEnvelopeCoeffs()
}

func (v *Vocoder) buildDownsampleGroups() {
	factorSet := make(map[int]struct{}, len(v.downsampleFactors))
	for _, factor := range v.downsampleFactors {
		factorSet[factor] = struct{}{}
	}

	factors := make([]int, 0, len(factorSet))
	for factor := range factorSet {
		factors = append(factors, factor)
	}

	sort.Ints(factors)
	v.downsampleGroupFactors = factors
	v.downsampleGroupMasks = make([]int, len(factors))
	v.downsampleGroupAAFilters = make([]biquad.Section, len(factors))
	v.downsampleGroupBands = make([][]int, len(factors))

	groupIndex := make(map[int]int, len(factors))
	for i, factor := range factors {
		v.downsampleGroupMasks[i] = factor - 1
		v.downsampleGroupAAFilters[i] = *biquad.NewSection(antiAliasDecimatorLowpass(v.sampleRate, factor))
		groupIndex[factor] = i
	}

	for band, factor := range v.downsampleFactors {
		g := groupIndex[factor]
		v.downsampleGroupBands[g] = append(v.downsampleGroupBands[g], band)
	}
}

func (v *Vocoder) computeDownsampleEnvelopeCoeffs() {
	if len(v.downsampleFactors) == 0 {
		return
	}

	for i, factor := range v.downsampleFactors {
		scale := float64(factor)
		if v.attackMs > 0 {
			v.downsampleAttackCoeffs[i] = 1.0 - math.Exp(-scale/(v.attackMs*0.001*v.sampleRate))
		} else {
			v.downsampleAttackCoeffs[i] = 1.0
		}

		if v.releaseMs > 0 {
			v.downsampleReleaseCoeffs[i] = math.Exp(-scale / (v.releaseMs * 0.001 * v.sampleRate))
		} else {
			v.downsampleReleaseCoeffs[i] = 0.0
		}
	}
}

// antiAliasDecimatorLowpass builds a 2nd-order Butterworth-like RBJ low-pass
// section for a decimator group. Factor 1 is passthrough.
func antiAliasDecimatorLowpass(sampleRate float64, factor int) biquad.Coefficients {
	if factor <= 1 {
		return biquad.Coefficients{B0: 1}
	}

	cutoff := 0.35 * sampleRate / float64(factor)
	if cutoff <= 0 || cutoff >= 0.5*sampleRate {
		return biquad.Coefficients{B0: 1}
	}

	w0 := 2 * math.Pi * cutoff / sampleRate
	cw := math.Cos(w0)
	sw := math.Sin(w0)

	const q = 0.7071067811865476 // Butterworth

	alpha := sw / (2 * q)

	a0 := 1 + alpha
	inv := 1.0 / a0

	return biquad.Coefficients{
		B0: ((1 - cw) * 0.5) * inv,
		B1: (1 - cw) * inv,
		B2: ((1 - cw) * 0.5) * inv,
		A1: (-2 * cw) * inv,
		A2: (1 - alpha) * inv,
	}
}

// cpgBandpass computes constant-peak-gain (CPG) bandpass biquad coefficients.
// Unlike the constant-skirt-gain (CSG) variant where peak gain equals Q,
// the CPG peak gain at center frequency is always 1.0 regardless of Q,
// making it suitable for filter bank summation where bands should not amplify.
func cpgBandpass(freq, q, sampleRate float64) biquad.Coefficients {
	w0 := 2 * math.Pi * freq / sampleRate
	if w0 <= 0 || w0 >= math.Pi {
		return biquad.Coefficients{}
	}

	cw := math.Cos(w0)
	sw := math.Sin(w0)
	alpha := sw / (2 * q)

	a0 := 1 + alpha
	inv := 1.0 / a0

	return biquad.Coefficients{
		B0: alpha * inv,
		B1: 0,
		B2: -alpha * inv,
		A1: -2 * cw * inv,
		A2: (1 - alpha) * inv,
	}
}

// barkBandQ computes a Bark-band Q value from spacing to adjacent bands.
func barkBandQ(i int) float64 {
	freqs := barkFrequencies[:]

	var lower, upper float64
	if i == 0 {
		lower = freqs[0] * 0.5
	} else {
		lower = (freqs[i-1] + freqs[i]) / 2
	}

	if i >= len(freqs)-1 {
		upper = freqs[i] * 1.25
	} else {
		upper = (freqs[i] + freqs[i+1]) / 2
	}

	bw := upper - lower
	if bw <= 0 {
		return thirdOctaveQ
	}

	return freqs[i] / bw
}

// ProcessSample processes a single modulator/carrier sample pair and returns
// the vocoded output. Both BandLayoutThirdOctave and BandLayoutBark use the
// same bandpass analysis/envelope/synthesis pipeline, differing only in the
// center frequencies and per-band Q values set up at construction time.
func (v *Vocoder) ProcessSample(modulator, carrier float64) float64 {
	vocoded := 0.0

	if v.downsample && len(v.downsampleGroupFactors) > 0 {
		cnt := v.downsampleCount
		for g := range v.downsampleGroupFactors {
			decimated := v.downsampleGroupAAFilters[g].ProcessSample(modulator)
			if cnt&v.downsampleGroupMasks[g] != 0 {
				continue
			}

			for _, i := range v.downsampleGroupBands[g] {
				bandSignal := v.downsampleAnalysisFilters[i].ProcessSample(decimated)
				abs := math.Abs(bandSignal)

				env := v.envelopes[i]
				if abs > env {
					env += (abs - env) * v.downsampleAttackCoeffs[i]
				} else {
					env = abs + (env-abs)*v.downsampleReleaseCoeffs[i]
				}

				v.envelopes[i] = env
			}
		}
		// Synthesis always runs at full rate.
		for i := range v.numBands {
			vocoded += v.envelopes[i] * v.synthesisFilters[i].ProcessSample(carrier)
		}

		v.downsampleCount++
		if v.downsampleCount >= v.downsampleMax {
			v.downsampleCount = 0
		}
	} else {
		for i := range v.numBands {
			bandSignal := v.analysisFilters[i].ProcessSample(modulator)
			abs := math.Abs(bandSignal)

			env := v.envelopes[i]
			if abs > env {
				env += (abs - env) * v.attackCoeff
			} else {
				env = abs + (env-abs)*v.releaseCoeff
			}

			v.envelopes[i] = env
			vocoded += env * v.synthesisFilters[i].ProcessSample(carrier)
		}
	}

	return v.vocoderLevel*vocoded + v.synthLevel*carrier + v.inputLevel*modulator
}

// ProcessBlock processes modulator and carrier buffers, writing the result to output.
// All three slices must have the same length. Output may alias modulator or carrier.
func (v *Vocoder) ProcessBlock(modulator, carrier, output []float64) error {
	if len(modulator) != len(carrier) || len(modulator) != len(output) {
		return fmt.Errorf("vocoder: buffer length mismatch: modulator=%d carrier=%d output=%d",
			len(modulator), len(carrier), len(output))
	}

	for i := range modulator {
		output[i] = v.ProcessSample(modulator[i], carrier[i])
	}

	return nil
}

// Reset clears all internal filter and envelope state.
func (v *Vocoder) Reset() {
	for i := range v.envelopes {
		v.envelopes[i] = 0
	}

	for i := range v.analysisFilters {
		v.analysisFilters[i].Reset()
	}

	for i := range v.downsampleAnalysisFilters {
		v.downsampleAnalysisFilters[i].Reset()
	}

	for i := range v.downsampleGroupAAFilters {
		v.downsampleGroupAAFilters[i].Reset()
	}

	for i := range v.synthesisFilters {
		v.synthesisFilters[i].Reset()
	}

	v.downsampleCount = 0
}

// SetSampleRate updates the sample rate and rebuilds all filter banks.
func (v *Vocoder) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("vocoder: sample rate must be > 0: %f", sampleRate)
	}

	v.sampleRate = sampleRate
	v.computeEnvelopeCoeffs()
	v.envelopes = nil

	return v.buildFilterBanks()
}

// Getters.

// SampleRate returns the current sample rate in Hz.
func (v *Vocoder) SampleRate() float64 { return v.sampleRate }

// Layout returns the frequency band distribution.
func (v *Vocoder) Layout() BandLayout { return v.layout }

// NumBands returns the number of active frequency bands.
func (v *Vocoder) NumBands() int { return v.numBands }

// SynthesisQ returns the globally configured synthesis Q override.
// When Bark layout is selected and no override is set, synthesis uses
// per-band Bark-derived Q values.
func (v *Vocoder) SynthesisQ() float64 { return v.synthQ }

// Attack returns the envelope follower attack time in milliseconds.
func (v *Vocoder) Attack() float64 { return v.attackMs }

// Release returns the envelope follower release time in milliseconds.
func (v *Vocoder) Release() float64 { return v.releaseMs }

// InputLevel returns the dry modulator level (linear gain).
func (v *Vocoder) InputLevel() float64 { return v.inputLevel }

// SynthLevel returns the dry carrier level (linear gain).
func (v *Vocoder) SynthLevel() float64 { return v.synthLevel }

// VocoderLevel returns the vocoded output level (linear gain).
func (v *Vocoder) VocoderLevel() float64 { return v.vocoderLevel }

// Downsampling returns whether per-band multirate analysis is enabled.
func (v *Vocoder) Downsampling() bool { return v.downsample }

// DownsampleFactors returns a copy of the per-band downsample factors.
// Each factor is a power of 2: 1 means full rate, 2 means every other sample, etc.
// Returns nil when downsampling is disabled.
func (v *Vocoder) DownsampleFactors() []int {
	if !v.downsample || v.downsampleFactors == nil {
		return nil
	}

	out := make([]int, len(v.downsampleFactors))
	copy(out, v.downsampleFactors)

	return out
}

// Setters.

// SetDownsampling enables or disables per-band multirate analysis.
// When toggled on, downsample factors are recomputed from the current sample rate.
func (v *Vocoder) SetDownsampling(enabled bool) {
	v.downsample = enabled
	if enabled {
		// Recompute factors for current bands.
		switch v.layout {
		case BandLayoutThirdOctave:
			qs := make([]float64, v.numBands)
			for i := range qs {
				qs[i] = thirdOctaveQ
			}

			v.computeDownsampleFactors(thirdOctaveFrequencies[:v.numBands], qs)
		case BandLayoutBark:
			qs := make([]float64, v.numBands)
			for i := range qs {
				qs[i] = barkBandQ(i)
			}

			v.computeDownsampleFactors(barkFrequencies[:v.numBands], qs)
		}
	} else {
		v.downsampleFactors = nil
		v.downsampleAnalysisFilters = nil
		v.downsampleGroupFactors = nil
		v.downsampleGroupMasks = nil
		v.downsampleGroupAAFilters = nil
		v.downsampleGroupBands = nil
		v.downsampleAttackCoeffs = nil
		v.downsampleReleaseCoeffs = nil
		v.downsampleCount = 0
		v.downsampleMax = 0
	}
}

// SetAttack sets the envelope follower attack time in milliseconds.
func (v *Vocoder) SetAttack(ms float64) error {
	if ms < minVocoderAttackMs || ms > maxVocoderAttackMs || math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("vocoder: attack must be in [%g, %g] ms: %g",
			minVocoderAttackMs, maxVocoderAttackMs, ms)
	}

	v.attackMs = ms
	v.computeEnvelopeCoeffs()
	v.computeDownsampleEnvelopeCoeffs()

	return nil
}

// SetRelease sets the envelope follower release time in milliseconds.
func (v *Vocoder) SetRelease(ms float64) error {
	if ms < minVocoderReleaseMs || ms > maxVocoderReleaseMs || math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("vocoder: release must be in [%g, %g] ms: %g",
			minVocoderReleaseMs, maxVocoderReleaseMs, ms)
	}

	v.releaseMs = ms
	v.computeEnvelopeCoeffs()
	v.computeDownsampleEnvelopeCoeffs()

	return nil
}

// SetInputLevel sets the dry modulator level (linear gain).
func (v *Vocoder) SetInputLevel(level float64) error {
	if level < minVocoderLevel || level > maxVocoderLevel || math.IsNaN(level) || math.IsInf(level, 0) {
		return fmt.Errorf("vocoder: input level must be in [%g, %g]: %g",
			minVocoderLevel, maxVocoderLevel, level)
	}

	v.inputLevel = level

	return nil
}

// SetSynthLevel sets the dry carrier level (linear gain).
func (v *Vocoder) SetSynthLevel(level float64) error {
	if level < minVocoderLevel || level > maxVocoderLevel || math.IsNaN(level) || math.IsInf(level, 0) {
		return fmt.Errorf("vocoder: synth level must be in [%g, %g]: %g",
			minVocoderLevel, maxVocoderLevel, level)
	}

	v.synthLevel = level

	return nil
}

// SetVocoderLevel sets the vocoded output level (linear gain).
func (v *Vocoder) SetVocoderLevel(level float64) error {
	if level < minVocoderLevel || level > maxVocoderLevel || math.IsNaN(level) || math.IsInf(level, 0) {
		return fmt.Errorf("vocoder: vocoder level must be in [%g, %g]: %g",
			minVocoderLevel, maxVocoderLevel, level)
	}

	v.vocoderLevel = level

	return nil
}

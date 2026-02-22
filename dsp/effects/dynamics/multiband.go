package dynamics

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/crossover"
)

const (
	// Parameter validation
	minMultibandOrder     = 2
	maxMultibandOrder     = 24
	maxMultibandBands     = 8
	minCrossoverFrequency = 20.0
)

// Float64Ptr returns a pointer to the given float64 value, for use in [BandConfig].
func Float64Ptr(v float64) *float64 { return &v }

// BandConfig holds the compressor configuration for a single frequency band.
// Pointer fields (ThresholdDB, KneeDB, MakeupGainDB, AutoMakeup) use nil to
// indicate "keep default". Non-pointer fields use zero to indicate "keep default";
// this is safe because zero is not a valid value for Ratio, AttackMs, or ReleaseMs.
type BandConfig struct {
	ThresholdDB        *float64 // Compression threshold in dB (nil = keep default)
	Ratio              float64  // Compression ratio (1.0 = no compression, 0 = keep default)
	KneeDB             *float64 // Soft-knee width in dB (nil = keep default, ptr to 0 = hard knee)
	AttackMs           float64  // Attack time in milliseconds (0 = keep default)
	ReleaseMs          float64  // Release time in milliseconds (0 = keep default)
	MakeupGainDB       *float64 // Manual makeup gain in dB (nil = keep default; disables auto makeup when set)
	AutoMakeup         *bool    // Auto makeup gain toggle (nil = keep default)
	Topology           *DynamicsTopology
	DetectorMode       *DetectorMode
	FeedbackRatioScale *bool
	RMSWindowMs        *float64
	SidechainLowCutHz  *float64
	SidechainHighCutHz *float64
}

// MultibandMetrics holds per-band metering information.
type MultibandMetrics struct {
	Bands []CompressorMetrics // Per-band compressor metrics, ordered low to high
}

// MultibandCompressor splits an input signal into frequency bands using
// Linkwitz-Riley crossover filters and applies independent soft-knee
// compression to each band before summing the results.
//
// The crossover network uses [crossover.MultiBand] for frequency splitting,
// which guarantees allpass reconstruction when band outputs are summed.
// Each band has its own [Compressor] instance with independently configurable
// threshold, ratio, knee, attack, release, and makeup gain.
//
// The crossover order is adjustable (LR2, LR4, LR8, …) and applies to all
// crossover points. Higher orders provide steeper roll-off between bands.
//
// Signal flow:
//
//	input → crossover → [band 0 compressor] → ╲
//	                   → [band 1 compressor] →  + → output
//	                   → [band N compressor] → ╱
//
// This implementation is mono and single-threaded. For stereo processing,
// instantiate two MultibandCompressor instances or implement stereo-linking
// externally.
type MultibandCompressor struct {
	xover       *crossover.MultiBand
	compressors []*Compressor

	// Configuration
	crossoverFreqs []float64
	crossoverOrder int
	sampleRate     float64
}

// NewMultibandCompressor creates a multiband compressor with the given
// crossover frequencies, Linkwitz-Riley order, and sample rate.
//
// Parameters:
//   - freqs: crossover frequencies in Hz, strictly ascending, each in (0, sampleRate/2).
//     The number of bands is len(freqs)+1.
//   - order: Linkwitz-Riley order, must be a positive even integer (2, 4, 6, 8, …).
//     Higher orders give steeper crossover slopes.
//   - sampleRate: sample rate in Hz, must be positive.
//
// Each band's compressor is initialized with default parameters (see [NewCompressor]).
// Use [MultibandCompressor.SetBandThreshold] and related methods to configure
// per-band compression.
func NewMultibandCompressor(freqs []float64, order int, sampleRate float64) (*MultibandCompressor, error) {
	if err := validateMultibandParams(freqs, order, sampleRate); err != nil {
		return nil, err
	}

	xo, err := crossover.NewMultiBand(freqs, order, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("multiband compressor: %w", err)
	}

	numBands := xo.NumBands()

	compressors := make([]*Compressor, numBands)
	for i := range compressors {
		c, err := NewCompressor(sampleRate)
		if err != nil {
			return nil, fmt.Errorf("multiband compressor: band %d: %w", i, err)
		}

		compressors[i] = c
	}

	storedFreqs := make([]float64, len(freqs))
	copy(storedFreqs, freqs)

	return &MultibandCompressor{
		xover:          xo,
		compressors:    compressors,
		crossoverFreqs: storedFreqs,
		crossoverOrder: order,
		sampleRate:     sampleRate,
	}, nil
}

// NewMultibandCompressorWithConfig creates a multiband compressor with
// per-band configuration. The configs slice must have len(freqs)+1 elements,
// one per band ordered from lowest to highest frequency.
func NewMultibandCompressorWithConfig(freqs []float64, order int, sampleRate float64, configs []BandConfig) (*MultibandCompressor, error) {
	expectedBands := len(freqs) + 1
	if len(configs) != expectedBands {
		return nil, fmt.Errorf("multiband compressor: expected %d band configs for %d crossover frequencies, got %d",
			expectedBands, len(freqs), len(configs))
	}

	mc, err := NewMultibandCompressor(freqs, order, sampleRate)
	if err != nil {
		return nil, err
	}

	for i, cfg := range configs {
		if err := mc.applyBandConfig(i, cfg); err != nil {
			return nil, fmt.Errorf("multiband compressor: band %d config: %w", i, err)
		}
	}

	return mc, nil
}

// NumBands returns the number of frequency bands.
func (mc *MultibandCompressor) NumBands() int {
	return len(mc.compressors)
}

// CrossoverFreqs returns a copy of the crossover frequencies in Hz.
func (mc *MultibandCompressor) CrossoverFreqs() []float64 {
	out := make([]float64, len(mc.crossoverFreqs))
	copy(out, mc.crossoverFreqs)

	return out
}

// CrossoverOrder returns the Linkwitz-Riley order used for all crossover points.
func (mc *MultibandCompressor) CrossoverOrder() int {
	return mc.crossoverOrder
}

// SampleRate returns the sample rate in Hz.
func (mc *MultibandCompressor) SampleRate() float64 {
	return mc.sampleRate
}

// Band returns the compressor for the given band index (0 = lowest frequency band).
// This provides direct read access to the compressor's getters. Use the
// MultibandCompressor setter methods (SetBandThreshold, etc.) to modify parameters.
func (mc *MultibandCompressor) Band(i int) *Compressor {
	return mc.compressors[i]
}

// Crossover returns the underlying multiband crossover for analysis
// (e.g., frequency response inspection).
func (mc *MultibandCompressor) Crossover() *crossover.MultiBand {
	return mc.xover
}

// --- Per-band parameter setters ---

// SetBandThreshold sets the compression threshold for the specified band.
func (mc *MultibandCompressor) SetBandThreshold(band int, dB float64) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetThreshold(dB)
}

// SetBandRatio sets the compression ratio for the specified band.
func (mc *MultibandCompressor) SetBandRatio(band int, ratio float64) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetRatio(ratio)
}

// SetBandKnee sets the soft-knee width for the specified band.
func (mc *MultibandCompressor) SetBandKnee(band int, kneeDB float64) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetKnee(kneeDB)
}

// SetBandAttack sets the attack time for the specified band.
func (mc *MultibandCompressor) SetBandAttack(band int, ms float64) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetAttack(ms)
}

// SetBandRelease sets the release time for the specified band.
func (mc *MultibandCompressor) SetBandRelease(band int, ms float64) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetRelease(ms)
}

// SetBandMakeupGain sets manual makeup gain for the specified band and
// disables auto makeup for that band.
func (mc *MultibandCompressor) SetBandMakeupGain(band int, dB float64) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetMakeupGain(dB)
}

// SetBandAutoMakeup enables or disables auto makeup gain for the specified band.
func (mc *MultibandCompressor) SetBandAutoMakeup(band int, enable bool) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetAutoMakeup(enable)
}

// SetBandTopology sets detector topology for the specified band.
func (mc *MultibandCompressor) SetBandTopology(band int, topology DynamicsTopology) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetTopology(topology)
}

// SetBandDetectorMode sets detector mode for the specified band.
func (mc *MultibandCompressor) SetBandDetectorMode(band int, mode DetectorMode) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetDetectorMode(mode)
}

// SetBandFeedbackRatioScale toggles legacy feedback ratio-dependent time scaling.
func (mc *MultibandCompressor) SetBandFeedbackRatioScale(band int, enable bool) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetFeedbackRatioScale(enable)
}

// SetBandRMSWindow sets RMS detector window length in milliseconds.
func (mc *MultibandCompressor) SetBandRMSWindow(band int, ms float64) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetRMSWindow(ms)
}

// SetBandSidechainLowCut sets detector-only low-cut frequency in Hz (0 disables).
func (mc *MultibandCompressor) SetBandSidechainLowCut(band int, hz float64) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetSidechainLowCut(hz)
}

// SetBandSidechainHighCut sets detector-only high-cut frequency in Hz (0 disables).
func (mc *MultibandCompressor) SetBandSidechainHighCut(band int, hz float64) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.compressors[band].SetSidechainHighCut(hz)
}

// SetBandConfig applies a full BandConfig to the specified band.
func (mc *MultibandCompressor) SetBandConfig(band int, cfg BandConfig) error {
	if err := mc.checkBand(band); err != nil {
		return err
	}

	return mc.applyBandConfig(band, cfg)
}

// SetAllBandsThreshold sets the same threshold for all bands.
func (mc *MultibandCompressor) SetAllBandsThreshold(dB float64) error {
	for i := range mc.compressors {
		if err := mc.compressors[i].SetThreshold(dB); err != nil {
			return fmt.Errorf("band %d: %w", i, err)
		}
	}

	return nil
}

// SetAllBandsRatio sets the same ratio for all bands.
func (mc *MultibandCompressor) SetAllBandsRatio(ratio float64) error {
	for i := range mc.compressors {
		if err := mc.compressors[i].SetRatio(ratio); err != nil {
			return fmt.Errorf("band %d: %w", i, err)
		}
	}

	return nil
}

// SetAllBandsKnee sets the same knee width for all bands.
func (mc *MultibandCompressor) SetAllBandsKnee(kneeDB float64) error {
	for i := range mc.compressors {
		if err := mc.compressors[i].SetKnee(kneeDB); err != nil {
			return fmt.Errorf("band %d: %w", i, err)
		}
	}

	return nil
}

// SetAllBandsAttack sets the same attack time for all bands.
func (mc *MultibandCompressor) SetAllBandsAttack(ms float64) error {
	for i := range mc.compressors {
		if err := mc.compressors[i].SetAttack(ms); err != nil {
			return fmt.Errorf("band %d: %w", i, err)
		}
	}

	return nil
}

// SetAllBandsRelease sets the same release time for all bands.
func (mc *MultibandCompressor) SetAllBandsRelease(ms float64) error {
	for i := range mc.compressors {
		if err := mc.compressors[i].SetRelease(ms); err != nil {
			return fmt.Errorf("band %d: %w", i, err)
		}
	}

	return nil
}

// SetAllBandsTopology sets detector topology for all bands.
func (mc *MultibandCompressor) SetAllBandsTopology(topology DynamicsTopology) error {
	for i := range mc.compressors {
		if err := mc.compressors[i].SetTopology(topology); err != nil {
			return fmt.Errorf("band %d: %w", i, err)
		}
	}

	return nil
}

// SetAllBandsDetectorMode sets detector mode for all bands.
func (mc *MultibandCompressor) SetAllBandsDetectorMode(mode DetectorMode) error {
	for i := range mc.compressors {
		if err := mc.compressors[i].SetDetectorMode(mode); err != nil {
			return fmt.Errorf("band %d: %w", i, err)
		}
	}

	return nil
}

// SetAllBandsFeedbackRatioScale sets feedback ratio scaling for all bands.
func (mc *MultibandCompressor) SetAllBandsFeedbackRatioScale(enable bool) error {
	for i := range mc.compressors {
		if err := mc.compressors[i].SetFeedbackRatioScale(enable); err != nil {
			return fmt.Errorf("band %d: %w", i, err)
		}
	}

	return nil
}

// SetAllBandsRMSWindow sets RMS window in milliseconds for all bands.
func (mc *MultibandCompressor) SetAllBandsRMSWindow(ms float64) error {
	for i := range mc.compressors {
		if err := mc.compressors[i].SetRMSWindow(ms); err != nil {
			return fmt.Errorf("band %d: %w", i, err)
		}
	}

	return nil
}

// --- Processing ---

// ProcessSample splits one input sample into frequency bands, compresses
// each band independently, and returns the summed output.
func (mc *MultibandCompressor) ProcessSample(input float64) float64 {
	bands := mc.xover.ProcessSample(input)

	sum := 0.0
	for i, b := range bands {
		sum += mc.compressors[i].ProcessSample(b)
	}

	return sum
}

// ProcessSampleMulti splits one input sample into frequency bands, compresses
// each band independently, and returns each band's compressed output separately.
// The returned slice has NumBands() elements, ordered lowest to highest frequency.
func (mc *MultibandCompressor) ProcessSampleMulti(input float64) []float64 {
	bands := mc.xover.ProcessSample(input)
	for i, b := range bands {
		bands[i] = mc.compressors[i].ProcessSample(b)
	}

	return bands
}

// ProcessInPlace applies multiband compression to buf in place.
// The output is the sum of all compressed bands.
func (mc *MultibandCompressor) ProcessInPlace(buf []float64) {
	if len(buf) == 0 {
		return
	}

	// Split into bands using block processing
	bandBlocks := mc.xover.ProcessBlock(buf)

	// Compress each band in place
	for i, block := range bandBlocks {
		mc.compressors[i].ProcessInPlace(block)
	}

	// Sum all bands back into buf
	copy(buf, bandBlocks[0])

	for i := 1; i < len(bandBlocks); i++ {
		for j, v := range bandBlocks[i] {
			buf[j] += v
		}
	}
}

// ProcessStereoInPlace applies multiband compression independently to left and
// right channels. Slices must have the same length.
func (mc *MultibandCompressor) ProcessStereoInPlace(left, right []float64) error {
	if len(left) != len(right) {
		return fmt.Errorf("multiband compressor: stereo buffers must have equal length, got %d and %d", len(left), len(right))
	}

	mc.ProcessInPlace(left)
	mc.ProcessInPlace(right)

	return nil
}

// ProcessInPlaceMulti applies multiband compression and returns per-band
// outputs. The returned slice has NumBands() elements, each of the same
// length as input. The input slice is not modified.
func (mc *MultibandCompressor) ProcessInPlaceMulti(input []float64) [][]float64 {
	if len(input) == 0 {
		out := make([][]float64, mc.NumBands())
		for i := range out {
			out[i] = []float64{}
		}

		return out
	}

	bandBlocks := mc.xover.ProcessBlock(input)
	for i, block := range bandBlocks {
		mc.compressors[i].ProcessInPlace(block)
	}

	return bandBlocks
}

// --- State management ---

// Reset clears all internal state (crossover filters and compressor envelopes).
func (mc *MultibandCompressor) Reset() {
	mc.xover.Reset()

	for _, c := range mc.compressors {
		c.Reset()
	}
}

// GetMetrics returns per-band compressor metrics.
func (mc *MultibandCompressor) GetMetrics() MultibandMetrics {
	bands := make([]CompressorMetrics, len(mc.compressors))
	for i, c := range mc.compressors {
		bands[i] = c.GetMetrics()
	}

	return MultibandMetrics{Bands: bands}
}

// ResetMetrics clears metering state for all bands.
func (mc *MultibandCompressor) ResetMetrics() {
	for _, c := range mc.compressors {
		c.ResetMetrics()
	}
}

// --- Internal helpers ---

func (mc *MultibandCompressor) checkBand(band int) error {
	if band < 0 || band >= len(mc.compressors) {
		return fmt.Errorf("multiband compressor: band index %d out of range [0, %d)", band, len(mc.compressors))
	}

	return nil
}

func (mc *MultibandCompressor) applyBandConfig(band int, cfg BandConfig) error {
	c := mc.compressors[band]

	if cfg.ThresholdDB != nil {
		if err := c.SetThreshold(*cfg.ThresholdDB); err != nil {
			return err
		}
	}

	if cfg.Ratio != 0 {
		if err := c.SetRatio(cfg.Ratio); err != nil {
			return err
		}
	}

	if cfg.KneeDB != nil {
		if err := c.SetKnee(*cfg.KneeDB); err != nil {
			return err
		}
	}

	if cfg.AttackMs != 0 {
		if err := c.SetAttack(cfg.AttackMs); err != nil {
			return err
		}
	}

	if cfg.ReleaseMs != 0 {
		if err := c.SetRelease(cfg.ReleaseMs); err != nil {
			return err
		}
	}

	if cfg.MakeupGainDB != nil {
		if err := c.SetMakeupGain(*cfg.MakeupGainDB); err != nil {
			return err
		}
	}

	if cfg.AutoMakeup != nil {
		if err := c.SetAutoMakeup(*cfg.AutoMakeup); err != nil {
			return err
		}
	}

	if cfg.Topology != nil {
		if err := c.SetTopology(*cfg.Topology); err != nil {
			return err
		}
	}

	if cfg.DetectorMode != nil {
		if err := c.SetDetectorMode(*cfg.DetectorMode); err != nil {
			return err
		}
	}

	if cfg.FeedbackRatioScale != nil {
		if err := c.SetFeedbackRatioScale(*cfg.FeedbackRatioScale); err != nil {
			return err
		}
	}

	if cfg.RMSWindowMs != nil {
		if err := c.SetRMSWindow(*cfg.RMSWindowMs); err != nil {
			return err
		}
	}

	if cfg.SidechainLowCutHz != nil {
		if err := c.SetSidechainLowCut(*cfg.SidechainLowCutHz); err != nil {
			return err
		}
	}

	if cfg.SidechainHighCutHz != nil {
		if err := c.SetSidechainHighCut(*cfg.SidechainHighCutHz); err != nil {
			return err
		}
	}

	return nil
}

func validateMultibandParams(freqs []float64, order int, sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("multiband compressor: sample rate must be positive and finite: %v", sampleRate)
	}

	if len(freqs) < 1 {
		return fmt.Errorf("multiband compressor: at least one crossover frequency is required (got 0)")
	}

	numBands := len(freqs) + 1
	if numBands > maxMultibandBands {
		return fmt.Errorf("multiband compressor: maximum %d bands (%d crossover frequencies), got %d bands",
			maxMultibandBands, maxMultibandBands-1, numBands)
	}

	if order < minMultibandOrder || order > maxMultibandOrder {
		return fmt.Errorf("multiband compressor: order must be in [%d, %d], got %d",
			minMultibandOrder, maxMultibandOrder, order)
	}

	if order%2 != 0 {
		return fmt.Errorf("multiband compressor: order must be even (Linkwitz-Riley), got %d", order)
	}

	nyquist := sampleRate / 2

	for i, f := range freqs {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("multiband compressor: crossover frequency %d must be finite: %v", i, f)
		}

		if f < minCrossoverFrequency {
			return fmt.Errorf("multiband compressor: crossover frequency %d must be >= %.0f Hz, got %.1f Hz",
				i, minCrossoverFrequency, f)
		}

		if f >= nyquist {
			return fmt.Errorf("multiband compressor: crossover frequency %d must be < Nyquist (%.0f Hz), got %.1f Hz",
				i, nyquist, f)
		}

		if i > 0 && f <= freqs[i-1] {
			return fmt.Errorf("multiband compressor: crossover frequencies must be strictly ascending, got %.1f after %.1f",
				f, freqs[i-1])
		}
	}

	return nil
}

package effectchain

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

func wrapConfigureErr(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("effectchain: configure: %w", err)
}

func configureChorus(fx *modulation.Chorus, sampleRate, mix, depth, speedHz float64, stages int) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetMix(mix)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDepth(depth)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetSpeedHz(speedHz)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetStages(stages))
}

func configureFlanger(fx *modulation.Flanger, sampleRate, rateHz, baseDelay, depth, feedback, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetRateHz(rateHz)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDepthSeconds(0)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetBaseDelaySeconds(baseDelay)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDepthSeconds(depth)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetFeedback(feedback)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetMix(mix))
}

func configureRingMod(fx *modulation.RingModulator, sampleRate, carrierHz, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetCarrierHz(carrierHz)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetMix(mix))
}

func configureBitCrusher(fx *effects.BitCrusher, sampleRate, bitDepth float64, downsample int, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetBitDepth(bitDepth)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDownsample(downsample)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetMix(mix))
}

//nolint:cyclop,funlen
func configureDistortion(
	fx *effects.Distortion,
	sampleRate float64,
	mode effects.DistortionMode,
	approx effects.DistortionApproxMode,
	drive, mix, outputLevel, clipLevel, shape, bias float64,
	chebOrder int,
	chebMode effects.ChebyshevHarmonicMode,
	chebInvert bool,
	chebGain float64,
	chebDCBypass bool,
) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	if chebMode == effects.ChebyshevHarmonicOdd && chebOrder%2 == 0 {
		chebOrder++
	}

	if chebMode == effects.ChebyshevHarmonicEven && chebOrder%2 != 0 {
		chebOrder++
	}

	if chebOrder > 16 {
		chebOrder = 16
	}

	if chebMode == effects.ChebyshevHarmonicOdd && chebOrder%2 == 0 {
		chebOrder--
	}

	if chebMode == effects.ChebyshevHarmonicEven && chebOrder%2 != 0 {
		chebOrder--
	}

	if chebOrder < 1 {
		chebOrder = 1
	}

	err = fx.SetMode(mode)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetApproxMode(approx)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDrive(drive)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetMix(mix)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetOutputLevel(outputLevel)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetClipLevel(clipLevel)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetShape(shape)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetBias(bias)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetChebyshevOrder(chebOrder)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetChebyshevHarmonicMode(chebMode)
	if err != nil {
		return wrapConfigureErr(err)
	}

	fx.SetChebyshevInvert(chebInvert)

	err = fx.SetChebyshevGainLevel(chebGain)
	if err != nil {
		return wrapConfigureErr(err)
	}

	fx.SetChebyshevDCBypass(chebDCBypass)

	return nil
}

func configureTransformer(
	fx *effects.TransformerSimulation,
	sampleRate float64,
	quality effects.TransformerQuality,
	drive, mix, outputLevel, highpassHz, dampingHz float64,
	oversampling int,
) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetQuality(quality)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDrive(drive)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetMix(mix)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetOutputLevel(outputLevel)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetHighpassHz(highpassHz)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDampingHz(dampingHz)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetOversampling(oversampling))
}

func configureWidener(fx *spatial.StereoWidener, sampleRate, width float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetWidth(width)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetBassMonoFreq(0))
}

func configurePhaser(fx *modulation.Phaser, sampleRate, rateHz, minFreqHz, maxFreqHz float64, stages int, feedback, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetRateHz(rateHz)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetFrequencyRangeHz(minFreqHz, maxFreqHz)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetStages(stages)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetFeedback(feedback)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetMix(mix))
}

func configureTremolo(fx *modulation.Tremolo, sampleRate, rateHz, depth, smoothingMs, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetRateHz(rateHz)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDepth(depth)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetSmoothingMs(smoothingMs)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetMix(mix))
}

func configureDelay(fx *effects.Delay, sampleRate, time, feedback, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetTargetTime(time)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetFeedback(feedback)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetMix(mix))
}

func configureTimePitch(fx *pitch.PitchShifter, sampleRate, semitones, sequence, overlap, search float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetPitchSemitones(semitones)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetSequence(sequence)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetOverlap(overlap)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetSearch(search))
}

func configureSpectralPitch(fx *pitch.SpectralPitchShifter, sampleRate, semitones float64, frameSize, analysisHop int) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetPitchSemitones(semitones)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetFrameSize(frameSize)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetAnalysisHop(analysisHop))
}

func configureSpectralFreeze(
	fx *effects.SpectralFreeze,
	sampleRate float64,
	frameSize, hopSize int,
	mix float64,
	phaseMode effects.SpectralFreezePhaseMode,
	frozen bool,
	windowType window.Type,
) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetFrameSize(frameSize)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetHopSize(hopSize)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetWindowType(windowType)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetMix(mix)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetPhaseMode(phaseMode)
	if err != nil {
		return wrapConfigureErr(err)
	}

	fx.SetFrozen(frozen)

	return nil
}

func configureGranular(
	fx *effects.Granular,
	sampleRate, grainSeconds, overlap, pitchRatio, spray, baseDelay, mix float64,
) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetGrainSeconds(grainSeconds)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetOverlap(overlap)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetPitch(pitchRatio)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetSpray(spray)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetBaseDelay(baseDelay)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetMix(mix))
}

func configureFDNReverb(fx *reverb.FDNReverb, sampleRate, wet, dry, rt60, preDelay, damp, modDepth, modRate float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetWet(wet)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDry(dry)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetRT60(rt60)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetPreDelay(preDelay)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDamp(damp)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetModDepth(modDepth)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetModRate(modRate))
}

func configureFreeverb(fx *reverb.Reverb, wet, dry, roomSize, damp, gain float64) {
	fx.SetWet(wet)
	fx.SetDry(dry)
	fx.SetRoomSize(roomSize)
	fx.SetDamp(damp)
	fx.SetGain(gain)
}

func configureHarmonicBass(fx *effects.HarmonicBass, sampleRate, frequency, inputGain, highGain, original, harmonic, decay, responseMs float64, highpass int) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetFrequency(frequency)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetInputLevel(inputGain)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetHighFrequencyLevel(highGain)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetOriginalBassLevel(original)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetHarmonicBassLevel(harmonic)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetDecay(decay)
	if err != nil {
		return wrapConfigureErr(err)
	}

	err = fx.SetResponse(responseMs)
	if err != nil {
		return wrapConfigureErr(err)
	}

	return wrapConfigureErr(fx.SetHighpassMode(effects.HighpassSelect(highpass)))
}

func sanitizeSpectralPitchFrameSize(n int) int {
	if n < 256 {
		return 256
	}

	if n > 4096 {
		return 4096
	}

	if n > 0 && (n&(n-1)) == 0 {
		return n
	}

	lower := 256
	for lower < n {
		lower <<= 1
	}

	upper := lower
	lower >>= 1

	if lower < 256 {
		lower = 256
	}

	if n-lower <= upper-n {
		return lower
	}

	return upper
}

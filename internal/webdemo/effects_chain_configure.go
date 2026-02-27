package webdemo

import (
	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
)

func configureChorus(fx *modulation.Chorus, sampleRate, mix, depth, speedHz float64, stages int) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetMix(mix)
	if err != nil {
		return err
	}

	err = fx.SetDepth(depth)
	if err != nil {
		return err
	}

	err = fx.SetSpeedHz(speedHz)
	if err != nil {
		return err
	}

	return fx.SetStages(stages)
}

func configureFlanger(fx *modulation.Flanger, sampleRate, rateHz, baseDelay, depth, feedback, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetRateHz(rateHz)
	if err != nil {
		return err
	}

	err = fx.SetDepthSeconds(0)
	if err != nil {
		return err
	}

	err = fx.SetBaseDelaySeconds(baseDelay)
	if err != nil {
		return err
	}

	err = fx.SetDepthSeconds(depth)
	if err != nil {
		return err
	}

	err = fx.SetFeedback(feedback)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureRingMod(fx *modulation.RingModulator, sampleRate, carrierHz, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetCarrierHz(carrierHz)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureBitCrusher(fx *effects.BitCrusher, sampleRate, bitDepth float64, downsample int, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetBitDepth(bitDepth)
	if err != nil {
		return err
	}

	err = fx.SetDownsample(downsample)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureWidener(fx *spatial.StereoWidener, sampleRate, width float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetWidth(width)
	if err != nil {
		return err
	}

	return fx.SetBassMonoFreq(0)
}

func configurePhaser(fx *modulation.Phaser, sampleRate, rateHz, minFreqHz, maxFreqHz float64, stages int, feedback, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetRateHz(rateHz)
	if err != nil {
		return err
	}

	err = fx.SetFrequencyRangeHz(minFreqHz, maxFreqHz)
	if err != nil {
		return err
	}

	err = fx.SetStages(stages)
	if err != nil {
		return err
	}

	err = fx.SetFeedback(feedback)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureTremolo(fx *modulation.Tremolo, sampleRate, rateHz, depth, smoothingMs, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetRateHz(rateHz)
	if err != nil {
		return err
	}

	err = fx.SetDepth(depth)
	if err != nil {
		return err
	}

	err = fx.SetSmoothingMs(smoothingMs)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureDelay(fx *effects.Delay, sampleRate, time, feedback, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetTargetTime(time)
	if err != nil {
		return err
	}

	err = fx.SetFeedback(feedback)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureTimePitch(fx *pitch.PitchShifter, sampleRate, semitones, sequence, overlap, search float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetPitchSemitones(semitones)
	if err != nil {
		return err
	}

	err = fx.SetSequence(sequence)
	if err != nil {
		return err
	}

	err = fx.SetOverlap(overlap)
	if err != nil {
		return err
	}

	return fx.SetSearch(search)
}

func configureSpectralPitch(fx *pitch.SpectralPitchShifter, sampleRate, semitones float64, frameSize, analysisHop int) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetPitchSemitones(semitones)
	if err != nil {
		return err
	}

	err = fx.SetFrameSize(frameSize)
	if err != nil {
		return err
	}

	return fx.SetAnalysisHop(analysisHop)
}

func configureFDNReverb(fx *reverb.FDNReverb, sampleRate, wet, dry, rt60, preDelay, damp, modDepth, modRate float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetWet(wet)
	if err != nil {
		return err
	}

	err = fx.SetDry(dry)
	if err != nil {
		return err
	}

	err = fx.SetRT60(rt60)
	if err != nil {
		return err
	}

	err = fx.SetPreDelay(preDelay)
	if err != nil {
		return err
	}

	err = fx.SetDamp(damp)
	if err != nil {
		return err
	}

	err = fx.SetModDepth(modDepth)
	if err != nil {
		return err
	}

	return fx.SetModRate(modRate)
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
		return err
	}

	err = fx.SetFrequency(frequency)
	if err != nil {
		return err
	}

	err = fx.SetInputLevel(inputGain)
	if err != nil {
		return err
	}

	err = fx.SetHighFrequencyLevel(highGain)
	if err != nil {
		return err
	}

	err = fx.SetOriginalBassLevel(original)
	if err != nil {
		return err
	}

	err = fx.SetHarmonicBassLevel(harmonic)
	if err != nil {
		return err
	}

	err = fx.SetDecay(decay)
	if err != nil {
		return err
	}

	err = fx.SetResponse(responseMs)
	if err != nil {
		return err
	}

	return fx.SetHighpassMode(effects.HighpassSelect(highpass))
}

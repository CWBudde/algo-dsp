package effectchain

import (
	"maps"
	"strings"
)

// addFactoryPresets augments owned metadata with practical starting points.
// Overrides are bounded by each descriptor, including sample-rate frequency
// limits; every preset retains complete canonical numeric/string parameters.
func addFactoryPresets(descriptors []Descriptor) {
	for i := range descriptors {
		d := &descriptors[i]

		id, name, num, str := factoryPresetValues(d.ID)
		if id == "" || len(d.Presets) == 0 {
			continue
		}

		base := d.Presets[0]
		preset := FactoryPreset{ID: id, Name: name, Num: maps.Clone(base.Num), Str: maps.Clone(base.Str)}

		for _, parameter := range d.Parameters {
			if value, ok := num[parameter.ID]; ok {
				preset.Num[parameter.ID] = min(max(value, parameter.Min), parameter.Max)
			}

			if value, ok := str[parameter.ID]; ok {
				for _, option := range parameter.Options {
					if value == option.Value {
						preset.Str[parameter.ID] = value
						break
					}
				}
			}
		}

		d.Presets = append(d.Presets, preset)
	}
}

//nolint:funlen,cyclop
func factoryPresetValues(effect string) (string, string, map[string]float64, map[string]string) {
	num := map[string]float64{}
	str := map[string]string{}
	id, name := "character", "Character"

	switch effect {
	case "chorus":
		name = "Wide chorus"
		num = map[string]float64{"stages": 4, "mix": .4, "depth": .004, "speedHz": .6}
	case "flanger":
		name = "Slow sweep"
		num = map[string]float64{"rateHz": .15, "depth": .003, "feedback": .45, "mix": .4}
	case "phaser":
		name = "Warm phaser"
		num = map[string]float64{"rateHz": .2, "stages": 8, "feedback": .3, "mix": .5}
	case "tremolo":
		name = "Deep tremolo"
		num = map[string]float64{"rateHz": 5, "depth": .85, "smoothingMs": 8}
	case "ringmod":
		name = "Bell sidebands"
		num = map[string]float64{"carrierHz": 880, "mix": .75}
	case "auto-wah":
		name = "Expressive wah"
		num = map[string]float64{"minFreqHz": 200, "maxFreqHz": 2800, "q": 2, "sensitivity": 4, "mix": .75}
	case "frequency-shifter":
		name = "Gentle downward shift"
		num = map[string]float64{"shiftHz": 30, "mix": .5}
		str["direction"] = "down"
	case "rotary":
		name = "Fast rotary"
		num = map[string]float64{"fast": 1, "drive": 1.5, "mix": .85, "stereoWidth": 1}
	case "widener":
		name = "Wider image"
		num = map[string]float64{"width": 1.5, "mix": 1}
	case "panner":
		name = "Slow auto-pan"
		num = map[string]float64{"autoPanRateHz": .3, "autoPanDepth": .8}
	case "haas":
		name = "Subtle right delay"
		num["delayMs"] = 8
		str["channel"] = "right"
	case "crosstalk":
		name = "Headphone crossfeed"
		num["crossfeedMix"] = .35
		str["preset"] = "hdphx"
	case "delay":
		name = "Short echo"
		num = map[string]float64{"time": .18, "feedback": .4, "mix": .3}
	case "delay-simple":
		name = "Slap delay"
		num["delayMs"] = 80
	case "reverb", "reverb-fdn":
		name = "Small room"
		num = map[string]float64{"wet": .18, "dry": 1, "rt60": .8, "preDelay": .005, "damp": .55}
		str["model"] = "fdn"
	case "reverb-freeverb":
		name = "Warm room"
		num = map[string]float64{"wet": .18, "dry": 1, "roomSize": .55, "damp": .6}
	case "reverb-conv":
		name = "Subtle impulse"
		num["wet"] = .15
	case "distortion":
		name = "Soft saturation"
		num = map[string]float64{"drive": 2.5, "mix": .5, "output": .8}
		str["mode"] = "tanh"
	case "dist-cheb":
		name = "Third harmonic"
		num = map[string]float64{"order": 3, "mix": .25, "output": .75}
		str["harmonic"] = "odd"
	case "bitcrusher":
		name = "Retro twelve-bit"
		num = map[string]float64{"bitDepth": 12, "downsample": 2, "mix": .6}
	case "transformer":
		name = "Warm transformer"
		num = map[string]float64{"drive": 3, "mix": .5, "output": .8, "oversampling": 4}
	case "bass":
		name = "Sub harmonics"
		num = map[string]float64{"frequency": 60, "original": 1, "harmonic": .3, "responseMs": 30}
	case "pitch-time", "pitch-spectral":
		name = "Octave up"
		num["semitones"] = 12
	case "spectral-freeze":
		name = "Frozen ambience"
		num = map[string]float64{"frameSize": 2048, "mix": .7, "frozen": 1}
		str["phaseMode"] = "advance"
	case "granular":
		name = "Shimmer grains"
		num = map[string]float64{"grainSeconds": .06, "overlap": .7, "pitch": 2, "spray": .2, "baseDelay": .05, "mix": .5}
	case "dyn-compressor":
		name = "Gentle vocal"
		num = map[string]float64{"thresholdDB": -18, "ratio": 2.5, "kneeDB": 8, "attackMs": 15, "releaseMs": 120, "makeupGainDB": 2}
	case "dyn-limiter":
		name = "Safety ceiling"
		num = map[string]float64{"thresholdDB": -1, "releaseMs": 80}
	case "dyn-lookahead":
		name = "Transparent ceiling"
		num = map[string]float64{"thresholdDB": -1, "lookaheadMs": 5, "releaseMs": 80}
	case "dyn-gate":
		name = "Gentle noise gate"
		num = map[string]float64{"thresholdDB": -45, "ratio": 4, "kneeDB": 8, "holdMs": 80, "releaseMs": 150, "rangeDB": -40}
	case "dyn-expander":
		name = "Gentle expansion"
		num = map[string]float64{"thresholdDB": -40, "ratio": 1.5, "kneeDB": 10, "releaseMs": 150, "rangeDB": -30}
		str["detector"] = "rms"
	case "dyn-deesser":
		name = "Soft sibilance"
		num = map[string]float64{"freqHz": 6500, "thresholdDB": -24, "ratio": 3, "rangeDB": -12}
	case "dyn-transient":
		name = "Sharper attack"
		num = map[string]float64{"attack": .3, "sustain": -.15, "attackMs": 8, "releaseMs": 100}
	case "dyn-multiband":
		name = "Gentle bus compression"
		num = map[string]float64{"lowRatio": 2, "midRatio": 2, "highRatio": 2, "attackMs": 15, "releaseMs": 150, "kneeDB": 10}
	case "dyn-eq":
		name = "Controlled low mids"
		num = map[string]float64{"bands": 1, "band1FreqHz": 250, "band1Q": 1, "band1ThresholdDB": -24, "band1Ratio": 3, "band1RangeDB": 6}
		str["band1Mode"] = "downward"
	case "eq-parametric":
		name = "Vocal presence"
		num = map[string]float64{"bands": 3, "band1FreqHz": 120, "band1GainDB": -3, "band2FreqHz": 2500, "band2GainDB": 3, "band3FreqHz": 8000, "band3GainDB": 2}
		str["band1Type"] = "lowshelf"
		str["band2Type"] = "peak"
		str["band3Type"] = "highshelf"
	case "eq-graphic":
		name = "Gentle smile"
		num = map[string]float64{"gain1DB": 2, "gain2DB": 2, "gain3DB": 1, "gain5DB": -1, "gain6DB": -1, "gain9DB": 1, "gain10DB": 2}
	case "vocoder":
		name = "Mixed vocoder"
		num = map[string]float64{"inputLevel": .25, "vocoderLevel": .8, "releaseMs": 5}
	case "filter-a-weighting", "filter-c-weighting":
		return "", "", nil, nil
	default:
		if !strings.HasPrefix(effect, "filter") {
			return "", "", nil, nil
		}

		switch effect {
		case "filter-highpass":
			name = "Remove low rumble"
			num["freq"] = 80
			num["order"] = 4
			str["family"] = "butterworth"
		case "filter-lowpass", "filter":
			name = "Warm low-pass"
			num["freq"] = 6000
			num["order"] = 4
			str["family"] = "butterworth"
		case "filter-peak":
			name = "Presence lift"
			num = map[string]float64{"freq": 2500, "gain": 3, "q": 1}
		case "filter-lowshelf":
			name = "Bass lift"
			num = map[string]float64{"freq": 120, "gain": 3, "q": .707}
		case "filter-highshelf":
			name = "Air lift"
			num = map[string]float64{"freq": 8000, "gain": 3, "q": .707}
		case "filter-notch":
			name = "Mains notch"
			num = map[string]float64{"freq": 50, "q": 8}
		case "filter-bandpass":
			name = "Speech band"
			num = map[string]float64{"freq": 1500, "q": .8}
		case "filter-allpass":
			name = "Low phase rotation"
			num = map[string]float64{"freq": 300, "q": .707}
		case "filter-moog":
			name = "Resonant low-pass"
			num = map[string]float64{"freq": 600, "q": 1.5, "gain": 3, "order": 4}
		}
	}

	return id, name, num, str
}

package dither

import "fmt"

// Preset identifies a predefined FIR noise-shaping coefficient set.
type Preset int

const (
	PresetNone         Preset = iota // No shaping
	PresetEFB                        // Simple error feedback, 1st order
	Preset2SC                        // Simple 2nd-order highpass
	Preset2MEC                       // Modified E-weighted, 2nd order
	Preset3MEC                       // Modified E-weighted, 3rd order
	Preset9MEC                       // Modified E-weighted, 9th order
	Preset5IEC                       // Improved E-weighted, 5th order
	Preset9IEC                       // Improved E-weighted, 9th order
	Preset3FC                        // F-weighted, 3rd order
	Preset9FC                        // F-weighted, 9th order (default)
	PresetSBM                        // Sony Super Bit Mapping, 12th order
	PresetSBMReduced                 // Reduced Super Bit Mapping, 10th order
	PresetSharp14k                   // Sharp 14 kHz rolloff, 7th order (44.1 kHz)
	PresetSharp15k                   // Sharp 15 kHz rolloff, 8th order (44.1 kHz)
	PresetSharp16k                   // Sharp 16 kHz rolloff, 9th order (44.1 kHz)
	PresetExperimental               // Experimental, 9th order

	presetCount // sentinel
)

var presetNames = [presetCount]string{
	"None", "EFB", "2SC", "2MEC", "3MEC", "9MEC", "5IEC", "9IEC",
	"3FC", "9FC", "SBM", "SBMReduced", "Sharp14k", "Sharp15k",
	"Sharp16k", "Experimental",
}

// String returns the name of the preset.
func (p Preset) String() string {
	if p >= 0 && p < presetCount {
		return presetNames[p]
	}
	return fmt.Sprintf("Preset(%d)", p)
}

// Valid reports whether p is a known preset.
func (p Preset) Valid() bool {
	return p >= 0 && p < presetCount
}

// Coefficients returns a copy of the FIR noise-shaping coefficients for this preset.
// Returns nil for PresetNone.
func (p Preset) Coefficients() []float64 {
	src := presetCoeffs[p]
	if len(src) == 0 {
		return nil
	}
	out := make([]float64, len(src))
	copy(out, src)
	return out
}

var presetCoeffs = [presetCount][]float64{
	PresetNone:         nil,
	PresetEFB:          coeffEFB,
	Preset2SC:          coeff2SC,
	Preset2MEC:         coeff2MEC,
	Preset3MEC:         coeff3MEC,
	Preset9MEC:         coeff9MEC,
	Preset5IEC:         coeff5IEC,
	Preset9IEC:         coeff9IEC,
	Preset3FC:          coeff3FC,
	Preset9FC:          coeff9FC,
	PresetSBM:          coeffSBM,
	PresetSBMReduced:   coeffSBMr,
	PresetSharp14k:     coeff14kSharp44100,
	PresetSharp15k:     coeff15kSharp44100,
	PresetSharp16k:     coeff16kSharp44100,
	PresetExperimental: coeffEX,
}

// Coefficient arrays — exact values from legacy DAV_DspDitherNoiseShaper.pas.

var coeffEFB = []float64{1}

var coeff2SC = []float64{1.0, -0.5}

var coeff2MEC = []float64{1.537, -0.8367}

var coeff3MEC = []float64{1.652, -1.049, 0.1382}

var coeff9MEC = []float64{
	1.662, -1.263, 0.4827, -0.2913, 0.1268,
	-0.1124, 0.03252, -0.01265, -0.03524,
}

var coeff5IEC = []float64{2.033, -2.165, 1.959, -1.590, 0.6149}

var coeff9IEC = []float64{
	2.847, -4.685, 6.214, -7.184, 6.639,
	-5.032, 3.263, -1.632, 0.4191,
}

var coeff3FC = []float64{1.623, -0.982, 0.109}

var coeff9FC = []float64{
	2.412, -3.370, 3.937, -4.174, 3.353,
	-2.205, 1.281, -0.569, 0.0847,
}

var coeffSBM = []float64{
	1.47933, -1.59032, 1.64436, -1.36613,
	0.926704, -0.557931, 0.26786, -0.106726,
	0.028516, 0.00123066, -0.00616555, 0.003067,
}

var coeffSBMr = []float64{
	1.47933, -1.59032, 1.64436, -1.36613,
	0.926704, -0.557931, 0.26786, -0.106726,
	0.028516, 0.00123066,
}

var coeffEX = []float64{
	1.2194769820734, -1.77912468394129,
	2.18256539389233, -2.33622087251503,
	2.2010985277411, -1.81964871362306,
	1.29830681491534, -0.767889385169331,
	0.320990893363264,
}

var coeff14kSharp44100 = []float64{
	1.62019206878484, -2.26551157411517,
	2.50884415683988, -2.25007947643775,
	1.62160867255441, -0.899114621685913,
	0.35350816625238,
}

var coeff15kSharp44100 = []float64{
	1.34860378444905, -1.80123976889643,
	2.04804746376671, -1.93234174830592,
	1.59264693241396, -1.04979311664936,
	0.599422666305319, -0.213194268754789,
}

var coeff16kSharp44100 = []float64{
	1.07618924753262, -1.41232919229157,
	1.61374140100329, -1.5996973679788,
	1.42711666927426, -1.09986023030973,
	0.750589080482029, -0.418709259968069,
	0.185132272731155,
}

// Sample-rate-adaptive sharp preset coefficient sets.

var coeff15kSharp40000 = []float64{
	0.919387305668676, -1.04843437730544,
	1.04843048925451, -0.868972788711174,
	0.60853001063849, -0.3449209471469,
	0.147484332561636, -0.0370652871194614,
}

var coeff15kSharp48000 = []float64{
	1.4247141061364, -1.5437678148854,
	1.0967969510044, -0.32075758107035,
	-0.32074811729292, 0.525494723539046,
	-0.38058984415197, 0.14824460513256,
}

var coeff15kSharp64000 = []float64{
	2.49725554745212, -3.23587161287721,
	2.31844946822861, -0.54326047010533,
	-0.54325301319653, 0.543289788745007,
	-0.142132484905, -0.0202120370327948,
}

var coeff15kSharp96000 = []float64{
	3.14014081409305, -3.76888037179035,
	1.26107138314221, 1.26088059917107,
	-0.807698715053922, -0.80767075968406,
	1.0101984930848, -0.322351688402064,
}

// SharpPresetForSampleRate returns the sharp 15 kHz noise-shaping coefficients
// optimized for the given sample rate. The selection logic matches the legacy
// TDitherSharpNoiseShaper32.ChooseNoiseshaper implementation.
func SharpPresetForSampleRate(sampleRate float64) []float64 {
	var src []float64
	switch {
	case sampleRate < 41000:
		src = coeff15kSharp40000
	case sampleRate < 46000:
		src = coeff15kSharp44100
	case sampleRate < 55000:
		src = coeff15kSharp48000
	case sampleRate < 75100:
		src = coeff15kSharp64000
	default:
		src = coeff15kSharp96000
	}
	out := make([]float64, len(src))
	copy(out, src)
	return out
}

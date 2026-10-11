package cqt

import (
	"math"
	"math/cmplx"
	"slices"
	"testing"
)

// This file pins values of nnAudio's CQT2010v2 for the [BasicPitch] and
// [NNAudio] presets. They were extracted once from the golden fixtures of
// PLAN.md task 46.1 (scripts/fixtures/cqt/gen.py with nnAudio 0.3.4,
// torch 2.14.1, scipy 1.18.1, librosa 1.0.0 and numpy 2.5.3), which task
// 46.4 deleted; no Python runs any more. Unless marked "f32", a value is
// nnAudio's own code run in float64 (gen.py's "f64" reference: its
// float32/complex64 casts redirected to float64/complex128). The
// configurations are those of refConfigs, under the fixture names.
//
// Precision given up: 46.1 compared every frame and every bin of all eight
// configurations against nnAudio float64 and matched to 1.3e-13 in the
// per-bin metric of perBinError (and basic-pitch's configuration against
// nnAudio float32 to 1.3e-6). Now only the values below are compared with
// nnAudio. The full transform is checked against the naive oracle in
// oracle_test.go, which implements the same algorithm but is not an
// independent reference for nnAudio's conventions; these pinned values are.
// Bit-level parity with basic-pitch is checked downstream, in
// github.com/cwbudde/algo-transcribe, against basic-pitch's own CQT output.
//
// Output values depend on the input signal, which gen.py drew from numpy's
// default_rng and which testSignal does not reproduce. Frame 0 of a
// top-octave bin, however, depends only on the first few input samples: the
// kernel's support in the frame, with xp[i] = y[pad-i] for i < pad under
// reflect padding (zero under constant padding) and xp[pad+j] = y[j]
// (pad = nfft/2), where y is the input, or with early downsampling by f the
// input low-passed by the 256-tap early filter, y[j] = sum_m
// h[m]*x[f*j-127+m], so that y[j] reaches x[f*j+128]. So the first
// len(pinnedExcerpt) samples of gen.py's signal are pinned and followed by
// an arbitrary tail; pinnedFrame0Taps computes each pinned bin's dependency
// span (through the early low-pass where there is one) and the test fails if
// it reaches past the excerpt, and the pinned values must be bit-identical
// whether the tail is testSignal or zeros. Frame-0 values are pinned for
// basic_pitch (magnitude, float64 and float32), basic_pitch_short,
// nnaudio_defaults, hamming_nonorm_wrap, and const_l2_conv_complex (complex
// output: real and imaginary part, which anchors nnAudio's sign of the
// imaginary part, constant padding and early downsampling by 2).
//
// What is not pinned against nnAudio, and why:
//
//   - Output of blackman, early_ds8_complex and no_early_ds. Their cheapest
//     top-octave bins need 1449 input samples (blackman bin 71, early
//     downsampling by 8), 537 (early_ds8_complex bin 47) and 410
//     (no_early_ds bin 47, a 2048-sample kernel frame), beyond the excerpt;
//     the oracle covers them. const_l2_conv_complex pins its complex output
//     and early downsampling path.
//   - Lower octaves: after each 2:1 decimation a frame-0 value depends on
//     2j+128 samples of the octave above, which soon spans the whole signal.
//   - The short-signal fallback itself: basic_pitch_short's four lowest
//     octaves (125, 62, 31 and 15 samples, not longer than nfft/2 = 128) are
//     zero padded, and their values depend on all 4000 input samples. Pinned
//     are nnAudio's frame count (16), that it fell back (its warning), and
//     that the top octave is unaffected; the zero-pad mechanics are checked
//     by the oracle.
//   - nnAudio's shifted placement of a single partial octave. It differs
//     from the generic placement only for fewer bins than one octave (see
//     46.3 and [NNAudio]), and no fixture had such a configuration. The two
//     partial-octave fixtures (basic_pitch, 309 bins with 36 per octave, and
//     const_l2_conv_complex, 100 with 24) crop the lowest octave; their
//     top-octave kernels, pinned below at both octave edges, sit at their
//     bins. The shift is pinned by presets_test.go, not against nnAudio.

// Tolerances of the pinned comparisons, with the largest errors measured
// over all pinned values when they were pinned (go test -v -run Pinned logs
// them per configuration).
const (
	// pinnedTolFreq bounds Frequencies() relative to the value: math.Pow vs
	// numpy may differ in the last bit. Measured 1.8e-16 (46.1's limit).
	pinnedTolFreq = 1e-15
	// pinnedTolKernel bounds kernel samples relative to the largest kernel
	// sample of the configuration, as 46.1 did; the difference comes from
	// sin/cos, the window and the order of the norm's sum. Measured 3.6e-15
	// (hamming_nonorm_wrap; at most 1.4e-15 elsewhere).
	pinnedTolKernel = 1e-13
	// pinnedTolLowpass bounds low-pass taps relative to the largest tap
	// (46.1's limit). Measured 4.6e-17, early low-passes 2.5e-16.
	pinnedTolLowpass = 1e-15
	// pinnedTolOut bounds a frame-0 output value against nnAudio float64,
	// relative to the rounding scale of the dot product that produced it,
	// scale(b) * sum_i |k_i|*|y_i| (pinnedFrame0Taps). That sum bounds the
	// real and the imaginary part, and summation-order and libm differences
	// grow with it; unlike |value| it does not shrink when the kernel's
	// products cancel. Measured 7.3e-16 (3.5e-14 relative to |value|; the
	// complex const_l2_conv_complex values 8.6e-17 and 1.9e-14); 46.1's
	// per-bin limit was 1e-12.
	pinnedTolOut = 1e-12
	// pinnedTolF32 bounds ProcessInto and ProcessInto32 against nnAudio as
	// shipped in float32 (what basic-pitch computes), in the same scale.
	// The difference is nnAudio's own float32 rounding: measured 2.2e-8
	// (3.1e-7 relative to |value|). 1e-5 is 46.1's float32 limit.
	pinnedTolF32 = 1e-5
)

// pinnedExcerpt is x[0:393] of gen.py's make_signal(): 43844 samples at
// 22050 Hz of a 20 Hz-10 kHz exponential chirp (amplitude 0.5), tones at
// 440 Hz (0.3) and 55 Hz (0.2) and numpy default_rng(20261010) Gaussian noise
// (0.05), rounded to float32 (numpy 2.5.3); every configuration used a
// prefix of it. Its length is set by const_l2_conv_complex's bins 97-99,
// whose frame 0 reaches through the early low-pass to 393, 385 and 377
// samples (8 per bin lower). The first 128 samples cover frame 0 of every
// top-octave bin of basic_pitch (at most 108 samples) and nnaudio_defaults
// (89), and of hamming_nonorm_wrap's bins 54-59 (at most 124).
var pinnedExcerpt = [...]float32{
	// x[0:]
	0.049426984, 0.08076296, 0.10124381, 0.10721004, 0.1184121, 0.14303936, 0.40030774, 0.276173,
	0.26865608, 0.22906436, 0.40945446, 0.32591322, 0.3775308, 0.32408935, 0.40659457, 0.3856851,
	0.39065373, 0.4765986, 0.16377552, 0.34467018, 0.2772594, 0.23924318, 0.20245536, 0.19176698,
	0.11710139, 0.19119419, 0.17480585, 0.08002362, 0.11887917, 0.021080837, 0.00898263, -0.059681848,
	-0.003359816, -0.12244682, -0.15593964, -0.1371303, -0.010543875, -0.07953915, -0.021948887, -0.06393189,
	-0.0015072685, -0.038647402, -0.07087604, 0.09374846, 0.04915029, 0.110571206, 0.2061337, 0.2323855,
	0.2482245, 0.2657371, 0.27792582, 0.32111263, 0.4126487, 0.3874768, 0.5020851, 0.48265722,
	0.5447273, 0.62158203, 0.6530199, 0.65827376, 0.6616975, 0.551205, 0.646113, 0.58290255,
	// x[64:]
	0.6496593, 0.57480335, 0.62177366, 0.6636102, 0.6022739, 0.62057847, 0.5106624, 0.5713961,
	0.4973774, 0.42982224, 0.4248275, 0.31588292, 0.4011858, 0.40373015, 0.24979694, 0.31611258,
	0.22950177, 0.15110222, 0.19022867, 0.10531691, 0.19528945, 0.27282044, 0.20375891, 0.09585856,
	0.09572044, 0.13048656, 0.1677989, 0.15449257, 0.25371355, 0.232426, 0.26501447, 0.23201801,
	0.37596914, 0.39047834, 0.3548122, 0.4244322, 0.5302228, 0.493248, 0.479582, 0.65253955,
	0.69835764, 0.5826494, 0.57937455, 0.6799465, 0.8095601, 0.78952014, 0.791335, 0.7797678,
	0.8070983, 0.7577661, 0.7473414, 0.789524, 0.80851245, 0.7425086, 0.75821006, 0.8386588,
	0.7003959, 0.6253513, 0.6555699, 0.5936228, 0.6035037, 0.48837987, 0.46940714, 0.4204268,
	// x[128:]
	0.41715848, 0.42849085, 0.32961679, 0.25643903, 0.22810523, 0.23182735, 0.2557927, 0.24039689,
	0.2162312, 0.22778732, 0.16840434, 0.28518596, 0.24974057, 0.28164998, 0.17480654, 0.31011897,
	0.32871544, 0.29210576, 0.44996473, 0.4109154, 0.4658979, 0.4993688, 0.4476775, 0.5246132,
	0.62711847, 0.66373783, 0.6746062, 0.6931071, 0.720587, 0.74997604, 0.7456411, 0.8107512,
	0.8043365, 0.8310627, 0.8456519, 0.9302862, 0.7750033, 0.8706224, 0.8334363, 0.74982816,
	0.8192397, 0.8035993, 0.7235544, 0.64630944, 0.6361913, 0.59343404, 0.5693287, 0.5453723,
	0.49395156, 0.45509484, 0.45944047, 0.38568568, 0.37738466, 0.32074606, 0.20384087, 0.30047563,
	0.24405551, 0.18988718, 0.19227001, 0.2882194, 0.18626891, 0.17227079, 0.18099158, 0.22323772,
	// x[192:]
	0.26634455, 0.21254373, 0.20451738, 0.21959881, 0.324153, 0.2016282, 0.34630468, 0.3554041,
	0.37757853, 0.48424247, 0.48094457, 0.5328867, 0.5617546, 0.6304977, 0.58695745, 0.7348442,
	0.7516484, 0.5898411, 0.6796143, 0.6891574, 0.6967234, 0.64561534, 0.7676126, 0.70175123,
	0.7066897, 0.54216254, 0.712916, 0.6526737, 0.6774635, 0.5559602, 0.50335443, 0.3879372,
	0.52141666, 0.41097525, 0.3323189, 0.35182887, 0.2659413, 0.22783299, 0.11567883, 0.15585218,
	0.12828073, 0.18694684, 0.11029905, 0.0856029, 0.05110979, 0.06475259, 0.13055865, 0.22025178,
	0.06271975, -0.021444172, 0.18225776, 0.033431128, 0.16408055, 0.1163991, 0.23326917, 0.19495256,
	0.31210786, 0.29339874, 0.27582198, 0.38486665, 0.49053973, 0.4361955, 0.43188587, 0.57484305,
	// x[256:]
	0.59211916, 0.66523176, 0.56589806, 0.61039656, 0.63417965, 0.63224703, 0.6207908, 0.6260097,
	0.5863412, 0.60189617, 0.6033267, 0.5568483, 0.6131415, 0.57228726, 0.58676004, 0.5186698,
	0.44784948, 0.4922833, 0.38069594, 0.43076357, 0.2854107, 0.19260363, 0.24682641, 0.14650644,
	0.17064525, 0.1169601, 0.117753975, 0.10083359, 0.06677912, 0.028190587, 0.039061446, -0.046330605,
	0.076761104, -0.053894937, -0.023845721, 0.008046018, 0.01713597, 0.09191742, 0.0044378224, 0.16749196,
	0.16050127, 0.21057406, 0.24562763, 0.1372263, 0.24875225, 0.29994237, 0.262774, 0.26664093,
	0.41900635, 0.41874123, 0.45995897, 0.5264004, 0.55676186, 0.56719136, 0.5275254, 0.5635639,
	0.5139927, 0.5622984, 0.61210877, 0.66480726, 0.6260572, 0.5590792, 0.4706224, 0.52066225,
	// x[320:]
	0.52459085, 0.47998515, 0.42615852, 0.36892772, 0.39214528, 0.3798398, 0.32845742, 0.2851256,
	0.15087582, 0.19188, 0.13970266, 0.12276551, 0.06936356, 0.051411193, 0.08151802, 0.024457116,
	0.07617923, -0.057980843, 0.063171566, 0.0019369872, 0.10156446, -0.032150716, 0.034024525, -0.016023407,
	0.07378682, 0.13904816, 0.1616167, 0.12826596, 0.196605, 0.20530431, 0.3256804, 0.29299438,
	0.3427732, 0.34479973, 0.38413686, 0.50624657, 0.49776572, 0.47933957, 0.4661448, 0.57751733,
	0.57875043, 0.591205, 0.5897247, 0.5450776, 0.6215421, 0.5585606, 0.57568693, 0.5789087,
	0.5427663, 0.51969445, 0.49179736, 0.39909053, 0.47097585, 0.40937796, 0.43073556, 0.36947128,
	0.32846984, 0.3198339, 0.1738904, 0.18693964, 0.20416059, 0.23592296, 0.15352407, 0.07737875,
	// x[384:]
	0.007510102, 0.0906624, 0.14751613, 0.059103146, -0.020160299, 0.08332218, 0.11086613, 0.051886145,
	0.13653904,
}

// pinnedFreq is a bin's centre frequency (nnAudio's frequencies) and nominal
// kernel length (nnAudio's lenghts, which the librosa normalization uses).
type pinnedFreq struct {
	bin          int
	freq, length float64
}

// pinnedKernel is sample index of top-octave filter filter in its nfft frame
// (nnAudio's cqt_kernels_real/imag, before the minus sign of the imaginary
// convolution).
type pinnedKernel struct {
	filter, index int
	re, im        float64
}

// pinnedTap is tap index of a 256-tap low-pass. The filters are symmetric
// only up to rounding: scipy's taps i and 255-i differ by up to 2.4e-15
// (5.4e-15 of the peak tap), so only the pinned index is compared.
type pinnedTap struct {
	index int
	value float64
}

// pinnedBin is the magnitude of frame 0 of bin bin for the input
// pinnedExcerpt+tail, from nnAudio float64 and, where non-zero, nnAudio
// float32.
type pinnedBin struct {
	bin      int
	f64, f32 float64
}

// pinnedComplexBin is frame 0 of bin bin for the input pinnedExcerpt+tail
// with complex output, from nnAudio float64. nnAudio's imaginary part is
// minus the correlation with the kernel's imaginary part.
type pinnedComplexBin struct {
	bin    int
	re, im float64
}

// pinnedConfig holds the pinned values of one refConfigs configuration.
type pinnedConfig struct {
	name string // refConfigs name, the 46.1 fixture name

	// Setup: n_fft, n_octaves, downsample_factor, hop_length after early
	// downsampling, n_bins and the frame count at refConfig.length samples.
	nfft, octaves, factor, hop, bins, frames int
	// fallback reports whether nnAudio warned that it could not reflect an
	// octave and zero padded it.
	fallback bool

	freqs   []pinnedFreq
	kernels []pinnedKernel
	early   []pinnedTap // early low-pass, nil without early downsampling
	frame0  []pinnedBin // magnitude output
	// frame0Complex is frame 0 for complex output.
	frame0Complex []pinnedComplexBin
}

// pinnedLowpass are taps of the octave anti-alias filter, nnAudio
// create_lowpass_filter(band_center=0.5, kernelLength=256,
// transitionBandwidth=0.001), that is scipy 1.18.1
// firwin2(256, [0, 0.5/1.001, 0.5*1.001, 1], [1, 1, 0, 0]). It is the same in
// every configuration; dsp/filter/design pins all 256 taps.
var pinnedLowpass = []pinnedTap{
	{0, -0.00011111023799078378},
	{1, -0.00011278991187029311},
	{64, -0.00182535997782757},
	{120, -0.029753782927282417},
	{127, 0.4501417212098745},
}

// pinnedEarlyLowpass8 are taps of nnAudio's early-downsampling filter for a
// factor of 8, create_lowpass_filter(band_center=1/8, kernelLength=256,
// transitionBandwidth=0.03): scipy 1.18.1
// firwin2(256, [0, 0.125/1.03, 0.125*1.03, 1], [1, 1, 0, 0]) (fixtures
// early_ds8_complex and blackman).
var pinnedEarlyLowpass8 = []pinnedTap{
	{0, -2.8405735172232312e-05},
	{1, -8.665169852389669e-05},
	{64, -0.0004883518118586084},
	{120, 0.008181151597039641},
	{127, 0.12422193937106554},
}

// pinnedBasicPitchFrame0 is frame 0 of six top-octave bins of basic-pitch's
// front end (sr 22050, hop 256, fmin 27.5, 309 bins, 36 per octave, L1,
// Hann, reflect, librosa normalization, magnitude) on pinnedExcerpt: nnAudio
// 0.3.4 CQT2010v2 in float64 and as shipped in float32 (fixture basic_pitch,
// fields f64 and f32). basic_pitch_short (4000 samples) has bit-identical
// float64 values: its top octave is long enough to reflect.
var pinnedBasicPitchFrame0 = []pinnedBin{
	{273, 0.08050862578854291, 0.08050862699747086},
	{278, 0.1254014703963395, 0.12540146708488464},
	{285, 0.12064955823841185, 0.12064952403306961},
	{297, 0.11713959543609564, 0.11713960021734238},
	{306, 0.18760958869580213, 0.18760953843593597},
	{308, 0.10902459964220963, 0.10902462899684906},
}

// pinnedBasicPitchConfigs are the [BasicPitch] configurations: nnAudio 0.3.4
// CQT2010v2 (float64) with basic-pitch's parameters, at 22050 Hz.
var pinnedBasicPitchConfigs = []pinnedConfig{
	{
		// basic_pitch: BasicPitch() on 43844 samples. 309 bins are 8 octaves
		// plus a partial one of 21 bins, which nnAudio crops from the lowest
		// octave: the top octave's filter 0 sits at bin 273 = 309-36.
		name: "basic_pitch",
		nfft: 256, octaves: 9, factor: 1, hop: 256, bins: 309, frames: 172,
		freqs: []pinnedFreq{
			{0, 27.5, 41245},
			{272, 5173.465412810107, 220},
			{273, 5274.040910605918, 216},
			{308, 10346.930825620215, 110},
		},
		kernels: []pinnedKernel{
			{0, 74, 0.00399991727314714, 0.0023311225440812255},
			{0, 133, 0.0030693420366183183, 0.008683904741639133},
			{18, 89, 0.0023011173609260084, -0.0060457287313778404},
			{18, 133, -0.004653021445026518, -0.012037909916872933},
			{35, 101, -0.0045195026780319, 0.00818572687290718},
			{35, 133, -0.010127456413045728, 0.014654623549101298},
		},
		frame0: pinnedBasicPitchFrame0,
	},
	{
		// basic_pitch_short: BasicPitch() on 4000 samples. nnAudio warned and
		// zero padded the octaves too short to reflect; the top octave is not
		// affected (float64 only; there was no float32 run).
		name: "basic_pitch_short",
		nfft: 256, octaves: 9, factor: 1, hop: 256, bins: 309, frames: 16, fallback: true,
		frame0: pinnedF64Only(pinnedBasicPitchFrame0),
	},
	{
		// blackman: BasicPitch() with a Blackman window, 72 bins from 110 Hz.
		// nnAudio ignores its window argument, so gen.py rebuilt the kernels
		// with nnAudio's create_cqt_kernels(window="blackman"). Its top bin
		// lets nnAudio downsample early by 8.
		name: "blackman",
		nfft: 1024, octaves: 2, factor: 8, hop: 32, bins: 72, frames: 172,
		freqs: []pinnedFreq{
			{0, 110, 1289},
			{71, 431.6092385743226, 329},
		},
		kernels: []pinnedKernel{
			{0, 350, 0.001133602443039692, 0.0005281160061441017},
			{0, 517, -0.002970492293701883, 0.0021841619198368213},
			{35, 429, 0.002442934177364521, 4.27916943752005e-05},
			{35, 517, 0.0014815211203714597, -0.007050286895956569},
		},
		early: pinnedEarlyLowpass8,
	},
}

// pinnedNNAudioConfigs are the [NNAudio] configurations: nnAudio 0.3.4
// CQT2010v2 (float64) with nnAudio's defaults and the options of refConfigs.
var pinnedNNAudioConfigs = []pinnedConfig{
	{
		// nnaudio_defaults: NNAudio() at 22050 Hz on 43844 samples (sr 22050,
		// hop 512, fmin 32.70, 84 bins, 12 per octave; early downsampling
		// evaluates to a factor of 1).
		name: "nnaudio_defaults",
		nfft: 256, octaves: 7, factor: 1, hop: 512, bins: 84, frames: 86,
		freqs: []pinnedFreq{
			{0, 32.7, 11341},
			{72, 2092.8, 178},
			{83, 3950.680323160497, 94},
		},
		kernels: []pinnedKernel{
			{0, 84, 0.0025599680885375516, -0.0051119562270428},
			{0, 133, -0.011006534930242217, 0.001774624081615337},
			{11, 105, 0.007971859201121529, -0.007570507204705609},
			{11, 133, 0.01641386199542117, -0.012592743871319886},
		},
		frame0: []pinnedBin{
			{72, 0.18607008560792274, 0},
			{78, 0.06640615141781889, 0},
			{81, 0.0538616178433066, 0},
			{83, 0.08631159188572257, 0},
		},
	},
	{
		// early_ds8_complex: hop 1024, fmin 30, 48 bins, complex output;
		// nnAudio downsamples early by 8.
		name: "early_ds8_complex",
		nfft: 256, octaves: 4, factor: 8, hop: 128, bins: 48, frames: 43,
		freqs: []pinnedFreq{
			{0, 30, 1546},
			{47, 453.05967008721285, 103},
		},
		kernels: []pinnedKernel{
			{0, 80, 0.002242425791245266, -0.004733847092786959},
			{0, 133, -0.009409031011874344, 0.004045416600674187},
		},
		early: pinnedEarlyLowpass8,
	},
	{
		// no_early_ds: the same with earlydownsample=False: 8 times longer
		// kernels at the full rate.
		name: "no_early_ds",
		nfft: 2048, octaves: 4, factor: 1, hop: 1024, bins: 48, frames: 43,
		freqs: []pinnedFreq{
			{0, 30, 12361},
			{47, 453.05967008721285, 819},
		},
		kernels: []pinnedKernel{
			{0, 638, 0.00019501102791675355, -0.0006181120948004808},
			{0, 1029, 0.0012186390471845539, 0.00043374221358458533},
		},
	},
	{
		// const_l2_conv_complex: sr 16000, hop 128, fmin 50, 100 bins with 24
		// per octave (4 octaves plus a partial one of 4 bins, cropped from the
		// lowest octave: filter 0 sits at bin 76), filter scale 0.8, L2,
		// constant padding, convolutional normalization, complex output, 20000
		// samples; nnAudio downsamples early by 2.
		name: "const_l2_conv_complex",
		nfft: 512, octaves: 5, factor: 2, hop: 64, bins: 100, frames: 157,
		freqs: []pinnedFreq{
			{0, 50, 4369},
			{75, 436.20309306610307, 501},
			{76, 448.9848193237491, 487},
			{99, 872.4061861322061, 251},
		},
		kernels: []pinnedKernel{
			{0, 134, 0.02125126122674744, 0.03043278379033003},
			{0, 261, -0.01412904309979632, 0.07254171358851373},
			{23, 193, 0.035547327444832064, 0.0377594710157744},
			{23, 261, -0.0984668429671861, -0.028777661959287194},
		},
		// nnAudio create_lowpass_filter(band_center=1/2, kernelLength=256,
		// transitionBandwidth=0.03): scipy 1.18.1
		// firwin2(256, [0, 0.5/1.03, 0.5*1.03, 1], [1, 1, 0, 0]).
		early: []pinnedTap{
			{0, 5.234906940160696e-06},
			{1, 9.896465841927282e-06},
			{64, -0.00010820746323494297},
			{120, -0.029038487361515285},
			{127, 0.4502449009790021},
		},
		// nnAudio 0.3.4 CQT2010v2, float64, output_format="Complex", frame 0 of
		// the three highest bins on pinnedExcerpt (fixture const_l2_conv_complex,
		// field f64).
		frame0Complex: []pinnedComplexBin{
			{97, -0.048663195079298636, -0.024840440185708225},
			{98, -0.05084087467272096, -0.015426473292415165},
			{99, -0.05044163474270972, -0.012958689836392208},
		},
	},
	{
		// hamming_nonorm_wrap: hop 64, fmin 100, 60 bins with 12 per octave,
		// filter scale 1.5, no basis norm, wrap normalization, 10000
		// samples; Hamming kernels rebuilt with nnAudio's
		// create_cqt_kernels(window="hamming").
		name: "hamming_nonorm_wrap",
		nfft: 512, octaves: 5, factor: 1, hop: 64, bins: 60, frames: 157,
		freqs: []pinnedFreq{
			{0, 100, 5563},
			{59, 3020.3978005814197, 185},
		},
		kernels: []pinnedKernel{
			{0, 168, -0.0011491539731501026, -0.001006876193505772},
			{0, 261, -0.0018670033422262674, 0.0021773282670556906},
			{11, 209, -0.0026809448967525765, -0.0010999179499953556},
			{11, 261, -0.002132736916163719, -0.004919765449934948},
		},
		frame0: []pinnedBin{
			{54, 0.0020363966308568375, 0},
			{56, 0.0024082667950199293, 0},
			{58, 0.004686843534823661, 0},
			{59, 0.003532592923379963, 0},
		},
	},
}

// pinnedF64Only returns bins without their float32 values.
func pinnedF64Only(bins []pinnedBin) []pinnedBin {
	out := slices.Clone(bins)
	for i := range out {
		out[i].f32 = 0
	}

	return out
}

// TestPinnedBasicPitch compares the BasicPitch() configurations with values
// pinned from nnAudio.
func TestPinnedBasicPitch(t *testing.T) {
	t.Parallel()

	for _, pc := range pinnedBasicPitchConfigs {
		t.Run(pc.name, func(t *testing.T) {
			t.Parallel()
			pinnedCheck(t, pc)
		})
	}
}

// TestPinnedNNAudio compares the NNAudio() configurations with values pinned
// from nnAudio.
func TestPinnedNNAudio(t *testing.T) {
	t.Parallel()

	for _, pc := range pinnedNNAudioConfigs {
		t.Run(pc.name, func(t *testing.T) {
			t.Parallel()
			pinnedCheck(t, pc)
		})
	}
}

// TestPinnedCoversRefConfigs checks that every reference configuration has
// pinned values.
func TestPinnedCoversRefConfigs(t *testing.T) {
	t.Parallel()

	var names []string
	for _, pc := range slices.Concat(pinnedBasicPitchConfigs, pinnedNNAudioConfigs) {
		names = append(names, pc.name)
	}

	for _, rc := range refConfigs() {
		if !slices.Contains(names, rc.name) {
			t.Errorf("reference configuration %s has no pinned values", rc.name)
		}
	}
}

func pinnedRefConfig(t *testing.T, name string) refConfig {
	t.Helper()

	for _, rc := range refConfigs() {
		if rc.name == name {
			return rc
		}
	}

	t.Fatalf("no reference configuration %q", name)

	return refConfig{}
}

func pinnedCheck(t *testing.T, pc pinnedConfig) {
	t.Helper()

	rc := pinnedRefConfig(t, pc.name)

	tr, err := New(rc.sr, rc.opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	pinnedCheckSetup(t, tr, rc, pc)
	pinnedCheckTaps(t, "Lowpass", tr.Lowpass(), pinnedLowpass)

	if pc.early == nil {
		if tr.EarlyLowpass() != nil {
			t.Errorf("EarlyLowpass: %d taps, want none", len(tr.EarlyLowpass()))
		}
	} else {
		pinnedCheckTaps(t, "EarlyLowpass", tr.EarlyLowpass(), pc.early)
	}

	pinnedCheckKernels(t, tr, pc.kernels)

	if pc.frame0 != nil || pc.frame0Complex != nil {
		pinnedCheckFrame0(t, tr, rc.length, pc.frame0, pc.frame0Complex)
	}
}

func pinnedCheckSetup(t *testing.T, tr *Transform, rc refConfig, pc pinnedConfig) {
	t.Helper()

	got := [...]int{tr.NFFT(), tr.Octaves(), tr.DownsampleFactor(), tr.Hop(), tr.Bins(), tr.FrameCount(rc.length)}
	want := [...]int{pc.nfft, pc.octaves, pc.factor, pc.hop, pc.bins, pc.frames}

	if got != want {
		t.Fatalf("NFFT/Octaves/DownsampleFactor/Hop/Bins/FrameCount(%d) = %v, want %v", rc.length, got, want)
	}

	if want := rc.sr / float64(pc.factor); tr.SampleRate() != want {
		t.Errorf("SampleRate = %g, want %g", tr.SampleRate(), want)
	}

	// nnAudio falls back exactly when the quirk-free transform, which
	// rejects octaves too short to reflect, cannot process the signal.
	generic, err := New(rc.sr, append(slices.Clone(rc.opts), func(c *config) error {
		c.nnAudioShortOctaves = false

		return nil
	})...)
	if err != nil {
		t.Fatalf("New without the short-octave quirk: %v", err)
	}

	if fallback := generic.FrameCount(rc.length) == 0; fallback != pc.fallback || rc.fallback != pc.fallback {
		t.Errorf("fallback: lowest octave too short to reflect = %v, refConfig %v, nnAudio warned = %v",
			fallback, rc.fallback, pc.fallback)
	}

	freqs, lengths := tr.Frequencies(), tr.Lengths()
	worst := 0.0

	for _, p := range pc.freqs {
		e := math.Abs(freqs[p.bin]-p.freq) / p.freq
		worst = max(worst, e)

		if !(e <= pinnedTolFreq) {
			t.Errorf("Frequencies()[%d] = %.17g, want %.17g (rel. error %.2e)", p.bin, freqs[p.bin], p.freq, e)
		}

		if lengths[p.bin] != p.length {
			t.Errorf("Lengths()[%d] = %g, want %g", p.bin, lengths[p.bin], p.length)
		}
	}

	t.Logf("Frequencies: max rel. error %.2e", worst)
}

func pinnedCheckTaps(t *testing.T, name string, got []float64, want []pinnedTap) {
	t.Helper()

	if len(got) != lowpassTaps {
		t.Fatalf("%s: %d taps, want %d", name, len(got), lowpassTaps)
	}

	peak := 0.0
	for _, v := range got {
		peak = max(peak, math.Abs(v))
	}

	worst := 0.0

	for _, p := range want {
		e := math.Abs(got[p.index]-p.value) / peak
		worst = max(worst, e)

		if !(e <= pinnedTolLowpass) {
			t.Errorf("%s[%d] = %.17g, want %.17g (error %.2e of the peak)", name, p.index, got[p.index], p.value, e)
		}
	}

	t.Logf("%s: max error %.2e of the peak tap", name, worst)
}

func pinnedCheckKernels(t *testing.T, tr *Transform, want []pinnedKernel) {
	t.Helper()

	kernels := tr.Kernels()
	peak := 0.0

	for _, row := range kernels {
		for _, v := range row {
			peak = max(peak, math.Abs(real(v)), math.Abs(imag(v)))
		}
	}

	worst := 0.0

	for _, p := range want {
		v := kernels[p.filter][p.index]
		e := max(math.Abs(real(v)-p.re), math.Abs(imag(v)-p.im)) / peak
		worst = max(worst, e)

		if !(e <= pinnedTolKernel) {
			t.Errorf("Kernels()[%d][%d] = %.17g, want (%.17g%+.17gi) (error %.2e of the peak)",
				p.filter, p.index, v, p.re, p.im, e)
		}
	}

	t.Logf("Kernels: max error %.2e of the peak sample", worst)
}

// pinnedInput returns n samples: pinnedExcerpt followed by tail[K:n] with
// K = len(pinnedExcerpt), or by zeros if tail is nil.
func pinnedInput(n int, tail []float64) []float64 {
	x := make([]float64, n)
	if tail != nil {
		copy(x, tail[:n])
	}

	for i, v := range pinnedExcerpt {
		x[i] = float64(v)
	}

	return x
}

// pinnedFrame0Taps returns the number of leading samples of x that frame 0
// of top-octave bin b depends on, and the rounding scale of that value,
// scale(b) * sum_i |k_i|*|y_i| over the kernel's support. y is the top
// octave's signal: x itself, or with early downsampling by f the low-passed
// y[j] = sum_m h[m]*x[f*j-127+m] (zero outside x), for which |y_j| is
// replaced by its own rounding scale sum_m |h[m]|*|x[f*j-127+m]|.
func pinnedFrame0Taps(t *testing.T, tr *Transform, x []float64, b int) (int, float64) {
	t.Helper()

	kernels := tr.Kernels()

	f := b - (tr.Bins() - len(kernels))
	if f < 0 {
		t.Fatalf("bin %d is not in the top octave (bins %d-%d)", b, tr.Bins()-len(kernels), tr.Bins()-1)
	}

	factor, h := tr.DownsampleFactor(), tr.EarlyLowpass()
	span := 0

	// absY returns the rounding scale of y[j] and extends span by the input
	// samples it reads.
	absY := func(j int) float64 {
		if factor == 1 {
			span = max(span, j+1)

			return math.Abs(x[j])
		}

		s := 0.0

		for m, hm := range h {
			i := factor*j - (len(h)-1)/2 + m
			if i < 0 || hm == 0 {
				continue
			}

			span = max(span, i+1)
			s += math.Abs(hm) * math.Abs(x[i])
		}

		return s
	}

	pad := tr.NFFT() / 2
	sum := 0.0

	for i, k := range kernels[f] {
		if k == 0 {
			continue
		}

		j := i - pad // frame 0 reads xp[i] = y[i-pad] ...

		if i < pad {
			if tr.Padding() != PadReflect {
				continue // ... or a padding zero
			}

			j = pad - i // ... or the reflection y[pad-i]
		}

		sum += cmplx.Abs(k) * absY(j)
	}

	return span, tr.scale[b] * sum
}

// pinnedValue is one pinned frame-0 output value at index idx of the
// output; abs is the bin's magnitude, for the relative error that is logged.
type pinnedValue struct {
	bin, idx      int
	part          string
	f64, f32, abs float64
}

func pinnedFrame0Values(magnitudes []pinnedBin, complexBins []pinnedComplexBin) []pinnedValue {
	var vals []pinnedValue

	for _, p := range magnitudes {
		vals = append(vals, pinnedValue{p.bin, p.bin, "magnitude", p.f64, p.f32, p.f64})
	}

	for _, p := range complexBins {
		abs := math.Hypot(p.re, p.im)
		vals = append(vals,
			pinnedValue{p.bin, 2 * p.bin, "real part", p.re, 0, abs},
			pinnedValue{p.bin, 2*p.bin + 1, "imaginary part", p.im, 0, abs})
	}

	return vals
}

func pinnedCheckFrame0(t *testing.T, tr *Transform, n int, magnitudes []pinnedBin, complexBins []pinnedComplexBin) {
	t.Helper()

	if (tr.output == OutputComplex) != (complexBins != nil) || (magnitudes != nil) == (complexBins != nil) {
		t.Fatalf("output %v: pinned %d magnitudes and %d complex values", tr.output, len(magnitudes), len(complexBins))
	}

	x := pinnedInput(n, testSignal(t))

	got, err := tr.Process(x)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// The pinned values must not depend on the samples after the excerpt.
	gotZeros, err := tr.Process(pinnedInput(n, nil))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	var got32 []float32

	if magnitudes != nil && magnitudes[0].f32 != 0 {
		x32 := make([]float32, n)
		for i, v := range x {
			x32[i] = float32(v) // exact: both parts of x are float32 values
		}

		got32 = make([]float32, tr.OutputLen(n))

		err = tr.ProcessInto32(got32, x32)
		if err != nil {
			t.Fatalf("ProcessInto32: %v", err)
		}
	}

	worst, worstRel, worst32, worst32Rel := 0.0, 0.0, 0.0, 0.0
	maxSpan := 0

	for _, p := range pinnedFrame0Values(magnitudes, complexBins) {
		span, scale := pinnedFrame0Taps(t, tr, x, p.bin)
		maxSpan = max(maxSpan, span)

		if span > len(pinnedExcerpt) {
			t.Fatalf("bin %d: frame 0 depends on %d input samples, the excerpt has %d", p.bin, span, len(pinnedExcerpt))
		}

		v := got[p.idx]
		if v != gotZeros[p.idx] {
			t.Errorf("bin %d %s: frame 0 = %.17g with the testSignal tail, %.17g with zeros", p.bin, p.part, v, gotZeros[p.idx])
		}

		e := math.Abs(v-p.f64) / scale
		worst = max(worst, e)
		worstRel = max(worstRel, math.Abs(v-p.f64)/p.abs)

		if !(e <= pinnedTolOut) {
			t.Errorf("bin %d %s: frame 0 = %.17g, want %.17g (nnAudio float64; error %.2e of scale %.3e)",
				p.bin, p.part, v, p.f64, e, scale)
		}

		if got32 == nil {
			continue
		}

		for name, v := range map[string]float64{"ProcessInto": v, "ProcessInto32": float64(got32[p.idx])} {
			e := math.Abs(v-p.f32) / scale
			worst32 = max(worst32, e)
			worst32Rel = max(worst32Rel, math.Abs(v-p.f32)/p.abs)

			if !(e <= pinnedTolF32) {
				t.Errorf("bin %d: %s frame 0 = %.9g, want %.9g (nnAudio float32; error %.2e of scale %.3e)",
					p.bin, name, v, p.f32, e, scale)
			}
		}
	}

	t.Logf("frame 0: the pinned bins depend on the first %d input samples", maxSpan)
	t.Logf("frame 0: max error %.2e of the dot-product scale (%.2e of |value|) vs nnAudio float64", worst, worstRel)

	if got32 != nil {
		t.Logf("frame 0: max error %.2e of the dot-product scale (%.2e of |value|) vs nnAudio float32", worst32, worst32Rel)
	}
}

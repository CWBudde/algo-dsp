package loudness_test

import (
	"errors"
	"fmt"
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/loudness"
)

// Sources: EBU Tech3341v2.0, Table1, integrated tests1–6:
// https://tech.ebu.ch/files/live/sites/tech/files/shared/tech/tech3341v2_0.pdf
// ITU-R BS.1770-5, Annex1 Tables1/2 and the gating equations:
// https://www.itu.int/dms_pubrec/itu-r/rec/bs/R-REC-BS.1770-5-202311-I!!PDF-E.pdf
// These are synthesized mathematical test signals, not copies of EBU WAVs.
// Passing them tests integrated analysis only, not legacy Meter, LRA or true peak.
func TestIntegratedEBUTech3341CasesOneThroughSix(t *testing.T) {
	type segment struct {
		seconds float64
		levels  []float64
	}

	stereo := func(seconds, level float64) segment { return segment{seconds, []float64{level, level}} }
	for _, tt := range []struct {
		name     string
		segments []segment
		weights  []float64
		want     float64
	}{
		{"case1_stereo", []segment{stereo(20, -23)}, []float64{1, 1}, -23},
		{"case2_attenuated", []segment{stereo(20, -33)}, []float64{1, 1}, -33},
		{"case3_relative_gate", []segment{stereo(10, -36), stereo(60, -23), stereo(10, -36)}, []float64{1, 1}, -23},
		{"case4_absolute_and_relative_gates", []segment{stereo(10, -72), stereo(10, -36), stereo(60, -23), stereo(10, -36), stereo(10, -72)}, []float64{1, 1}, -23},
		{"case5_relative_gate_average", []segment{stereo(20, -26), stereo(20.1, -20), stereo(20, -26)}, []float64{1, 1}, -23},
		// Explicit L/R/C/Ls/Rs ordering. No inferred speaker layout or LFE.
		{"case6_five_channels", []segment{{20, []float64{-28, -28, -24, -30, -30}}}, []float64{1, 1, 1, 1.41, 1.41}, -23},
	} {
		t.Run(tt.name, func(t *testing.T) {
			const rate = 48000

			ends := make([]int64, len(tt.segments))
			amplitudes := make([][]float64, len(tt.segments))

			var frames int64
			for i, s := range tt.segments {
				frames += int64(math.Round(s.seconds * rate))
				ends[i] = frames

				amplitudes[i] = make([]float64, len(s.levels))
				for channel, level := range s.levels {
					amplitudes[i][channel] = math.Pow(10, level/20)
				}
			}
			// A 48-sample period avoids millions of trig calls, without keeping
			// any duration-sized sample buffer. Phase continues across segments.
			var sine [48]float64
			for i := range sine {
				sine[i] = math.Sin(2 * math.Pi * float64(i) / 48)
			}

			source := func(channel int, frame int64) float64 {
				segmentIndex := 0
				for frame >= ends[segmentIndex] {
					segmentIndex++
				}

				return amplitudes[segmentIndex][channel] * sine[frame%48]
			}

			got := conformanceAnalyze(t, loudness.IntegratedConfig{SampleRate: rate, Channels: len(tt.weights), ChannelWeights: tt.weights, MaxFrames: frames}, frames, []int{4096, 997, 65536, 17}, source, false)
			if math.Abs(got.LUFS-tt.want) > 0.1 {
				t.Fatalf("integrated %.12f LUFS, EBU expected %.1f ±0.1", got.LUFS, tt.want)
			}

			if got.Frames != frames {
				t.Fatalf("frames %d, want %d", got.Frames, frames)
			}
		})
	}
}

func TestIntegratedIndependent48kFilterGateGolden(t *testing.T) {
	const frames int64 = 57600
	// A checked-in result derived offline using an independent ECMAScript
	// direct-form-I implementation of the published coefficients and prefix
	// sums over complete400ms windows, with separate absolute/relative gates.
	// Powers: .023877391081946017,.017913173122416984,.011946571849979059,
	// .005972325623804198,.0000004355472448776728,.023906480607466727,
	// .04778098057079046,.07166050079457252,.09554972235611228.
	// The central near-silent gate is rejected; transitions remain represented.
	source := func(channel int, frame int64) float64 {
		amplitude := 1.0
		if frame >= 38400 {
			amplitude = 2
		} else if frame >= 19200 {
			amplitude = 0.0001
		}

		if channel == 0 {
			return float64(frame%31-15) / 128 * amplitude
		}

		return float64(frame%17-8) / 64 * amplitude
	}
	config := loudness.IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: frames}
	got := conformanceAnalyze(t, config, frames, []int{13, 4096, 257, 65536}, source, false)

	const goldenLUFS = -14.970897903199129
	conformanceNear(t, "static LUFS", got.LUFS, goldenLUFS, 1e-9)
	conformanceNear(t, "static peak", got.SamplePeak, 0.25, 0)

	want := conformanceReference(config, frames, source)
	conformanceNear(t, "independent DF1 LUFS", got.LUFS, want.LUFS, 1e-9)
}

func TestIntegratedIndependentRateMappingAndCalibration(t *testing.T) {
	for _, rate := range []int{8000, 44100, 48000, 96000, 192000, 384000} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			frames := int64(rate * 6 / 5)
			config := loudness.IntegratedConfig{SampleRate: float64(rate), Channels: 2, ChannelWeights: []float64{1, 1}, MaxFrames: frames}
			amplitude := math.Pow(10, -23.0/20)
			source := func(_ int, frame int64) float64 {
				return amplitude * math.Sin(2*math.Pi*997*float64(frame)/float64(rate))
			}
			got := conformanceAnalyze(t, config, frames, []int{1, 509, 65536, 31}, source, false)
			want := conformanceReference(config, frames, source)
			conformanceNear(t, "rate DF1", got.LUFS, want.LUFS, 1e-9)
			// Independently evaluate the transfer function at997Hz. Stereo
			// power is A²|H|²; startup/window leakage permits .015 LU here.
			shelf, hpf := conformanceCoefficients(float64(rate))
			z := cmplx.Exp(complex(0, -2*math.Pi*997/float64(rate)))
			response := conformanceResponse(shelf, z) * conformanceResponse(hpf, z)
			steady := -0.691 + 10*math.Log10(amplitude*amplitude*cmplx.Abs(response)*cmplx.Abs(response))
			conformanceNear(t, "analytic calibration", got.LUFS, steady, 0.015)
		})
	}
}

func TestIntegratedIrregularPartitionsAndFloat32Reference(t *testing.T) {
	const frames int64 = 62413

	source := func(channel int, frame int64) float64 {
		value := float64((frame*37+int64(channel)*13)%211-105) / 256
		if frame > 24000 && frame < 39000 {
			value *= 0.001
		}

		return value
	}
	config := loudness.IntegratedConfig{SampleRate: 48000, Channels: 3, ChannelWeights: []float64{1, 0.5, 1.41}, MaxFrames: frames}
	whole := conformanceAnalyze(t, config, frames, []int{65536}, source, false)

	irregular := conformanceAnalyze(t, config, frames, []int{1, 7, 4799, 31, 65536, 3}, source, false)
	if math.Float64bits(whole.LUFS) != math.Float64bits(irregular.LUFS) || whole.SamplePeak != irregular.SamplePeak || whole.Frames != irregular.Frames {
		t.Fatal("same-architecture chunk partition changed result", whole, irregular)
	}

	float32Result := conformanceAnalyze(t, config, frames, []int{17, 8191, 3, 65536}, source, true)
	rounded := func(channel int, frame int64) float64 { return float64(float32(source(channel, frame))) }
	want := conformanceReference(config, frames, rounded)
	conformanceNear(t, "float32 DF1", float32Result.LUFS, want.LUFS, 1e-9)
	conformanceNear(t, "float32 peak", float32Result.SamplePeak, want.SamplePeak, 0)
}

func TestIntegratedIndependentFractionalNominalEndpoints(t *testing.T) {
	for _, rate := range []float64{8005, 11025, 48000.5} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			frames := int64(math.Round(rate))
			config := loudness.IntegratedConfig{SampleRate: rate, Channels: 2, MaxFrames: frames}
			source := func(channel int, frame int64) float64 {
				// Keep all seven windows above the absolute/relative gates.
				// A time-varying envelope exposes differing window endpoints.
				amplitude := 0.1 + 0.07*float64(frame)/rate
				return amplitude * math.Sin(2*math.Pi*(997+113*float64(channel))*float64(frame)/rate)
			}

			whole, calls := conformanceAnalyzeBudget(t, config, frames, []int{65536}, source, false, 1)
			if calls != 7 {
				t.Fatalf("one second must contain seven complete nominal windows, got %d", calls)
			}

			irregular, irregularCalls := conformanceAnalyzeBudget(t, config, frames, []int{1, 13, 799, 3, 4096, 17}, source, false, 1)
			if irregularCalls != 7 || math.Float64bits(whole.LUFS) != math.Float64bits(irregular.LUFS) || whole.SamplePeak != irregular.SamplePeak || whole.Frames != irregular.Frames {
				t.Fatal("fractional-rate partition changed result", whole, irregular, irregularCalls)
			}

			want := conformanceReference(config, frames, source)
			conformanceNear(t, "nearest nominal endpoint DF1", whole.LUFS, want.LUFS, 1e-9)
			conformanceNear(t, "nearest nominal endpoint peak", whole.SamplePeak, want.SamplePeak, 0)
		})
	}
}

func TestIntegratedComplete400msAndEOFTail(t *testing.T) {
	const window int64 = 19200

	source := func(_ int, frame int64) float64 {
		if frame >= window {
			return 7
		} // Tail affects peak, not a new complete gate.

		return 0.2 * math.Sin(2*math.Pi*1000*float64(frame)/48000)
	}

	short, err := loudness.NewIntegratedAnalyzer(loudness.IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: window - 1})
	if err != nil {
		t.Fatal(err)
	}

	input := make([]float64, window-1)
	for i := range input {
		input[i] = source(0, int64(i))
	}

	if err := short.ProcessPlanar([][]float64{input}); err != nil {
		t.Fatal(err)
	}

	if _, err := short.FinishStep(1); !errors.Is(err, loudness.ErrTooShort) {
		t.Fatalf("399.98ms not too short: %v", err)
	}

	var first loudness.IntegratedResult

	for _, frames := range []int64{window, window + 1, window + 4799} {
		config := loudness.IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: frames}
		got := conformanceAnalyze(t, config, frames, []int{4093, 11}, source, false)
		want := conformanceReference(config, frames, source)
		conformanceNear(t, "complete gate", got.LUFS, want.LUFS, 1e-9)

		if frames == window {
			first = got
		} else {
			conformanceNear(t, "discarded tail LUFS", got.LUFS, first.LUFS, 0)
			conformanceNear(t, "tail peak", got.SamplePeak, 7, 0)
		}

		if got.Frames != frames {
			t.Fatalf("discarded tail lost source frames: %d", got.Frames)
		}
	}
}

func TestIntegratedExplicitWeightsZeroChannelAndLFEPeak(t *testing.T) {
	const frames int64 = 48000

	config := loudness.IntegratedConfig{SampleRate: 48000, Channels: 4, ChannelWeights: []float64{1, 0, 1.41, 0.5}, MaxFrames: frames}
	source := func(channel int, frame int64) float64 {
		switch channel {
		case 0:
			return 0.1 * math.Sin(2*math.Pi*997*float64(frame)/48000)
		case 1:
			if frame%2 == 0 {
				return 2
			}

			return -2 // Explicit excluded LFE.
		case 2:
			return 0.05 * math.Sin(2*math.Pi*733*float64(frame)/48000)
		default:
			return 0 // A positive-weight, silent channel is still present.
		}
	}
	got := conformanceAnalyze(t, config, frames, []int{1021, 17, 8192}, source, false)
	want := conformanceReference(config, frames, source)
	conformanceNear(t, "explicit weights", got.LUFS, want.LUFS, 1e-9)
	conformanceNear(t, "LFE source peak", got.SamplePeak, 2, 0)

	withoutLFE := func(channel int, frame int64) float64 {
		if channel == 1 {
			return 0
		}

		return source(channel, frame)
	}
	quiet := conformanceAnalyze(t, config, frames, []int{1021, 17, 8192}, withoutLFE, false)
	conformanceNear(t, "LFE excluded energy", got.LUFS, quiet.LUFS, 0)
}

// conformanceAnalyze owns only a bounded reusable planar buffer. Input is
// checked after each Process call; no duration-sized samples are retained.
func conformanceAnalyze(t *testing.T, config loudness.IntegratedConfig, frames int64, chunks []int, source func(int, int64) float64, asFloat32 bool) loudness.IntegratedResult {
	t.Helper()
	result, _ := conformanceAnalyzeBudget(t, config, frames, chunks, source, asFloat32, 3)

	return result
}

func conformanceAnalyzeBudget(t *testing.T, config loudness.IntegratedConfig, frames int64, chunks []int, source func(int, int64) float64, asFloat32 bool, finishBudget int) (loudness.IntegratedResult, int) {
	t.Helper()

	analyzer, err := loudness.NewIntegratedAnalyzer(config)
	if err != nil {
		t.Fatal(err)
	}

	capacity := 0
	for _, chunk := range chunks {
		capacity = max(capacity, chunk)
	}

	planar := make([][]float64, config.Channels)

	planar32 := make([][]float32, config.Channels)
	for channel := range planar {
		planar[channel] = make([]float64, capacity)
		if asFloat32 {
			planar32[channel] = make([]float32, capacity)
		}
	}

	chunkIndex := 0
	for start := int64(0); start < frames; {
		count := int(min(int64(chunks[chunkIndex%len(chunks)]), frames-start))
		chunkIndex++

		for channel := range planar {
			planar[channel] = planar[channel][:count]
			if asFloat32 {
				planar32[channel] = planar32[channel][:count]
			}

			for frame := range count {
				value := source(channel, start+int64(frame))

				planar[channel][frame] = value
				if asFloat32 {
					planar32[channel][frame] = float32(value)
				}
			}
		}

		if asFloat32 {
			err = analyzer.ProcessPlanar32(planar32)
		} else {
			err = analyzer.ProcessPlanar(planar)
		}

		if err != nil {
			t.Fatal(err)
		}

		for channel := range planar {
			for frame := range count {
				want := source(channel, start+int64(frame))
				if asFloat32 {
					if math.Float32bits(planar32[channel][frame]) != math.Float32bits(float32(want)) {
						t.Fatal("analyzer mutated float32 input")
					}
				} else if math.Float64bits(planar[channel][frame]) != math.Float64bits(want) {
					t.Fatal("analyzer mutated float64 input")
				}
			}
		}

		start += int64(count)
	}

	finishCalls := 0
	for {
		finishCalls++

		done, err := analyzer.FinishStep(finishBudget)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			break
		}

		if int64(finishCalls) > frames/10+100 {
			t.Fatal("bounded finalization made no progress")
		}
	}

	result, err := analyzer.Result()
	if err != nil {
		t.Fatal(err)
	}

	return result, finishCalls
}

// This oracle intentionally does not call a production filter/designer, meter,
// analyzer, gain helper or shared gating routine. Prefix sums independently
// measure full rectangular windows; only these small reference fixtures retain
// per-frame powers. The long EBU signals above remain fully block-streaming.
func conformanceReference(config loudness.IntegratedConfig, frames int64, source func(int, int64) float64) loudness.IntegratedResult {
	shelf, hpf := conformanceCoefficients(config.SampleRate)

	filters := make([][2]conformanceDF1, config.Channels)
	for channel := range filters {
		filters[channel][0].coefficients = shelf
		filters[channel][1].coefficients = hpf
	}

	prefix := make([]float64, frames+1)
	peak := 0.0

	for frame := int64(0); frame < frames; frame++ {
		power := 0.0

		for channel := range config.Channels {
			input := source(channel, frame)
			peak = math.Max(peak, math.Abs(input))

			weight := 1.0
			if config.ChannelWeights != nil {
				weight = config.ChannelWeights[channel]
			}

			if weight == 0 {
				continue
			}

			filtered := filters[channel][1].sample(filters[channel][0].sample(input))
			power += weight * filtered * filtered
		}

		prefix[frame+1] = prefix[frame] + power
	}

	var powers []float64

	for block := int64(0); ; block++ {
		// Round independently accumulated nominal timestamps, not a fixed
		// rounded hop. Fractional-rate windows can differ by one frame.
		start := int64(math.Round(config.SampleRate * float64(block) * 0.1))

		end := int64(math.Round(config.SampleRate * (0.4 + float64(block)*0.1)))
		if end > frames {
			break
		}

		powers = append(powers, (prefix[end]-prefix[start])/float64(end-start))
	}

	absPower := math.Pow(10, (-70+0.691)/10)
	absSum, absCount := 0.0, 0

	for _, power := range powers {
		if power > absPower {
			absSum += power
			absCount++
		}
	}

	if absCount == 0 {
		return loudness.IntegratedResult{LUFS: math.Inf(-1), SamplePeak: peak, Frames: frames}
	}

	relativePower := absSum / float64(absCount) / 10
	finalSum, finalCount := 0.0, 0

	for _, power := range powers {
		if power > absPower && power > relativePower {
			finalSum += power
			finalCount++
		}
	}

	return loudness.IntegratedResult{LUFS: -0.691 + 10*math.Log10(finalSum/float64(finalCount)), SamplePeak: peak, Frames: frames}
}

type conformanceDF1 struct {
	coefficients   [5]float64 // b0,b1,b2,a1,a2; a0=1
	x1, x2, y1, y2 float64
}

func (f *conformanceDF1) sample(input float64) float64 {
	c := f.coefficients
	output := c[0]*input + c[1]*f.x1 + c[2]*f.x2 - c[3]*f.y1 - c[4]*f.y2
	f.x2, f.x1, f.y2, f.y1 = f.x1, input, f.y1, output

	return output
}

func conformanceCoefficients(rate float64) ([5]float64, [5]float64) {
	shelf := [5]float64{1.53512485958697, -2.69169618940638, 1.19839281085285, -1.69065929318241, 0.73248077421585}

	hpf := [5]float64{1, -2, 1, -1.99004745483398, 0.99007225036621}
	if rate == 48000 {
		return shelf, hpf
	}
	// Algebraic inverse-bilinear mapping of published48k polynomials. This
	// preserves the s-domain response; no production coefficient code is used.
	remap := func(c [5]float64) [5]float64 {
		polynomial := func(v0, v1, v2 float64) [3]float64 {
			p0, p1, p2 := v0+v1+v2, 2*(v0-v2), v0-v1+v2
			ratio := rate / 48000

			return [3]float64{p0 + p1*ratio + p2*ratio*ratio, 2*p0 - 2*p2*ratio*ratio, p0 - p1*ratio + p2*ratio*ratio}
		}
		numerator, denominator := polynomial(c[0], c[1], c[2]), polynomial(1, c[3], c[4])

		return [5]float64{numerator[0] / denominator[0], numerator[1] / denominator[0], numerator[2] / denominator[0], denominator[1] / denominator[0], denominator[2] / denominator[0]}
	}

	return remap(shelf), remap(hpf)
}

func conformanceResponse(c [5]float64, z complex128) complex128 {
	return (complex(c[0], 0) + complex(c[1], 0)*z + complex(c[2], 0)*z*z) / (1 + complex(c[3], 0)*z + complex(c[4], 0)*z*z)
}

func conformanceNear(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()

	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > tolerance {
		t.Fatalf("%s %.15g, want %.15g ±%g", name, got, want, tolerance)
	}
}

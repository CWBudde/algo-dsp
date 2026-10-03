package loudness

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func safetySignal(frames, channels int) [][]float64 {
	planar := make([][]float64, channels)
	for channel := range planar {
		planar[channel] = make([]float64, frames)
		for frame := range planar[channel] {
			planar[channel][frame] = (0.2 + float64(channel)*0.025) * math.Sin(2*math.Pi*1000*float64(frame)/48000)
		}
	}

	return planar
}

func safetyAnalyzer(t *testing.T, frames int64, channels int) *IntegratedAnalyzer {
	t.Helper()

	analyzer, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: channels, MaxFrames: frames})
	if err != nil {
		t.Fatal(err)
	}

	return analyzer
}

func safetyFinish(t *testing.T, analyzer *IntegratedAnalyzer) IntegratedResult {
	t.Helper()

	for attempts := 0; attempts < 10000; attempts++ {
		done, err := analyzer.FinishStep(1)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			result, err := analyzer.Result()
			if err != nil {
				t.Fatal(err)
			}

			return result
		}
	}

	t.Fatal("bounded finish did not complete")

	return IntegratedResult{}
}

func TestIntegratedSafetyConfigValidation(t *testing.T) {
	base := IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 48000}

	tests := []struct {
		name string
		edit func(*IntegratedConfig)
	}{
		{"zero rate", func(c *IntegratedConfig) { c.SampleRate = 0 }},
		{"below rate", func(c *IntegratedConfig) { c.SampleRate = 7999 }},
		{"above rate", func(c *IntegratedConfig) { c.SampleRate = 384001 }},
		{"nan rate", func(c *IntegratedConfig) { c.SampleRate = math.NaN() }},
		{"infinite rate", func(c *IntegratedConfig) { c.SampleRate = math.Inf(1) }},
		{"max float rate", func(c *IntegratedConfig) { c.SampleRate = math.MaxFloat64 }},
		{"zero channels", func(c *IntegratedConfig) { c.Channels = 0 }},
		{"negative channels", func(c *IntegratedConfig) { c.Channels = -1 }},
		{"above channels", func(c *IntegratedConfig) { c.Channels = 33 }},
		{"max int channels", func(c *IntegratedConfig) { c.Channels = int(^uint(0) >> 1) }},
		{"zero max frames", func(c *IntegratedConfig) { c.MaxFrames = 0 }},
		{"negative max frames", func(c *IntegratedConfig) { c.MaxFrames = -1 }},
		{"max int64 frames", func(c *IntegratedConfig) { c.MaxFrames = math.MaxInt64 }},
		{"mismatched weights", func(c *IntegratedConfig) { c.ChannelWeights = []float64{1} }},
		{"negative weights", func(c *IntegratedConfig) { c.ChannelWeights = []float64{-1, 1} }},
		{"oversized weights", func(c *IntegratedConfig) { c.ChannelWeights = []float64{17, 1} }},
		{"zero weights", func(c *IntegratedConfig) { c.ChannelWeights = []float64{0, 0} }},
		{"nan weights", func(c *IntegratedConfig) { c.ChannelWeights = []float64{1, math.NaN()} }},
		{"infinite weights", func(c *IntegratedConfig) { c.ChannelWeights = []float64{math.Inf(1), 1} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := base
			tc.edit(&config)

			if analyzer, err := NewIntegratedAnalyzer(config); err == nil || analyzer != nil {
				t.Fatalf("invalid configuration returned analyzer=%v error=%v", analyzer, err)
			}
		})
	}
	// Individually valid bounds can still exceed the workspace ceiling. This
	// must reject before allocating, on 32-bit WASM as well as native builds.
	_, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 384000, Channels: 32, MaxFrames: 1 << 40})
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("oversized workspace: got %v, want ErrLimit", err)
	}

	for _, rate := range []float64{8000, 48000.5, 384000} {
		if _, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: rate, Channels: 32, MaxFrames: 1}); err != nil {
			t.Fatalf("valid rate/channel boundary %g rejected: %v", rate, err)
		}
	}
}

func TestIntegratedSafetyPreflightLeavesStreamingStateUntouched(t *testing.T) {
	valid := safetySignal(24000, 2)
	invalid := []struct {
		name string
		data [][]float64
	}{
		{"missing channel", valid[:1]},
		{"extra channel", append(append([][]float64{}, valid...), valid[0])},
		{"unequal lengths", [][]float64{valid[0], valid[1][:23999]}},
		{"oversized block", safetySignal(65537, 2)},
		{"late nan", safetySignal(24000, 2)},
		{"late infinity", safetySignal(24000, 2)},
	}
	invalid[4].data[1][23999] = math.NaN()

	invalid[5].data[1][23999] = math.Inf(-1)
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			got := safetyAnalyzer(t, 48000, 2)

			want := safetyAnalyzer(t, 48000, 2)
			for _, analyzer := range []*IntegratedAnalyzer{got, want} {
				if err := analyzer.ProcessPlanar(valid); err != nil {
					t.Fatal(err)
				}
			}

			if err := got.ProcessPlanar(tc.data); err == nil {
				t.Fatal("invalid block accepted")
			}

			for _, analyzer := range []*IntegratedAnalyzer{got, want} {
				if err := analyzer.ProcessPlanar(valid); err != nil {
					t.Fatalf("preflight rejection poisoned valid stream: %v", err)
				}
			}

			if actual, expected := safetyFinish(t, got), safetyFinish(t, want); actual != expected {
				t.Fatalf("preflight advanced state: got %+v, want %+v", actual, expected)
			}
		})
	}
}

func TestIntegratedSafetyFloat32PreflightAndParity(t *testing.T) {
	source := safetySignal(24000, 2)
	planar32 := make([][]float32, 2)

	quantized := make([][]float64, 2)
	for channel := range source {
		planar32[channel] = make([]float32, len(source[channel]))

		quantized[channel] = make([]float64, len(source[channel]))
		for frame, value := range source[channel] {
			planar32[channel][frame] = float32(value)
			quantized[channel][frame] = float64(float32(value))
		}
	}

	bad := []struct {
		name string
		data [][]float32
	}{
		{"missing", planar32[:1]},
		{"unequal", [][]float32{planar32[0], planar32[1][:23999]}},
		{"nan", [][]float32{append([]float32{}, planar32[0]...), append([]float32{}, planar32[1]...)}},
		{"infinity", [][]float32{append([]float32{}, planar32[0]...), append([]float32{}, planar32[1]...)}},
		{"oversized", [][]float32{make([]float32, 65537), make([]float32, 65537)}},
	}
	bad[2].data[1][23999] = float32(math.NaN())

	bad[3].data[1][23999] = float32(math.Inf(1))
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			analyzer := safetyAnalyzer(t, 24000, 2)
			if err := analyzer.ProcessPlanar32(tc.data); err == nil {
				t.Fatal("invalid float32 block accepted")
			}

			if err := analyzer.ProcessPlanar32(planar32); err != nil {
				t.Fatal(err)
			}

			reference := safetyAnalyzer(t, 24000, 2)
			if err := reference.ProcessPlanar(quantized); err != nil {
				t.Fatal(err)
			}

			if got, want := safetyFinish(t, analyzer), safetyFinish(t, reference); got != want {
				t.Fatalf("float32 preflight/parity mismatch: %+v != %+v", got, want)
			}
		})
	}
}

func TestIntegratedSafetyOwnsWeightsAndNeverMutatesInput(t *testing.T) {
	weights := []float64{1, 0}
	config := IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 24000, ChannelWeights: weights}

	analyzer, err := NewIntegratedAnalyzer(config)
	if err != nil {
		t.Fatal(err)
	}

	weights[0], weights[1] = 0, 16
	input := safetySignal(24000, 1)
	// Sharing a caller-owned backing array between channels is valid read-only input.
	shared := [][]float64{input[0], input[0]}
	before := append([]float64{}, input[0]...)

	if err := analyzer.ProcessPlanar(shared); err != nil {
		t.Fatal(err)
	}

	reference := safetyAnalyzer(t, 24000, 1)
	if err := reference.ProcessPlanar(input); err != nil {
		t.Fatal(err)
	}

	if got, want := safetyFinish(t, analyzer), safetyFinish(t, reference); got != want {
		t.Fatalf("weights alias caller input: got %+v want %+v", got, want)
	}

	if !reflect.DeepEqual(input[0], before) {
		t.Fatal("ProcessPlanar mutated caller samples")
	}
}

func TestIntegratedSafetyFrameLimitAndNumericalPoisonReset(t *testing.T) {
	t.Run("limit rejects atomically", func(t *testing.T) {
		analyzer := safetyAnalyzer(t, 24000, 1)

		input := safetySignal(24000, 1)
		if err := analyzer.ProcessPlanar(input); err != nil {
			t.Fatal(err)
		}

		if err := analyzer.ProcessPlanar([][]float64{{0.1}}); !errors.Is(err, ErrLimit) {
			t.Fatalf("frame limit: got %v", err)
		}

		result := safetyFinish(t, analyzer)
		if result.Frames != 24000 {
			t.Fatalf("limit rejection advanced frames to %d", result.Frames)
		}
	})
	t.Run("overflow terminal until reset", func(t *testing.T) {
		analyzer := safetyAnalyzer(t, 24000, 1)
		if err := analyzer.ProcessPlanar([][]float64{{math.MaxFloat64}}); !errors.Is(err, ErrNumericalOverflow) {
			t.Fatalf("finite arithmetic overflow: %v", err)
		}

		if err := analyzer.ProcessPlanar(safetySignal(24000, 1)); !errors.Is(err, ErrNumericalOverflow) {
			t.Fatalf("overflow state accepted more input: %v", err)
		}

		if _, err := analyzer.FinishStep(1); !errors.Is(err, ErrNumericalOverflow) {
			t.Fatalf("poisoned finish: %v", err)
		}

		if _, err := analyzer.Result(); !errors.Is(err, ErrNumericalOverflow) {
			t.Fatalf("poisoned result: %v", err)
		}

		analyzer.Reset()

		if err := analyzer.ProcessPlanar(safetySignal(24000, 1)); err != nil {
			t.Fatalf("Reset did not recover overflow: %v", err)
		}

		if result := safetyFinish(t, analyzer); result.Frames != 24000 {
			t.Fatalf("Reset retained old frames: %+v", result)
		}
	})
}

func TestIntegratedSafetyFinishLifecycle(t *testing.T) {
	analyzer := safetyAnalyzer(t, 48000, 1)
	if _, err := analyzer.Result(); err == nil {
		t.Fatal("unfinished result accepted")
	}

	input := safetySignal(48000, 1)
	if err := analyzer.ProcessPlanar(input); err != nil {
		t.Fatal(err)
	}

	for _, budget := range []int{-1, 0} {
		if _, err := analyzer.FinishStep(budget); err == nil {
			t.Fatalf("invalid finish budget %d accepted", budget)
		}
	}

	done, err := analyzer.FinishStep(1)
	if err != nil || done {
		t.Fatalf("one step prematurely finished seven gating blocks: done=%v error=%v", done, err)
	}

	if _, err := analyzer.Result(); err == nil {
		t.Fatal("partially finished result accepted")
	}

	if err := analyzer.ProcessPlanar([][]float64{{0.1}}); err == nil {
		t.Fatal("processing accepted during partial finish")
	}

	first := safetyFinish(t, analyzer)
	if done, err := analyzer.FinishStep(1); !done || err != nil {
		t.Fatalf("finished job was not idempotent: done=%v error=%v", done, err)
	}

	analyzer.Reset()

	if err := analyzer.ProcessPlanar(input); err != nil {
		t.Fatal(err)
	}

	if second := safetyFinish(t, analyzer); second != first {
		t.Fatalf("Reset changed deterministic result: %+v != %+v", second, first)
	}
}

func TestIntegratedSafetyInvalidFinishDoesNotFreezeInput(t *testing.T) {
	analyzer := safetyAnalyzer(t, 48000, 1)

	input := safetySignal(24000, 1)
	if err := analyzer.ProcessPlanar(input); err != nil {
		t.Fatal(err)
	}

	for _, budget := range []int{-1, 0} {
		if _, err := analyzer.FinishStep(budget); err == nil {
			t.Fatalf("invalid budget %d accepted", budget)
		}
	}

	if err := analyzer.ProcessPlanar(input); err != nil {
		t.Fatalf("invalid finish froze input: %v", err)
	}

	if done, err := analyzer.FinishStep(int(^uint(0) >> 1)); err != nil || !done {
		t.Fatalf("MaxInt budget overflowed: done=%v error=%v", done, err)
	}

	if result, err := analyzer.Result(); err != nil || result.Frames != 48000 {
		t.Fatalf("invalid finish advanced state: %+v error=%v", result, err)
	}
}

func TestIntegratedSafetyWindowOverflowAndZeroWeightPreflight(t *testing.T) {
	t.Run("finite individual energies overflow window", func(t *testing.T) {
		analyzer := safetyAnalyzer(t, 24000, 1)

		input := make([]float64, 24000)
		for frame := range input {
			input[frame] = 1e152
			if frame%2 == 0 {
				input[frame] = -input[frame]
			}
		}

		if err := analyzer.ProcessPlanar([][]float64{input}); !errors.Is(err, ErrNumericalOverflow) {
			t.Fatalf("overflowed gating sum was accepted: %v", err)
		}

		if _, err := analyzer.FinishStep(1); !errors.Is(err, ErrNumericalOverflow) {
			t.Fatalf("window overflow was not terminal: %v", err)
		}

		analyzer.Reset()

		if err := analyzer.ProcessPlanar(safetySignal(24000, 1)); err != nil {
			t.Fatal(err)
		}

		_ = safetyFinish(t, analyzer)
	})
	t.Run("zero weights do not bypass input validation or peaks", func(t *testing.T) {
		analyzer, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, ChannelWeights: []float64{1, 0}, MaxFrames: 24000})
		if err != nil {
			t.Fatal(err)
		}

		input := safetySignal(24000, 2)

		input[1][23999] = math.Inf(1)
		if err := analyzer.ProcessPlanar(input); !errors.Is(err, ErrNonFinite) {
			t.Fatalf("zero-weight nonfinite sample accepted: %v", err)
		}

		input[1][23999] = 3
		if err := analyzer.ProcessPlanar(input); err != nil {
			t.Fatal(err)
		}

		if result := safetyFinish(t, analyzer); result.SamplePeak != 3 || result.Frames != 24000 {
			t.Fatalf("zero-weight channel omitted from peak/count: %+v", result)
		}
	})
}

func TestIntegratedSafetyShortSilenceAndZeroAllocationAdvance(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames int
		want   error
	}{
		{"empty", 0, ErrTooShort},
		{"partial window", 19199, ErrTooShort},
		{"silence full window", 19200, ErrBelowGate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			analyzer := safetyAnalyzer(t, 24000, 1)
			if err := analyzer.ProcessPlanar([][]float64{make([]float64, tc.frames)}); err != nil {
				t.Fatal(err)
			}

			_, err := analyzer.FinishStep(1)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}

	analyzer := safetyAnalyzer(t, 48000, 2)
	input := safetySignal(48000, 2)

	var failure error

	allocations := testing.AllocsPerRun(4, func() {
		analyzer.Reset()

		if failure = analyzer.ProcessPlanar(input); failure != nil {
			return
		}

		for {
			var done bool

			done, failure = analyzer.FinishStep(1)
			if failure != nil || done {
				return
			}
		}
	})
	if failure != nil || allocations != 0 {
		t.Fatalf("stream/finish allocated: allocs=%g error=%v", allocations, failure)
	}

	input32 := make([][]float32, 2)
	for channel := range input32 {
		input32[channel] = make([]float32, len(input[channel]))
		for frame, value := range input[channel] {
			input32[channel][frame] = float32(value)
		}
	}

	allocations = testing.AllocsPerRun(4, func() {
		analyzer.Reset()

		if failure = analyzer.ProcessPlanar32(input32); failure != nil {
			return
		}

		for {
			var done bool

			done, failure = analyzer.FinishStep(1)
			if failure != nil || done {
				return
			}
		}
	})
	if failure != nil || allocations != 0 {
		t.Fatalf("float32 stream/finish allocated: allocs=%g error=%v", allocations, failure)
	}
}

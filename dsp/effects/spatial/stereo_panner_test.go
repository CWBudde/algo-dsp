package spatial

import (
	"math"
	"testing"
)

// panGainTolerance is the tolerance used for the closed-form pan-law
// invariants; the gains are pure trigonometry, so they hold to near machine
// precision.
const panGainTolerance = 1e-12

func newTestPanner(t *testing.T, opts ...StereoPannerOption) *StereoPanner {
	t.Helper()

	p, err := NewStereoPanner(48000, opts...)
	if err != nil {
		t.Fatalf("NewStereoPanner: %v", err)
	}

	return p
}

func TestNewStereoPannerValidation(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		opts       []StereoPannerOption
		wantErr    bool
	}{
		{name: "defaults", sampleRate: 48000},
		{name: "nil option skipped", sampleRate: 48000, opts: []StereoPannerOption{nil}},
		{name: "zero sample rate", sampleRate: 0, wantErr: true},
		{name: "negative sample rate", sampleRate: -48000, wantErr: true},
		{name: "nan sample rate", sampleRate: math.NaN(), wantErr: true},
		{name: "inf sample rate", sampleRate: math.Inf(1), wantErr: true},
		{
			name: "position hard left", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanPosition(-1)},
		},
		{
			name: "position hard right", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanPosition(1)},
		},
		{
			name: "position below range", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanPosition(-1.0001)}, wantErr: true,
		},
		{
			name: "position above range", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanPosition(1.0001)}, wantErr: true,
		},
		{
			name: "position nan", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanPosition(math.NaN())}, wantErr: true,
		},
		{
			name: "position inf", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanPosition(math.Inf(-1))}, wantErr: true,
		},
		{
			name: "law linear", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanLaw(PanLawLinear)},
		},
		{
			name: "law compromise", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanLaw(PanLawCompromise)},
		},
		{
			name: "law invalid", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanLaw(PanLaw(99))}, wantErr: true,
		},
		{
			name: "law negative", sampleRate: 48000,
			opts: []StereoPannerOption{WithPanLaw(PanLaw(-1))}, wantErr: true,
		},
		{
			name: "auto-pan rate disabled", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanRate(0)},
		},
		{
			name: "auto-pan rate at max", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanRate(maxAutoPanRate)},
		},
		{
			name: "auto-pan rate above max", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanRate(maxAutoPanRate + 0.1)}, wantErr: true,
		},
		{
			name: "auto-pan rate negative", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanRate(-1)}, wantErr: true,
		},
		{
			name: "auto-pan rate nan", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanRate(math.NaN())}, wantErr: true,
		},
		{
			name: "auto-pan rate inf", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanRate(math.Inf(1))}, wantErr: true,
		},
		{
			name: "auto-pan depth zero", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanDepth(0)},
		},
		{
			name: "auto-pan depth above range", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanDepth(1.5)}, wantErr: true,
		},
		{
			name: "auto-pan depth negative", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanDepth(-0.1)}, wantErr: true,
		},
		{
			name: "auto-pan depth nan", sampleRate: 48000,
			opts: []StereoPannerOption{WithAutoPanDepth(math.NaN())}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStereoPanner(tt.sampleRate, tt.opts...)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestNewStereoPannerDefaults(t *testing.T) {
	p := newTestPanner(t)

	if p.SampleRate() != 48000 {
		t.Errorf("SampleRate = %g, want 48000", p.SampleRate())
	}

	if p.Position() != defaultPanPosition {
		t.Errorf("Position = %g, want %g", p.Position(), defaultPanPosition)
	}

	if p.Law() != PanLawEqualPower {
		t.Errorf("Law = %d, want %d", p.Law(), PanLawEqualPower)
	}

	if p.AutoPanRate() != defaultAutoPanRate {
		t.Errorf("AutoPanRate = %g, want %g", p.AutoPanRate(), defaultAutoPanRate)
	}

	if p.AutoPanDepth() != defaultAutoPanDepth {
		t.Errorf("AutoPanDepth = %g, want %g", p.AutoPanDepth(), defaultAutoPanDepth)
	}
}

// TestStereoPannerEqualPowerPreservesPower is the phase exit criterion: the
// equal-power law must keep gL^2+gR^2 at unity across the whole sweep.
func TestStereoPannerEqualPowerPreservesPower(t *testing.T) {
	const steps = 100

	for i := 0; i <= steps; i++ {
		position := -1 + 2*float64(i)/steps

		gainL, gainR := panGains(PanLawEqualPower, position)

		power := gainL*gainL + gainR*gainR
		if math.Abs(power-1) > panGainTolerance {
			t.Errorf("position %g: gL^2+gR^2 = %.17g, want 1", position, power)
		}
	}
}

func TestStereoPannerLinearPreservesAmplitude(t *testing.T) {
	const steps = 100

	for i := 0; i <= steps; i++ {
		position := -1 + 2*float64(i)/steps

		gainL, gainR := panGains(PanLawLinear, position)

		sum := gainL + gainR
		if math.Abs(sum-1) > panGainTolerance {
			t.Errorf("position %g: gL+gR = %.17g, want 1", position, sum)
		}
	}
}

func TestStereoPannerCentreLevels(t *testing.T) {
	tests := []struct {
		name string
		law  PanLaw
		want float64 // centre gain, i.e. the law's name in linear terms
		dB   float64
	}{
		// The dB column holds the exact centre level; the conventional "-3 dB",
		// "-6 dB" and "-4.5 dB" names for these laws are rounded.
		{name: "equal power", law: PanLawEqualPower, want: math.Sqrt2 / 2, dB: -3.0103},
		{name: "linear", law: PanLawLinear, want: 0.5, dB: -6.0206},
		{name: "compromise", law: PanLawCompromise, want: 0.5946035575013605, dB: -4.5154},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gainL, gainR := panGains(tt.law, 0)

			if math.Abs(gainL-tt.want) > 1e-12 || math.Abs(gainR-tt.want) > 1e-12 {
				t.Errorf("centre gains = (%g, %g), want (%g, %g)", gainL, gainR, tt.want, tt.want)
			}

			gotDB := 20 * math.Log10(gainL)
			if math.Abs(gotDB-tt.dB) > 0.01 {
				t.Errorf("centre level = %.4f dB, want %.4f dB", gotDB, tt.dB)
			}
		})
	}
}

func TestStereoPannerHardLeftCentreRight(t *testing.T) {
	for _, law := range []PanLaw{PanLawEqualPower, PanLawLinear, PanLawCompromise} {
		gainL, gainR := panGains(law, -1)
		if math.Abs(gainL-1) > panGainTolerance || math.Abs(gainR) > panGainTolerance {
			t.Errorf("law %d hard left gains = (%g, %g), want (1, 0)", law, gainL, gainR)
		}

		gainL, gainR = panGains(law, 1)
		if math.Abs(gainL) > panGainTolerance || math.Abs(gainR-1) > panGainTolerance {
			t.Errorf("law %d hard right gains = (%g, %g), want (0, 1)", law, gainL, gainR)
		}
	}
}

func TestStereoPannerGainsMonotonic(t *testing.T) {
	const steps = 200

	for _, law := range []PanLaw{PanLawEqualPower, PanLawLinear, PanLawCompromise} {
		prevL, prevR := panGains(law, -1)

		for i := 1; i <= steps; i++ {
			position := -1 + 2*float64(i)/steps

			gainL, gainR := panGains(law, position)
			if gainL > prevL+panGainTolerance {
				t.Errorf("law %d: gL rose from %g to %g at position %g", law, prevL, gainL, position)
			}

			if gainR < prevR-panGainTolerance {
				t.Errorf("law %d: gR fell from %g to %g at position %g", law, prevR, gainR, position)
			}

			prevL, prevR = gainL, gainR
		}
	}
}

func TestStereoPannerProcessMono(t *testing.T) {
	p := newTestPanner(t, WithPanPosition(0))

	outL, outR := p.ProcessMono(1)

	want := math.Sqrt2 / 2
	if math.Abs(outL-want) > panGainTolerance || math.Abs(outR-want) > panGainTolerance {
		t.Errorf("centred ProcessMono(1) = (%g, %g), want (%g, %g)", outL, outR, want, want)
	}

	if err := p.SetPosition(1); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}

	outL, outR = p.ProcessMono(1)
	if math.Abs(outL) > panGainTolerance || math.Abs(outR-1) > panGainTolerance {
		t.Errorf("hard-right ProcessMono(1) = (%g, %g), want (0, 1)", outL, outR)
	}
}

func TestStereoPannerBalanceAttenuateOnly(t *testing.T) {
	const steps = 200

	for _, law := range []PanLaw{PanLawEqualPower, PanLawLinear, PanLawCompromise} {
		gainL, gainR := balanceGains(law, 0)
		if gainL != 1 || gainR != 1 {
			t.Errorf("law %d: centre balance gains = (%g, %g), want (1, 1)", law, gainL, gainR)
		}

		for i := 0; i <= steps; i++ {
			position := -1 + 2*float64(i)/steps

			gainL, gainR = balanceGains(law, position)
			if gainL > 1+panGainTolerance || gainR > 1+panGainTolerance {
				t.Errorf("law %d position %g: balance gains = (%g, %g), must never exceed unity",
					law, position, gainL, gainR)
			}

			if gainL < 0 || gainR < 0 {
				t.Errorf("law %d position %g: balance gains = (%g, %g), must be non-negative",
					law, position, gainL, gainR)
			}
		}

		gainL, gainR = balanceGains(law, 1)
		if math.Abs(gainL) > panGainTolerance || math.Abs(gainR-1) > panGainTolerance {
			t.Errorf("law %d: hard-right balance gains = (%g, %g), want (0, 1)", law, gainL, gainR)
		}

		gainL, gainR = balanceGains(law, -1)
		if math.Abs(gainL-1) > panGainTolerance || math.Abs(gainR) > panGainTolerance {
			t.Errorf("law %d: hard-left balance gains = (%g, %g), want (1, 0)", law, gainL, gainR)
		}
	}
}

func TestStereoPannerProcessStereoCentreIsPassThrough(t *testing.T) {
	p := newTestPanner(t)

	left := []float64{1, -0.5, 0.25, 0}
	right := []float64{0, 0.5, -0.25, 1}

	wantL := append([]float64(nil), left...)
	wantR := append([]float64(nil), right...)

	if err := p.ProcessStereoInPlace(left, right); err != nil {
		t.Fatalf("ProcessStereoInPlace: %v", err)
	}

	for i := range left {
		if left[i] != wantL[i] || right[i] != wantR[i] {
			t.Errorf("[%d] = (%g, %g), want (%g, %g)", i, left[i], right[i], wantL[i], wantR[i])
		}
	}
}

func TestStereoPannerInterleaved(t *testing.T) {
	p := newTestPanner(t, WithPanPosition(1))

	buf := []float64{1, 1, 0.5, 0.5}
	if err := p.ProcessInterleavedInPlace(buf); err != nil {
		t.Fatalf("ProcessInterleavedInPlace: %v", err)
	}

	// Hard right in balance mode mutes the left channel and passes the right.
	want := []float64{0, 1, 0, 0.5}
	for i := range buf {
		if math.Abs(buf[i]-want[i]) > panGainTolerance {
			t.Errorf("[%d] = %g, want %g", i, buf[i], want[i])
		}
	}
}

func TestStereoPannerMonoToStereoAndInterleaved(t *testing.T) {
	in := []float64{1, 0.5, -0.25}

	p := newTestPanner(t, WithPanPosition(-1))

	left := make([]float64, len(in))
	right := make([]float64, len(in))

	if err := p.ProcessMonoToStereo(in, left, right); err != nil {
		t.Fatalf("ProcessMonoToStereo: %v", err)
	}

	for i, v := range in {
		if math.Abs(left[i]-v) > panGainTolerance || math.Abs(right[i]) > panGainTolerance {
			t.Errorf("[%d] = (%g, %g), want (%g, 0)", i, left[i], right[i], v)
		}
	}

	out := make([]float64, 2*len(in))
	if err := p.ProcessMonoToInterleaved(in, out); err != nil {
		t.Fatalf("ProcessMonoToInterleaved: %v", err)
	}

	for i, v := range in {
		if math.Abs(out[2*i]-v) > panGainTolerance || math.Abs(out[2*i+1]) > panGainTolerance {
			t.Errorf("interleaved [%d] = (%g, %g), want (%g, 0)", i, out[2*i], out[2*i+1], v)
		}
	}
}

func TestStereoPannerBufferValidation(t *testing.T) {
	p := newTestPanner(t)

	if err := p.ProcessStereoInPlace(make([]float64, 4), make([]float64, 3)); err == nil {
		t.Error("expected error for mismatched stereo buffer lengths")
	}

	if err := p.ProcessInterleavedInPlace(make([]float64, 3)); err == nil {
		t.Error("expected error for odd interleaved buffer length")
	}

	if err := p.ProcessMonoToStereo(make([]float64, 4), make([]float64, 3), make([]float64, 4)); err == nil {
		t.Error("expected error for mismatched left buffer length")
	}

	if err := p.ProcessMonoToStereo(make([]float64, 4), make([]float64, 4), make([]float64, 3)); err == nil {
		t.Error("expected error for mismatched right buffer length")
	}

	if err := p.ProcessMonoToInterleaved(make([]float64, 4), make([]float64, 4)); err == nil {
		t.Error("expected error for undersized interleaved output")
	}
}

func TestStereoPannerAutoPanSweepsFullRange(t *testing.T) {
	const rate = 2.0

	p := newTestPanner(t, WithAutoPanRate(rate), WithAutoPanDepth(1))

	period := int(math.Round(48000 / rate))

	minPos, maxPos := math.Inf(1), math.Inf(-1)

	for range period {
		pos := p.effectivePosition()
		minPos = math.Min(minPos, pos)
		maxPos = math.Max(maxPos, pos)

		p.ProcessMono(1)
	}

	if maxPos < 0.999 || minPos > -0.999 {
		t.Errorf("auto-pan swept [%g, %g], want approximately [-1, 1]", minPos, maxPos)
	}

	if p.lfoPhase < 0 || p.lfoPhase >= 2*math.Pi {
		t.Errorf("lfoPhase = %g, want wrapped into [0, 2pi)", p.lfoPhase)
	}
}

func TestStereoPannerAutoPanDepthLimitsSwing(t *testing.T) {
	const (
		rate  = 4.0
		depth = 0.25
	)

	p := newTestPanner(t, WithAutoPanRate(rate), WithAutoPanDepth(depth))

	period := int(math.Round(48000 / rate))

	for range period {
		pos := p.effectivePosition()
		if math.Abs(pos) > depth+panGainTolerance {
			t.Fatalf("position %g exceeds depth %g", pos, depth)
		}

		p.ProcessMono(1)
	}
}

func TestStereoPannerEffectivePosition(t *testing.T) {
	static := newTestPanner(t, WithPanPosition(-0.4))
	if got := static.effectivePosition(); got != -0.4 {
		t.Errorf("static effectivePosition = %g, want -0.4", got)
	}

	// An offset position plus a full-depth swing must clamp at the pan limits
	// rather than running past hard left/right.
	swinging := newTestPanner(t, WithPanPosition(0.9), WithAutoPanRate(1), WithAutoPanDepth(1))

	sawMax := false

	for range 48000 {
		pos := swinging.effectivePosition()
		if pos < minPanPosition || pos > maxPanPosition {
			t.Fatalf("effectivePosition = %g, want within [%g, %g]", pos, minPanPosition, maxPanPosition)
		}

		if pos == maxPanPosition {
			sawMax = true
		}

		swinging.ProcessMono(1)
	}

	if !sawMax {
		t.Error("expected the swing to clamp at hard right")
	}
}

func TestStereoPannerAutoPanDeterministicAfterReset(t *testing.T) {
	p := newTestPanner(t, WithAutoPanRate(5), WithAutoPanDepth(0.8))

	const n = 256

	first := make([]float64, 2*n)
	for i := range n {
		first[2*i], first[2*i+1] = p.ProcessMono(1)
	}

	p.Reset()

	for i := range n {
		gotL, gotR := p.ProcessMono(1)
		if gotL != first[2*i] || gotR != first[2*i+1] {
			t.Fatalf("[%d] after Reset = (%g, %g), want (%g, %g)",
				i, gotL, gotR, first[2*i], first[2*i+1])
		}
	}
}

func TestStereoPannerAutoPanBalanceNeverBoosts(t *testing.T) {
	p := newTestPanner(t, WithAutoPanRate(3), WithAutoPanDepth(1))

	for i := range 16000 {
		outL, outR := p.ProcessStereo(1, 1)
		if outL > 1+panGainTolerance || outR > 1+panGainTolerance {
			t.Fatalf("[%d] auto-pan balance output = (%g, %g), must never exceed unity", i, outL, outR)
		}
	}
}

func TestStereoPannerGainsAccessors(t *testing.T) {
	p := newTestPanner(t, WithPanPosition(0.5))

	gainL, gainR := p.Gains()

	wantL, wantR := panGains(PanLawEqualPower, 0.5)
	if gainL != wantL || gainR != wantR {
		t.Errorf("Gains = (%g, %g), want (%g, %g)", gainL, gainR, wantL, wantR)
	}

	balL, balR := p.BalanceGains()

	wantBalL, wantBalR := balanceGains(PanLawEqualPower, 0.5)
	if balL != wantBalL || balR != wantBalR {
		t.Errorf("BalanceGains = (%g, %g), want (%g, %g)", balL, balR, wantBalL, wantBalR)
	}

	// Reading the gains must not advance the LFO.
	if err := p.SetAutoPanRate(2); err != nil {
		t.Fatalf("SetAutoPanRate: %v", err)
	}

	before := p.lfoPhase

	p.Gains()
	p.BalanceGains()

	if p.lfoPhase != before {
		t.Errorf("lfoPhase advanced from %g to %g while reading gains", before, p.lfoPhase)
	}
}

func TestStereoPannerSetters(t *testing.T) {
	p := newTestPanner(t)

	if err := p.SetSampleRate(96000); err != nil {
		t.Fatalf("SetSampleRate: %v", err)
	}

	if p.SampleRate() != 96000 {
		t.Errorf("SampleRate = %g, want 96000", p.SampleRate())
	}

	if err := p.SetPosition(-0.5); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}

	if p.Position() != -0.5 {
		t.Errorf("Position = %g, want -0.5", p.Position())
	}

	if err := p.SetLaw(PanLawLinear); err != nil {
		t.Fatalf("SetLaw: %v", err)
	}

	if p.Law() != PanLawLinear {
		t.Errorf("Law = %d, want %d", p.Law(), PanLawLinear)
	}

	if err := p.SetAutoPanRate(6); err != nil {
		t.Fatalf("SetAutoPanRate: %v", err)
	}

	if p.AutoPanRate() != 6 {
		t.Errorf("AutoPanRate = %g, want 6", p.AutoPanRate())
	}

	if err := p.SetAutoPanDepth(0.25); err != nil {
		t.Fatalf("SetAutoPanDepth: %v", err)
	}

	if p.AutoPanDepth() != 0.25 {
		t.Errorf("AutoPanDepth = %g, want 0.25", p.AutoPanDepth())
	}

	wantInc := 2 * math.Pi * 6 / 96000
	if math.Abs(p.phaseInc-wantInc) > panGainTolerance {
		t.Errorf("phaseInc = %g, want %g", p.phaseInc, wantInc)
	}
}

func TestStereoPannerSettersRejectInvalidValues(t *testing.T) {
	p := newTestPanner(t, WithPanPosition(0.25), WithPanLaw(PanLawLinear),
		WithAutoPanRate(3), WithAutoPanDepth(0.5))

	tests := []struct {
		name string
		call func() error
	}{
		{name: "sample rate zero", call: func() error { return p.SetSampleRate(0) }},
		{name: "sample rate nan", call: func() error { return p.SetSampleRate(math.NaN()) }},
		{name: "position out of range", call: func() error { return p.SetPosition(2) }},
		{name: "law invalid", call: func() error { return p.SetLaw(PanLaw(42)) }},
		{name: "rate above max", call: func() error { return p.SetAutoPanRate(1000) }},
		{name: "depth out of range", call: func() error { return p.SetAutoPanDepth(-1) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Error("expected error")
			}
		})
	}

	// State must be untouched by the rejected calls.
	if p.SampleRate() != 48000 || p.Position() != 0.25 || p.Law() != PanLawLinear ||
		p.AutoPanRate() != 3 || p.AutoPanDepth() != 0.5 {
		t.Errorf("state changed after rejected setters: sr=%g pos=%g law=%d rate=%g depth=%g",
			p.SampleRate(), p.Position(), p.Law(), p.AutoPanRate(), p.AutoPanDepth())
	}
}

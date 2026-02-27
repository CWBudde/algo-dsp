package effectchain

import (
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func TestDefaultRegistry(t *testing.T) {
	t.Parallel()

	expectedTypes := []string{
		"chorus", "flanger", "ringmod", "bitcrusher",
		"distortion", "dist-cheb", "transformer",
		"widener", "phaser", "tremolo",
		"delay", "delay-simple",
		"filter", "filter-lowpass", "filter-highpass", "filter-bandpass",
		"filter-notch", "filter-allpass", "filter-peak",
		"filter-lowshelf", "filter-highshelf", "filter-moog",
		"bass", "pitch-time", "pitch-spectral",
		"spectral-freeze", "granular",
		"reverb", "reverb-freeverb", "reverb-fdn", "reverb-conv",
		"dyn-compressor", "dyn-limiter", "dyn-lookahead",
		"dyn-gate", "dyn-expander", "dyn-deesser",
		"dyn-transient", "dyn-multiband",
		"vocoder",
	}

	reg := DefaultRegistry()

	for _, effectType := range expectedTypes {
		if reg.Lookup(effectType) == nil {
			t.Errorf("DefaultRegistry missing effect type: %s", effectType)
		}
	}
}

func TestDefaultRegistryCreatesRuntimes(t *testing.T) {
	t.Parallel()

	ctx := Context{SampleRate: 44100}
	reg := DefaultRegistry()

	// Test a representative subset that exercises different factory paths.
	types := []string{
		"chorus", "delay", "delay-simple",
		"reverb-freeverb", "reverb-fdn",
		"dyn-compressor", "dyn-multiband",
		"widener", "phaser",
	}

	for _, effectType := range types {
		t.Run(effectType, func(t *testing.T) {
			t.Parallel()

			factory := reg.Lookup(effectType)
			if factory == nil {
				t.Fatalf("no factory for %s", effectType)
			}

			rt, err := factory(ctx)
			if err != nil {
				t.Fatalf("factory error for %s: %v", effectType, err)
			}

			if rt == nil {
				t.Fatalf("factory returned nil runtime for %s", effectType)
			}
		})
	}
}

func TestDefaultRegistryWithFilterDesigner(t *testing.T) {
	t.Parallel()

	designer := &stubFilterDesigner{}
	reg := DefaultRegistry(WithFilterDesigner(designer))

	ctx := Context{SampleRate: 44100}
	factory := reg.Lookup("filter")

	rt, err := factory(ctx)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}

	fr, ok := rt.(*filterRuntime)
	if !ok {
		t.Fatalf("expected *filterRuntime, got %T", rt)
	}

	if fr.designer != designer {
		t.Error("filter designer was not injected")
	}
}

type stubFilterDesigner struct{}

func (d *stubFilterDesigner) NormalizeFamily(family string) string                { return family }
func (d *stubFilterDesigner) NormalizeFamilyForType(_, family string) string      { return family }
func (d *stubFilterDesigner) NormalizeOrder(_, _ string, order int) int           { return order }
func (d *stubFilterDesigner) ClampShape(_, _ string, _, _, value float64) float64 { return value }
func (d *stubFilterDesigner) BuildChain(_, _ string, _ int, _, _, _, _ float64) *biquad.Chain {
	return biquad.NewChain([]biquad.Coefficients{{B0: 1}})
}

func TestDefaultRegistryWithIRProvider(t *testing.T) {
	t.Parallel()

	provider := &stubIRProvider{}
	reg := DefaultRegistry(WithIRProvider(provider))

	ctx := Context{SampleRate: 44100}
	factory := reg.Lookup("reverb-conv")

	rt, err := factory(ctx)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}

	cr, ok := rt.(*convReverbRuntime)
	if !ok {
		t.Fatalf("expected *convReverbRuntime, got %T", rt)
	}

	if cr.irProvider != provider {
		t.Error("IR provider was not injected")
	}
}

type stubIRProvider struct{}

func (p *stubIRProvider) GetIR(_ int) ([][]float64, float64, bool) {
	return nil, 0, false
}

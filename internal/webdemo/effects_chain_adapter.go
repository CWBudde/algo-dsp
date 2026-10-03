package webdemo

import (
	"github.com/cwbudde/algo-dsp/dsp/effectchain"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// irLibAdapter adapts IRLibrary to the effectchain.IRProvider interface.
type irLibAdapter struct {
	lib *IRLibrary
}

func (a *irLibAdapter) GetIR(index int) ([][]float64, float64, bool) {
	if a.lib == nil {
		return nil, 0, false
	}

	ir := a.lib.GetIR(index)
	if ir == nil || len(ir.Samples) == 0 {
		return nil, 0, false
	}

	return ir.Samples, ir.SampleRate, true
}

// eqFilterDesigner adapts the webdemo EQ building functions to the effectchain.FilterDesigner interface.
type eqFilterDesigner struct{}

func (d *eqFilterDesigner) NormalizeFamily(family string) string {
	return (effectchain.BuiltInFilterDesigner{}).NormalizeFamily(family)
}

func (d *eqFilterDesigner) NormalizeFamilyForType(kind, family string) string {
	return (effectchain.BuiltInFilterDesigner{}).NormalizeFamilyForType(kind, family)
}

func (d *eqFilterDesigner) NormalizeOrder(kind, family string, order int) int {
	return (effectchain.BuiltInFilterDesigner{}).NormalizeOrder(kind, family, order)
}

func (d *eqFilterDesigner) ClampShape(kind, family string, freq, sampleRate, value float64) float64 {
	return (effectchain.BuiltInFilterDesigner{}).ClampShape(kind, family, freq, sampleRate, value)
}

func (d *eqFilterDesigner) BuildChain(family, kind string, order int, freq, gainDB, q, sampleRate float64) *biquad.Chain {
	return (effectchain.BuiltInFilterDesigner{}).BuildChain(family, kind, order, freq, gainDB, q, sampleRate)
}

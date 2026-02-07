package vecmath

import (
	"sync"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

var (
	scaleBlockImpl        func([]float64, []float64, float64)
	scaleBlockInPlaceImpl func([]float64, float64)
	scaleInitOnce         sync.Once
)

func initScaleOperations() {
	features := cpu.DetectFeatures()
	entry := registry.Global.Lookup(features)
	if entry == nil {
		panic("vecmath: no scale implementation registered")
	}
	if entry.ScaleBlock == nil || entry.ScaleBlockInPlace == nil {
		panic("vecmath: selected implementation missing scale operations")
	}
	scaleBlockImpl = entry.ScaleBlock
	scaleBlockInPlaceImpl = entry.ScaleBlockInPlace
}

func ScaleBlock(dst, src []float64, scalar float64) {
	scaleInitOnce.Do(initScaleOperations)
	scaleBlockImpl(dst, src, scalar)
}

func ScaleBlockInPlace(dst []float64, scalar float64) {
	scaleInitOnce.Do(initScaleOperations)
	scaleBlockInPlaceImpl(dst, scalar)
}

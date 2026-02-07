package vecmath

import (
	"sync"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

var (
	mulBlockImpl        func([]float64, []float64, []float64)
	mulBlockInPlaceImpl func([]float64, []float64)
	mulInitOnce         sync.Once
)

func initMulOperations() {
	features := cpu.DetectFeatures()
	entry := registry.Global.Lookup(features)
	if entry == nil {
		panic("vecmath: no mul implementation registered")
	}
	if entry.MulBlock == nil || entry.MulBlockInPlace == nil {
		panic("vecmath: selected implementation missing mul operations")
	}
	mulBlockImpl = entry.MulBlock
	mulBlockInPlaceImpl = entry.MulBlockInPlace
}

func MulBlock(dst, a, b []float64) {
	mulInitOnce.Do(initMulOperations)
	mulBlockImpl(dst, a, b)
}

func MulBlockInPlace(dst, src []float64) {
	mulInitOnce.Do(initMulOperations)
	mulBlockInPlaceImpl(dst, src)
}

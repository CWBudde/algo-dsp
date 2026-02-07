package vecmath

import (
	"sync"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

var (
	addMulBlockImpl func([]float64, []float64, []float64, float64)
	mulAddBlockImpl func([]float64, []float64, []float64, []float64)
	fusedInitOnce   sync.Once
)

func initFusedOperations() {
	features := cpu.DetectFeatures()
	entry := registry.Global.Lookup(features)
	if entry == nil {
		panic("vecmath: no fused implementation registered")
	}
	if entry.AddMulBlock == nil || entry.MulAddBlock == nil {
		panic("vecmath: selected implementation missing fused operations")
	}
	addMulBlockImpl = entry.AddMulBlock
	mulAddBlockImpl = entry.MulAddBlock
}

func AddMulBlock(dst, a, b []float64, scalar float64) {
	fusedInitOnce.Do(initFusedOperations)
	addMulBlockImpl(dst, a, b, scalar)
}

func MulAddBlock(dst, a, b, c []float64) {
	fusedInitOnce.Do(initFusedOperations)
	mulAddBlockImpl(dst, a, b, c)
}

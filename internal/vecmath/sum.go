package vecmath

import (
	"sync"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

var (
	sumImpl     func([]float64) float64
	sumInitOnce sync.Once
)

func initSumOperation() {
	features := cpu.DetectFeatures()
	entry := registry.Global.Lookup(features)
	if entry == nil {
		panic("vecmath: no sum implementation registered")
	}
	if entry.Sum == nil {
		panic("vecmath: selected implementation missing sum operation")
	}
	sumImpl = entry.Sum
}

// Sum returns the sum of all elements in x.
// Returns 0 for an empty slice.
func Sum(x []float64) float64 {
	sumInitOnce.Do(initSumOperation)
	return sumImpl(x)
}

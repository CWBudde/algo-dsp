package pass

import (
	"fmt"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ---------------------------------------------------------------------------
// Cross-topology tests
// ---------------------------------------------------------------------------

func TestAllCascades_FiniteAcrossFrequencies(t *testing.T) {
	sr := 48000.0
	freqs := []float64{100, 500, 1000, 5000, 10000}


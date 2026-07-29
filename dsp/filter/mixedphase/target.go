package mixedphase

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/spectrum"
)

// prescribedResponse builds the conjugate-symmetric complex target whose
// magnitude follows targetMagnitude and whose phase interpolates between the
// unwrapped minimum phase and a pure delay of the given number of samples.
//
// A mix of zero prescribes minimum phase, a mix of one prescribes linear phase.
// Everything in between is the mixed-phase family both direct designs
// approximate.
func prescribedResponse(
	w *fftWorkspace,
	targetMagnitude []float64,
	minimumSpectrum []complex128,
	mix float64,
	delay float64,
) []complex128 {
	half := w.size / 2

	minimumPhase := make([]float64, half+1)
	for i := range minimumPhase {
		minimumPhase[i] = cmplx.Phase(minimumSpectrum[i])
	}

	minimumPhase = spectrum.UnwrapPhase(minimumPhase)

	desired := make([]complex128, w.size)

	for i := 0; i <= half; i++ {
		omega := 2 * math.Pi * float64(i) / float64(w.size)
		linearPhase := -omega * delay
		phase := (1-mix)*minimumPhase[i] + mix*linearPhase
		value := cmplx.Rect(targetMagnitude[i], phase)

		switch {
		case i == 0:
			desired[i] = complex(real(value), 0)
		case i == half && w.size%2 == 0:
			desired[i] = complex(real(value), 0)
		default:
			desired[i] = value
			desired[w.size-i] = cmplx.Conj(value)
		}
	}

	return desired
}

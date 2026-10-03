//nolint:all // Verbatim reference copy; kept byte-for-byte close to the original.
package features_test

// Verbatim copy (adapted to standalone functions) of barLevelDB, p95DB and
// the active closure of Roles from AudioVisualizer/internal/story/roles.go,
// plus Grid.frames from internal/story/grid.go and DB and Gate from
// internal/audioanalysis. The *audioanalysis.Track argument is replaced by
// its RMS slice, the Grid by a list of [start, end) spans in seconds (bar b
// is {g.BarStart(b), g.BarStart(b+1)}), and identifiers carry a ref prefix.
// The arithmetic and its operation order are unchanged; TestActivityParity
// compares Activity with this copy bit for bit. Do not "fix" or reformat
// the arithmetic below.

import (
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/dsp/core"
)

const refGate = 1e-4

const refFrameRate = float64(SampleRate) / Hop

func refActivityDB(v float64) float64 { return core.LinearToDBFloor(v, 1e-6) }

// refFrames maps [start, end) seconds to feature frame indices clamped to n.
func refFrames(start, end float64, n int) (int, int) {
	lo := min(n, max(0, int(math.Ceil(start*refFrameRate))))
	return lo, min(n, max(lo, int(math.Ceil(end*refFrameRate))))
}

// refBarLevelDB is the RMS (power mean) of a track over a bar, in dBFS.
func refBarLevelDB(rms []float64, span [2]float64) float64 {
	lo, hi := refFrames(span[0], span[1], len(rms))
	sum := 0.0
	for i := lo; i < hi; i++ {
		sum += rms[i] * rms[i]
	}
	if hi <= lo {
		return refActivityDB(0)
	}
	return refActivityDB(math.Sqrt(sum / float64(hi-lo)))
}

// refP95DB is the 95th percentile frame level of a track above the -80 dBFS gate.
func refP95DB(rms []float64) float64 {
	v := []float64{}
	for _, x := range rms {
		if x > refGate {
			v = append(v, x)
		}
	}
	if len(v) == 0 {
		return refActivityDB(0)
	}
	sort.Float64s(v)
	return refActivityDB(v[int(0.95*float64(len(v)-1))])
}

// refActive is the active closure of Roles for one track.
func refActive(rms []float64, spans [][2]float64, activeDB float64) []bool {
	ref := refP95DB(rms)
	out := make([]bool, len(spans))
	for b := range out {
		out[b] = refBarLevelDB(rms, spans[b]) > ref+activeDB
	}
	return out
}

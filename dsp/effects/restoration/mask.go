package restoration

import (
	"fmt"
	"math"
)

// Point is a vertex in sample-frame and Hz coordinates.
type (
	Point struct{ Frame, Hz float64 }
	// Mask defines a time-frequency rectangle or a polygon with 3..128 vertices.
	// Start and End are exclusive sample bounds. Points, when present, supersede
	// the rectangle's interior; the rectangle still bounds its extent.
	Mask struct {
		Start, End    int64
		LowHz, HighHz float64
		Points        []Point
	}
)

// Validate checks geometry before processing or allocating audio buffers.
func (m Mask) Validate(frames int64, sampleRate float64) error {
	if m.Start < 0 || m.End <= m.Start || m.End > frames || !finite(m.LowHz) || !finite(m.HighHz) || m.LowHz < 0 || m.HighHz <= m.LowHz || m.HighHz > sampleRate/2 || len(m.Points) > 128 || (len(m.Points) > 0 && len(m.Points) < 3) {
		return fmt.Errorf("restoration.mask: invalid geometry")
	}

	for _, p := range m.Points {
		if !finite(p.Frame) || !finite(p.Hz) || p.Frame < float64(m.Start) || p.Frame > float64(m.End) || p.Hz < m.LowHz || p.Hz > m.HighHz {
			return fmt.Errorf("restoration.mask: invalid vertex")
		}
	}

	return nil
}

// Contains reports whether the bin centre lies in the selection.
func (m Mask) Contains(frame, hz float64) bool {
	if frame < float64(m.Start) || frame >= float64(m.End) || hz < m.LowHz || hz > m.HighHz {
		return false
	}

	if len(m.Points) == 0 {
		return true
	}

	inside := false

	j := len(m.Points) - 1
	for i, p := range m.Points {
		q := m.Points[j]
		if (p.Hz > hz) != (q.Hz > hz) && frame < (q.Frame-p.Frame)*(hz-p.Hz)/(q.Hz-p.Hz)+p.Frame {
			inside = !inside
		}

		j = i
	}

	return inside
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

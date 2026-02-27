package effectchain

import "math"

// Params holds the parsed parameters for a single chain node.
type Params struct {
	ID       string
	Type     string
	Bypassed bool
	Num      map[string]float64
	Str      map[string]string
}

// GetNum safely extracts a numeric parameter, returning def if missing or invalid.
func (p Params) GetNum(key string, def float64) float64 {
	if p.Num == nil {
		return def
	}

	v, ok := p.Num[key]
	if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
		return def
	}

	return v
}

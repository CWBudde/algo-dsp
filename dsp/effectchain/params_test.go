package effectchain

import (
	"math"
	"testing"
)

func TestParamsGetNum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		p    Params
		key  string
		def  float64
		want float64
	}{
		{
			name: "existing key returns value",
			p:    Params{Num: map[string]float64{"gain": 0.75}},
			key:  "gain",
			def:  1.0,
			want: 0.75,
		},
		{
			name: "missing key returns default",
			p:    Params{Num: map[string]float64{"gain": 0.75}},
			key:  "mix",
			def:  0.5,
			want: 0.5,
		},
		{
			name: "nil map returns default",
			p:    Params{},
			key:  "gain",
			def:  1.0,
			want: 1.0,
		},
		{
			name: "NaN returns default",
			p:    Params{Num: map[string]float64{"gain": math.NaN()}},
			key:  "gain",
			def:  1.0,
			want: 1.0,
		},
		{
			name: "positive Inf returns default",
			p:    Params{Num: map[string]float64{"gain": math.Inf(1)}},
			key:  "gain",
			def:  1.0,
			want: 1.0,
		},
		{
			name: "negative Inf returns default",
			p:    Params{Num: map[string]float64{"gain": math.Inf(-1)}},
			key:  "gain",
			def:  1.0,
			want: 1.0,
		},
		{
			name: "zero value is valid",
			p:    Params{Num: map[string]float64{"gain": 0}},
			key:  "gain",
			def:  1.0,
			want: 0,
		},
		{
			name: "negative value is valid",
			p:    Params{Num: map[string]float64{"gain": -3.5}},
			key:  "gain",
			def:  1.0,
			want: -3.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.p.GetNum(tt.key, tt.def)
			if got != tt.want {
				t.Errorf("GetNum(%q, %v) = %v, want %v", tt.key, tt.def, got, tt.want)
			}
		})
	}
}

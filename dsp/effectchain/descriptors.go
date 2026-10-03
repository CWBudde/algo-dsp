package effectchain

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Descriptor describes the parameters accepted by a built-in effect factory.
// All numeric bounds are inclusive. Frequency bounds follow the sample rate
// supplied to DefaultDescriptors. Each call returns independently owned data.
type Descriptor struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Category    string                `json:"category"`
	ChannelMode string                `json:"channelMode"`
	View        string                `json:"view"`
	Parameters  []ParameterDescriptor `json:"parameters"`
	Presets     []FactoryPreset       `json:"presets"`
}

// ParameterDescriptor defines one canonical graph parameter. Boolean parameters
// use numeric zero/one; string enumeration parameters use DefaultString.
type ParameterDescriptor struct {
	ID            string            `json:"id"`
	Label         string            `json:"label"`
	Unit          string            `json:"unit"`
	Type          string            `json:"type"`
	Min           float64           `json:"min"`
	Max           float64           `json:"max"`
	Default       float64           `json:"default"`
	Scale         string            `json:"scale"`
	Step          float64           `json:"step"`
	DefaultString string            `json:"defaultString,omitempty"`
	Options       []ParameterOption `json:"options,omitempty"`
}

// ParameterOption is one accepted string enumeration value.
type ParameterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// FactoryPreset contains canonical graph parameters, independently owned by
// the caller. Its stable identifier is scoped to its effect descriptor.
type FactoryPreset struct {
	ID   string             `json:"id"`
	Name string             `json:"name"`
	Num  map[string]float64 `json:"num"`
	Str  map[string]string  `json:"str"`
}

func parameterLabel(id string) string {
	var out strings.Builder

	for i, r := range id {
		if i > 0 && unicode.IsUpper(r) {
			out.WriteByte(' ')
		}

		if i == 0 {
			r = unicode.ToUpper(r)
		}

		out.WriteRune(r)
	}

	return out.String()
}

func numberParameter(id string, def, lo, hi float64) ParameterDescriptor {
	p := ParameterDescriptor{ID: id, Label: parameterLabel(id), Type: "number", Min: lo, Max: hi, Default: min(max(def, lo), hi), Scale: "lin", Step: 0.01}
	switch {
	case strings.HasSuffix(id, "Hz") || id == "freq" || id == "frequency":
		p.Unit = "Hz"
		p.Scale = "log"
		p.Step = 1
	case strings.HasSuffix(id, "DB") || id == "gain":
		p.Unit = "dB"
		p.Scale = "dB"
		p.Step = 0.1
	case strings.HasSuffix(id, "Ms"):
		p.Unit = "ms"
		p.Step = 0.1
	case id == "time" || id == "baseDelay" || id == "preDelay" || id == "grainSeconds" || id == "rt60" || id == "modDepth":
		p.Unit = "s"
	}

	return p
}

func integerParameter(id string, def, lo, hi float64) ParameterDescriptor {
	p := numberParameter(id, def, lo, hi)
	p.Step = 1

	return p
}

func booleanParameter(id string, def float64) ParameterDescriptor {
	p := integerParameter(id, def, 0, 1)
	p.Type = "boolean"

	return p
}

func enumParameter(id, def string, choices ...string) ParameterDescriptor {
	p := ParameterDescriptor{ID: id, Label: parameterLabel(id), Type: "enum", Scale: "lin", DefaultString: def}
	for _, v := range choices {
		p.Options = append(p.Options, ParameterOption{Value: v, Label: parameterLabel(v)})
	}

	return p
}

func descriptor(id, name, category, view string, params ...ParameterDescriptor) Descriptor {
	for i := range params {
		p := &params[i]
		switch {
		case id == "pitch-time" && (p.ID == "sequence" || p.ID == "overlap" || p.ID == "search"):
			p.Unit = "ms"
		case p.ID == "gain" && !strings.HasPrefix(id, "filter"):
			p.Unit, p.Scale = "", "lin"
		case p.ID == "depth" && (id == "chorus" || id == "flanger"):
			p.Unit = "s"
		case id == "reverb-conv" && p.ID == "wet":
			p.Step = 0.01
		case id == "phaser" && p.ID == "stages":
			p.Max = 12
		}

		if id == "filter-moog" && p.ID == "kind" {
			*p = enumParameter("kind", "lowpass", "lowpass")
		}

		if id == "filter-moog" && p.ID == "family" {
			*p = enumParameter("family", "moog", "moog")
		}

		if id == "filter-moog" && p.ID == "q" {
			p.Min = 0
			p.Max = 4
			p.Label = "Resonance"
		}
	}

	if strings.HasPrefix(id, "filter") && id != "filter-moog" && id != "filter-a-weighting" && id != "filter-c-weighting" {
		ripple := numberParameter("rippleDB", 1, 0.05, 12)
		ripple.Label = "Ripple (Chebyshev I / elliptic)"
		stop := numberParameter("stopbandDB", 40, 10, 120)
		stop.Label = "Stopband (Chebyshev II / elliptic)"
		bandwidth := numberParameter("bandwidthHz", 1200/0.707, 1, params[0].Max)
		bandwidth.Label = "Bandwidth (high-order peak)"
		params = append(params, ripple, stop, bandwidth)
	}

	d := Descriptor{ID: id, Name: name, Category: category, ChannelMode: "mono", View: view, Parameters: params}
	switch id {
	case "widener", "rotary", "panner", "haas", "crosstalk":
		d.ChannelMode = "stereo"
	}

	p := FactoryPreset{ID: "default", Name: "Default", Num: map[string]float64{}, Str: map[string]string{}}

	for _, param := range params {
		if param.Type == "enum" {
			p.Str[param.ID] = param.DefaultString
		} else {
			p.Num[param.ID] = param.Default
		}
	}

	d.Presets = []FactoryPreset{p}

	return d
}

// DefaultDescriptors returns the complete built-in effect catalogue in lexical
// identifier order. Invalid sample rates use 44100 Hz. Descriptors do not depend
// on application state; convolution requires an IRProvider when configured.
func DefaultDescriptors(sampleRate float64) []Descriptor {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		sampleRate = 44100
	}

	descriptors := builtInDescriptors(sampleRate)
	descriptors = append(descriptors, extendedDescriptors(sampleRate)...)
	addFactoryPresets(descriptors)
	sort.Slice(descriptors, func(i, j int) bool { return descriptors[i].ID < descriptors[j].ID })

	return descriptors
}

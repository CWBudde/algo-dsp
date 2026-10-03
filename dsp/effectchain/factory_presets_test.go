package effectchain

import (
	"math"
	"reflect"
	"testing"
)

func TestFactoryPresetsCompleteCanonicalParametersAndOwnership(t *testing.T) {
	for _, rate := range []float64{8000, 44100, 48000, 384000} {
		first, second := DefaultDescriptors(rate), DefaultDescriptors(rate)
		for i, descriptor := range first {
			if len(descriptor.Presets) < 2 && len(descriptor.Parameters) > 0 {
				t.Errorf("%s lacks a useful factory preset", descriptor.ID)
			}

			parameters := make(map[string]ParameterDescriptor, len(descriptor.Parameters))
			for _, p := range descriptor.Parameters {
				parameters[p.ID] = p
			}

			seen := map[string]bool{}
			for presetIndex, preset := range descriptor.Presets {
				if preset.ID == "" || preset.Name == "" || seen[preset.ID] {
					t.Errorf("invalid preset identity %s/%s", descriptor.ID, preset.ID)
				}

				seen[preset.ID] = true
				if len(preset.Num)+len(preset.Str) != len(parameters) {
					t.Errorf("%s/%s incomplete parameters", descriptor.ID, preset.ID)
				}

				for id, value := range preset.Num {
					parameter, exists := parameters[id]
					if !exists || parameter.Type == "enum" || math.IsNaN(value) || math.IsInf(value, 0) || value < parameter.Min || value > parameter.Max {
						t.Errorf("%s/%s invalid numeric %s=%v", descriptor.ID, preset.ID, id, value)
					}
				}

				for id, value := range preset.Str {
					parameter, exists := parameters[id]
					valid := false

					for _, option := range parameter.Options {
						if option.Value == value {
							valid = true
						}
					}

					if !exists || parameter.Type != "enum" || !valid {
						t.Errorf("%s/%s invalid enum %s=%s", descriptor.ID, preset.ID, id, value)
					}
				}

				if presetIndex > 0 && reflect.DeepEqual(preset.Num, descriptor.Presets[0].Num) && reflect.DeepEqual(preset.Str, descriptor.Presets[0].Str) {
					t.Errorf("%s/%s equals default", descriptor.ID, preset.ID)
				}

				for id, value := range preset.Num {
					preset.Num[id] = value + 1000
					if second[i].Presets[presetIndex].Num[id] != value {
						t.Errorf("preset %s map aliased across catalogue calls", descriptor.ID)
					}

					if presetIndex > 0 && descriptor.Presets[0].Num[id] == value+1000 {
						t.Errorf("%s preset aliases default", descriptor.ID)
					}

					preset.Num[id] = value

					break
				}

				for id, value := range preset.Str {
					preset.Str[id] = "modified"
					if second[i].Presets[presetIndex].Str[id] != value {
						t.Errorf("preset %s enum map aliased", descriptor.ID)
					}

					preset.Str[id] = value

					break
				}
			}
		}
	}
}

package dither

import "testing"

func TestPresetCoefficients(t *testing.T) {
	tests := []struct {
		name   string
		preset Preset
		order  int
		first  float64
		last   float64
	}{
		{"EFB", PresetEFB, 1, 1.0, 1.0},
		{"2SC", Preset2SC, 2, 1.0, -0.5},
		{"2MEC", Preset2MEC, 2, 1.537, -0.8367},
		{"3MEC", Preset3MEC, 3, 1.652, 0.1382},
		{"9MEC", Preset9MEC, 9, 1.662, -0.03524},
		{"5IEC", Preset5IEC, 5, 2.033, 0.6149},
		{"9IEC", Preset9IEC, 9, 2.847, 0.4191},
		{"3FC", Preset3FC, 3, 1.623, 0.109},
		{"9FC", Preset9FC, 9, 2.412, 0.0847},
		{"SBM", PresetSBM, 12, 1.47933, 0.003067},
		{"SBMReduced", PresetSBMReduced, 10, 1.47933, 0.00123066},
		{"Sharp14k", PresetSharp14k, 7, 1.62019206878484, 0.35350816625238},
		{"Sharp15k", PresetSharp15k, 8, 1.34860378444905, -0.213194268754789},
		{"Sharp16k", PresetSharp16k, 9, 1.07618924753262, 0.185132272731155},
		{"Experimental", PresetExperimental, 9, 1.2194769820734, 0.320990893363264},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.preset.Coefficients()
			if len(c) != tt.order {
				t.Fatalf("order = %d, want %d", len(c), tt.order)
			}

			if c[0] != tt.first {
				t.Errorf("first coeff = %v, want %v", c[0], tt.first)
			}

			if c[len(c)-1] != tt.last {
				t.Errorf("last coeff = %v, want %v", c[len(c)-1], tt.last)
			}
		})
	}
}

func TestPresetNoneIsEmpty(t *testing.T) {
	c := PresetNone.Coefficients()
	if c != nil {
		t.Errorf("PresetNone should return nil, got %v", c)
	}
}

func TestPresetString(t *testing.T) {
	if got := PresetSBM.String(); got != "SBM" {
		t.Errorf("got %q, want %q", got, "SBM")
	}

	if got := Preset(99).String(); got != "Preset(99)" {
		t.Errorf("got %q", got)
	}
}

func TestPresetValid(t *testing.T) {
	if !Preset9FC.Valid() {
		t.Error("Preset9FC should be valid")
	}

	if Preset(99).Valid() {
		t.Error("Preset(99) should be invalid")
	}
}

func TestSharpPresetForSampleRate(t *testing.T) {
	tests := []struct {
		sr    float64
		first float64
	}{
		{40000, 0.919387305668676},
		{44100, 1.34860378444905},
		{48000, 1.4247141061364},
		{64000, 2.49725554745212},
		{96000, 3.14014081409305},
	}
	for _, tt := range tests {
		c := SharpPresetForSampleRate(tt.sr)
		if len(c) != 8 {
			t.Errorf("sr=%g: order = %d, want 8", tt.sr, len(c))
		}

		if c[0] != tt.first {
			t.Errorf("sr=%g: first = %v, want %v", tt.sr, c[0], tt.first)
		}
	}
}

func TestSharpPresetBoundaries(t *testing.T) {
	// Verify boundary selection matches legacy thresholds.
	c40999 := SharpPresetForSampleRate(40999)

	c41000 := SharpPresetForSampleRate(41000)
	if c40999[0] == c41000[0] {
		t.Error("40999 and 41000 should use different coefficients")
	}

	c45999 := SharpPresetForSampleRate(45999)

	c46000 := SharpPresetForSampleRate(46000)
	if c45999[0] == c46000[0] {
		t.Error("45999 and 46000 should use different coefficients")
	}

	c54999 := SharpPresetForSampleRate(54999)

	c55000 := SharpPresetForSampleRate(55000)
	if c54999[0] == c55000[0] {
		t.Error("54999 and 55000 should use different coefficients")
	}

	c75099 := SharpPresetForSampleRate(75099)

	c75100 := SharpPresetForSampleRate(75100)
	if c75099[0] == c75100[0] {
		t.Error("75099 and 75100 should use different coefficients")
	}
}

func TestPresetCoefficientsReturnsCopy(t *testing.T) {
	c1 := Preset9FC.Coefficients()
	c2 := Preset9FC.Coefficients()
	c1[0] = 999

	if c2[0] == 999 {
		t.Error("Coefficients() should return a copy, not a reference")
	}
}

func TestSharpPresetReturnsCopy(t *testing.T) {
	c1 := SharpPresetForSampleRate(44100)
	c2 := SharpPresetForSampleRate(44100)
	c1[0] = 999

	if c2[0] == 999 {
		t.Error("SharpPresetForSampleRate should return a copy")
	}
}

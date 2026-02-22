# Dither and Noise Shaping Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add quantization support with configurable dither PDFs, FIR/IIR noise-shaping, predefined presets, and a psychoacoustic coefficient designer.

**Architecture:** A `dsp/dither/` package provides the runtime (`Quantizer` struct combining dither + noise shaping + bit-depth quantization) with a `NoiseShaper` interface backed by FIR (ring buffer) and IIR (biquad shelf) implementations. A `dsp/dither/design/` sub-package provides an ATH-weighted stochastic coefficient optimizer. All coefficients are ported from legacy Pascal sources with exact value parity.

**Tech Stack:** Go, `math/rand/v2`, existing `dsp/filter/biquad` and `dsp/filter/design` packages, `internal/testutil`, `algo-fft` for the designer's FFT.

**Design doc:** `docs/plans/2026-02-22-dither-noiseshaper-design.md`

**Legacy references:**
- `legacy/Source/DSP/DAV_DspDitherNoiseShaper.pas`
- `legacy/Source/DSP/DAV_DspNoiseShapingFilterDesigner.pas`

---

### Task 1: Package scaffolding and DitherType enum

**Files:**
- Create: `dsp/dither/doc.go`
- Create: `dsp/dither/dither.go`
- Test: `dsp/dither/dither_test.go`

**Step 1: Write the failing test**

```go
// dsp/dither/dither_test.go
package dither

import "testing"

func TestDitherTypeString(t *testing.T) {
	tests := []struct {
		dt   DitherType
		want string
	}{
		{DitherNone, "None"},
		{DitherRectangular, "Rectangular"},
		{DitherTriangular, "Triangular"},
		{DitherGaussian, "Gaussian"},
		{DitherFastGaussian, "FastGaussian"},
		{DitherType(99), "DitherType(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.dt.String(); got != tt.want {
				t.Errorf("DitherType(%d).String() = %q, want %q", tt.dt, got, tt.want)
			}
		})
	}
}

func TestDitherTypeValid(t *testing.T) {
	if !DitherTriangular.Valid() {
		t.Error("DitherTriangular should be valid")
	}
	if DitherType(99).Valid() {
		t.Error("DitherType(99) should be invalid")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dsp/dither/ -run TestDitherType -v`
Expected: compilation failure — package and types don't exist yet.

**Step 3: Write minimal implementation**

```go
// dsp/dither/doc.go
// Package dither provides bit-depth quantization with configurable dither PDFs
// and noise-shaping filters.
package dither
```

```go
// dsp/dither/dither.go
package dither

import "fmt"

// DitherType selects the probability distribution used for dither noise.
type DitherType int

const (
	// DitherNone applies no dither (plain rounding/truncation).
	DitherNone DitherType = iota
	// DitherRectangular uses a uniform (rectangular) PDF.
	DitherRectangular
	// DitherTriangular uses a triangular PDF (TPDF), the most common choice.
	DitherTriangular
	// DitherGaussian uses an exact Gaussian PDF.
	DitherGaussian
	// DitherFastGaussian uses an approximated Gaussian PDF (sum of uniform draws).
	DitherFastGaussian

	ditherTypeCount // sentinel for validation
)

var ditherTypeNames = [ditherTypeCount]string{
	"None", "Rectangular", "Triangular", "Gaussian", "FastGaussian",
}

// String returns the name of the dither type.
func (dt DitherType) String() string {
	if dt >= 0 && dt < ditherTypeCount {
		return ditherTypeNames[dt]
	}
	return fmt.Sprintf("DitherType(%d)", dt)
}

// Valid reports whether dt is a known dither type.
func (dt DitherType) Valid() bool {
	return dt >= 0 && dt < ditherTypeCount
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./dsp/dither/ -run TestDitherType -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dsp/dither/doc.go dsp/dither/dither.go dsp/dither/dither_test.go
git commit -m "feat(dither): add package scaffolding and DitherType enum"
```

---

### Task 2: FIR presets — coefficient constants

**Files:**
- Create: `dsp/dither/presets.go`
- Create: `dsp/dither/presets_test.go`

**Step 1: Write the failing test**

```go
// dsp/dither/presets_test.go
package dither

import (
	"testing"
)

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
	if len(c) != 0 {
		t.Errorf("PresetNone should have 0 coefficients, got %d", len(c))
	}
}

func TestPresetString(t *testing.T) {
	if PresetSBM.String() != "SBM" {
		t.Errorf("got %q", PresetSBM.String())
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
		sr   float64
		want int // expected order (all sharp presets are order 8)
	}{
		{40000, 8},
		{44100, 8},
		{48000, 8},
		{64000, 8},
		{96000, 8},
	}
	for _, tt := range tests {
		c := SharpPresetForSampleRate(tt.sr)
		if len(c) != tt.want {
			t.Errorf("SharpPresetForSampleRate(%g): order = %d, want %d", tt.sr, len(c), tt.want)
		}
	}
}

func TestSharpPresetSampleRateBoundaries(t *testing.T) {
	// Verify the correct coefficient set is selected at boundaries.
	c40k := SharpPresetForSampleRate(40000)
	c44k := SharpPresetForSampleRate(44100)
	c48k := SharpPresetForSampleRate(48000)
	c96k := SharpPresetForSampleRate(96000)

	// These should be different coefficient sets.
	if c40k[0] == c44k[0] {
		t.Error("40kHz and 44.1kHz should use different coefficients")
	}
	if c44k[0] == c48k[0] {
		t.Error("44.1kHz and 48kHz should use different coefficients")
	}
	if c48k[0] == c96k[0] {
		t.Error("48kHz and 96kHz should use different coefficients")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dsp/dither/ -run "TestPreset|TestSharp" -v`
Expected: compilation failure.

**Step 3: Write minimal implementation**

Create `dsp/dither/presets.go` with:
- `Preset` type (int enum)
- All `Preset*` constants
- `Coefficients() []float64` method returning a copy of the coefficient slice
- `String()`, `Valid()` methods
- `SharpPresetForSampleRate(sr float64) []float64` function
- All coefficient arrays as package-level `var` slices, values ported exactly from legacy

The coefficient values to port (exact from legacy Pascal):

```go
var coeffEFB = []float64{1}
var coeff2SC = []float64{1.0, -0.5}
var coeff2MEC = []float64{1.537, -0.8367}
var coeff3MEC = []float64{1.652, -1.049, 0.1382}
var coeff9MEC = []float64{1.662, -1.263, 0.4827, -0.2913, 0.1268, -0.1124, 0.03252, -0.01265, -0.03524}
var coeff5IEC = []float64{2.033, -2.165, 1.959, -1.590, 0.6149}
var coeff9IEC = []float64{2.847, -4.685, 6.214, -7.184, 6.639, -5.032, 3.263, -1.632, 0.4191}
var coeff3FC = []float64{1.623, -0.982, 0.109}
var coeff9FC = []float64{2.412, -3.370, 3.937, -4.174, 3.353, -2.205, 1.281, -0.569, 0.0847}
var coeffSBM = []float64{1.47933, -1.59032, 1.64436, -1.36613, 0.926704, -0.557931, 0.26786, -0.106726, 0.028516, 0.00123066, -0.00616555, 0.003067}
var coeffSBMr = []float64{1.47933, -1.59032, 1.64436, -1.36613, 0.926704, -0.557931, 0.26786, -0.106726, 0.028516, 0.00123066}
var coeffEX = []float64{1.2194769820734, -1.77912468394129, 2.18256539389233, -2.33622087251503, 2.2010985277411, -1.81964871362306, 1.29830681491534, -0.767889385169331, 0.320990893363264}
var coeff14kSharp44100 = []float64{1.62019206878484, -2.26551157411517, 2.50884415683988, -2.25007947643775, 1.62160867255441, -0.899114621685913, 0.35350816625238}
var coeff15kSharp44100 = []float64{1.34860378444905, -1.80123976889643, 2.04804746376671, -1.93234174830592, 1.59264693241396, -1.04979311664936, 0.599422666305319, -0.213194268754789}
var coeff16kSharp44100 = []float64{1.07618924753262, -1.41232919229157, 1.61374140100329, -1.5996973679788, 1.42711666927426, -1.09986023030973, 0.750589080482029, -0.418709259968069, 0.185132272731155}

// Sharp sample-rate-adaptive sets:
var coeff15kSharp40000 = []float64{0.919387305668676, -1.04843437730544, 1.04843048925451, -0.868972788711174, 0.60853001063849, -0.3449209471469, 0.147484332561636, -0.0370652871194614}
var coeff15kSharp48000 = []float64{1.4247141061364, -1.5437678148854, 1.0967969510044, -0.32075758107035, -0.32074811729292, 0.525494723539046, -0.38058984415197, 0.14824460513256}
var coeff15kSharp64000 = []float64{2.49725554745212, -3.23587161287721, 2.31844946822861, -0.54326047010533, -0.54325301319653, 0.543289788745007, -0.142132484905, -0.0202120370327948}
var coeff15kSharp96000 = []float64{3.14014081409305, -3.76888037179035, 1.26107138314221, 1.26088059917107, -0.807698715053922, -0.80767075968406, 1.0101984930848, -0.322351688402064}
```

`SharpPresetForSampleRate` selection logic (matches legacy exactly):
```go
func SharpPresetForSampleRate(sr float64) []float64 {
	switch {
	case sr < 41000:
		return copyCoeffs(coeff15kSharp40000)
	case sr < 46000:
		return copyCoeffs(coeff15kSharp44100)
	case sr < 55000:
		return copyCoeffs(coeff15kSharp48000)
	case sr < 75100:
		return copyCoeffs(coeff15kSharp64000)
	default:
		return copyCoeffs(coeff15kSharp96000)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./dsp/dither/ -run "TestPreset|TestSharp" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dsp/dither/presets.go dsp/dither/presets_test.go
git commit -m "feat(dither): add FIR noise-shaping coefficient presets from legacy"
```

---

### Task 3: NoiseShaper interface and FIR shaper

**Files:**
- Create: `dsp/dither/shaper.go`
- Create: `dsp/dither/shaper_fir.go`
- Create: `dsp/dither/shaper_test.go`

**Step 1: Write the failing test**

```go
// dsp/dither/shaper_test.go
package dither

import (
	"math"
	"testing"
)

func TestFIRShaperZeroCoeffs(t *testing.T) {
	// With no coefficients, Shape should return input unchanged.
	s := NewFIRShaper(nil)
	for i := 0; i < 10; i++ {
		got := s.Shape(float64(i), float64(i)*0.1)
		if got != float64(i) {
			t.Errorf("sample %d: got %v, want %v", i, got, float64(i))
		}
	}
}

func TestFIRShaperEFB(t *testing.T) {
	// With [1.0] coefficients (simple error feedback), verify the shaping loop.
	s := NewFIRShaper([]float64{1.0})

	// First sample: no history, so Shape(1.0, 0) = 1.0
	got := s.Shape(1.0, 0.0)
	if got != 1.0 {
		t.Fatalf("sample 0: got %v, want 1.0", got)
	}
	// Feed back an error of 0.5
	got = s.Shape(1.0, 0.5)
	// Expected: 1.0 - 1.0*0.5 = 0.5
	if got != 0.5 {
		t.Fatalf("sample 1: got %v, want 0.5", got)
	}
}

func TestFIRShaperReset(t *testing.T) {
	s := NewFIRShaper([]float64{1.0})
	// Feed some errors.
	s.Shape(1.0, 0.0)
	s.Shape(1.0, 0.3)
	s.Reset()
	// After reset, history is zeroed. Same as fresh shaper.
	got := s.Shape(1.0, 0.0)
	if got != 1.0 {
		t.Errorf("after reset: got %v, want 1.0", got)
	}
}

func TestFIRShaperHighOrder(t *testing.T) {
	// Use 9FC preset, run 1000 samples, verify no NaN/Inf.
	coeffs := Preset9FC.Coefficients()
	s := NewFIRShaper(coeffs)
	for i := 0; i < 1000; i++ {
		v := s.Shape(0.5, 0.01*float64(i%10))
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("sample %d: got %v", i, v)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dsp/dither/ -run TestFIRShaper -v`
Expected: compilation failure.

**Step 3: Write minimal implementation**

`dsp/dither/shaper.go`:
```go
package dither

// NoiseShaper applies spectral shaping to quantization error.
type NoiseShaper interface {
	// Shape returns the noise-shaped input value. The caller provides
	// the quantization error from the previous sample via feedbackError.
	Shape(input, feedbackError float64) float64
	// Reset clears internal state.
	Reset()
}
```

`dsp/dither/shaper_fir.go`:
```go
package dither

// FIRShaper implements error-feedback noise shaping with FIR coefficients
// and a ring buffer for error history.
type FIRShaper struct {
	coeffs  []float64
	history []float64
	pos     int
	order   int
}

// NewFIRShaper creates a new FIR noise shaper with the given coefficients.
// A nil or empty slice creates a no-op shaper.
func NewFIRShaper(coeffs []float64) *FIRShaper {
	order := len(coeffs)
	c := make([]float64, order)
	copy(c, coeffs)
	return &FIRShaper{
		coeffs:  c,
		history: make([]float64, max(order, 1)),
		order:   order,
	}
}

func (s *FIRShaper) Shape(input, feedbackError float64) float64 {
	if s.order == 0 {
		return input
	}
	// Store the feedback error at the current position.
	s.history[s.pos] = feedbackError
	// Apply FIR filter: subtract weighted past errors.
	for i := 0; i < s.order; i++ {
		idx := (s.order + s.pos - i) % s.order
		input -= s.coeffs[i] * s.history[idx]
	}
	// Advance ring buffer position.
	s.pos = (s.pos + 1) % s.order
	return input
}

func (s *FIRShaper) Reset() {
	for i := range s.history {
		s.history[i] = 0
	}
	s.pos = 0
}
```

Note on the shaping algorithm: The legacy code stores `feedbackError` AFTER the FIR convolution and AFTER advancing `pos`. Our implementation stores it BEFORE the convolution at the current pos, so the convolution reads back error[pos] (current), error[pos-1] (previous), etc. The key difference: **we pass in the error from the caller (the Quantizer)** rather than computing it inside the shaper, which makes the shaper purely a filter. The Quantizer will call `Shape(input, previousError)` where `previousError` is `quantized - preQuantizedInput` from the prior sample.

Actually, looking more carefully at the legacy code, the FIR feedback loop works like this:
1. Convolve input with history (past errors)
2. Advance historyPos
3. After quantization, store `quantized - input` at the new historyPos

This means the shaper needs to manage both the convolution AND the error storage. Let me revise — the `Shape` method should take the current input, apply the FIR filter using internally stored errors, and the caller should call a separate method to record the error. This keeps the interface cleaner:

```go
type NoiseShaper interface {
	Shape(input float64) float64
	RecordError(quantizationError float64)
	Reset()
}
```

The FIR implementation then internally manages its ring buffer. The Quantizer calls:
1. `shaped := shaper.Shape(scaledInput)`
2. (quantize)
3. `shaper.RecordError(quantized - shaped)`

**Step 4: Run test to verify it passes**

Run: `go test ./dsp/dither/ -run TestFIRShaper -v`
Expected: PASS (after adjusting test to match the revised interface)

**Step 5: Commit**

```bash
git add dsp/dither/shaper.go dsp/dither/shaper_fir.go dsp/dither/shaper_test.go
git commit -m "feat(dither): add NoiseShaper interface and FIR ring-buffer implementation"
```

---

### Task 4: IIR shelf noise shaper

**Files:**
- Create: `dsp/dither/shaper_iir.go`
- Modify: `dsp/dither/shaper_test.go` (add IIR tests)

**Step 1: Write the failing test**

Add to `shaper_test.go`:
```go
func TestIIRShelfShaperCreation(t *testing.T) {
	s, err := NewIIRShelfShaper(10000, 44100)
	if err != nil {
		t.Fatal(err)
	}
	// Verify it implements NoiseShaper.
	var _ NoiseShaper = s
}

func TestIIRShelfShaperInvalidParams(t *testing.T) {
	tests := []struct {
		name string
		freq float64
		sr   float64
	}{
		{"zero freq", 0, 44100},
		{"negative freq", -100, 44100},
		{"zero sr", 10000, 0},
		{"NaN freq", math.NaN(), 44100},
		{"Inf sr", 10000, math.Inf(1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIIRShelfShaper(tt.freq, tt.sr)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestIIRShelfShaperReset(t *testing.T) {
	s, _ := NewIIRShelfShaper(10000, 44100)
	// Process some samples to build up state.
	for i := 0; i < 100; i++ {
		s.Shape(0.5)
		s.RecordError(0.01)
	}
	s.Reset()
	// After reset, shape of 0 should be 0.
	if got := s.Shape(0); got != 0 {
		t.Errorf("after reset: Shape(0) = %v, want 0", got)
	}
}

func TestIIRShelfShaperStability(t *testing.T) {
	s, _ := NewIIRShelfShaper(10000, 44100)
	for i := 0; i < 10000; i++ {
		v := s.Shape(0.5)
		s.RecordError(0.01)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("sample %d: got %v", i, v)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dsp/dither/ -run TestIIRShelf -v`
Expected: compilation failure.

**Step 3: Write minimal implementation**

```go
// dsp/dither/shaper_iir.go
package dither

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

// IIRShelfShaper implements noise shaping using a biquad low-shelf filter
// applied to the quantization error signal. This is a lightweight alternative
// to FIR shaping with less precise spectral control but lower CPU cost.
type IIRShelfShaper struct {
	filter    *biquad.Section
	lastError float64
}

const (
	iirShelfDefaultGainDB = -5.0
	iirShelfDefaultQ      = 0.707 // 1/sqrt(2), Butterworth
)

// NewIIRShelfShaper creates an IIR shelf noise shaper with the given corner
// frequency and sample rate. The shelf applies -5 dB of low-frequency
// de-emphasis to the error signal, pushing quantization noise above the
// shelf frequency.
func NewIIRShelfShaper(freq, sampleRate float64) (*IIRShelfShaper, error) {
	if freq <= 0 || math.IsNaN(freq) || math.IsInf(freq, 0) {
		return nil, fmt.Errorf("dither: IIR shelf frequency must be > 0 and finite: %f", freq)
	}
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("dither: IIR shelf sample rate must be > 0 and finite: %f", sampleRate)
	}
	coeffs := design.LowShelf(freq, iirShelfDefaultGainDB, iirShelfDefaultQ, sampleRate)
	return &IIRShelfShaper{
		filter: biquad.NewSection(coeffs),
	}, nil
}

func (s *IIRShelfShaper) Shape(input float64) float64 {
	return input - s.filter.ProcessSample(s.lastError)
}

func (s *IIRShelfShaper) RecordError(quantizationError float64) {
	s.lastError = quantizationError
}

func (s *IIRShelfShaper) Reset() {
	s.filter.Reset()
	s.lastError = 0
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./dsp/dither/ -run TestIIRShelf -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dsp/dither/shaper_iir.go dsp/dither/shaper_test.go
git commit -m "feat(dither): add IIR shelf noise shaper wrapping biquad low-shelf"
```

---

### Task 5: Options and config

**Files:**
- Create: `dsp/dither/options.go`
- Create: `dsp/dither/options_test.go`

**Step 1: Write the failing test**

```go
// dsp/dither/options_test.go
package dither

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	if cfg.bitDepth != 16 {
		t.Errorf("default bitDepth = %d, want 16", cfg.bitDepth)
	}
	if cfg.ditherType != DitherTriangular {
		t.Errorf("default ditherType = %v, want Triangular", cfg.ditherType)
	}
	if cfg.ditherAmplitude != 1.0 {
		t.Errorf("default ditherAmplitude = %v, want 1.0", cfg.ditherAmplitude)
	}
	if !cfg.limit {
		t.Error("default limit should be true")
	}
}

func TestOptionValidation(t *testing.T) {
	tests := []struct {
		name string
		opt  Option
	}{
		{"bitDepth 0", WithBitDepth(0)},
		{"bitDepth 33", WithBitDepth(33)},
		{"negative amplitude", WithDitherAmplitude(-1)},
		{"NaN amplitude", WithDitherAmplitude(math.NaN())},
		{"invalid dither type", WithDitherType(DitherType(99))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			if err := tt.opt(&cfg); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestOptionNil(t *testing.T) {
	cfg := defaultConfig()
	var opt Option
	// nil option should be a no-op, not a panic.
	if opt != nil {
		if err := opt(&cfg); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWithRNG(t *testing.T) {
	cfg := defaultConfig()
	rng := rand.New(rand.NewPCG(42, 0))
	if err := WithRNG(rng)(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.rng != rng {
		t.Error("RNG not set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dsp/dither/ -run "TestDefault|TestOption|TestWith" -v`
Expected: compilation failure.

**Step 3: Write minimal implementation**

```go
// dsp/dither/options.go
package dither

import (
	"fmt"
	"math"
	"math/rand/v2"
)

const (
	defaultBitDepth        = 16
	defaultDitherType      = DitherTriangular
	defaultDitherAmplitude = 1.0
	defaultLimit           = true
	minBitDepth            = 1
	maxBitDepth            = 32
)

type config struct {
	bitDepth        int
	ditherType      DitherType
	ditherAmplitude float64
	limit           bool
	shaper          NoiseShaper
	rng             *rand.Rand
	sharpPreset     bool
	iirShelfFreq    float64 // >0 means use IIR shelf
}

func defaultConfig() config {
	return config{
		bitDepth:        defaultBitDepth,
		ditherType:      defaultDitherType,
		ditherAmplitude: defaultDitherAmplitude,
		limit:           defaultLimit,
	}
}

// Option configures a Quantizer.
type Option func(*config) error

func WithBitDepth(bits int) Option {
	return func(cfg *config) error {
		if bits < minBitDepth || bits > maxBitDepth {
			return fmt.Errorf("dither: bit depth must be in [%d, %d]: %d", minBitDepth, maxBitDepth, bits)
		}
		cfg.bitDepth = bits
		return nil
	}
}

func WithDitherType(dt DitherType) Option {
	return func(cfg *config) error {
		if !dt.Valid() {
			return fmt.Errorf("dither: invalid dither type: %d", dt)
		}
		cfg.ditherType = dt
		return nil
	}
}

func WithDitherAmplitude(amp float64) Option {
	return func(cfg *config) error {
		if amp < 0 || math.IsNaN(amp) || math.IsInf(amp, 0) {
			return fmt.Errorf("dither: amplitude must be >= 0 and finite: %f", amp)
		}
		cfg.ditherAmplitude = amp
		return nil
	}
}

func WithLimit(enabled bool) Option {
	return func(cfg *config) error {
		cfg.limit = enabled
		return nil
	}
}

func WithNoiseShaper(ns NoiseShaper) Option {
	return func(cfg *config) error {
		cfg.shaper = ns
		return nil
	}
}

func WithFIRPreset(p Preset) Option {
	return func(cfg *config) error {
		if !p.Valid() {
			return fmt.Errorf("dither: invalid preset: %d", p)
		}
		cfg.shaper = NewFIRShaper(p.Coefficients())
		return nil
	}
}

func WithSharpPreset() Option {
	return func(cfg *config) error {
		cfg.sharpPreset = true
		return nil
	}
}

func WithIIRShelf(freq float64) Option {
	return func(cfg *config) error {
		if freq <= 0 || math.IsNaN(freq) || math.IsInf(freq, 0) {
			return fmt.Errorf("dither: IIR shelf frequency must be > 0 and finite: %f", freq)
		}
		cfg.iirShelfFreq = freq
		return nil
	}
}

func WithRNG(rng *rand.Rand) Option {
	return func(cfg *config) error {
		cfg.rng = rng
		return nil
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./dsp/dither/ -run "TestDefault|TestOption|TestWith" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dsp/dither/options.go dsp/dither/options_test.go
git commit -m "feat(dither): add option functions and config with validation"
```

---

### Task 6: Quantizer core — constructor, ProcessSample, ProcessInteger

**Files:**
- Create: `dsp/dither/quantizer.go`
- Create: `dsp/dither/quantizer_test.go`

**Step 1: Write the failing test**

```go
// dsp/dither/quantizer_test.go
package dither

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestNewQuantizerValidation(t *testing.T) {
	tests := []struct {
		name string
		sr   float64
		opts []Option
	}{
		{"zero sr", 0, nil},
		{"negative sr", -44100, nil},
		{"NaN sr", math.NaN(), nil},
		{"Inf sr", math.Inf(1), nil},
		{"bad option", 44100, []Option{WithBitDepth(0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewQuantizer(tt.sr, tt.opts...)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestNewQuantizerDefaults(t *testing.T) {
	q, err := NewQuantizer(44100)
	if err != nil {
		t.Fatal(err)
	}
	if q.BitDepth() != 16 {
		t.Errorf("BitDepth() = %d, want 16", q.BitDepth())
	}
	if q.DitherType() != DitherTriangular {
		t.Errorf("DitherType() = %v, want Triangular", q.DitherType())
	}
}

func TestQuantizerSilencePreservation(t *testing.T) {
	// With no dither, zero input should produce zero output.
	q, err := NewQuantizer(44100,
		WithDitherType(DitherNone),
		WithFIRPreset(PresetNone),
	)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if got := q.ProcessSample(0); got != 0 {
			t.Fatalf("sample %d: ProcessSample(0) = %v, want 0", i, got)
		}
	}
}

func TestQuantizerProcessInteger(t *testing.T) {
	q, err := NewQuantizer(44100,
		WithBitDepth(8),
		WithDitherType(DitherNone),
		WithFIRPreset(PresetNone),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Zero input -> zero output.
	if got := q.ProcessInteger(0); got != 0 {
		t.Errorf("ProcessInteger(0) = %d, want 0", got)
	}
}

func TestQuantizerDeterministic(t *testing.T) {
	// Two quantizers with same RNG seed should produce identical output.
	mk := func() *Quantizer {
		q, _ := NewQuantizer(44100,
			WithDitherType(DitherTriangular),
			WithRNG(rand.New(rand.NewPCG(42, 0))),
		)
		return q
	}
	q1, q2 := mk(), mk()
	for i := 0; i < 1000; i++ {
		a := q1.ProcessSample(0.3)
		b := q2.ProcessSample(0.3)
		if a != b {
			t.Fatalf("sample %d: %v != %v", i, a, b)
		}
	}
}

func TestQuantizerReset(t *testing.T) {
	q, _ := NewQuantizer(44100,
		WithDitherType(DitherNone),
		WithFIRPreset(PresetEFB),
	)
	// Process some samples to build state.
	for i := 0; i < 100; i++ {
		q.ProcessSample(0.5)
	}
	q.Reset()
	// After reset with no dither, zero input should produce zero.
	if got := q.ProcessSample(0); got != 0 {
		t.Errorf("after Reset: ProcessSample(0) = %v, want 0", got)
	}
}

func TestQuantizerLimiting(t *testing.T) {
	q, _ := NewQuantizer(44100,
		WithBitDepth(8),
		WithDitherType(DitherNone),
		WithFIRPreset(PresetNone),
		WithLimit(true),
	)
	// Input of 1.0 should clamp to max positive value for 8-bit.
	got := q.ProcessInteger(1.0)
	if got > 127 {
		t.Errorf("ProcessInteger(1.0) = %d, exceeds 127", got)
	}
	got = q.ProcessInteger(-1.0)
	if got < -128 {
		t.Errorf("ProcessInteger(-1.0) = %d, below -128", got)
	}
}

func TestQuantizerStability(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	q, _ := NewQuantizer(44100,
		WithDitherType(DitherTriangular),
		WithFIRPreset(Preset9FC),
		WithLimit(true),
		WithRNG(rng),
	)
	// Run 10000 hot samples; verify no NaN/Inf.
	for i := 0; i < 10000; i++ {
		input := (rng.Float64()*2 - 1) * 100 // deliberately clipping
		v := q.ProcessSample(input)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("sample %d: got %v for input %v", i, v, input)
		}
	}
}

func TestQuantizerProcessSampleProcessInPlaceParity(t *testing.T) {
	rng1 := rand.New(rand.NewPCG(99, 0))
	rng2 := rand.New(rand.NewPCG(99, 0))

	q1, _ := NewQuantizer(44100,
		WithDitherType(DitherTriangular),
		WithFIRPreset(Preset9FC),
		WithRNG(rng1),
	)
	q2, _ := NewQuantizer(44100,
		WithDitherType(DitherTriangular),
		WithFIRPreset(Preset9FC),
		WithRNG(rng2),
	)

	input := make([]float64, 512)
	rng := rand.New(rand.NewPCG(7, 0))
	for i := range input {
		input[i] = rng.Float64()*2 - 1
	}

	// ProcessSample loop
	sampleResults := make([]float64, len(input))
	for i, v := range input {
		sampleResults[i] = q1.ProcessSample(v)
	}

	// ProcessInPlace
	buf := make([]float64, len(input))
	copy(buf, input)
	q2.ProcessInPlace(buf)

	for i := range buf {
		if buf[i] != sampleResults[i] {
			t.Fatalf("sample %d: ProcessInPlace=%v, ProcessSample=%v", i, buf[i], sampleResults[i])
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dsp/dither/ -run TestQuantizer -v` or `go test ./dsp/dither/ -run "TestNew|TestQuantizer" -v`
Expected: compilation failure.

**Step 3: Write minimal implementation**

`dsp/dither/quantizer.go`:
```go
package dither

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// Quantizer performs bit-depth quantization with optional dither noise
// and noise shaping.
type Quantizer struct {
	sampleRate      float64
	bitDepth        int
	ditherType      DitherType
	ditherAmplitude float64
	limit           bool
	shaper          NoiseShaper
	rng             *rand.Rand

	// derived
	bitMul  float64
	bitDiv  float64
	limitLo int
	limitHi int
}

// NewQuantizer creates a new Quantizer. The default configuration is:
// 16-bit, triangular dither, amplitude 1.0, limiting enabled,
// F-weighted 9th-order FIR noise shaper.
func NewQuantizer(sampleRate float64, opts ...Option) (*Quantizer, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("dither: sample rate must be > 0 and finite: %f", sampleRate)
	}
	cfg := defaultConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	// Resolve noise shaper.
	var shaper NoiseShaper
	switch {
	case cfg.shaper != nil:
		shaper = cfg.shaper
	case cfg.sharpPreset:
		shaper = NewFIRShaper(SharpPresetForSampleRate(sampleRate))
	case cfg.iirShelfFreq > 0:
		var err error
		shaper, err = NewIIRShelfShaper(cfg.iirShelfFreq, sampleRate)
		if err != nil {
			return nil, err
		}
	default:
		shaper = NewFIRShaper(Preset9FC.Coefficients())
	}

	q := &Quantizer{
		sampleRate:      sampleRate,
		bitDepth:        cfg.bitDepth,
		ditherType:      cfg.ditherType,
		ditherAmplitude: cfg.ditherAmplitude,
		limit:           cfg.limit,
		shaper:          shaper,
		rng:             cfg.rng,
	}
	if q.rng == nil {
		q.rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	q.updateDerived()
	return q, nil
}

func (q *Quantizer) updateDerived() {
	q.bitMul = math.Exp2(float64(q.bitDepth-1)) - 0.5
	q.bitDiv = 1.0 / q.bitMul
	q.limitLo = -int(math.Round(q.bitMul + 0.5))
	q.limitHi = int(math.Round(q.bitMul - 0.5))
}

// ProcessInteger quantizes the input (expected in [-1, +1]) to an integer
// in the bit-depth range.
func (q *Quantizer) ProcessInteger(input float64) int {
	// Scale to integer range.
	scaled := q.bitMul * input

	// Apply noise shaping.
	shaped := q.shaper.Shape(scaled)

	// Add dither and quantize.
	result := q.quantize(shaped)

	// Optional limiting.
	if q.limit {
		result = max(q.limitLo, min(q.limitHi, result))
	}

	// Record quantization error for noise shaping feedback.
	q.shaper.RecordError(float64(result) - shaped)

	return result
}

// ProcessSample quantizes the input and returns a normalized float64
// in approximately [-1, +1].
func (q *Quantizer) ProcessSample(input float64) float64 {
	return (float64(q.ProcessInteger(input)) + 0.5) * q.bitDiv
}

// ProcessInPlace quantizes each sample in buf in-place.
func (q *Quantizer) ProcessInPlace(buf []float64) {
	for i, v := range buf {
		buf[i] = q.ProcessSample(v)
	}
}

// Reset clears all internal state (noise shaper history, etc.).
func (q *Quantizer) Reset() {
	q.shaper.Reset()
}

// quantize adds dither noise and rounds to the nearest integer.
func (q *Quantizer) quantize(input float64) int {
	switch q.ditherType {
	case DitherNone:
		return int(math.Round(input - 0.5))
	case DitherRectangular:
		return int(math.Round(input - 0.5 + q.ditherAmplitude*(q.rng.Float64()*2-1)))
	case DitherTriangular:
		return int(math.Round(input - 0.5 + q.ditherAmplitude*(q.rng.Float64()-q.rng.Float64())))
	case DitherGaussian:
		return int(math.Round(input - 0.5 + q.ditherAmplitude*q.rng.NormFloat64()))
	case DitherFastGaussian:
		return int(math.Round(input - 0.5 + q.ditherAmplitude*q.fastGaussian()))
	default:
		return int(math.Round(input - 0.5))
	}
}

// fastGaussian approximates a Gaussian distribution by summing uniform draws.
// The central limit theorem gives a good approximation with 6 draws.
func (q *Quantizer) fastGaussian() float64 {
	sum := 0.0
	for i := 0; i < 6; i++ {
		sum += q.rng.Float64()
	}
	return (sum - 3.0) // mean-centered, approximate stddev ~0.5
}

// Getters.

func (q *Quantizer) BitDepth() int            { return q.bitDepth }
func (q *Quantizer) DitherType() DitherType    { return q.ditherType }
func (q *Quantizer) DitherAmplitude() float64  { return q.ditherAmplitude }
func (q *Quantizer) Limit() bool               { return q.limit }
func (q *Quantizer) SampleRate() float64       { return q.sampleRate }

// Setters.

func (q *Quantizer) SetBitDepth(bits int) error {
	if bits < minBitDepth || bits > maxBitDepth {
		return fmt.Errorf("dither: bit depth must be in [%d, %d]: %d", minBitDepth, maxBitDepth, bits)
	}
	q.bitDepth = bits
	q.updateDerived()
	return nil
}

func (q *Quantizer) SetDitherType(dt DitherType) error {
	if !dt.Valid() {
		return fmt.Errorf("dither: invalid dither type: %d", dt)
	}
	q.ditherType = dt
	return nil
}

func (q *Quantizer) SetDitherAmplitude(amp float64) error {
	if amp < 0 || math.IsNaN(amp) || math.IsInf(amp, 0) {
		return fmt.Errorf("dither: amplitude must be >= 0 and finite: %f", amp)
	}
	q.ditherAmplitude = amp
	return nil
}

func (q *Quantizer) SetLimit(enabled bool) {
	q.limit = enabled
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./dsp/dither/ -run "TestNew|TestQuantizer" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dsp/dither/quantizer.go dsp/dither/quantizer_test.go
git commit -m "feat(dither): add Quantizer with dither, noise shaping, and quantization"
```

---

### Task 7: Spectral validation tests

**Files:**
- Modify: `dsp/dither/quantizer_test.go` (add spectral test)

These tests verify that noise shaping actually reduces in-band noise. Use the project's FFT dependency.

**Step 1: Write the failing test**

```go
func TestQuantizerNoiseShapingSpectralEffect(t *testing.T) {
	// Compare quantization noise spectrum with and without noise shaping.
	// With shaping, low-frequency energy should be lower than without.
	const (
		sr     = 44100.0
		nSamps = 8192
		seed   = 42
	)

	// Generate a low-level test signal (sine at 1 kHz, -20 dBFS).
	input := make([]float64, nSamps)
	amp := math.Pow(10, -20.0/20.0) // -20 dBFS
	for i := range input {
		input[i] = amp * math.Sin(2*math.Pi*1000*float64(i)/sr)
	}

	quantize := func(shaper NoiseShaper) []float64 {
		q, err := NewQuantizer(sr,
			WithBitDepth(8), // aggressive quantization for visible noise
			WithDitherType(DitherTriangular),
			WithNoiseShaper(shaper),
			WithRNG(rand.New(rand.NewPCG(seed, 0))),
		)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]float64, len(input))
		copy(out, input)
		q.ProcessInPlace(out)
		return out
	}

	// Compute quantization noise (output - input).
	noiseUnshaped := quantize(NewFIRShaper(nil))  // no shaping
	noiseShaped := quantize(NewFIRShaper(Preset9FC.Coefficients()))

	for i := range noiseUnshaped {
		noiseUnshaped[i] -= input[i]
		noiseShaped[i] -= input[i]
	}

	// Compute RMS of low-frequency noise (bins 0..nSamps/8, roughly 0-2.75 kHz).
	lowBins := nSamps / 8
	rmsLow := func(noise []float64) float64 {
		// Simple DFT energy in low bins (we don't need a full FFT for a test).
		var energy float64
		for k := 1; k < lowBins; k++ {
			var re, im float64
			freq := 2 * math.Pi * float64(k) / float64(nSamps)
			for n, v := range noise {
				re += v * math.Cos(freq*float64(n))
				im -= v * math.Sin(freq*float64(n))
			}
			energy += re*re + im*im
		}
		return math.Sqrt(energy / float64(lowBins))
	}

	unshaped := rmsLow(noiseUnshaped)
	shaped := rmsLow(noiseShaped)

	t.Logf("low-freq noise RMS: unshaped=%g, shaped=%g, ratio=%g",
		unshaped, shaped, shaped/unshaped)

	// Noise shaping should reduce low-frequency noise by at least 6 dB.
	if shaped >= unshaped*0.5 {
		t.Errorf("noise shaping did not reduce low-freq noise enough: ratio=%g (want < 0.5)",
			shaped/unshaped)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dsp/dither/ -run TestQuantizerNoiseShaping -v`
Expected: PASS or FAIL depending on shaper correctness (this is a validation test, not a red-green test).

**Step 3: If test fails, debug the shaper implementation**

The DFT-based spectral check validates the core algorithm. If it fails, the FIR shaper feedback loop has a bug.

**Step 4: Run test to verify it passes**

Run: `go test ./dsp/dither/ -run TestQuantizerNoiseShaping -v`
Expected: PASS with at least 6 dB reduction in low-frequency noise.

**Step 5: Commit**

```bash
git add dsp/dither/quantizer_test.go
git commit -m "test(dither): add spectral validation for noise shaping"
```

---

### Task 8: Setter round-trip tests and all dither type tests

**Files:**
- Modify: `dsp/dither/quantizer_test.go`

**Step 1: Write the failing test**

```go
func TestQuantizerSetters(t *testing.T) {
	q, _ := NewQuantizer(44100)

	if err := q.SetBitDepth(24); err != nil {
		t.Fatal(err)
	}
	if q.BitDepth() != 24 {
		t.Errorf("BitDepth = %d after Set", q.BitDepth())
	}

	if err := q.SetDitherType(DitherGaussian); err != nil {
		t.Fatal(err)
	}
	if q.DitherType() != DitherGaussian {
		t.Errorf("DitherType = %v after Set", q.DitherType())
	}

	if err := q.SetDitherAmplitude(0.5); err != nil {
		t.Fatal(err)
	}
	if q.DitherAmplitude() != 0.5 {
		t.Errorf("DitherAmplitude = %v after Set", q.DitherAmplitude())
	}

	q.SetLimit(false)
	if q.Limit() {
		t.Error("Limit should be false after Set")
	}
}

func TestQuantizerSetterValidation(t *testing.T) {
	q, _ := NewQuantizer(44100)

	if err := q.SetBitDepth(0); err == nil {
		t.Error("expected error for SetBitDepth(0)")
	}
	if err := q.SetBitDepth(33); err == nil {
		t.Error("expected error for SetBitDepth(33)")
	}
	if err := q.SetDitherType(DitherType(99)); err == nil {
		t.Error("expected error for invalid DitherType")
	}
	if err := q.SetDitherAmplitude(-1); err == nil {
		t.Error("expected error for negative amplitude")
	}
}

func TestQuantizerAllDitherTypes(t *testing.T) {
	types := []DitherType{
		DitherNone, DitherRectangular, DitherTriangular,
		DitherGaussian, DitherFastGaussian,
	}
	for _, dt := range types {
		t.Run(dt.String(), func(t *testing.T) {
			q, err := NewQuantizer(44100,
				WithDitherType(dt),
				WithRNG(rand.New(rand.NewPCG(42, 0))),
			)
			if err != nil {
				t.Fatal(err)
			}
			// Run 1000 samples, verify no NaN/Inf and that output is bounded.
			for i := 0; i < 1000; i++ {
				v := q.ProcessSample(0.3)
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("sample %d: %v", i, v)
				}
			}
		})
	}
}

func TestQuantizerSharpPreset(t *testing.T) {
	rates := []float64{40000, 44100, 48000, 64000, 96000}
	for _, sr := range rates {
		q, err := NewQuantizer(sr,
			WithSharpPreset(),
			WithDitherType(DitherNone),
		)
		if err != nil {
			t.Fatalf("sr=%g: %v", sr, err)
		}
		// Verify silence preservation.
		if got := q.ProcessSample(0); got != 0 {
			t.Errorf("sr=%g: ProcessSample(0) = %v", sr, got)
		}
	}
}

func TestQuantizerIIRShelf(t *testing.T) {
	q, err := NewQuantizer(44100,
		WithIIRShelf(10000),
		WithDitherType(DitherNone),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Verify silence preservation.
	if got := q.ProcessSample(0); got != 0 {
		t.Errorf("ProcessSample(0) = %v", got)
	}
}

func TestQuantizerNilOption(t *testing.T) {
	// Nil options should be skipped without panic.
	q, err := NewQuantizer(44100, nil, WithBitDepth(8), nil)
	if err != nil {
		t.Fatal(err)
	}
	if q.BitDepth() != 8 {
		t.Errorf("BitDepth = %d, want 8", q.BitDepth())
	}
}
```

**Step 2-4: Run, verify, fix as needed**

Run: `go test ./dsp/dither/ -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add dsp/dither/quantizer_test.go
git commit -m "test(dither): add setter, dither type, and option coverage tests"
```

---

### Task 9: Examples

**Files:**
- Create: `dsp/dither/example_test.go`

**Step 1: Write runnable examples**

```go
// dsp/dither/example_test.go
package dither_test

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/cwbudde/algo-dsp/dsp/dither"
)

func ExampleNewQuantizer() {
	q, err := dither.NewQuantizer(44100,
		dither.WithBitDepth(16),
		dither.WithDitherType(dither.DitherTriangular),
		dither.WithRNG(rand.New(rand.NewPCG(42, 0))),
	)
	if err != nil {
		panic(err)
	}

	// Quantize a sine wave sample.
	input := 0.5 * math.Sin(2*math.Pi*1000/44100)
	output := q.ProcessSample(input)
	fmt.Printf("input=%.6f output=%.6f\n", input, output)
	// Output will vary by RNG seed but should be close to input.
}

func ExampleQuantizer_ProcessInPlace() {
	q, _ := dither.NewQuantizer(44100,
		dither.WithBitDepth(16),
		dither.WithDitherType(dither.DitherNone),
		dither.WithFIRPreset(dither.PresetNone),
	)

	buf := []float64{0.0, 0.25, 0.5, 0.75, 1.0}
	q.ProcessInPlace(buf)
	for _, v := range buf {
		fmt.Printf("%.6f ", v)
	}
	fmt.Println()
}

func ExampleNewQuantizer_sharpPreset() {
	// The sharp preset automatically selects coefficients
	// optimized for the given sample rate.
	q, err := dither.NewQuantizer(48000,
		dither.WithSharpPreset(),
		dither.WithBitDepth(16),
	)
	if err != nil {
		panic(err)
	}
	_ = q // use q for processing
}
```

**Step 2: Run examples**

Run: `go test ./dsp/dither/ -run Example -v`
Expected: PASS

**Step 3: Commit**

```bash
git add dsp/dither/example_test.go
git commit -m "docs(dither): add runnable examples"
```

---

### Task 10: Benchmarks

**Files:**
- Create: `dsp/dither/quantizer_bench_test.go`

**Step 1: Write benchmarks**

```go
// dsp/dither/quantizer_bench_test.go
package dither

import (
	"math/rand/v2"
	"testing"
)

func BenchmarkQuantizerProcessSample(b *testing.B) {
	q, _ := NewQuantizer(44100,
		WithRNG(rand.New(rand.NewPCG(42, 0))),
	)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q.ProcessSample(0.3)
	}
}

func BenchmarkQuantizerProcessInPlace(b *testing.B) {
	q, _ := NewQuantizer(44100,
		WithRNG(rand.New(rand.NewPCG(42, 0))),
	)
	buf := make([]float64, 1024)
	rng := rand.New(rand.NewPCG(7, 0))
	for i := range buf {
		buf[i] = rng.Float64()*2 - 1
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q.ProcessInPlace(buf)
	}
}

func BenchmarkQuantizerNoDither(b *testing.B) {
	q, _ := NewQuantizer(44100,
		WithDitherType(DitherNone),
		WithFIRPreset(PresetNone),
	)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q.ProcessSample(0.3)
	}
}

func BenchmarkQuantizerIIRShelf(b *testing.B) {
	q, _ := NewQuantizer(44100,
		WithIIRShelf(10000),
		WithRNG(rand.New(rand.NewPCG(42, 0))),
	)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q.ProcessSample(0.3)
	}
}
```

**Step 2: Run benchmarks**

Run: `go test ./dsp/dither/ -bench=. -benchmem -count=3`
Expected: 0 allocs/op for all benchmarks.

**Step 3: Commit**

```bash
git add dsp/dither/quantizer_bench_test.go
git commit -m "bench(dither): add per-sample and block processing benchmarks"
```

---

### Task 11: Design package — ATH and critical bandwidth models

**Files:**
- Create: `dsp/dither/design/doc.go`
- Create: `dsp/dither/design/ath.go`
- Create: `dsp/dither/design/ath_test.go`

**Step 1: Write the failing test**

```go
// dsp/dither/design/ath_test.go
package design

import (
	"math"
	"testing"
)

func TestATH(t *testing.T) {
	// ATH should have a minimum around 3-4 kHz (most sensitive hearing range).
	ath1k := ATH(1000)
	ath3k := ATH(3500)
	ath10k := ATH(10000)

	if ath3k >= ath1k {
		t.Errorf("ATH(3.5kHz)=%g should be < ATH(1kHz)=%g", ath3k, ath1k)
	}
	if ath10k <= ath3k {
		t.Errorf("ATH(10kHz)=%g should be > ATH(3.5kHz)=%g", ath10k, ath3k)
	}
}

func TestATHLowFrequencyClamp(t *testing.T) {
	// Very low frequencies should not produce NaN/Inf.
	v := ATH(1)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		t.Errorf("ATH(1Hz) = %v", v)
	}
	v = ATH(0)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		t.Errorf("ATH(0Hz) = %v", v)
	}
}

func TestCriticalBandwidth(t *testing.T) {
	// Critical bandwidth increases with frequency.
	cb1k := CriticalBandwidth(1000)
	cb4k := CriticalBandwidth(4000)
	cb10k := CriticalBandwidth(10000)

	if cb1k <= 0 {
		t.Errorf("CriticalBandwidth(1kHz) = %g", cb1k)
	}
	if cb4k <= cb1k {
		t.Errorf("CB(4kHz)=%g should be > CB(1kHz)=%g", cb4k, cb1k)
	}
	if cb10k <= cb4k {
		t.Errorf("CB(10kHz)=%g should be > CB(4kHz)=%g", cb10k, cb4k)
	}
}

func TestCriticalBandwidthZero(t *testing.T) {
	// At 0 Hz: 25 + 75 * (1 + 0)^0.69 = 25 + 75 = 100
	got := CriticalBandwidth(0)
	if math.Abs(got-100) > 0.01 {
		t.Errorf("CriticalBandwidth(0) = %g, want ~100", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dsp/dither/design/ -run "TestATH|TestCritical" -v`
Expected: compilation failure.

**Step 3: Write minimal implementation**

`dsp/dither/design/doc.go`:
```go
// Package design provides a stochastic coefficient optimizer for
// psychoacoustically weighted noise-shaping filters.
package design
```

`dsp/dither/design/ath.go`:
```go
package design

import "math"

// ATH returns the absolute threshold of hearing in dB SPL at the given
// frequency in Hz. Based on Painter & Spanias (1997), modified by
// Gabriel Bouvigne.
func ATH(freqHz float64) float64 {
	f := math.Max(0.01, freqHz*0.001) // convert to kHz, clamp minimum
	return 3.640*math.Pow(f, -0.8) -
		6.800*math.Exp(-0.6*(f-3.4)*(f-3.4)) +
		6.000*math.Exp(-0.15*(f-8.7)*(f-8.7)) +
		0.0006*f*f*f*f
}

// CriticalBandwidth returns the critical bandwidth in Hz at the given
// frequency in Hz, per Zwicker (1982).
func CriticalBandwidth(freqHz float64) float64 {
	f := freqHz * 0.001 // convert to kHz
	return 25 + 75*math.Pow(1+1.4*f*f, 0.69)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./dsp/dither/design/ -run "TestATH|TestCritical" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dsp/dither/design/doc.go dsp/dither/design/ath.go dsp/dither/design/ath_test.go
git commit -m "feat(dither/design): add ATH and critical bandwidth models"
```

---

### Task 12: Design package — coefficient optimizer

**Files:**
- Create: `dsp/dither/design/designer.go`
- Create: `dsp/dither/design/designer_test.go`

**Step 1: Write the failing test**

```go
// dsp/dither/design/designer_test.go
package design

import (
	"context"
	"testing"
	"time"
)

func TestDesignerValidation(t *testing.T) {
	tests := []struct {
		name string
		sr   float64
		opts []DesignerOption
	}{
		{"zero sr", 0, nil},
		{"negative sr", -44100, nil},
		{"order too low", 44100, []DesignerOption{WithOrder(0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDesigner(tt.sr, tt.opts...)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestDesignerConverges(t *testing.T) {
	// Run the optimizer for a short time and verify it produces valid coefficients.
	d, err := NewDesigner(44100,
		WithOrder(5),
		WithIterations(500),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coeffs, err := d.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(coeffs) != 5 {
		t.Fatalf("got %d coefficients, want 5", len(coeffs))
	}

	// Coefficients should not all be zero (optimizer should have found something).
	allZero := true
	for _, c := range coeffs {
		if c != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("all coefficients are zero — optimizer did not converge")
	}
}

func TestDesignerCancellation(t *testing.T) {
	d, _ := NewDesigner(44100, WithOrder(8), WithIterations(1000000))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	coeffs, err := d.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Should return whatever best was found before cancellation.
	if len(coeffs) != 8 {
		t.Errorf("got %d coefficients, want 8", len(coeffs))
	}
}

func TestDesignerProgressCallback(t *testing.T) {
	var called int
	d, _ := NewDesigner(44100,
		WithOrder(3),
		WithIterations(200),
		WithOnProgress(func(coeffs []float64, score float64) {
			called++
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d.Run(ctx)
	// Callback should have been called at least once.
	if called == 0 {
		t.Error("progress callback was never called")
	}
}

func TestDesignerDeterministic(t *testing.T) {
	mk := func() []float64 {
		d, _ := NewDesigner(44100,
			WithOrder(3),
			WithIterations(100),
			WithSeed(42),
		)
		c, _ := d.Run(context.Background())
		return c
	}
	c1 := mk()
	c2 := mk()
	for i := range c1 {
		if c1[i] != c2[i] {
			t.Fatalf("coeff[%d]: %v != %v", i, c1[i], c2[i])
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dsp/dither/design/ -run TestDesigner -v`
Expected: compilation failure.

**Step 3: Write minimal implementation**

`dsp/dither/design/designer.go`:

The designer implements the stochastic search from the legacy code with these improvements:
- `context.Context` for cancellation (replaces infinite loop)
- Configurable iteration count (replaces `LoopCount`)
- Deterministic RNG via `WithSeed`
- Progress callback
- Uses Go's `math/cmplx` and a simple DFT (or the project's FFT if available) for spectral evaluation

Key algorithm (matches legacy):
1. Build ATH weight table: `ath[k] = 10^(ATH(f)*0.1) / CriticalBandwidth(f)` for each FFT bin
2. Store `iath[k] = 1/ath[k]`
3. Outer loop (until context cancelled):
   - Inner loop of `iterations` steps:
     - Perturb best coefficients with annealing: each coeff has 50% chance of being shifted by `uniform(-0.5, 0.5) * (iterations-i)/iterations`
     - Build impulse response `[1, c0, c1, ..., 0, 0, ...]`
     - FFT, compute squared magnitude per bin
     - Fitness = `max(|H[k]|^2 * iath[k])` over all bins
     - Keep candidate if fitness improved
   - If new global best found, fire callback
   - If `NTRYMAX=20` retries exhausted without improvement, reset and continue
4. Return best coefficients on context cancellation

FFT size: 64 (matching legacy's order 6), adequate for the short impulse responses.

```go
package design

import (
	"context"
	"fmt"
	"math"
	"math/cmplx"
	"math/rand/v2"
)

const (
	defaultOrder      = 8
	defaultIterations = 10000
	fftOrder          = 6
	fftSize           = 1 << fftOrder // 64
	fftSizeHalf       = fftSize / 2   // 32
	ntryMax           = 20
)

// ProgressFunc is called when the optimizer finds a new best coefficient set.
type ProgressFunc func(coeffs []float64, score float64)

// DesignerOption configures a Designer.
type DesignerOption func(*designerConfig) error

type designerConfig struct {
	order      int
	iterations int
	seed       uint64
	hasSeed    bool
	onProgress ProgressFunc
}

func WithOrder(n int) DesignerOption {
	return func(cfg *designerConfig) error {
		if n < 1 {
			return fmt.Errorf("design: order must be >= 1: %d", n)
		}
		cfg.order = n
		return nil
	}
}

func WithIterations(n int) DesignerOption {
	return func(cfg *designerConfig) error {
		if n < 1 {
			return fmt.Errorf("design: iterations must be >= 1: %d", n)
		}
		cfg.iterations = n
		return nil
	}
}

func WithOnProgress(fn ProgressFunc) DesignerOption {
	return func(cfg *designerConfig) error {
		cfg.onProgress = fn
		return nil
	}
}

func WithSeed(seed uint64) DesignerOption {
	return func(cfg *designerConfig) error {
		cfg.seed = seed
		cfg.hasSeed = true
		return nil
	}
}

// Designer finds optimal FIR noise-shaping coefficients using a stochastic
// search weighted by the absolute threshold of hearing.
type Designer struct {
	sampleRate float64
	order      int
	iterations int
	rng        *rand.Rand
	onProgress ProgressFunc
	iath       [fftSizeHalf]float64
}

// NewDesigner creates a new coefficient optimizer.
func NewDesigner(sampleRate float64, opts ...DesignerOption) (*Designer, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("design: sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := designerConfig{
		order:      defaultOrder,
		iterations: defaultIterations,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	var rng *rand.Rand
	if cfg.hasSeed {
		rng = rand.New(rand.NewPCG(cfg.seed, 0))
	} else {
		rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}

	d := &Designer{
		sampleRate: sampleRate,
		order:      cfg.order,
		iterations: cfg.iterations,
		rng:        rng,
		onProgress: cfg.onProgress,
	}

	// Build inverse ATH table.
	for i := 0; i < fftSizeHalf; i++ {
		freq := sampleRate * float64(i) / float64(fftSize)
		athVal := math.Pow(10, ATH(freq)*0.1) / CriticalBandwidth(freq)
		d.iath[i] = 1.0 / athVal
	}

	return d, nil
}

// Run executes the optimizer until ctx is cancelled or iterations complete.
// Returns the best coefficient set found.
func (d *Designer) Run(ctx context.Context) ([]float64, error) {
	best := make([]float64, d.order)
	xtry := make([]float64, d.order)
	bestGlobal := make([]float64, d.order)
	bestGlobalScore := math.MaxFloat64

	iterRecip := 1.0 / float64(d.iterations)

	for {
		// Check for cancellation.
		select {
		case <-ctx.Done():
			out := make([]float64, d.order)
			copy(out, bestGlobal)
			return out, nil
		default:
		}

		ntry := 0
		lastBestMax := math.MaxFloat64
		bestMax := math.MaxFloat64

		for ntry == 0 || (lastBestMax < bestMax && ntry < ntryMax) {
			for i := 0; i < d.iterations; i++ {
				// Check cancellation periodically.
				if i%1000 == 0 {
					select {
					case <-ctx.Done():
						out := make([]float64, d.order)
						copy(out, bestGlobal)
						return out, nil
					default:
					}
				}

				// Perturb best into xtry.
				anneal := float64(d.iterations-i) * iterRecip
				for j := 0; j < d.order; j++ {
					xtry[j] = best[j]
					if d.rng.IntN(2) == 0 {
						xtry[j] += (d.rng.Float64() - 0.5) * anneal
					}
				}

				// Evaluate fitness.
				score := d.evaluate(xtry)
				if score < bestMax {
					bestMax = score
					copy(best, xtry)
				}
			}
			ntry++
		}

		// Update global best.
		if bestMax < bestGlobalScore {
			bestGlobalScore = bestMax
			copy(bestGlobal, best)
			if d.onProgress != nil {
				out := make([]float64, d.order)
				copy(out, bestGlobal)
				d.onProgress(out, math.Log10(bestGlobalScore))
			}
		}

		if ntry >= ntryMax {
			lastBestMax = math.MaxFloat64
			continue
		}
		lastBestMax = bestMax
	}
}

// evaluate computes the ATH-weighted peak spectral energy for the given coefficients.
func (d *Designer) evaluate(coeffs []float64) float64 {
	// Build time-domain impulse response: [1, c0, c1, ..., 0, ...]
	// Then DFT (small enough that a direct DFT is fine for 64 points).
	var peak float64
	for k := 0; k < fftSizeHalf; k++ {
		// Compute DFT bin k.
		var re, im float64
		omega := 2 * math.Pi * float64(k) / float64(fftSize)
		// Bin 0: DC component.
		re = 1.0 // from the leading 1 in the impulse response
		for j := 0; j < d.order; j++ {
			angle := omega * float64(j+1)
			re += coeffs[j] * math.Cos(angle)
			im -= coeffs[j] * math.Sin(angle)
		}
		mag2 := re*re + im*im
		weighted := mag2 * d.iath[k]
		if weighted > peak {
			peak = weighted
		}
	}
	return peak
}
```

Note: We use a direct DFT instead of FFT since fftSize=64 is small enough (64*32 = 2048 multiply-adds per evaluation). This avoids a dependency on `algo-fft` in the design package. For production use with larger FFT sizes, the FFT dependency could be added later.

The `cmplx` import can be removed if not used directly (kept for reference).

**Step 4: Run test to verify it passes**

Run: `go test ./dsp/dither/design/ -v -timeout 30s`
Expected: PASS

**Step 5: Commit**

```bash
git add dsp/dither/design/designer.go dsp/dither/design/designer_test.go
git commit -m "feat(dither/design): add stochastic ATH-weighted coefficient optimizer"
```

---

### Task 13: Design package — examples

**Files:**
- Create: `dsp/dither/design/example_test.go`

**Step 1: Write runnable examples**

```go
// dsp/dither/design/example_test.go
package design_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cwbudde/algo-dsp/dsp/dither/design"
)

func ExampleDesigner() {
	d, err := design.NewDesigner(44100,
		design.WithOrder(5),
		design.WithIterations(1000),
		design.WithSeed(42),
	)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	coeffs, err := d.Run(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Found %d coefficients\n", len(coeffs))
	// Output: Found 5 coefficients
}
```

**Step 2: Run example**

Run: `go test ./dsp/dither/design/ -run Example -v -timeout 30s`
Expected: PASS

**Step 3: Commit**

```bash
git add dsp/dither/design/example_test.go
git commit -m "docs(dither/design): add runnable example for coefficient designer"
```

---

### Task 14: Race detector and final validation

**Files:** No new files.

**Step 1: Run all tests with race detector**

Run: `go test -race ./dsp/dither/... -v -timeout 60s`
Expected: PASS with no race conditions.

**Step 2: Run benchmarks**

Run: `go test ./dsp/dither/ -bench=. -benchmem -count=3`
Expected: 0 allocs/op for processing benchmarks.

**Step 3: Run full project test suite**

Run: `go test -race ./... -timeout 120s`
Expected: PASS — no regressions.

**Step 4: Commit any fixes**

If any issues arise, fix and commit individually.

---

### Task 15: Update PLAN.md checkboxes

**Files:**
- Modify: `PLAN.md` (check off Phase 29 tasks)

**Step 1: Update checkboxes**

Mark all Phase 29 task checkboxes as complete in PLAN.md.

**Step 2: Commit**

```bash
git add PLAN.md
git commit -m "docs: mark Phase 29 (Dither and Noise Shaping) tasks as complete"
```

---

## Task Dependency Graph

```
Task 1 (enum) ──┐
                 ├── Task 3 (FIR shaper) ──┐
Task 2 (presets)─┤                         ├── Task 6 (Quantizer) ── Task 7 (spectral) ── Task 8 (setters)
                 ├── Task 4 (IIR shaper) ──┘
                 └── Task 5 (options) ──────┘
                                            ├── Task 9 (examples)
                                            └── Task 10 (benchmarks)

Task 11 (ATH) ── Task 12 (designer) ── Task 13 (design examples)

Task 14 (race/validation) depends on all above.
Task 15 (PLAN.md) depends on Task 14.
```

Tasks 1-5 can be partially parallelized (1 and 2 are independent; 3-5 depend on 1+2).
Tasks 11-13 are independent of Tasks 1-10 and can be done in parallel.

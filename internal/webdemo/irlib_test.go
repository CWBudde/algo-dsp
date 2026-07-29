package webdemo

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// --- Test-only IRLB writer -------------------------------------------------
//
// irlib.go is a reader with no writer anywhere in the repo, so round-trip
// coverage needs one here. Keeping it in the test file also means the expected
// byte layout is written down independently of the parser, rather than the
// tests simply agreeing with whatever the parser happens to do.

type testIR struct {
	name        string
	description string
	category    string
	sampleRate  float64
	tags        []string
	// samples[ch][frame]; all channels must be the same length.
	samples [][]float32
}

func writeString(buf *bytes.Buffer, s string) {
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(s)))
	buf.WriteString(s)
}

// encodeF16 converts a float32 to IEEE 754 half precision.
//
// Only the cases the fixtures need are handled exactly: zero, normals, and
// values that round to subnormal. This is the inverse of decodeF16 for the
// range the tests exercise.
func encodeF16(f float32) uint16 {
	bits := math.Float32bits(f)
	sign := uint16(bits >> 31 << 15)
	exp := int((bits>>23)&0xFF) - 127
	frac := bits & 0x7FFFFF

	switch {
	case math.IsInf(float64(f), 0):
		return sign | 0x7C00
	case math.IsNaN(float64(f)):
		return sign | 0x7E00
	case f == 0:
		return sign
	case exp < -24:
		// Underflows to zero.
		return sign
	case exp < -14:
		// Subnormal.
		shift := uint32(-exp - 14)

		return sign | uint16((frac|0x800000)>>(14+shift))
	case exp > 15:
		return sign | 0x7C00
	default:
		return sign | uint16(exp+15)<<10 | uint16(frac>>13)
	}
}

// buildIRLib serialises irs into the IRLB container format.
func buildIRLib(t *testing.T, irs []testIR) []byte {
	t.Helper()

	var body bytes.Buffer

	const headerSize = 18

	offsets := make([]uint64, len(irs))

	for i, ir := range irs {
		offsets[i] = headerSize + uint64(body.Len())

		// META sub-chunk body.
		var meta bytes.Buffer

		frames := 0
		if len(ir.samples) > 0 {
			frames = len(ir.samples[0])
		}

		_ = binary.Write(&meta, binary.LittleEndian, ir.sampleRate)
		_ = binary.Write(&meta, binary.LittleEndian, uint32(len(ir.samples)))
		_ = binary.Write(&meta, binary.LittleEndian, uint32(frames))
		writeString(&meta, ir.name)
		writeString(&meta, ir.description)
		writeString(&meta, ir.category)
		_ = binary.Write(&meta, binary.LittleEndian, uint16(len(ir.tags)))

		for _, tag := range ir.tags {
			writeString(&meta, tag)
		}

		// AUDI sub-chunk body: interleaved half-precision samples.
		var audi bytes.Buffer

		for frame := range frames {
			for ch := range ir.samples {
				_ = binary.Write(&audi, binary.LittleEndian, encodeF16(ir.samples[ch][frame]))
			}
		}

		// chunkSize counts each sub-chunk's 8-byte header plus its body,
		// matching how readIRChunk advances chunkRead.
		chunkSize := uint64(8+meta.Len()) + uint64(8+audi.Len())

		body.WriteString("IR--")
		_ = binary.Write(&body, binary.LittleEndian, chunkSize)

		body.WriteString("META")
		_ = binary.Write(&body, binary.LittleEndian, uint32(meta.Len()))
		body.Write(meta.Bytes())

		body.WriteString("AUDI")
		_ = binary.Write(&body, binary.LittleEndian, uint32(audi.Len()))
		body.Write(audi.Bytes())
	}

	indexOffset := headerSize + uint64(body.Len())

	var index bytes.Buffer

	for i, ir := range irs {
		frames := 0
		if len(ir.samples) > 0 {
			frames = len(ir.samples[0])
		}

		_ = binary.Write(&index, binary.LittleEndian, offsets[i])
		_ = binary.Write(&index, binary.LittleEndian, ir.sampleRate)
		_ = binary.Write(&index, binary.LittleEndian, uint32(len(ir.samples)))
		_ = binary.Write(&index, binary.LittleEndian, uint32(frames))
		writeString(&index, ir.name)
		writeString(&index, ir.category)
	}

	var out bytes.Buffer

	out.WriteString("IRLB")
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(irs)))
	_ = binary.Write(&out, binary.LittleEndian, indexOffset)
	out.Write(body.Bytes())
	out.WriteString("INDX")
	_ = binary.Write(&out, binary.LittleEndian, uint64(index.Len()))
	out.Write(index.Bytes())

	if uint64(out.Len()) < indexOffset {
		t.Fatalf("builder produced %d bytes, index offset %d", out.Len(), indexOffset)
	}

	return out.Bytes()
}

// --- decodeF16 -------------------------------------------------------------

func TestDecodeF16(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		bits uint16
		want float32
	}{
		{"zero", 0x0000, 0},
		{"negative zero", 0x8000, float32(math.Copysign(0, -1))},
		{"one", 0x3C00, 1},
		{"negative two", 0xC000, -2},
		{"half", 0x3800, 0.5},
		{"largest normal", 0x7BFF, 65504},
		{"smallest normal", 0x0400, 1.0 / 16384.0},         // 2^-14
		{"largest subnormal", 0x03FF, 1023.0 / 16777216.0}, // 1023 * 2^-24
		{"smallest subnormal", 0x0001, 1.0 / 16777216.0},   // 2^-24
		{"negative subnormal", 0x8001, -1.0 / 16777216.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := decodeF16(tc.bits)
			if got != tc.want {
				t.Fatalf("decodeF16(%#04x) = %v, want %v", tc.bits, got, tc.want)
			}

			// Signed zero must survive: 0 == -0 compares equal, so check the
			// sign bit explicitly.
			if tc.want == 0 && math.Signbit(float64(got)) != math.Signbit(float64(tc.want)) {
				t.Fatalf("decodeF16(%#04x) sign mismatch: got %v", tc.bits, got)
			}
		})
	}
}

func TestDecodeF16NonFinite(t *testing.T) {
	t.Parallel()

	if got := decodeF16(0x7C00); !math.IsInf(float64(got), 1) {
		t.Fatalf("decodeF16(0x7C00) = %v, want +Inf", got)
	}

	if got := decodeF16(0xFC00); !math.IsInf(float64(got), -1) {
		t.Fatalf("decodeF16(0xFC00) = %v, want -Inf", got)
	}

	if got := decodeF16(0x7E00); !math.IsNaN(float64(got)) {
		t.Fatalf("decodeF16(0x7E00) = %v, want NaN", got)
	}
}

// TestDecodeF16SubnormalsAreMonotonic guards the hand-written subnormal
// normalisation loop: every step across the subnormal range must increase by
// exactly 2^-24, and the range must join the normals without a discontinuity.
func TestDecodeF16SubnormalsAreMonotonic(t *testing.T) {
	t.Parallel()

	const step = 1.0 / 16777216.0 // 2^-24

	for bits := uint16(1); bits <= 0x03FF; bits++ {
		got := float64(decodeF16(bits))
		want := float64(bits) * step

		if math.Abs(got-want) > 1e-12 {
			t.Fatalf("decodeF16(%#04x) = %v, want %v", bits, got, want)
		}
	}

	// The smallest normal continues the same progression.
	if got, want := float64(decodeF16(0x0400)), 1024*step; math.Abs(got-want) > 1e-12 {
		t.Fatalf("decodeF16(0x0400) = %v, want %v", got, want)
	}
}

func TestEncodeDecodeF16RoundTrip(t *testing.T) {
	t.Parallel()

	// Values exactly representable in half precision.
	values := []float32{0, 1, -1, 0.5, -0.25, 2, -8, 0.001953125, 65504, -65504}

	for _, want := range values {
		if got := decodeF16(encodeF16(want)); got != want {
			t.Fatalf("round trip of %v gave %v", want, got)
		}
	}
}

// --- readIRLib -------------------------------------------------------------

func TestReadIRLibRoundTrip(t *testing.T) {
	t.Parallel()

	fixtures := []testIR{
		{
			name:        "Mono Room",
			description: "a description that the parser discards",
			category:    "Rooms",
			sampleRate:  48000,
			tags:        []string{"small", "bright"},
			samples:     [][]float32{{1, 0.5, -0.25, 0}},
		},
		{
			name:       "Stereo Hall",
			category:   "Halls",
			sampleRate: 44100,
			samples: [][]float32{
				{1, 0.5, 0.25},
				{-1, -0.5, -0.25},
			},
		},
	}

	data := buildIRLib(t, fixtures)

	irs, err := readIRLib(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("readIRLib: %v", err)
	}

	if len(irs) != len(fixtures) {
		t.Fatalf("got %d IRs, want %d", len(irs), len(fixtures))
	}

	for i, want := range fixtures {
		got := irs[i]

		if got.Name != want.name {
			t.Errorf("IR %d name = %q, want %q", i, got.Name, want.name)
		}

		if got.Category != want.category {
			t.Errorf("IR %d category = %q, want %q", i, got.Category, want.category)
		}

		if got.SampleRate != want.sampleRate {
			t.Errorf("IR %d sample rate = %v, want %v", i, got.SampleRate, want.sampleRate)
		}

		if got.Channels != len(want.samples) {
			t.Fatalf("IR %d channels = %d, want %d", i, got.Channels, len(want.samples))
		}

		if len(got.Samples) != len(want.samples) {
			t.Fatalf("IR %d has %d sample slices, want %d", i, len(got.Samples), len(want.samples))
		}

		for ch := range want.samples {
			if len(got.Samples[ch]) != len(want.samples[ch]) {
				t.Fatalf("IR %d channel %d has %d frames, want %d",
					i, ch, len(got.Samples[ch]), len(want.samples[ch]))
			}

			for frame, wantSample := range want.samples[ch] {
				if got.Samples[ch][frame] != float64(wantSample) {
					t.Errorf("IR %d channel %d frame %d = %v, want %v",
						i, ch, frame, got.Samples[ch][frame], wantSample)
				}
			}
		}
	}
}

// TestReadIRLibDeinterleaves pins the channel/frame mapping, which is easy to
// get backwards and would silently swap or interleave channels.
func TestReadIRLibDeinterleaves(t *testing.T) {
	t.Parallel()

	data := buildIRLib(t, []testIR{{
		name:       "Interleave",
		category:   "Test",
		sampleRate: 48000,
		samples: [][]float32{
			{1, 2, 4},
			{0.5, 0.25, 0.125},
		},
	}})

	irs, err := readIRLib(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("readIRLib: %v", err)
	}

	left := irs[0].Samples[0]
	right := irs[0].Samples[1]

	if want := []float64{1, 2, 4}; !equalFloat64(left, want) {
		t.Errorf("left channel = %v, want %v", left, want)
	}

	if want := []float64{0.5, 0.25, 0.125}; !equalFloat64(right, want) {
		t.Errorf("right channel = %v, want %v", right, want)
	}
}

func TestReadIRLibRejectsMalformedHeaders(t *testing.T) {
	t.Parallel()

	valid := buildIRLib(t, []testIR{{
		name:       "One",
		category:   "Test",
		sampleRate: 48000,
		samples:    [][]float32{{1, 0.5}},
	}})

	tests := []struct {
		name    string
		corrupt func([]byte) []byte
	}{
		{
			name:    "empty input",
			corrupt: func([]byte) []byte { return nil },
		},
		{
			name: "bad magic",
			corrupt: func(b []byte) []byte {
				out := append([]byte(nil), b...)
				copy(out, "XXXX")

				return out
			},
		},
		{
			name: "unsupported version",
			corrupt: func(b []byte) []byte {
				out := append([]byte(nil), b...)
				binary.LittleEndian.PutUint16(out[4:], 2)

				return out
			},
		},
		{
			name:    "truncated header",
			corrupt: func(b []byte) []byte { return b[:10] },
		},
		{
			name: "index offset past end of file",
			corrupt: func(b []byte) []byte {
				out := append([]byte(nil), b...)
				binary.LittleEndian.PutUint64(out[10:], uint64(len(b))+4096)

				return out
			},
		},
		{
			name: "index chunk magic wrong",
			corrupt: func(b []byte) []byte {
				out := append([]byte(nil), b...)
				offset := binary.LittleEndian.Uint64(out[10:])
				copy(out[offset:], "BAAD")

				return out
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := readIRLib(bytes.NewReader(tc.corrupt(valid)))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

// TestReadIRLibSkipsUnreadableChunks documents the deliberate non-fatal path in
// readIRLib: one corrupt IR must not take the rest of the library down.
func TestReadIRLibSkipsUnreadableChunks(t *testing.T) {
	t.Parallel()

	data := buildIRLib(t, []testIR{
		{name: "Good", category: "Test", sampleRate: 48000, samples: [][]float32{{1, 0.5}}},
		{name: "Broken", category: "Test", sampleRate: 48000, samples: [][]float32{{1, 0.5}}},
	})

	// Point the second index entry's offset at a location holding no IR-- magic.
	indexOffset := binary.LittleEndian.Uint64(data[10:])
	// Skip "INDX" + size, then the first entry.
	firstEntryStart := indexOffset + 4 + 8
	firstEntryLen := uint64(8 + 8 + 4 + 4 + 2 + len("Good") + 2 + len("Test"))
	binary.LittleEndian.PutUint64(data[firstEntryStart+firstEntryLen:], 4)

	irs, err := readIRLib(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("readIRLib: %v", err)
	}

	if len(irs) != 1 {
		t.Fatalf("got %d IRs, want 1 (the readable one)", len(irs))
	}

	if irs[0].Name != "Good" {
		t.Fatalf("surviving IR is %q, want %q", irs[0].Name, "Good")
	}
}

func TestReadIRLibSkipsUnknownSubChunks(t *testing.T) {
	t.Parallel()

	// Unknown sub-chunks must be seeked past, not treated as an error: that is
	// the format's only forward-compatibility mechanism.
	var extra bytes.Buffer

	extra.WriteString("XTRA")
	_ = binary.Write(&extra, binary.LittleEndian, uint32(4))
	extra.Write([]byte{1, 2, 3, 4})

	data := buildIRLib(t, []testIR{{
		name: "WithExtra", category: "Test", sampleRate: 48000,
		samples: [][]float32{{1, 0.5}},
	}})

	// Splice the unknown sub-chunk in after the IR-- header and grow chunkSize.
	const irChunkStart = 18

	chunkSize := binary.LittleEndian.Uint64(data[irChunkStart+4:])
	binary.LittleEndian.PutUint64(data[irChunkStart+4:], chunkSize+uint64(extra.Len()))

	insertAt := irChunkStart + 12
	spliced := make([]byte, 0, len(data)+extra.Len())
	spliced = append(spliced, data[:insertAt]...)
	spliced = append(spliced, extra.Bytes()...)
	spliced = append(spliced, data[insertAt:]...)

	// The index offset and the entry offset both shift by the inserted bytes.
	indexOffset := binary.LittleEndian.Uint64(spliced[10:])
	binary.LittleEndian.PutUint64(spliced[10:], indexOffset+uint64(extra.Len()))

	irs, err := readIRLib(bytes.NewReader(spliced))
	if err != nil {
		t.Fatalf("readIRLib: %v", err)
	}

	if len(irs) != 1 {
		t.Fatalf("got %d IRs, want 1", len(irs))
	}

	if want := []float64{1, 0.5}; !equalFloat64(irs[0].Samples[0], want) {
		t.Fatalf("samples = %v, want %v", irs[0].Samples[0], want)
	}
}

// --- The embedded library --------------------------------------------------

// TestEmbeddedIRLib parses the library that actually ships inside the WASM
// binary. It is the demo's convolution reverb source, and until now nothing
// checked that it was even readable.
func TestEmbeddedIRLib(t *testing.T) {
	t.Parallel()

	lib, err := loadEmbeddedIRLib()
	if err != nil {
		t.Fatalf("loadEmbeddedIRLib: %v", err)
	}

	if len(lib.IRs) == 0 {
		t.Fatal("embedded library contains no IRs")
	}

	names := lib.IRNames()
	if len(names) != len(lib.IRs) {
		t.Fatalf("IRNames returned %d names for %d IRs", len(names), len(lib.IRs))
	}

	seen := make(map[string]bool, len(names))

	for i, ir := range lib.IRs {
		if ir.Name == "" {
			t.Errorf("IR %d has an empty name", i)
		}

		if seen[ir.Name] {
			t.Errorf("duplicate IR name %q", ir.Name)
		}

		seen[ir.Name] = true

		if ir.SampleRate < 8000 || ir.SampleRate > 384000 {
			t.Errorf("IR %q has implausible sample rate %v", ir.Name, ir.SampleRate)
		}

		if ir.Channels < 1 || ir.Channels > 8 {
			t.Errorf("IR %q has %d channels", ir.Name, ir.Channels)
		}

		if len(ir.Samples) != ir.Channels {
			t.Errorf("IR %q declares %d channels but carries %d", ir.Name, ir.Channels, len(ir.Samples))
		}

		peak := 0.0

		for ch, channel := range ir.Samples {
			if len(channel) == 0 {
				t.Errorf("IR %q channel %d is empty", ir.Name, ch)
			}

			for _, v := range channel {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("IR %q channel %d contains a non-finite sample", ir.Name, ch)
				}

				peak = math.Max(peak, math.Abs(v))
			}
		}

		// A silent IR would make the convolution reverb inaudible.
		if peak == 0 {
			t.Errorf("IR %q is entirely silent", ir.Name)
		}
	}
}

func TestIRLibraryGetIR(t *testing.T) {
	t.Parallel()

	lib := &IRLibrary{IRs: []IRData{
		{IREntry: IREntry{Name: "first"}},
		{IREntry: IREntry{Name: "second"}},
	}}

	if got := lib.GetIR(0); got == nil || got.Name != "first" {
		t.Errorf("GetIR(0) = %v, want the first entry", got)
	}

	if got := lib.GetIR(1); got == nil || got.Name != "second" {
		t.Errorf("GetIR(1) = %v, want the second entry", got)
	}

	for _, index := range []int{-1, 2, 100} {
		if got := lib.GetIR(index); got != nil {
			t.Errorf("GetIR(%d) = %v, want nil", index, got)
		}
	}
}

func equalFloat64(got, want []float64) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

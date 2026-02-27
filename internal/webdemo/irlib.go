package webdemo

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

//go:embed data/irs.irlib
var embeddedIRLib []byte

// loadEmbeddedIRLib parses the bundled IRLB file and returns an IRLibrary.
func loadEmbeddedIRLib() (*IRLibrary, error) {
	irs, err := readIRLib(bytes.NewReader(embeddedIRLib))
	if err != nil {
		return nil, fmt.Errorf("irlib: loading embedded library: %w", err)
	}

	return &IRLibrary{IRs: irs}, nil
}

// IREntry holds metadata for one impulse response.
type IREntry struct {
	Name       string
	Category   string
	SampleRate float64
	Channels   int
	Length     int // samples per channel
}

// IRData holds a loaded impulse response.
type IRData struct {
	IREntry

	// Samples[ch] is the mono float64 slice for channel ch.
	Samples [][]float64
}

// IRLibrary holds pre-loaded IRs for the web demo.
type IRLibrary struct {
	IRs []IRData
}

// IRNames returns the list of IR names.
func (lib *IRLibrary) IRNames() []string {
	names := make([]string, len(lib.IRs))
	for i, ir := range lib.IRs {
		names[i] = ir.Name
	}

	return names
}

// GetIR returns the IR at index, or nil if out of range.
func (lib *IRLibrary) GetIR(index int) *IRData {
	if index < 0 || index >= len(lib.IRs) {
		return nil
	}

	return &lib.IRs[index]
}

// decodeF16 converts an IEEE 754 half-precision float16 (uint16) to float32.
func decodeF16(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := int((h >> 10) & 0x1F)
	frac := uint32(h & 0x3FF)

	var bits uint32

	switch exp {
	case 0:
		if frac == 0 {
			bits = sign
		} else {
			// Subnormal: normalize it.
			e, m := 0, frac
			for m&0x400 == 0 {
				m <<= 1
				e++
			}

			bits = sign | uint32(127-14-e+1)<<23 | (m&0x3FF)<<13
		}
	case 31:
		// Inf or NaN.
		bits = sign | 0x7F800000 | frac<<13
	default:
		bits = sign | uint32(exp+112)<<23 | frac<<13
	}

	return math.Float32frombits(bits)
}

// readString reads a uint16-length-prefixed UTF-8 string from r.
func readString(r io.Reader) (string, error) {
	var length uint16

	err := binary.Read(r, binary.LittleEndian, &length)
	if err != nil {
		return "", fmt.Errorf("irlib: reading string length: %w", err)
	}

	if length == 0 {
		return "", nil
	}

	buf := make([]byte, length)

	_, err = io.ReadFull(r, buf)
	if err != nil {
		return "", fmt.Errorf("irlib: reading string bytes: %w", err)
	}

	return string(buf), nil
}

// indexEntry holds the raw index data for one IR.
type indexEntry struct {
	offset     uint64
	sampleRate float64
	channels   uint32
	length     uint32
	name       string
	category   string
}

// readIRLib reads an IRLB file from r and returns all IRs.
//
//nolint:cyclop
func readIRLib(r io.ReadSeeker) ([]IRData, error) {
	// --- File header (18 bytes) ---
	var magic [4]byte

	_, err := io.ReadFull(r, magic[:])
	if err != nil {
		return nil, fmt.Errorf("irlib: reading magic: %w", err)
	}

	if magic != [4]byte{'I', 'R', 'L', 'B'} {
		return nil, fmt.Errorf("irlib: invalid magic %q", magic)
	}

	var version uint16

	err = binary.Read(r, binary.LittleEndian, &version)
	if err != nil {
		return nil, fmt.Errorf("irlib: reading version: %w", err)
	}

	if version != 1 {
		return nil, fmt.Errorf("irlib: unsupported version %d", version)
	}

	var irCount uint32

	err = binary.Read(r, binary.LittleEndian, &irCount)
	if err != nil {
		return nil, fmt.Errorf("irlib: reading ir_count: %w", err)
	}

	var indexOffset uint64

	err = binary.Read(r, binary.LittleEndian, &indexOffset)
	if err != nil {
		return nil, fmt.Errorf("irlib: reading index_offset: %w", err)
	}

	// --- INDEX chunk ---
	_, err = r.Seek(int64(indexOffset), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("irlib: seeking to index: %w", err)
	}

	var indxMagic [4]byte

	_, err = io.ReadFull(r, indxMagic[:])
	if err != nil {
		return nil, fmt.Errorf("irlib: reading INDX magic: %w", err)
	}

	if indxMagic != [4]byte{'I', 'N', 'D', 'X'} {
		return nil, fmt.Errorf("irlib: expected INDX chunk, got %q", indxMagic)
	}

	var indxSize uint64

	err = binary.Read(r, binary.LittleEndian, &indxSize)
	if err != nil {
		return nil, fmt.Errorf("irlib: reading INDX size: %w", err)
	}

	// Read all index entries.
	entries := make([]indexEntry, 0, irCount)
	indxBodyRead := uint64(0)

	for indxBodyRead < indxSize {
		var entry indexEntry

		err = binary.Read(r, binary.LittleEndian, &entry.offset)
		if err != nil {
			return nil, fmt.Errorf("irlib: reading index entry offset: %w", err)
		}

		indxBodyRead += 8

		err = binary.Read(r, binary.LittleEndian, &entry.sampleRate)
		if err != nil {
			return nil, fmt.Errorf("irlib: reading index entry sampleRate: %w", err)
		}

		indxBodyRead += 8

		err = binary.Read(r, binary.LittleEndian, &entry.channels)
		if err != nil {
			return nil, fmt.Errorf("irlib: reading index entry channels: %w", err)
		}

		indxBodyRead += 4

		err = binary.Read(r, binary.LittleEndian, &entry.length)
		if err != nil {
			return nil, fmt.Errorf("irlib: reading index entry length: %w", err)
		}

		indxBodyRead += 4

		name, err := readString(r)
		if err != nil {
			return nil, fmt.Errorf("irlib: reading index entry name: %w", err)
		}

		indxBodyRead += uint64(2 + len(name))
		entry.name = name

		category, err := readString(r)
		if err != nil {
			return nil, fmt.Errorf("irlib: reading index entry category: %w", err)
		}

		indxBodyRead += uint64(2 + len(category))
		entry.category = category

		entries = append(entries, entry)
	}

	// --- Read each IR chunk ---
	result := make([]IRData, 0, len(entries))

	for _, entry := range entries {
		irData, err := readIRChunk(r, entry)
		if err != nil {
			// Non-fatal: skip bad chunks.
			continue
		}

		result = append(result, irData)
	}

	return result, nil
}

// readIRChunk seeks to entry.offset and reads one IR-- chunk.
func readIRChunk(r io.ReadSeeker, entry indexEntry) (IRData, error) {
	_, err := r.Seek(int64(entry.offset), io.SeekStart)
	if err != nil {
		return IRData{}, fmt.Errorf("irlib: seeking to IR at %d: %w", entry.offset, err)
	}

	var irMagic [4]byte

	_, err = io.ReadFull(r, irMagic[:])
	if err != nil {
		return IRData{}, fmt.Errorf("irlib: reading IR magic: %w", err)
	}

	if irMagic != [4]byte{'I', 'R', '-', '-'} {
		return IRData{}, fmt.Errorf("irlib: expected IR-- at offset %d, got %q", entry.offset, irMagic)
	}

	var chunkSize uint64

	err = binary.Read(r, binary.LittleEndian, &chunkSize)
	if err != nil {
		return IRData{}, fmt.Errorf("irlib: reading IR chunk size: %w", err)
	}

	// Track how many bytes of this chunk we've consumed (past the 12-byte header).
	var chunkRead uint64

	var (
		meta    IREntry
		samples [][]float64
	)

	hasMeta := false
	hasAudio := false

	for chunkRead < chunkSize {
		var subMagic [4]byte

		_, err = io.ReadFull(r, subMagic[:])
		if err != nil {
			break
		}

		chunkRead += 4

		var subSize uint32

		err := binary.Read(r, binary.LittleEndian, &subSize)
		if err != nil {
			break
		}

		chunkRead += 4

		switch subMagic {
		case [4]byte{'M', 'E', 'T', 'A'}:
			var sampleRate float64

			err = binary.Read(r, binary.LittleEndian, &sampleRate)
			if err != nil {
				return IRData{}, fmt.Errorf("irlib: reading META sampleRate: %w", err)
			}

			var channels uint32

			err = binary.Read(r, binary.LittleEndian, &channels)
			if err != nil {
				return IRData{}, fmt.Errorf("irlib: reading META channels: %w", err)
			}

			var length uint32

			err = binary.Read(r, binary.LittleEndian, &length)
			if err != nil {
				return IRData{}, fmt.Errorf("irlib: reading META length: %w", err)
			}

			name, err := readString(r)
			if err != nil {
				return IRData{}, fmt.Errorf("irlib: reading META name: %w", err)
			}

			description, err := readString(r)
			if err != nil {
				return IRData{}, fmt.Errorf("irlib: reading META description: %w", err)
			}

			_ = description

			category, err := readString(r)
			if err != nil {
				return IRData{}, fmt.Errorf("irlib: reading META category: %w", err)
			}

			var tagCount uint16

			err = binary.Read(r, binary.LittleEndian, &tagCount)
			if err != nil {
				return IRData{}, fmt.Errorf("irlib: reading META tag count: %w", err)
			}

			for i := range int(tagCount) {
				_ = i

				_, err := readString(r)
				if err != nil {
					return IRData{}, fmt.Errorf("irlib: reading META tag: %w", err)
				}
			}

			meta = IREntry{
				Name:       name,
				Category:   category,
				SampleRate: sampleRate,
				Channels:   int(channels),
				Length:     int(length),
			}
			hasMeta = true

		case [4]byte{'A', 'U', 'D', 'I'}:
			rawAudio := make([]byte, subSize)

			_, err := io.ReadFull(r, rawAudio)
			if err != nil {
				return IRData{}, fmt.Errorf("irlib: reading AUDI data: %w", err)
			}

			channels := int(entry.channels)
			if hasMeta {
				channels = meta.Channels
			}

			totalSamples := len(rawAudio) / 2

			frames := totalSamples / channels
			if frames == 0 {
				break
			}

			// Allocate per-channel slices.
			samples = make([][]float64, channels)
			for ch := range channels {
				samples[ch] = make([]float64, frames)
			}

			// Decode interleaved f16 samples.
			for i := range totalSamples {
				h := binary.LittleEndian.Uint16(rawAudio[i*2 : i*2+2])
				val := float64(decodeF16(h))
				ch := i % channels

				frame := i / channels
				if frame < frames {
					samples[ch][frame] = val
				}
			}

			hasAudio = true

		default:
			// Skip unknown sub-chunks.
			_, err = r.Seek(int64(subSize), io.SeekCurrent)
			if err != nil {
				return IRData{}, fmt.Errorf("irlib: skipping sub-chunk %q: %w", subMagic, err)
			}
		}

		chunkRead += uint64(subSize)
	}

	if !hasMeta || !hasAudio {
		return IRData{}, fmt.Errorf("irlib: incomplete IR chunk for %q", entry.name)
	}

	if samples == nil {
		return IRData{}, fmt.Errorf("irlib: no audio decoded for %q", entry.name)
	}

	return IRData{
		IREntry: meta,
		Samples: samples,
	}, nil
}

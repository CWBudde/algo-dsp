package loudness_test

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/loudness"
	"github.com/cwbudde/algo-dsp/measure/truepeak"
)

// The original EBU fixtures cannot be redistributed. Set ALGO_DSP_EBU_TEST_SET
// to an extracted v5.0 archive to run all 66 Tech 3341/3342 sequences (© EBU).
// Source: https://tech.ebu.ch/publications/ebu_loudness_test_set
// Archive: ebu-loudness-test-setv05.zip, 91631421 bytes, SHA512:
// 60d022fdac47ad0be2688411be9daecbff85da994d6fa4921bba6cffab841b081d8b15d9ce284ad2253efb686463450a84a0d19cb0bad7a934546cc52dd73771
// Expectations and tolerances come from the 2023 editions of EBU Tech 3341,
// Table 1, and EBU Tech 3342, Table 1. No algorithm generates its own reference.
func TestStreamingEBUPublishedSequences(t *testing.T) {
	root := os.Getenv("ALGO_DSP_EBU_TEST_SET")
	if root == "" {
		t.Skip("set ALGO_DSP_EBU_TEST_SET to the original EBU v5.0 fixtures (© EBU)")
	}

	paths, err := filepath.Glob(filepath.Join(root, "seq-*.wav*"))
	if err != nil || len(paths) != 66 {
		t.Fatalf("complete EBU v5.0 set required: got %d sequences, error %v", len(paths), err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			fixture := readEBUPCM(t, path)
			name := filepath.Base(path)

			sequence := ebuSequence(t, name)
			if strings.HasPrefix(name, "seq-3341-") && sequence >= 15 && sequence <= 23 {
				ebuTruePeak(t, fixture, sequence)
				return
			}

			ebuLoudness(t, fixture, name, sequence)
		})
	}
}

type ebuPCM struct {
	data                 []byte
	channels, rate, bits int
	frames               int
}

func readEBUPCM(t *testing.T, path string) ebuPCM {
	t.Helper()

	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(file) < 12 || string(file[:4]) != "RIFF" || string(file[8:12]) != "WAVE" {
		t.Fatal("fixture is not RIFF WAVE")
	}

	var f ebuPCM

	for offset := 12; offset+8 <= len(file); {
		size := int(binary.LittleEndian.Uint32(file[offset+4:]))

		start := offset + 8
		if size > len(file)-start {
			t.Fatal("fixture chunk exceeds file")
		}

		chunk := file[start : start+size]
		switch string(file[offset : offset+4]) {
		case "fmt ":
			if len(chunk) < 16 {
				t.Fatal("short fixture format")
			}

			code := binary.LittleEndian.Uint16(chunk)
			if code != 1 && (code != 0xfffe || len(chunk) < 40 || binary.LittleEndian.Uint16(chunk[24:]) != 1) {
				t.Fatal("fixture is not PCM/WAVEEX PCM")
			}

			f.channels = int(binary.LittleEndian.Uint16(chunk[2:]))
			f.rate = int(binary.LittleEndian.Uint32(chunk[4:]))
			f.bits = int(binary.LittleEndian.Uint16(chunk[14:]))
		case "data":
			f.data = chunk
		}

		offset = start + size + size%2
	}

	if f.rate != 48000 || f.channels < 1 || f.channels > 6 || (f.bits != 16 && f.bits != 24) || len(f.data) == 0 {
		t.Fatalf("unexpected fixture format: %d Hz, %d channels, %d bits", f.rate, f.channels, f.bits)
	}

	stride := f.channels * (f.bits / 8)
	if len(f.data)%stride != 0 {
		t.Fatal("partial PCM fixture frame")
	}

	f.frames = len(f.data) / stride

	return f
}

func (f ebuPCM) decode(dst [][]float64, start, count int) {
	bytes := f.bits / 8

	for ch := range dst {
		dst[ch] = dst[ch][:count]
		for frame := range count {
			offset := ((start+frame)*f.channels + ch) * bytes
			if bytes == 2 {
				dst[ch][frame] = float64(int16(binary.LittleEndian.Uint16(f.data[offset:]))) / 32768
			} else {
				value := int32(f.data[offset]) | int32(f.data[offset+1])<<8 | int32(f.data[offset+2])<<16
				value = value << 8 >> 8
				dst[ch][frame] = float64(value) / 8388608
			}
		}
	}
}

func ebuSequence(t *testing.T, name string) int {
	t.Helper()

	if strings.HasPrefix(name, "seq-3341-2011-8_") {
		return 8
	}

	match := regexp.MustCompile(`^seq-334[12]-(\d+)`).FindStringSubmatch(name)
	if len(match) != 2 {
		t.Fatalf("unknown sequence %s", name)
	}

	value, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func ebuBuffer(channels, capacity int) [][]float64 {
	block := make([][]float64, channels)
	for ch := range block {
		block[ch] = make([]float64, capacity)
	}

	return block
}

func ebuClose(t *testing.T, label string, actual, expected, tolerance float64) {
	t.Helper()

	if math.IsNaN(actual) || math.IsInf(actual, 0) || math.Abs(actual-expected) > tolerance {
		t.Fatalf("%s = %.9f, expected %.3f ±%.3f", label, actual, expected, tolerance)
	}
}

func ebuTruePeak(t *testing.T, f ebuPCM, sequence int) {
	t.Helper()

	meter, err := truepeak.NewMeter(f.channels)
	if err != nil {
		t.Fatal(err)
	}
	// An irregular partition crosses FIR history boundaries independently of
	// the fixture's tone periods and does not omit the reconstruction tail.
	block := ebuBuffer(f.channels, 257)
	for start := 0; start < f.frames; start += 257 {
		f.decode(block, start, min(257, f.frames-start))

		if err := meter.ProcessPlanar(block); err != nil {
			t.Fatal(err)
		}
	}

	meter.Flush()

	peaks := make([]float64, f.channels)
	if err := meter.PeaksInto(peaks); err != nil {
		t.Fatal(err)
	}

	expected := -6.0
	if sequence == 19 {
		expected = 3
	} else if sequence >= 20 {
		expected = 0
	}

	for ch, peak := range peaks {
		db := 20 * math.Log10(peak)
		if math.IsNaN(db) || db < expected-.4 || db > expected+.2 {
			t.Fatalf("channel %d true peak %.9f dBTP, expected %.1f +0.2/-0.4", ch+1, db, expected)
		}

		t.Logf("channel %d: true peak %.6f dBTP", ch+1, db)
	}
}

func ebuLoudness(t *testing.T, f ebuPCM, name string, sequence int) {
	t.Helper()

	weights := make([]float64, f.channels)
	for ch := range weights {
		weights[ch] = 1
	}

	switch f.channels {
	case 5:
		weights[3], weights[4] = 1.41, 1.41
	case 6:
		weights[3], weights[4], weights[5] = 0, 1.41, 1.41
	}

	meter, err := loudness.NewStreamingMeter(loudness.IntegratedConfig{
		SampleRate: float64(f.rate), Channels: f.channels, ChannelWeights: weights, MaxFrames: int64(f.frames),
	})
	if err != nil {
		t.Fatal(err)
	}

	block := ebuBuffer(f.channels, 480)
	for start := 0; start < f.frames; start += 480 {
		count := min(480, f.frames-start)
		f.decode(block, start, count)

		if err := meter.ProcessPlanar(block); err != nil {
			t.Fatal(err)
		}

		end := start + count

		if strings.HasPrefix(name, "seq-3341-") {
			switch sequence {
			case 9:
				if end >= 3*f.rate && end%480 == 0 {
					ebuClose(t, "continuous S", meter.Snapshot().ShortTerm, -23, .1)
				}
			case 12:
				if end >= f.rate && end%480 == 0 {
					ebuClose(t, "continuous M", meter.Snapshot().Momentary, -23, .1)
				}
			case 11, 14:
				segmentFrames := 6 * f.rate
				if sequence == 14 {
					segmentFrames = f.rate * 4 / 5
				}

				if end%segmentFrames == 0 {
					segment := end/segmentFrames - 1

					reading := meter.Snapshot().MaxShortTerm
					if sequence == 14 {
						reading = meter.Snapshot().MaxMomentary
					}

					ebuClose(t, fmt.Sprintf("successive maximum %d", segment+1), reading, float64(-38+segment), .1)
				}
			}
		}
	}

	r := meter.Snapshot()

	if strings.HasPrefix(name, "seq-3342-") {
		expected := []float64{0, 10, 5, 20, 15}[sequence]
		ebuClose(t, "LRA", r.LRA, expected, 1)
	} else {
		switch sequence {
		case 1, 2:
			expected := -23.0
			if sequence == 2 {
				expected = -33
			}

			ebuClose(t, "M", r.Momentary, expected, .1)
			ebuClose(t, "S", r.ShortTerm, expected, .1)
			ebuClose(t, "I", r.Integrated, expected, .1)
		case 3, 4, 5, 6, 7, 8:
			ebuClose(t, "I", r.Integrated, -23, .1)

			switch sequence {
			case 7:
				ebuClose(t, "programme LRA", r.LRA, 5, 1)
			case 8:
				ebuClose(t, "programme LRA", r.LRA, 15, 1)
			}
		case 10:
			ebuClose(t, "file maximum S", r.MaxShortTerm, -23, .1)
		case 13:
			ebuClose(t, "file maximum M", r.MaxMomentary, -23, .1)
		}
	}

	t.Logf("M=%.6f S=%.6f I=%.6f LRA=%.6f maxM=%.6f maxS=%.6f", r.Momentary, r.ShortTerm, r.Integrated, r.LRA, r.MaxMomentary, r.MaxShortTerm)
}

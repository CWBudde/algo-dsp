// Package benchguard parses `go test -bench` output and compares a run against a
// recorded baseline, so allocation and timing regressions in hot paths surface
// during review instead of at release time.
//
// The package is deliberately dependency-free and does not import any DSP code:
// it is tooling support for the benchmark regression guard, not a DSP algorithm.
package benchguard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// modulePrefix is trimmed from `pkg:` lines so benchmark keys stay readable
// (`dsp/filter/biquad.BenchmarkProcessSample` rather than the full import path).
const modulePrefix = "github.com/cwbudde/algo-dsp/"

// Measurement holds the metrics `go test -benchmem` reports for one benchmark.
// MB/s and custom units are intentionally ignored: they are derived values that
// add noise without catching anything ns/op and allocs/op do not already catch.
type Measurement struct {
	NsPerOp     float64 `json:"nsPerOp"`
	BytesPerOp  int64   `json:"bytesPerOp"`
	AllocsPerOp int64   `json:"allocsPerOp"`
}

// Run is a parsed benchmark run together with the environment it was measured in.
type Run struct {
	GOOS         string
	GOARCH       string
	CPU          string
	Measurements map[string]Measurement
}

// Baseline is the on-disk form of a recorded run. It carries the environment so
// a comparison can tell whether timings are meaningful on the current machine.
type Baseline struct {
	Generated  string                 `json:"generated"`
	GoVersion  string                 `json:"goVersion"`
	GOOS       string                 `json:"goos"`
	GOARCH     string                 `json:"goarch"`
	CPU        string                 `json:"cpu"`
	Command    string                 `json:"command"`
	Benchmarks map[string]Measurement `json:"benchmarks"`
}

// Parse reads `go test -bench ... -benchmem` output and extracts every benchmark
// result. Non-benchmark lines (PASS, ok, build noise) are ignored. It returns an
// error only when the input contains no benchmark results at all, which almost
// always means the -bench pattern matched nothing.
func Parse(r io.Reader) (*Run, error) {
	run := &Run{Measurements: make(map[string]Measurement)}
	pkg := ""

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if key, value, ok := metaLine(line); ok {
			switch key {
			case "goos":
				run.GOOS = value
			case "goarch":
				run.GOARCH = value
			case "cpu":
				run.CPU = value
			case "pkg":
				pkg = strings.TrimPrefix(value, modulePrefix)
			}

			continue
		}

		name, m, ok := benchmarkLine(line)
		if !ok {
			continue
		}

		if pkg != "" {
			name = pkg + "." + name
		}

		run.Measurements[name] = m
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read benchmark output: %w", err)
	}

	if len(run.Measurements) == 0 {
		return nil, ErrNoBenchmarks
	}

	return run, nil
}

// metaLine recognizes the `key: value` header lines go test emits before results.
func metaLine(line string) (key, value string, ok bool) {
	key, value, ok = strings.Cut(line, ": ")
	if !ok {
		return "", "", false
	}

	switch key {
	case "goos", "goarch", "cpu", "pkg":
		return key, strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

// benchmarkLine parses a single tab-separated result row, e.g.
//
//	BenchmarkProcessSample-11   \t92863960\t  11.31 ns/op\t  0 B/op\t  0 allocs/op
//
// The -N GOMAXPROCS suffix is stripped so baselines survive a move to a machine
// with a different core count.
func benchmarkLine(line string) (string, Measurement, bool) {
	fields := strings.Split(line, "\t")
	if len(fields) < 3 {
		return "", Measurement{}, false
	}

	name := strings.TrimSpace(fields[0])
	if !strings.HasPrefix(name, "Benchmark") {
		return "", Measurement{}, false
	}

	if idx := strings.LastIndex(name, "-"); idx > 0 {
		if _, err := strconv.Atoi(name[idx+1:]); err == nil {
			name = name[:idx]
		}
	}

	// Field 1 is the iteration count; metrics start at field 2.
	var (
		m    Measurement
		seen bool
	)

	for _, field := range fields[2:] {
		value, unit, ok := strings.Cut(strings.TrimSpace(field), " ")
		if !ok {
			continue
		}

		switch unit {
		case "ns/op":
			ns, err := strconv.ParseFloat(value, 64)
			if err != nil {
				continue
			}

			m.NsPerOp, seen = ns, true
		case "B/op":
			b, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				continue
			}

			m.BytesPerOp, seen = b, true
		case "allocs/op":
			a, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				continue
			}

			m.AllocsPerOp, seen = a, true
		}
	}

	if !seen {
		return "", Measurement{}, false
	}

	return name, m, true
}

// Tolerances configures what counts as a regression.
//
// Allocation counts are deterministic, so any increase is a regression and there
// is no tolerance knob for them. Bytes and nanoseconds per op both vary with the
// machine and the allocator, so they get fractional headroom.
type Tolerances struct {
	// Ns is the fractional ns/op increase tolerated before a benchmark is
	// reported as slower, e.g. 0.25 for 25%.
	Ns float64
	// Bytes is the fractional B/op increase tolerated.
	Bytes float64
}

// DefaultTolerances are tuned for shared CI runners, where wall-clock noise of
// 10-20% between runs of the same commit is routine.
func DefaultTolerances() Tolerances {
	return Tolerances{Ns: 0.25, Bytes: 0.10}
}

// Entry is the comparison of one benchmark between baseline and current run.
type Entry struct {
	Name    string
	Base    Measurement
	Current Measurement
	// HasBase is false for benchmarks that are new since the baseline was taken;
	// HasCurrent is false for baseline entries the current run did not produce.
	HasBase    bool
	HasCurrent bool
	// NsRatio is Current.NsPerOp / Base.NsPerOp, or 0 when either side is missing.
	NsRatio        float64
	SlowerThanTol  bool
	MoreBytes      bool
	MoreAllocs     bool
	FewerAllocs    bool
	FasterThanTol  bool
}

// Regressed reports whether this entry regressed in any enforced dimension.
// Timing counts only when the comparison is machine-comparable.
func (e Entry) Regressed(timingEnforced bool) bool {
	return e.MoreAllocs || e.MoreBytes || (timingEnforced && e.SlowerThanTol)
}

// Report is the result of comparing a run against a baseline.
type Report struct {
	Entries []Entry
	// TimingEnforced is false when the baseline was recorded on different
	// hardware, in which case ns/op differences are shown but never counted as
	// regressions. Allocation checks stay enforced: they are machine-independent.
	TimingEnforced bool
	BaseEnv        string
	CurrentEnv     string
	Tolerances     Tolerances
}

// Regressions returns the entries that regressed in an enforced dimension.
func (r *Report) Regressions() []Entry {
	var out []Entry

	for _, e := range r.Entries {
		if e.Regressed(r.TimingEnforced) {
			out = append(out, e)
		}
	}

	return out
}

// Compare diffs a current run against a baseline. Timing comparisons are enforced
// only when the baseline was recorded on the same GOOS/GOARCH/CPU, because ns/op
// numbers do not transfer between machines.
func Compare(base *Baseline, current *Run, tol Tolerances) *Report {
	report := &Report{
		TimingEnforced: base.GOOS == current.GOOS &&
			base.GOARCH == current.GOARCH &&
			base.CPU == current.CPU,
		BaseEnv:    envString(base.GOOS, base.GOARCH, base.CPU),
		CurrentEnv: envString(current.GOOS, current.GOARCH, current.CPU),
		Tolerances: tol,
	}

	names := make(map[string]struct{}, len(base.Benchmarks)+len(current.Measurements))
	for name := range base.Benchmarks {
		names[name] = struct{}{}
	}

	for name := range current.Measurements {
		names[name] = struct{}{}
	}

	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}

	sort.Strings(sorted)

	report.Entries = make([]Entry, 0, len(sorted))

	for _, name := range sorted {
		b, hasBase := base.Benchmarks[name]
		c, hasCurrent := current.Measurements[name]

		entry := Entry{
			Name:       name,
			Base:       b,
			Current:    c,
			HasBase:    hasBase,
			HasCurrent: hasCurrent,
		}

		if hasBase && hasCurrent {
			entry.MoreAllocs = c.AllocsPerOp > b.AllocsPerOp
			entry.FewerAllocs = c.AllocsPerOp < b.AllocsPerOp
			entry.MoreBytes = exceeds(float64(b.BytesPerOp), float64(c.BytesPerOp), tol.Bytes)

			if b.NsPerOp > 0 {
				entry.NsRatio = c.NsPerOp / b.NsPerOp
				entry.SlowerThanTol = exceeds(b.NsPerOp, c.NsPerOp, tol.Ns)
				entry.FasterThanTol = exceeds(c.NsPerOp, b.NsPerOp, tol.Ns)
			}
		}

		report.Entries = append(report.Entries, entry)
	}

	return report
}

// exceeds reports whether want grew beyond base by more than the given fraction.
// A zero baseline is treated as a regression only when the current value is
// non-zero, which is the case that matters: 0 allocs/op becoming non-zero.
func exceeds(base, current, tolerance float64) bool {
	if base <= 0 {
		return current > 0
	}

	return current > base*(1+tolerance)
}

func envString(goos, goarch, cpu string) string {
	if cpu == "" {
		return goos + "/" + goarch
	}

	return goos + "/" + goarch + " " + cpu
}

// WriteBaseline encodes a baseline as indented JSON with a trailing newline, so
// the checked-in file stays diff-friendly.
func WriteBaseline(w io.Writer, b *Baseline) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(b); err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}

	return nil
}

// ReadBaseline decodes a baseline written by WriteBaseline.
func ReadBaseline(r io.Reader) (*Baseline, error) {
	var b Baseline

	if err := json.NewDecoder(r).Decode(&b); err != nil {
		return nil, fmt.Errorf("decode baseline: %w", err)
	}

	if b.Benchmarks == nil {
		b.Benchmarks = make(map[string]Measurement)
	}

	return &b, nil
}

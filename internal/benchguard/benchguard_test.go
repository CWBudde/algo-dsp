package benchguard

import (
	"errors"
	"strings"
	"testing"
)

// sampleOutput mirrors real `go test -bench -benchmem` output, including the
// GOMAXPROCS suffix, a MB/s column on one line only, and trailing PASS/ok lines.
const sampleOutput = "goos: linux\n" +
	"goarch: amd64\n" +
	"pkg: github.com/cwbudde/algo-dsp/dsp/filter/biquad\n" +
	"cpu: 12th Gen Intel(R) Core(TM) i7-1255U\n" +
	"BenchmarkProcessSample-11            \t92863960\t        11.31 ns/op\t       0 B/op\t       0 allocs/op\n" +
	"BenchmarkProcessBlock/N=1024-11      \t  215401\t      5898 ns/op\t1388.89 MB/s\t      24 B/op\t       1 allocs/op\n" +
	"PASS\n" +
	"ok  \tgithub.com/cwbudde/algo-dsp/dsp/filter/biquad\t6.407s\n"

func mustParse(t *testing.T, s string) *Run {
	t.Helper()

	run, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	return run
}

func TestParse(t *testing.T) {
	run := mustParse(t, sampleOutput)

	if run.GOOS != "linux" || run.GOARCH != "amd64" {
		t.Errorf("env = %s/%s, want linux/amd64", run.GOOS, run.GOARCH)
	}

	if want := "12th Gen Intel(R) Core(TM) i7-1255U"; run.CPU != want {
		t.Errorf("CPU = %q, want %q", run.CPU, want)
	}

	if len(run.Measurements) != 2 {
		t.Fatalf("parsed %d benchmarks, want 2", len(run.Measurements))
	}

	// The module prefix is trimmed and the -11 GOMAXPROCS suffix dropped.
	sample, ok := run.Measurements["dsp/filter/biquad.BenchmarkProcessSample"]
	if !ok {
		t.Fatalf("missing qualified benchmark name; got %v", keys(run))
	}

	if sample.NsPerOp != 11.31 || sample.BytesPerOp != 0 || sample.AllocsPerOp != 0 {
		t.Errorf("ProcessSample = %+v, want {11.31 0 0}", sample)
	}

	// The MB/s column must not disturb the B/op and allocs/op fields after it.
	block := run.Measurements["dsp/filter/biquad.BenchmarkProcessBlock/N=1024"]
	if block.NsPerOp != 5898 || block.BytesPerOp != 24 || block.AllocsPerOp != 1 {
		t.Errorf("ProcessBlock = %+v, want {5898 24 1}", block)
	}
}

func TestParseNoBenchmarks(t *testing.T) {
	_, err := Parse(strings.NewReader("goos: linux\nPASS\nok  \tpkg\t0.1s\n"))
	if !errors.Is(err, ErrNoBenchmarks) {
		t.Errorf("Parse() error = %v, want ErrNoBenchmarks", err)
	}
}

func TestParseSubtestNameWithTrailingNumber(t *testing.T) {
	// "N=1024" ends in digits after a "-"-free segment; only the GOMAXPROCS
	// suffix may be stripped, never part of the sub-benchmark name.
	run := mustParse(t, "pkg: x\nBenchmarkFoo/size-1024-8\t100\t1.00 ns/op\n")

	if _, ok := run.Measurements["x.BenchmarkFoo/size-1024"]; !ok {
		t.Errorf("got %v, want x.BenchmarkFoo/size-1024", keys(run))
	}
}

// With -count=N a benchmark is reported N times; the guard must collapse the
// repeats to the fastest observation rather than keeping whichever came last.
func TestParseRepeatedRunsKeepsMinimum(t *testing.T) {
	run := mustParse(t, "pkg: x\n"+
		"BenchmarkA-8\t100\t     50.0 ns/op\t     128 B/op\t       2 allocs/op\n"+
		"BenchmarkA-8\t100\t     20.0 ns/op\t      64 B/op\t       1 allocs/op\n"+
		"BenchmarkA-8\t100\t     90.0 ns/op\t     256 B/op\t       3 allocs/op\n")

	got := run.Measurements["x.BenchmarkA"]

	want := Measurement{NsPerOp: 20, BytesPerOp: 64, AllocsPerOp: 1}
	if got != want {
		t.Errorf("repeated runs = %+v, want %+v", got, want)
	}
}

func keys(run *Run) []string {
	out := make([]string, 0, len(run.Measurements))
	for name := range run.Measurements {
		out = append(out, name)
	}

	return out
}

func baselineFrom(cpu string, m map[string]Measurement) *Baseline {
	return &Baseline{GOOS: "linux", GOARCH: "amd64", CPU: cpu, Benchmarks: m}
}

func runFrom(cpu string, m map[string]Measurement) *Run {
	return &Run{GOOS: "linux", GOARCH: "amd64", CPU: cpu, Measurements: m}
}

func TestCompare(t *testing.T) {
	const cpu = "test-cpu"

	base := baselineFrom(cpu, map[string]Measurement{
		"a.BenchmarkSteady":  {NsPerOp: 100, BytesPerOp: 64, AllocsPerOp: 1},
		"a.BenchmarkSlower":  {NsPerOp: 100, BytesPerOp: 0, AllocsPerOp: 0},
		"a.BenchmarkAllocs":  {NsPerOp: 100, BytesPerOp: 0, AllocsPerOp: 0},
		"a.BenchmarkFaster":  {NsPerOp: 100, BytesPerOp: 0, AllocsPerOp: 2},
		"a.BenchmarkDropped": {NsPerOp: 100},
	})

	current := runFrom(cpu, map[string]Measurement{
		"a.BenchmarkSteady": {NsPerOp: 105, BytesPerOp: 64, AllocsPerOp: 1},
		"a.BenchmarkSlower": {NsPerOp: 180, BytesPerOp: 0, AllocsPerOp: 0},
		"a.BenchmarkAllocs": {NsPerOp: 100, BytesPerOp: 48, AllocsPerOp: 1},
		"a.BenchmarkFaster": {NsPerOp: 50, BytesPerOp: 0, AllocsPerOp: 1},
		"a.BenchmarkAdded":  {NsPerOp: 10},
	})

	// Explicit tolerances so the table below does not silently depend on the
	// calibrated defaults, and timing enforcement opted into so the ns/op cases
	// below actually gate.
	report := Compare(base, current, Tolerances{Ns: 0.25, Bytes: 0.10, EnforceTiming: true})

	if !report.TimingEnforced {
		t.Fatal("TimingEnforced = false for identical environments with EnforceTiming set")
	}

	byName := make(map[string]Entry, len(report.Entries))
	for _, e := range report.Entries {
		byName[e.Name] = e
	}

	// Entries must come out sorted so the rendered table is stable.
	for i := 1; i < len(report.Entries); i++ {
		if report.Entries[i-1].Name > report.Entries[i].Name {
			t.Fatalf("entries not sorted at %d: %s > %s",
				i, report.Entries[i-1].Name, report.Entries[i].Name)
		}
	}

	tests := []struct {
		name       string
		regressed  bool
		hasBase    bool
		hasCurrent bool
	}{
		{"a.BenchmarkSteady", false, true, true},   // +5% is inside tolerance
		{"a.BenchmarkSlower", true, true, true},    // +80% ns/op
		{"a.BenchmarkAllocs", true, true, true},    // 0 -> 1 allocs/op
		{"a.BenchmarkFaster", false, true, true},   // faster and fewer allocs
		{"a.BenchmarkDropped", false, true, false}, // vanished, not a regression
		{"a.BenchmarkAdded", false, false, true},   // new, no baseline to compare
	}

	for _, tt := range tests {
		e, ok := byName[tt.name]
		if !ok {
			t.Errorf("%s missing from report", tt.name)

			continue
		}

		if got := e.Regressed(report.TimingEnforced); got != tt.regressed {
			t.Errorf("%s: Regressed() = %t, want %t", tt.name, got, tt.regressed)
		}

		if e.HasBase != tt.hasBase || e.HasCurrent != tt.hasCurrent {
			t.Errorf("%s: HasBase/HasCurrent = %t/%t, want %t/%t",
				tt.name, e.HasBase, e.HasCurrent, tt.hasBase, tt.hasCurrent)
		}
	}

	if got := len(report.Regressions()); got != 2 {
		t.Errorf("Regressions() = %d, want 2", got)
	}

	if e := byName["a.BenchmarkFaster"]; !e.FasterThanTol || !e.FewerAllocs {
		t.Errorf("Faster: FasterThanTol=%t FewerAllocs=%t, want both true", e.FasterThanTol, e.FewerAllocs)
	}
}

// A baseline recorded elsewhere still catches allocation regressions, because
// allocs/op is machine-independent, but must not fail on wall-clock differences.
func TestCompareDifferentMachineIgnoresTiming(t *testing.T) {
	base := baselineFrom("cpu-a", map[string]Measurement{
		"a.BenchmarkSlower": {NsPerOp: 100},
		"a.BenchmarkAllocs": {NsPerOp: 100, AllocsPerOp: 0},
	})

	current := runFrom("cpu-b", map[string]Measurement{
		"a.BenchmarkSlower": {NsPerOp: 300},
		"a.BenchmarkAllocs": {NsPerOp: 100, AllocsPerOp: 3},
	})

	// EnforceTiming is on, so only the machine mismatch can disable timing.
	tol := DefaultTolerances()
	tol.EnforceTiming = true

	report := Compare(base, current, tol)

	if report.SameMachine {
		t.Fatal("SameMachine = true across different CPUs")
	}

	if report.TimingEnforced {
		t.Fatal("TimingEnforced = true across different CPUs")
	}

	regressions := report.Regressions()
	if len(regressions) != 1 {
		t.Fatalf("Regressions() = %d, want 1 (allocations only)", len(regressions))
	}

	if regressions[0].Name != "a.BenchmarkAllocs" {
		t.Errorf("regression = %s, want a.BenchmarkAllocs", regressions[0].Name)
	}
}

// The calibrated default is that timing never gates, even on the machine the
// baseline came from: a sustained sweep throttles the CPU enough to push real
// benchmarks past any bound loose enough to be useful. Allocations still gate.
func TestCompareDefaultDoesNotEnforceTiming(t *testing.T) {
	const cpu = "same-cpu"

	base := baselineFrom(cpu, map[string]Measurement{
		"a.BenchmarkSlow":   {NsPerOp: 100},
		"a.BenchmarkAllocs": {NsPerOp: 100, AllocsPerOp: 0},
	})

	current := runFrom(cpu, map[string]Measurement{
		"a.BenchmarkSlow":   {NsPerOp: 500},
		"a.BenchmarkAllocs": {NsPerOp: 100, AllocsPerOp: 1},
	})

	report := Compare(base, current, DefaultTolerances())

	if !report.SameMachine {
		t.Fatal("SameMachine = false for identical environments")
	}

	if report.TimingEnforced {
		t.Fatal("TimingEnforced = true under DefaultTolerances")
	}

	regressions := report.Regressions()
	if len(regressions) != 1 || regressions[0].Name != "a.BenchmarkAllocs" {
		t.Fatalf("Regressions() = %v, want only a.BenchmarkAllocs", regressions)
	}

	// The 5x slowdown must still be visible, just not fatal.
	var buf strings.Builder

	if err := report.Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(buf.String(), "slower (advisory)") {
		t.Errorf("a 5x slowdown should be reported as advisory:\n%s", buf.String())
	}
}

func TestCompareZeroBaselineNs(t *testing.T) {
	// A zero ns/op baseline would divide by zero; the entry must stay comparable
	// on the memory axes and simply report no ratio.
	base := baselineFrom("cpu", map[string]Measurement{"a.BenchmarkZero": {}})
	current := runFrom("cpu", map[string]Measurement{"a.BenchmarkZero": {NsPerOp: 5}})

	report := Compare(base, current, DefaultTolerances())

	e := report.Entries[0]
	if e.NsRatio != 0 || e.SlowerThanTol {
		t.Errorf("entry = %+v, want zero ratio and no timing verdict", e)
	}
}

func TestExceeds(t *testing.T) {
	tests := []struct {
		name               string
		base, current, tol float64
		want               bool
	}{
		{"within tolerance", 100, 120, 0.25, false},
		{"beyond tolerance", 100, 130, 0.25, true},
		{"exactly at bound", 100, 125, 0.25, false},
		{"zero base stays zero", 0, 0, 0.25, false},
		{"zero base grows", 0, 1, 0.25, true},
		{"improvement", 100, 50, 0.25, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exceeds(tt.base, tt.current, tt.tol); got != tt.want {
				t.Errorf("exceeds(%v, %v, %v) = %t, want %t", tt.base, tt.current, tt.tol, got, tt.want)
			}
		})
	}
}

func TestBaselineRoundTrip(t *testing.T) {
	base := &Baseline{
		Generated:  "2026-07-29",
		GoVersion:  "go1.25.0",
		GOOS:       "linux",
		GOARCH:     "amd64",
		CPU:        "test-cpu",
		Command:    "just bench-ci",
		Benchmarks: map[string]Measurement{"a.BenchmarkX": {NsPerOp: 1.5, BytesPerOp: 8, AllocsPerOp: 1}},
	}

	var buf strings.Builder

	if err := WriteBaseline(&buf, base); err != nil {
		t.Fatalf("WriteBaseline() error = %v", err)
	}

	got, err := ReadBaseline(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("ReadBaseline() error = %v", err)
	}

	if got.Generated != base.Generated || got.CPU != base.CPU || got.Command != base.Command {
		t.Errorf("metadata round-trip mismatch: %+v", got)
	}

	if got.Benchmarks["a.BenchmarkX"] != base.Benchmarks["a.BenchmarkX"] {
		t.Errorf("Benchmarks = %+v, want %+v", got.Benchmarks, base.Benchmarks)
	}
}

func TestReadBaselineEmptyMap(t *testing.T) {
	got, err := ReadBaseline(strings.NewReader(`{"generated":"2026-07-29"}`))
	if err != nil {
		t.Fatalf("ReadBaseline() error = %v", err)
	}

	if got.Benchmarks == nil {
		t.Error("Benchmarks is nil; want an initialized map so Compare need not nil-check")
	}
}

func TestReadBaselineInvalid(t *testing.T) {
	if _, err := ReadBaseline(strings.NewReader("not json")); err == nil {
		t.Error("ReadBaseline() error = nil, want a decode error")
	}
}

func TestReportRender(t *testing.T) {
	base := baselineFrom("cpu", map[string]Measurement{
		"a.BenchmarkOK":   {NsPerOp: 100, AllocsPerOp: 0},
		"a.BenchmarkBad":  {NsPerOp: 100, AllocsPerOp: 0},
		"a.BenchmarkGone": {NsPerOp: 100},
	})

	current := runFrom("cpu", map[string]Measurement{
		"a.BenchmarkOK":  {NsPerOp: 101, AllocsPerOp: 0},
		"a.BenchmarkBad": {NsPerOp: 100, BytesPerOp: 32, AllocsPerOp: 2},
		"a.BenchmarkNew": {NsPerOp: 7},
	})

	var buf strings.Builder

	report := Compare(base, current, DefaultTolerances())
	if err := report.Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	out := buf.String()

	for _, want := range []string{
		"BENCHMARK", "a.BenchmarkOK", "ok",
		"REGRESSED", "allocs/op 0 -> 2",
		"MISSING", "NEW",
		"1 regression(s):",
		"allocs/op exact",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Render() output missing %q\n%s", want, out)
		}
	}
}

func TestReportRenderClean(t *testing.T) {
	base := baselineFrom("cpu-a", map[string]Measurement{"a.BenchmarkX": {NsPerOp: 100}})
	current := runFrom("cpu-b", map[string]Measurement{"a.BenchmarkX": {NsPerOp: 400}})

	var buf strings.Builder

	report := Compare(base, current, DefaultTolerances())
	if err := report.Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "no regressions") {
		t.Errorf("cross-machine slowdown should not be a regression:\n%s", out)
	}

	if !strings.Contains(out, "slower (advisory)") {
		t.Errorf("cross-machine slowdown should still be visible:\n%s", out)
	}

	if !strings.Contains(out, "different hardware") {
		t.Errorf("output should explain why timing is not enforced:\n%s", out)
	}
}

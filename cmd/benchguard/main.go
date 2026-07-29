// Command benchguard compares `go test -bench` output against a recorded
// baseline and reports allocation and timing regressions.
//
// It reads benchmark output on stdin:
//
//	just bench-ci | go run ./cmd/benchguard
//
// To record the current numbers as the new baseline:
//
//	just bench-ci | go run ./cmd/benchguard -update
//
// By default the command exits 0 even when it finds regressions, so it can be
// wired into CI as advisory output. Pass -fail to turn regressions into a
// non-zero exit status.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cwbudde/algo-dsp/internal/benchguard"
)

const defaultBaselinePath = "benchmarks/baseline.json"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "benchguard:", err)
		os.Exit(2)
	}
}

func run() error {
	var (
		baselinePath = flag.String("baseline", defaultBaselinePath, "path to the baseline JSON file")
		update       = flag.Bool("update", false, "overwrite the baseline with the parsed run instead of comparing")
		command      = flag.String("command", "just bench-ci", "command recorded in the baseline for reproducing it")
		nsTol        = flag.Float64("ns-tolerance", benchguard.DefaultTolerances().Ns,
			"fractional ns/op increase tolerated before a benchmark counts as slower")
		memTol = flag.Float64("mem-tolerance", benchguard.DefaultTolerances().Bytes,
			"fractional B/op increase tolerated; allocs/op is always compared exactly")
		enforceTiming = flag.Bool("enforce-timing", false,
			"count ns/op regressions toward the verdict; only meaningful on quiet, thermally stable hardware")
		fail = flag.Bool("fail", false, "exit non-zero when regressions are found")
	)

	flag.Parse()

	current, err := benchguard.Parse(os.Stdin)
	if err != nil {
		return err
	}

	if *update {
		return writeBaseline(*baselinePath, *command, current)
	}

	base, err := readBaseline(*baselinePath)
	if err != nil {
		return err
	}

	report := benchguard.Compare(base, current, benchguard.Tolerances{
		Ns:            *nsTol,
		Bytes:         *memTol,
		EnforceTiming: *enforceTiming,
	})

	if err := report.Render(os.Stdout); err != nil {
		return err
	}

	if *fail && len(report.Regressions()) > 0 {
		os.Exit(1)
	}

	return nil
}

func readBaseline(path string) (*benchguard.Baseline, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open baseline (run with -update to create it): %w", err)
	}
	defer func() { _ = f.Close() }()

	base, err := benchguard.ReadBaseline(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return base, nil
}

func writeBaseline(path, command string, current *benchguard.Run) error {
	base := &benchguard.Baseline{
		Generated:  time.Now().UTC().Format(time.DateOnly),
		GoVersion:  runtime.Version(),
		GOOS:       current.GOOS,
		GOARCH:     current.GOARCH,
		CPU:        current.CPU,
		Command:    command,
		Benchmarks: current.Measurements,
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create baseline directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create baseline: %w", err)
	}

	if err := benchguard.WriteBaseline(f, base); err != nil {
		_ = f.Close()

		return err
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close baseline: %w", err)
	}

	fmt.Fprintf(os.Stderr, "benchguard: wrote %d benchmarks to %s\n", len(base.Benchmarks), path)

	return nil
}

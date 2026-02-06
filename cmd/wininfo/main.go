// Command wininfo prints spectral properties of DSP window functions.
//
// Usage:
//
//	wininfo [flags] [window-name ...]
//
// Without arguments it prints info for all known window types.
//
// Examples:
//
//	wininfo hann
//	wininfo -size 1024 blackman kaiser
//	wininfo -size 4096 -alpha 8 kaiser
//	wininfo -all
//	wininfo -list
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

type windowEntry struct {
	name     string
	typ      window.Type
	hasAlpha bool
	defAlpha float64
}

var registry = []windowEntry{
	{"rectangular", window.TypeRectangular, false, 0},
	{"hann", window.TypeHann, false, 0},
	{"hamming", window.TypeHamming, false, 0},
	{"blackman", window.TypeBlackman, false, 0},
	{"exact-blackman", window.TypeExactBlackman, false, 0},
	{"blackman-harris-3t", window.TypeBlackmanHarris3Term, false, 0},
	{"blackman-harris-4t", window.TypeBlackmanHarris4Term, false, 0},
	{"blackman-nuttall", window.TypeBlackmanNuttall, false, 0},
	{"nuttall-ctd", window.TypeNuttallCTD, false, 0},
	{"nuttall-cfd", window.TypeNuttallCFD, false, 0},
	{"flat-top", window.TypeFlatTop, false, 0},
	{"kaiser", window.TypeKaiser, true, 8.6},
	{"tukey", window.TypeTukey, true, 0.5},
	{"triangle", window.TypeTriangle, false, 0},
	{"cosine", window.TypeCosine, false, 0},
	{"welch", window.TypeWelch, false, 0},
	{"lanczos", window.TypeLanczos, false, 0},
	{"gauss", window.TypeGauss, true, 2.5},
	{"lawrey-5t", window.TypeLawrey5Term, false, 0},
	{"lawrey-6t", window.TypeLawrey6Term, false, 0},
	{"burgess-59db", window.TypeBurgessOptimized59dB, false, 0},
	{"burgess-71db", window.TypeBurgessOptimized71dB, false, 0},
	{"albrecht-2t", window.TypeAlbrecht2Term, false, 0},
	{"albrecht-3t", window.TypeAlbrecht3Term, false, 0},
	{"albrecht-4t", window.TypeAlbrecht4Term, false, 0},
	{"albrecht-5t", window.TypeAlbrecht5Term, false, 0},
	{"albrecht-6t", window.TypeAlbrecht6Term, false, 0},
	{"albrecht-7t", window.TypeAlbrecht7Term, false, 0},
	{"albrecht-8t", window.TypeAlbrecht8Term, false, 0},
	{"albrecht-9t", window.TypeAlbrecht9Term, false, 0},
	{"albrecht-10t", window.TypeAlbrecht10Term, false, 0},
	{"albrecht-11t", window.TypeAlbrecht11Term, false, 0},
}

func main() {
	size := flag.Int("size", 1024, "window length in samples")
	alpha := flag.Float64("alpha", math.NaN(), "alpha/beta parameter for parametric windows (kaiser, tukey, gauss)")
	all := flag.Bool("all", false, "show all window types")
	list := flag.Bool("list", false, "list available window names")
	periodic := flag.Bool("periodic", false, "use periodic (FFT) form instead of symmetric")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: wininfo [flags] [window-name ...]\n\n")
		fmt.Fprintf(os.Stderr, "Prints spectral properties of DSP window functions.\n")
		fmt.Fprintf(os.Stderr, "Without arguments or with -all, prints info for all windows.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  wininfo hann blackman\n")
		fmt.Fprintf(os.Stderr, "  wininfo -size 4096 -alpha 8 kaiser\n")
		fmt.Fprintf(os.Stderr, "  wininfo -all\n")
		fmt.Fprintf(os.Stderr, "  wininfo -list\n")
	}
	flag.Parse()

	if *list {
		printList()
		return
	}

	names := flag.Args()
	if len(names) == 0 || *all {
		names = nil
		for _, e := range registry {
			names = append(names, e.name)
		}
	}

	entries := resolveEntries(names, *alpha)
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "error: no matching window types\n")
		os.Exit(1)
	}

	var opts []window.Option
	if *periodic {
		opts = append(opts, window.WithPeriodic())
	}

	printAnalysis(entries, *size, opts)
}

func printList() {
	names := make([]string, len(registry))
	for i, e := range registry {
		names[i] = e.name
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Println(n)
	}
}

type resolvedEntry struct {
	windowEntry
	alphaOverride float64
}

func resolveEntries(names []string, alphaFlag float64) []resolvedEntry {
	byName := make(map[string]windowEntry, len(registry))
	for _, e := range registry {
		byName[e.name] = e
	}

	var result []resolvedEntry
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		e, ok := byName[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: unknown window %q (use -list to see available)\n", name)
			continue
		}
		a := e.defAlpha
		if e.hasAlpha && !math.IsNaN(alphaFlag) {
			a = alphaFlag
		}
		result = append(result, resolvedEntry{e, a})
	}
	return result
}

func printAnalysis(entries []resolvedEntry, size int, baseOpts []window.Option) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintf(tw, "Window\tSize\tCoherent Gain\tENBW [bins]\tBW 3dB [bins]\tSidelobe [dB]\t1st Min [bins]\tScallop [dB]\n"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: failed to write output header: %v\n", err)
		return
	}
	if _, err := fmt.Fprintf(tw, "------\t----\t-------------\t----------\t-------------\t-------------\t--------------\t-----------\n"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: failed to write output header: %v\n", err)
		return
	}

	for _, e := range entries {
		opts := append([]window.Option(nil), baseOpts...)
		if e.hasAlpha {
			opts = append(opts, window.WithAlpha(e.alphaOverride))
		}

		coeffs := window.Generate(e.typ, size, opts...)
		a := window.Analyze(coeffs)

		label := e.name
		if e.hasAlpha {
			label = fmt.Sprintf("%s (a=%.2f)", e.name, e.alphaOverride)
		}

		if _, err := fmt.Fprintf(tw, "%s\t%d\t%.6f\t%.4f\t%.4f\t%.2f\t%.4f\t%.4f\n",
			label,
			size,
			a.CoherentGain,
			a.ENBW,
			a.Bandwidth3dB,
			a.HighestSidelobedB,
			a.FirstMinimumBins,
			a.ScallopLossdB,
		); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: failed to write output row: %v\n", err)
			return
		}
	}
	if err := tw.Flush(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: failed to flush output: %v\n", err)
	}
}

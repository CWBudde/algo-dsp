package benchguard

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// ErrNoBenchmarks is returned by Parse when the input held no benchmark results,
// which in practice means the -bench pattern matched nothing.
var ErrNoBenchmarks = errors.New("no benchmark results found in input")

// Render writes a human-readable comparison table followed by a verdict line.
// The output is plain text so it reads correctly both in a terminal and inside a
// fenced block in a CI job summary.
func (r *Report) Render(w io.Writer) error {
	var buf strings.Builder

	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	// Writes go to a strings.Builder via tabwriter and cannot fail; Flush reports
	// any real error below.
	_, _ = fmt.Fprintln(tw, "BENCHMARK\tBASE ns/op\tNEW ns/op\tDELTA\tB/op\tallocs/op\tSTATUS")

	for _, e := range r.Entries {
		_, _ = fmt.Fprintf(
			tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			e.Name,
			formatNs(e.Base.NsPerOp, e.HasBase),
			formatNs(e.Current.NsPerOp, e.HasCurrent),
			formatDelta(e),
			formatMem(e, memBytes),
			formatMem(e, memAllocs),
			status(e, r.TimingEnforced),
		)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush report: %w", err)
	}

	if _, err := io.WriteString(w, buf.String()); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return r.renderSummary(w)
}

func (r *Report) renderSummary(w io.Writer) error {
	var b strings.Builder

	b.WriteString("\n")
	fmt.Fprintf(&b, "baseline machine: %s\n", r.BaseEnv)
	fmt.Fprintf(&b, "current machine:  %s\n", r.CurrentEnv)
	fmt.Fprintf(&b, "tolerances:       ns/op +%.0f%%, B/op +%.0f%%, allocs/op exact\n",
		r.Tolerances.Ns*100, r.Tolerances.Bytes*100)

	if !r.TimingEnforced {
		reason := "not enforced by default (benchmark timing is too noisy to gate on; " +
			"pass -enforce-timing on controlled hardware)"
		if !r.SameMachine {
			reason = "not enforced: baseline was recorded on different hardware"
		}

		fmt.Fprintf(&b, "ns/op:            %s\n", reason)
		b.WriteString("                  allocation checks still apply\n")
	}

	regressions := r.Regressions()

	b.WriteString("\n")

	if len(regressions) == 0 {
		b.WriteString("no regressions\n")
	} else {
		fmt.Fprintf(&b, "%d regression(s):\n", len(regressions))

		for _, e := range regressions {
			fmt.Fprintf(&b, "  - %s: %s\n", e.Name, describe(e, r.TimingEnforced))
		}
	}

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	return nil
}

type memKind int

const (
	memBytes memKind = iota
	memAllocs
)

func formatNs(ns float64, has bool) string {
	if !has {
		return "-"
	}

	switch {
	case ns >= 1000:
		return fmt.Sprintf("%.0f", ns)
	case ns >= 10:
		return fmt.Sprintf("%.1f", ns)
	default:
		return fmt.Sprintf("%.2f", ns)
	}
}

func formatDelta(e Entry) string {
	if !e.HasBase || !e.HasCurrent || e.NsRatio == 0 {
		return "-"
	}

	return fmt.Sprintf("%+.1f%%", (e.NsRatio-1)*100)
}

func formatMem(e Entry, kind memKind) string {
	base, current := e.Base.BytesPerOp, e.Current.BytesPerOp
	if kind == memAllocs {
		base, current = e.Base.AllocsPerOp, e.Current.AllocsPerOp
	}

	switch {
	case !e.HasBase:
		return fmt.Sprintf("- -> %d", current)
	case !e.HasCurrent:
		return fmt.Sprintf("%d -> -", base)
	case base == current:
		return fmt.Sprintf("%d", current)
	default:
		return fmt.Sprintf("%d -> %d", base, current)
	}
}

func status(e Entry, timingEnforced bool) string {
	switch {
	case !e.HasBase:
		return "NEW"
	case !e.HasCurrent:
		return "MISSING"
	case e.Regressed(timingEnforced):
		return "REGRESSED"
	case e.SlowerThanTol:
		// Past the ns/op bound, but timing is not gating this comparison, so it
		// is a hint to investigate rather than a failure.
		return "slower (advisory)"
	case e.FewerAllocs || e.FasterThanTol:
		return "improved"
	default:
		return "ok"
	}
}

// describe explains, in one line, why an entry counts as a regression.
func describe(e Entry, timingEnforced bool) string {
	var parts []string

	if e.MoreAllocs {
		parts = append(parts, fmt.Sprintf("allocs/op %d -> %d", e.Base.AllocsPerOp, e.Current.AllocsPerOp))
	}

	if e.MoreBytes {
		parts = append(parts, fmt.Sprintf("B/op %d -> %d", e.Base.BytesPerOp, e.Current.BytesPerOp))
	}

	if timingEnforced && e.SlowerThanTol {
		parts = append(parts, fmt.Sprintf("ns/op %s -> %s (%s)",
			formatNs(e.Base.NsPerOp, true), formatNs(e.Current.NsPerOp, true), formatDelta(e)))
	}

	return strings.Join(parts, ", ")
}

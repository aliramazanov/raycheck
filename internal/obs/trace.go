package obs

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"time"

	"github.com/aliramazanov/raycheck/internal/humanize"
)

func WriteTrace(w io.Writer, root *Span) error {
	if root == nil {
		return nil
	}

	var b strings.Builder

	b.WriteString("\ntrace\n")
	writeSpan(&b, root, 0, root.Duration)

	_, err := io.WriteString(w, b.String())

	return err
}

func writeSpan(b *strings.Builder, s *Span, depth int, total time.Duration) {
	indent := strings.Repeat("  ", depth+1)
	name := indent + s.Name

	status := ""
	if s.Failed {
		status = "  FAILED"
	}

	fmt.Fprintf(b, "  %-44s %8s %6s%s\n", name, humanize.Duration(s.Duration), share(s.Duration, total), status)

	for _, a := range s.Attrs {
		fmt.Fprintf(b, "  %s  %s=%v\n", indent+"  ", a.Key, a.Value)
	}

	for _, c := range s.Children {
		writeSpan(b, c, depth+1, total)
	}
}

func WriteMetrics(w io.Writer, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	width := 0

	for _, m := range metrics {
		if len(m.Name) > width {
			width = len(m.Name)
		}
	}

	var b strings.Builder

	b.WriteString("\nmetrics\n")

	for _, m := range metrics {
		fmt.Fprintf(&b, "  %-*s  %s\n", width+2, m.Name, humanize.Commas(m.Value))
	}

	_, err := io.WriteString(w, b.String())

	return err
}

func WriteFailures(w io.Writer, failures []Failure) error {
	if len(failures) == 0 {
		return nil
	}

	var b strings.Builder

	fmt.Fprintf(&b, "\nerrors (%d)\n", len(failures))

	for _, f := range failures {
		fmt.Fprintf(&b, "  %-24s %s\n", f.Span, f.Message)

		for _, a := range f.Attrs {
			fmt.Fprintf(&b, "    %s=%v\n", a.Key, a.Value)
		}
	}

	_, err := io.WriteString(w, b.String())

	return err
}

func Flatten(root *Span) []*Span {
	var out []*Span

	var walk func(*Span)

	walk = func(s *Span) {
		out = append(out, s)

		for _, c := range s.Children {
			walk(c)
		}
	}

	if root != nil {
		walk(root)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Duration > out[j].Duration })

	return out
}

func share(d, total time.Duration) string {
	if total <= 0 {
		return ""
	}

	p := float64(d) / float64(total) * 100

	if p >= 99.95 {
		return "100%"
	}

	return strconv.FormatFloat(p, 'f', 1, 64) + "%"
}

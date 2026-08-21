package report

import (
	"io"

	"github.com/aliramazanov/raycheck/internal/humanize"
)

func Summary(w io.Writer, reports []Report) error {
	if len(reports) < 2 {
		return nil
	}

	out := &errWriter{w: w}

	var passed, failed int

	for _, r := range reports {
		if r.Passed() {
			passed++
		} else {
			failed++
		}
	}

	out.printf("\nraycheck: %s: %d ok, %d failed\n\n", humanize.Count(int64(len(reports)), "dataset"), passed, failed)

	width := 0

	for _, r := range reports {
		if n := len(r.Name); n > width {
			width = n
		}
	}

	for _, r := range reports {
		status := "ok  "
		if !r.Passed() {
			status = "FAIL"
		}

		out.printf("  %s   %-*s   k=%-6d threshold %d", status, width, r.Name, r.Result.K, r.Result.Threshold)

		if n := len(r.Concerns()); n > 0 {
			out.printf(", %s", humanize.Count(int64(n), "concern"))
		}

		out.print("\n")
	}

	return out.err
}

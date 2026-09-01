package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aliramazanov/raycheck/internal/humanize"
	"github.com/aliramazanov/raycheck/internal/measure"
)

const textGroups = 5

func Text(w io.Writer, r Report) error {
	out := &errWriter{w: w}

	writeHeader(out, r)
	writeCounts(out, r.Result)
	writeMeasure(out, r.Result)
	writeDiversity(out, r.Result)
	writeCloseness(out, r.Result)
	writeConcerns(out, r)
	writeGroups(out, r.Result)
	writeLargest(out, r.Result)
	writeRisk(out, r.Result)
	writeDeferred(out, r)
	writeVerdict(out, r)

	return out.err
}

func writeHeader(w *errWriter, r Report) {
	label := ""
	if r.Name != "" {
		label = quoteUnprintable(r.Name) + ": "
	}

	w.printf("raycheck: %sk=%d (threshold %d)\n\n", label, r.Result.K, r.Result.Threshold)
}

func writeCounts(w *errWriter, res measure.Result) {
	w.printf("  %s checked across %s\n",
		humanize.Count(res.Rows, "row"), humanize.Count(int64(len(res.QuasiIdentifiers)), "quasi-identifier"))

	if res.Suppressed > 0 {
		w.printf("  %s excluded as suppressed\n", humanize.Count(res.Suppressed, "row"))
	}

	if res.EmptyQI > 0 {
		w.printf("  %s with an empty quasi-identifier cell, grouped as written\n",
			humanize.Count(res.EmptyQI, "row"))
	}

	w.print("\n")
}

func writeMeasure(w *errWriter, res measure.Result) {
	status := "ok  "
	if !res.KThresholdMet() {
		status = "FAIL"
	}

	w.printf("  %s   k-anonymity      k=%d     %s\n", status, res.K, headline(res))

	if res.KThresholdMet() {
		return
	}

	if res.UniqueRows > 0 {
		w.printf("         %s (%s of the rows checked) are alone in their group and can be\n",
			humanize.Count(res.UniqueRows, "row"), percent(res.UniqueShare()))
		w.print("         singled out by someone who already knows their target is here.\n")
	}

	w.printf("         %s (%s) sit in a group smaller than %d, so each of them can be\n",
		humanize.Count(res.RowsAtRisk, "row"), percent(res.AtRiskShare()), res.Threshold)
	w.printf("         narrowed to fewer than %d people.\n", res.Threshold)
}

func writeDiversity(w *errWriter, res measure.Result) {
	worst, ok := res.WorstDiversity()

	if !ok {
		return
	}

	status := gauge(res.LThreshold > 0, res.DiversityPassed())

	threshold := ""
	if res.LThreshold > 0 {
		threshold = fmt.Sprintf(" (threshold %d)", res.LThreshold)
	}

	w.printf("  %s   l-diversity      l=%-5d %s\n", status, worst.L, headlineL(worst, threshold))

	if !res.DiversityPassed() {
		writeWorstClass(w, worst.Worst, worst.WorstRow)
	}

	if worst.L == 1 && !res.DiversityPassed() {
		w.printf("         every row in that class carries the same %q, so knowing which class\n", worst.Attribute)
		w.print("         someone falls in is enough to know their value.\n")
	}
}

func writeWorstClass(w *errWriter, values []string, row int64) {
	if len(values) == 0 {
		return
	}

	w.printf("         that class is (%s), first at row %d\n", display(values), row)
}

func gauge(gated, passed bool) string {
	switch {
	case !gated:
		return "--  "
	case passed:
		return "ok  "
	default:
		return "FAIL"
	}
}

func headlineL(d measure.Diversity, threshold string) string {
	if threshold == "" {
		threshold = ", not gated"
	}

	if d.L == 1 {
		return fmt.Sprintf("one class is uniform on %q%s", d.Attribute, threshold)
	}

	return fmt.Sprintf("every class holds at least %d values of %q%s", d.L, d.Attribute, threshold)
}

func writeCloseness(w *errWriter, res measure.Result) {
	worst, ok := res.WorstCloseness()

	if !ok {
		return
	}

	status := gauge(res.TThreshold > 0, res.ClosenessPassed())

	threshold := ""
	if res.TThreshold > 0 {
		threshold = fmt.Sprintf(" (threshold %s)", trim(res.TThreshold))
	}

	w.printf("  %s   t-closeness      t=%-5s %s\n", status, trim(worst.T), headlineT(worst, threshold))

	if !res.ClosenessPassed() {
		writeWorstClass(w, worst.Worst, worst.WorstRow)
	}

	if !res.ClosenessPassed() {
		w.printf("         that class's spread of %q is far enough from the whole file that\n", worst.Attribute)
		w.print("         knowing which class someone falls in narrows their value.\n")
	}
}

func headlineT(c measure.Closeness, threshold string) string {
	if threshold == "" {
		threshold = ", not gated"
	}

	return fmt.Sprintf("one class sits %s from the file on %q%s", trim(c.T), c.Attribute, threshold)
}

func trim(f float64) string {
	s := strconv.FormatFloat(f, 'f', 3, 64)

	for strings.HasSuffix(s, "0") {
		s = strings.TrimSuffix(s, "0")
	}

	return strings.TrimSuffix(s, ".")
}

func writeConcerns(w *errWriter, r Report) {
	status := "note"
	if r.Strict {
		status = "FAIL"
	}

	for _, c := range r.Concerns() {
		w.printf("  %s   %s\n", status, c.Message)
	}
}

func writeGroups(w *errWriter, res measure.Result) {
	if len(res.Below) == 0 {
		return
	}

	shown := res.Below

	if len(shown) > textGroups {
		shown = shown[:textGroups]
	}

	w.print("\n  smallest groups\n")

	for _, g := range shown {
		writeGroupLine(w, g.Count, g.Values, g.FirstRow)
	}

	if res.BelowGroups > len(shown) {
		w.printf("    showing %d of %s below the threshold\n",
			len(shown), humanize.Count(int64(res.BelowGroups), "group"))
	}
}

func writeLargest(w *errWriter, res measure.Result) {
	if !res.KThresholdMet() {
		return
	}

	w.print("\n  largest group\n")
	writeGroupLine(w, res.Largest.Count, res.Largest.Values, res.Largest.FirstRow)
}

func writeRisk(w *errWriter, res measure.Result) {
	w.print("\n  risk to a person in this file, assuming an attacker who knows their\n")
	w.printf("  target is present: %s at worst, %s on average, %s at best\n",
		percent(res.HighestRisk()), percent(res.AverageRisk()), percent(res.LowestRisk()))
}

func writeDeferred(w *errWriter, r Report) {
	if !r.HasSensitive {
		return
	}

	w.print("\n  every measure raycheck implements ran against the sensitive columns you\n")
	w.print("  declared. That is not the same as the data being safe to publish.\n")
}

func writeVerdict(w *errWriter, r Report) {
	res := r.Result

	w.print("\n")

	switch {
	case !res.KThresholdMet() && res.UniqueRows > 0:
		w.printf("  %s can be singled out. This data is not anonymous.\n",
			humanize.Count(res.UniqueRows, "row"))

	case !res.KThresholdMet():
		w.printf("  no row is unique, but %s sit in a group smaller than %d, so this\n",
			humanize.Count(res.RowsAtRisk, "row"), res.Threshold)
		w.print("  data does not meet the threshold it was checked against.\n")

	case !res.DiversityPassed():
		d, _ := res.WorstDiversity()

		w.printf("  k=%d clears the threshold, but a class holds only %s of %q, so knowing\n",
			res.K, humanize.Count(int64(d.L), "value"), d.Attribute)
		w.print("  which class someone falls in narrows their value that far. This data\n")
		w.print("  does not meet the l-diversity threshold it was checked against.\n")

	case !res.ClosenessPassed():
		c, _ := res.WorstCloseness()

		w.printf("  k=%d clears the threshold, but a class sits %.3g from the whole file on\n",
			res.K, c.T)
		w.printf("  %q, so membership still says more about a person than the file does.\n", c.Attribute)
		w.print("  This data does not meet the t-closeness threshold it was checked against.\n")

	case r.strictBreach():
		w.printf("  k=%d clears the threshold, but the concerns above stand, so this is\n", res.K)
		w.print("  not an all clear under --strict.\n")

	default:
		w.printf("  no row can be singled out on these quasi-identifiers at k=%d", res.Threshold)

		if res.Suppressed > 0 {
			w.printf(",\n  among the %s that were measured", humanize.Count(res.Rows, "row"))
		}

		w.print(".\n")
		w.print("  k-anonymity is a necessary check, not a guarantee, and passing it does\n")
		w.print("  not make this data anonymous under the GDPR.\n")
	}
}

type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}

	_, e.err = fmt.Fprintf(e.w, format, args...)
}

func (e *errWriter) print(s string) {
	if e.err != nil {
		return
	}

	_, e.err = io.WriteString(e.w, s)
}

func writeGroupLine(w *errWriter, count int, values []string, row int64) {
	w.printf("    %-8s %-38s first at row %d\n",
		humanize.Count(int64(count), "row"), display(values), row)
}

func display(values []string) string {
	out := make([]string, len(values))

	for i, v := range values {
		out[i] = quoteUnprintable(v)
	}

	return strings.Join(out, ", ")
}

func quoteUnprintable(s string) string {
	if !utf8.ValidString(s) {
		return strconv.Quote(s)
	}

	for _, r := range s {
		if !unicode.IsPrint(r) {
			return strconv.Quote(s)
		}
	}

	return s
}

func headline(res measure.Result) string {
	cols := "(" + display(res.QuasiIdentifiers) + ")"

	if res.UniqueRows > 0 {
		return fmt.Sprintf("%s unique on %s", humanize.Count(res.UniqueRows, "row"), cols)
	}

	if res.KThresholdMet() {
		return fmt.Sprintf("every group on %s holds at least %d rows", cols, res.K)
	}

	return fmt.Sprintf("%s in groups smaller than %d on %s",
		humanize.Count(res.RowsAtRisk, "row"), res.Threshold, cols)
}

func percent(f float64) string {
	switch p := f * 100; {
	case p == 0:
		return "0%"
	case p < 0.01:
		return "<0.01%"
	case p >= 10:
		return strconv.FormatFloat(p, 'f', 0, 64) + "%"
	case p >= 1:
		return strconv.FormatFloat(p, 'f', 1, 64) + "%"
	default:
		return strconv.FormatFloat(p, 'f', 2, 64) + "%"
	}
}

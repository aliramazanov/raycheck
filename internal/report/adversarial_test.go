package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"strconv"

	"github.com/aliramazanov/raycheck/internal/measure"
)

func TestAdversarialJSONStrictContradictsItself(t *testing.T) {
	g := measure.NewGrouper([]string{"a", "b"}, []int{0, 1}, nil)

	for i := 0; i < 3; i++ {
		g.Add([]string{"1984", ""}, int64(i+2))
	}

	res, err := g.Finish(3, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	r := Report{Result: res, Strict: true}

	if !Failed([]Report{r}) {
		t.Fatal("expected the strict breach to fail the run")
	}

	var buf bytes.Buffer

	if err := JSON(&buf, []Report{r}, nil); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Failed   bool `json:"failed"`
		Datasets []struct {
			Passed bool `json:"passed"`
		} `json:"datasets"`
	}

	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}

	t.Logf("failed=%v while datasets[0].passed=%v", doc.Failed, doc.Datasets[0].Passed)

	if doc.Failed && doc.Datasets[0].Passed {
		t.Error("the document says the run failed and the only dataset passed")
	}
}

func TestAdversarialJSONOmitsStrictReason(t *testing.T) {
	g := measure.NewGrouper([]string{"a", "b"}, []int{0, 1}, nil)

	for i := 0; i < 3; i++ {
		g.Add([]string{"1984", ""}, int64(i+2))
	}

	res, _ := g.Finish(3, 0, 0)

	var buf bytes.Buffer

	_ = JSON(&buf, []Report{{Result: res, Strict: true}}, nil)

	if !strings.Contains(buf.String(), "strict") {
		t.Errorf("the JSON never mentions strict:\n%s", buf.String())
	}
}

func TestAdversarialTinyRiskRendersAsZero(t *testing.T) {
	for _, f := range []float64{0.0000001, 0.00004} {
		if got := percent(f); got == "0.00%" {
			t.Errorf("a nonzero risk of %v renders as %q, which reads as no risk at all", f, got)
		}
	}
}

func TestAdversarialPercentRounding(t *testing.T) {
	tests := map[float64]string{
		0.995:   "100%",
		0.09996: "10.0%",
		0.0999:  "10.0%",
	}

	for f, want := range tests {
		if got := percent(f); got != want {
			t.Errorf("percent(%v) = %s, want %s", f, got, want)
		}
	}
}

func TestAdversarialMajoritySuppressedPassStatesTheExclusion(t *testing.T) {
	marker := "*"

	g := measure.NewGrouper([]string{"a"}, []int{0}, &marker)

	for i := range 90 {
		g.Add([]string{"*"}, int64(i+2))
	}

	for i := range 10 {
		g.Add([]string{"same"}, int64(i+92))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: res, Strict: true}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	if !res.KThresholdMet() {
		t.Fatalf("k=10 clears a threshold of 5, so this must pass on k alone:\n%s", out)
	}

	for _, want := range []string{
		"90 rows excluded as suppressed",
		"90 of 100 rows were excluded",
		"not an all clear",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("a pass reached by excluding 90 of 100 rows must say %q:\n%s", want, out)
		}
	}
}

func TestAdversarialValueCannotForgeOutput(t *testing.T) {
	forged := "x\n  ok     k-anonymity      k=99    every group holds at least 99 rows\n"

	g := measure.NewGrouper([]string{"a"}, []int{0}, nil)
	g.Add([]string{forged}, 2)
	g.Add([]string{"\x1b[2J\x1b[H"}, 3)
	g.Add([]string{"plain"}, 4)

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	t.Logf("%s", out)

	if strings.Contains(out, "\n  ok     k-anonymity      k=99") {
		t.Error("a value forged a passing measure line")
	}
	if strings.Contains(out, "\x1b") {
		t.Error("an escape sequence reached the terminal")
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "line") || line == "  ok     k-anonymity      k=99    every group holds at least 99 rows" {
			t.Errorf("unexpected forged line: %q", line)
		}
	}

	if !strings.Contains(out, "plain") {
		t.Error("an ordinary value should still print unquoted")
	}
}

func TestNumericColumnHoldingNonNumbersIsRaised(t *testing.T) {
	t.Parallel()

	build := func(values []string, kind measure.Kind) Report {
		g := measure.NewGrouper([]string{"q"}, []int{0}, nil).
			WithSensitive([]string{"s"}, []int{1}, []measure.Kind{kind})

		row := int64(2)

		for i, v := range values {
			g.Add([]string{string(rune('a' + i%2)), v}, row)
			row++
		}

		res, err := g.Finish(1, 0, 0.9)

		if err != nil {
			t.Fatal(err)
		}

		return Report{Result: res, HasSensitive: true}
	}

	mixed := build([]string{"1", "2", "1,5", "unknown"}, measure.Numeric)

	if !hasConcern(mixed, "numeric_column_holds_non_numbers") {
		t.Errorf("a numeric column holding %q and %q must be raised, got %v", "1,5", "unknown", mixed.Concerns())
	}

	clean := build([]string{"1", "2", "10", "3"}, measure.Numeric)

	if hasConcern(clean, "numeric_column_holds_non_numbers") {
		t.Errorf("a column of real numbers must raise nothing, got %v", clean.Concerns())
	}

	categorical := build([]string{"1", "2", "1,5", "unknown"}, measure.Categorical)

	if hasConcern(categorical, "numeric_column_holds_non_numbers") {
		t.Error("a categorical column has no line to sit on, so nothing is off it")
	}
}

func TestControlBytesAreEscapedForPeopleAndFaithfulForMachines(t *testing.T) {
	t.Parallel()

	controls := []string{"\x01", "\x02", "\a", "\b", "\v", "\f", "\x1a", "\x1b", "\x1f", "\x7f"}

	g := measure.NewGrouper([]string{"a"}, []int{0}, nil)

	row := int64(2)

	for _, c := range controls {
		for range 2 {
			g.Add([]string{"x" + c + "y"}, row)
			row++
		}
	}

	res, err := g.Finish(99, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if res.Groups != len(controls) {
		t.Fatalf("a control byte is part of the value: want %d groups, got %d", len(controls), res.Groups)
	}

	var text strings.Builder

	if err := Text(&text, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	for _, b := range []byte(text.String()) {
		if b < 9 || (b > 10 && b < 32) || b == 127 {
			t.Fatalf("a raw control byte %#x reached the human report", b)
		}
	}

	var doc bytes.Buffer

	if err := JSON(&doc, []Report{{Result: res}}, nil); err != nil {
		t.Fatal(err)
	}

	var parsed struct {
		Datasets []struct {
			WorstGroups []struct {
				Values map[string]string `json:"values"`
			} `json:"worst_groups"`
		} `json:"datasets"`
	}

	if err := json.Unmarshal(doc.Bytes(), &parsed); err != nil {
		t.Fatalf("the document does not parse: %v", err)
	}

	got := map[string]bool{}

	for _, gr := range parsed.Datasets[0].WorstGroups {
		got[gr.Values["a"]] = true
	}

	for _, c := range controls {
		if !got["x"+c+"y"] {
			t.Errorf("JSON lost the value carrying %#x", c[0])
		}
	}
}

func TestPercentAtTheFormatBoundaries(t *testing.T) {
	t.Parallel()

	tests := map[float64]string{
		0:       "0%",
		0.00005: "<0.01%",
		0.0001:  "0.01%",
		0.0099:  "0.99%",
		0.01:    "1.0%",
		0.0999:  "10.0%",
		0.1:     "10%",
		1.0:     "100%",
	}

	for f, want := range tests {
		if got := percent(f); got != want {
			t.Errorf("percent(%v) = %q, want %q", f, got, want)
		}
	}
}

func TestDiversityMessageDistinguishesUniformFromMerelyNarrow(t *testing.T) {
	t.Parallel()

	build := func(perClass int) string {
		g := measure.NewGrouper([]string{"q"}, []int{0}, nil).
			WithSensitive([]string{"s"}, []int{1}, []measure.Kind{measure.Categorical})

		row := int64(2)

		for class := range 2 {
			for i := range 4 {
				g.Add([]string{strconv.Itoa(class), strconv.Itoa(i % perClass)}, row)
				row++
			}
		}

		res, err := g.Finish(1, 3, 0)

		if err != nil {
			t.Fatal(err)
		}

		if res.DiversityPassed() {
			t.Fatalf("perClass %d: the fixture must breach l", perClass)
		}

		var b strings.Builder

		if err := Text(&b, Report{Result: res, HasSensitive: true}); err != nil {
			t.Fatal(err)
		}

		return b.String()
	}

	uniform := build(1)

	if !strings.Contains(uniform, "uniform") {
		t.Errorf("l=1 should be described as uniform:\n%s", uniform)
	}

	narrow := build(2)

	if strings.Contains(narrow, "uniform") {
		t.Errorf("l=2 is not uniform, only narrow:\n%s", narrow)
	}
}

func TestJSONReportsWhetherStrictWasAsked(t *testing.T) {
	t.Parallel()

	for _, strict := range []bool{false, true} {
		g := measure.NewGrouper([]string{"q"}, []int{0}, nil)

		for i := 0; i < 4; i++ {
			g.Add([]string{"a"}, int64(i+2))
		}

		res, err := g.Finish(1, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer

		if err := JSON(&buf, []Report{{Result: res, Strict: strict}}, nil); err != nil {
			t.Fatal(err)
		}

		var doc struct {
			Strict bool `json:"strict"`
		}

		if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}

		if doc.Strict != strict {
			t.Errorf("ran with strict=%v, the document says %v", strict, doc.Strict)
		}
	}
}

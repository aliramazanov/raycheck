package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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

	if !res.Passed() {
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

package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aliramazanov/raycheck/internal/measure"
)

func result(t *testing.T, rows [][]string, threshold int, suppression *string) measure.Result {
	t.Helper()

	names := []string{"birth_date", "postcode", "gender"}

	g := measure.NewGrouper(names, []int{0, 1, 2}, suppression)

	for i, r := range rows {
		g.Add(r, int64(i+2))
	}

	res, err := g.Finish(threshold, 0, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return res
}

func marker(s string) *string { return &s }

var leaky = [][]string{
	{"1984-02-11", "11000", "F"},
	{"1984-03-02", "11001", "M"},
	{"1990-01-01", "11002", "F"},
	{"1990-01-01", "11002", "F"},
	{"*", "*", "*"},
}

const wantFailing = `raycheck: k=1 (threshold 5)

  4 rows checked across 3 quasi-identifiers
  1 row excluded as suppressed

  FAIL   k-anonymity      k=1     2 rows unique on (birth_date, postcode, gender)
         2 rows (50% of the rows checked) are alone in their group and can be
         singled out by someone who already knows their target is here.
         4 rows (100%) sit in a group smaller than 5, so each of them can be
         narrowed to fewer than 5 people.

  smallest groups
    1 row    1984-02-11, 11000, F                   first at row 2
    1 row    1984-03-02, 11001, M                   first at row 3
    2 rows   1990-01-01, 11002, F                   first at row 4

  risk to a person in this file, assuming an attacker who knows their
  target is present: 100% at worst, 75% on average, 50% at best

  every measure raycheck implements ran against the sensitive columns you
  declared. That is not the same as the data being safe to publish.

  2 rows can be singled out. This data is not anonymous.
`

func TestTextFailing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: result(t, leaky, 5, marker("*")), HasSensitive: true}); err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); got != wantFailing {
		t.Errorf("output mismatch\n--- want ---\n%s\n--- got ---\n%s", wantFailing, got)
	}
}

func TestTextPassingStatesItsLimits(t *testing.T) {
	t.Parallel()

	rows := make([][]string, 5)

	for i := range rows {
		rows[i] = []string{"1984", "110", "F"}
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: result(t, rows, 5, nil)}); err != nil {
		t.Fatal(err)
	}

	got := buf.String()

	for _, want := range []string{
		"k=5",
		"necessary check, not a guarantee",
		"not make this data anonymous under the GDPR",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("passing output should mention %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "FAIL") {
		t.Error("a passing run should not print FAIL")
	}
}

func TestTextNeverClaimsUnmeasuredMeasures(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: result(t, leaky, 5, nil), HasSensitive: true}); err != nil {
		t.Fatal(err)
	}

	got := buf.String()

	for _, forbidden := range []string{"l-diversity", "t-closeness"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("output reports %q for a dataset with no sensitive columns:\n%s", forbidden, got)
		}
	}
}

func TestTextReportsSuppressionAndEmptyCells(t *testing.T) {
	t.Parallel()

	rows := [][]string{
		{"*", "*", "*"},
		{"1984", "", "F"},
		{"1984", "", "F"},
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: result(t, rows, 2, marker("*"))}); err != nil {
		t.Fatal(err)
	}

	got := buf.String()

	if !strings.Contains(got, "1 row excluded as suppressed") {
		t.Errorf("want the suppressed count reported, got:\n%s", got)
	}
	if !strings.Contains(got, "2 rows with an empty quasi-identifier cell") {
		t.Errorf("want empty cells reported, got:\n%s", got)
	}
}

func TestJSONShape(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	if err := JSON(&buf, []Report{{Name: "seed", Result: result(t, leaky, 5, marker("*"))}}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc struct {
		Datasets []struct {
			Name                 string   `json:"name"`
			K                    int      `json:"k"`
			ThresholdK           int      `json:"threshold_k"`
			Passed               bool     `json:"passed"`
			RowsChecked          int64    `json:"rows_checked"`
			RowsSuppressed       int64    `json:"rows_suppressed"`
			UniqueRows           int64    `json:"unique_rows"`
			GroupsBelowThreshold int      `json:"groups_below_threshold"`
			NotMeasured          []string `json:"not_measured"`
			Risk                 struct {
				Model   string  `json:"model"`
				Highest float64 `json:"highest"`
				Average float64 `json:"average"`
			} `json:"risk"`
			WorstGroups []struct {
				Values   map[string]string `json:"values"`
				Count    int               `json:"count"`
				FirstRow int64             `json:"first_row"`
			} `json:"worst_groups"`
		} `json:"datasets"`
		Failed bool `json:"failed"`
	}

	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if !doc.Failed {
		t.Error("want failed=true")
	}

	d := doc.Datasets[0]

	switch {
	case d.Name != "seed":
		t.Errorf("want name seed, got %q", d.Name)
	case d.K != 1 || d.ThresholdK != 5 || d.Passed:
		t.Errorf("unexpected verdict fields: %+v", d)
	case d.RowsChecked != 4 || d.RowsSuppressed != 1 || d.UniqueRows != 2:
		t.Errorf("unexpected counts: %+v", d)
	case d.GroupsBelowThreshold != 3 || len(d.WorstGroups) != 3:
		t.Errorf("unexpected group counts: %+v", d)
	case d.Risk.Model != "prosecutor" || d.Risk.Highest != 1 || d.Risk.Average != 0.75:
		t.Errorf("unexpected risk block: %+v", d.Risk)
	}

	if len(d.NotMeasured) != 1 || d.NotMeasured[0] != "t_closeness" {
		t.Errorf("want t-closeness listed as not measured, got %v", d.NotMeasured)
	}

	if got := d.WorstGroups[0].Values["postcode"]; got != "11000" {
		t.Errorf("want group values keyed by column name, got %v", d.WorstGroups[0].Values)
	}
}

func TestJSONMarksTruncation(t *testing.T) {
	t.Parallel()

	names := []string{"birth_date", "postcode", "gender"}

	g := measure.NewGrouper(names, []int{0, 1, 2}, nil)
	g.SetMaxBelow(1)

	for i, r := range leaky[:4] {
		g.Add(r, int64(i+2))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := JSON(&buf, []Report{{Result: res}}, nil); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), `"worst_groups_truncated": true`) {
		t.Errorf("want truncation flagged, got:\n%s", buf.String())
	}
}

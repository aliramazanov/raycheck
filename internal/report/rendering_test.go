package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/aliramazanov/raycheck/internal/measure"
)

func TestAdversarialInvalidUTF8Values(t *testing.T) {
	a := "caf\xe9"
	b := "caf\xe8"

	g := measure.NewGrouper([]string{"name"}, []int{0}, nil)
	g.Add([]string{a}, 2)
	g.Add([]string{b}, 3)

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if res.Groups != 2 {
		t.Fatalf("want 2 distinct groups, got %d", res.Groups)
	}

	var text bytes.Buffer

	if err := Text(&text, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	t.Logf("text:\n%s", text.String())

	var buf bytes.Buffer

	if err := JSON(&buf, []Report{{Result: res}}, nil); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Datasets []struct {
			WorstGroups []struct {
				Values map[string]string `json:"values"`
			} `json:"worst_groups"`
		} `json:"datasets"`
	}

	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}

	groups := doc.Datasets[0].WorstGroups

	if len(groups) != 2 {
		t.Fatalf("want 2 groups in the document, got %d", len(groups))
	}

	first, second := groups[0].Values["name"], groups[1].Values["name"]

	t.Logf("JSON rendered them as %q and %q", first, second)

	if first == second {
		t.Errorf("two distinct values became indistinguishable in the JSON: both %q", first)
	}
}

func TestAdversarialDatasetNameCannotForgeOutput(t *testing.T) {
	g := measure.NewGrouper([]string{"a"}, []int{0}, nil)
	g.Add([]string{"x"}, 2)

	res, err := g.Finish(1, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Name: "seed\n  ok     k-anonymity      k=99", Result: res}); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(buf.String(), "\n  ok     k-anonymity      k=99") {
		t.Errorf("a dataset name forged a measure line:\n%s", buf.String())
	}
}

func TestAdversarialColumnNameCannotForgeOutput(t *testing.T) {
	g := measure.NewGrouper([]string{"a\nFAIL   forged"}, []int{0}, nil)
	g.Add([]string{"x"}, 2)

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(buf.String(), "\nFAIL   forged") {
		t.Errorf("a column name forged a line:\n%s", buf.String())
	}
}

func TestAdversarialJSONAlwaysParses(t *testing.T) {
	values := []string{"plain", "with\nnewline", "\x1b[2J", "caf\xe9", "\x00nul", strings.Repeat("x", 5000)}

	g := measure.NewGrouper([]string{"v"}, []int{0}, nil)

	for i, v := range values {
		g.Add([]string{v}, int64(i+2))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := JSON(&buf, []Report{{Result: res, Strict: true}}, nil); err != nil {
		t.Fatal(err)
	}

	var doc map[string]any

	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("the document does not parse: %v", err)
	}
}

func TestAdversarialConcernsAgreeAcrossOutputs(t *testing.T) {
	marker := "*"

	g := measure.NewGrouper([]string{"a"}, []int{0}, &marker)

	for i := 0; i < 9; i++ {
		g.Add([]string{"*"}, int64(i+2))
	}
	for i := 0; i < 5; i++ {
		g.Add([]string{""}, int64(i+11))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	r := Report{Result: res, Strict: true, SuppressionDeclared: true}

	var text bytes.Buffer

	if err := Text(&text, r); err != nil {
		t.Fatal(err)
	}

	var js bytes.Buffer

	if err := JSON(&js, []Report{r}, nil); err != nil {
		t.Fatal(err)
	}

	for _, c := range r.Concerns() {
		if !strings.Contains(text.String(), c.Message) {
			t.Errorf("concern %q missing from the text output", c.Key)
		}
		if !strings.Contains(js.String(), c.Key) {
			t.Errorf("concern %q missing from the document", c.Key)
		}
	}

	if got := len(r.Concerns()); got != 3 {
		t.Errorf("want three concerns raised, got %d", got)
	}
	if r.Passed() {
		t.Error("strict should fail on these concerns")
	}
}

func TestAdversarialFailingWithoutAnyUniqueRows(t *testing.T) {
	g := measure.NewGrouper([]string{"a"}, []int{0}, nil)

	for i := 0; i < 3; i++ {
		g.Add([]string{"x"}, int64(i+2))
	}
	for i := 0; i < 3; i++ {
		g.Add([]string{"y"}, int64(i+5))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if res.UniqueRows != 0 {
		t.Fatalf("no row should be unique, got %d", res.UniqueRows)
	}
	if res.Passed() {
		t.Fatal("k=3 must not clear a threshold of 5")
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	t.Logf("%s", out)

	if strings.Contains(out, "singled out") {
		t.Error("no row is unique, so nothing can be singled out")
	}
}

func TestAdversarialSingledOutCountIsTheUniqueCount(t *testing.T) {
	g := measure.NewGrouper([]string{"a"}, []int{0}, nil)

	g.Add([]string{"alone"}, 2)

	for i := 0; i < 3; i++ {
		g.Add([]string{"trio"}, int64(i+3))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if res.UniqueRows != 1 || res.RowsAtRisk != 4 {
		t.Fatalf("want 1 unique of 4 at risk, got %d and %d", res.UniqueRows, res.RowsAtRisk)
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	t.Logf("%s", out)

	if strings.Contains(out, "4 rows can be singled out") {
		t.Error("the at-risk count was reported as the singled-out count")
	}
	if !strings.Contains(out, "1 row can be singled out") {
		t.Error("the unique count should be the one called singled out")
	}
}

func TestTextCapsTheGroupList(t *testing.T) {
	t.Parallel()

	g := measure.NewGrouper([]string{"a"}, []int{0}, nil)

	for i := 0; i < 40; i++ {
		g.Add([]string{fmt.Sprintf("v%02d", i)}, int64(i+2))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	if n := strings.Count(out, "first at row "); n != textGroups {
		t.Errorf("want %d groups listed, got %d:\n%s", textGroups, n, out)
	}
	if !strings.Contains(out, "showing 5 of 40 groups below the threshold") {
		t.Errorf("want the cap stated, got:\n%s", out)
	}
}

func TestConcernAdaptsToADeclaredMarker(t *testing.T) {
	t.Parallel()

	build := func(declared bool) string {
		g := measure.NewGrouper([]string{"a", "b"}, []int{0, 1}, nil)

		for i := 0; i < 3; i++ {
			g.Add([]string{"x", ""}, int64(i+2))
		}

		res, err := g.Finish(3, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		concerns := Report{Result: res, SuppressionDeclared: declared}.Concerns()

		if len(concerns) != 1 {
			t.Fatalf("want one concern, got %d", len(concerns))
		}

		return concerns[0].Message
	}

	if msg := build(false); !strings.Contains(msg, "Declare the marker") {
		t.Errorf("with no marker declared the advice should be there, got %q", msg)
	}
	if msg := build(true); strings.Contains(msg, "Declare the marker") {
		t.Errorf("a marker is already declared, so the advice is wrong: %q", msg)
	}
}

func TestPassingRunShowsTheLargestGroup(t *testing.T) {
	t.Parallel()

	g := measure.NewGrouper([]string{"a", "b"}, []int{0, 1}, nil)

	for i := 0; i < 9; i++ {
		g.Add([]string{"*", "*"}, int64(i+2))
	}
	for i := 0; i < 5; i++ {
		g.Add([]string{"real", "value"}, int64(i+11))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed() {
		t.Fatal("this should pass on k alone, which is the point")
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	if !strings.Contains(out, "largest group") {
		t.Errorf("a passing run should show the largest group:\n%s", out)
	}
	if !strings.Contains(out, "9 rows   *, *") {
		t.Errorf("the pile of redactions should be visible:\n%s", out)
	}
}

func TestFailingRunOmitsTheLargestGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: result(t, leaky, 5, nil)}); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(buf.String(), "largest group") {
		t.Errorf("a failing run already shows the small groups:\n%s", buf.String())
	}
}

func TestTextDistinguishesInvalidUTF8(t *testing.T) {
	t.Parallel()

	a, b := "Pl\xe7en", "Pl\xf8en"

	if a == b {
		t.Fatal("the fixture is wrong")
	}

	g := measure.NewGrouper([]string{"city"}, []int{0}, nil)
	g.Add([]string{a}, 2)
	g.Add([]string{b}, 3)

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}
	if res.Groups != 2 {
		t.Fatalf("want 2 groups, got %d", res.Groups)
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	t.Logf("%s", out)

	if strings.Contains(out, "�") {
		t.Error("the replacement character reached the output, hiding the real bytes")
	}
	if !strings.Contains(out, `"Pl\xe7en"`) || !strings.Contains(out, `"Pl\xf8en"`) {
		t.Error("two distinct values must stay distinguishable in the report")
	}
}

func TestJSONEscapedValuesCarryNoStrayQuotes(t *testing.T) {
	t.Parallel()

	g := measure.NewGrouper([]string{"city"}, []int{0}, nil)
	g.Add([]string{"Pl\xe7en"}, 2)

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := JSON(&buf, []Report{{Result: res}}, nil); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Datasets []struct {
			WorstGroups []struct {
				Values        map[string]string `json:"values"`
				ValuesEscaped bool              `json:"values_escaped"`
			} `json:"worst_groups"`
		} `json:"datasets"`
	}

	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}

	got := doc.Datasets[0].WorstGroups[0]

	if !got.ValuesEscaped {
		t.Error("the group should be flagged as escaped")
	}
	if want := `Pl\xe7en`; got.Values["city"] != want {
		t.Errorf("want %q, got %q", want, got.Values["city"])
	}
}

func TestMarkerMatchingARealValueIsRaised(t *testing.T) {
	t.Parallel()

	marker := "M"

	build := func(rows [][]string) Report {
		g := measure.NewGrouper([]string{"city", "gender"}, []int{0, 1}, &marker)

		for i, r := range rows {
			g.Add(r, int64(i+2))
		}

		res, err := g.Finish(1, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		return Report{Result: res, SuppressionDeclared: true}
	}

	real := build([][]string{{"Praha", "M"}, {"Brno", "M"}, {"Praha", "F"}, {"Brno", "F"}})

	if !hasConcern(real, "marker_matched_one_column") {
		t.Errorf("a marker confined to one column should be raised, got %v", keys(real))
	}

	masked := build([][]string{{"M", "M"}, {"M", "M"}, {"Praha", "F"}, {"Brno", "F"}})

	if hasConcern(masked, "marker_matched_one_column") {
		t.Errorf("a marker spanning every column is doing its job, got %v", keys(masked))
	}
}

func hasConcern(r Report, key string) bool {
	for _, c := range r.Concerns() {
		if c.Key == key {
			return true
		}
	}

	return false
}

func keys(r Report) []string {
	var out []string

	for _, c := range r.Concerns() {
		out = append(out, c.Key)
	}

	return out
}

func TestSummaryNamesTheFailingDataset(t *testing.T) {
	t.Parallel()

	pass := result(t, [][]string{{"a", "a", "a"}, {"a", "a", "a"}}, 2, nil)
	fail := result(t, leaky, 5, nil)

	var buf bytes.Buffer

	reports := []Report{
		{Name: "patients", Result: pass},
		{Name: "staff", Result: fail},
	}

	if err := Summary(&buf, reports); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	for _, want := range []string{"2 datasets: 1 ok, 1 failed", "ok     patients", "FAIL   staff"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary should contain %q, got:\n%s", want, out)
		}
	}
}

func TestSummarySkippedForASingleDataset(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	if err := Summary(&buf, []Report{{Name: "only", Result: result(t, leaky, 5, nil)}}); err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 {
		t.Errorf("want nothing, got:\n%s", buf.String())
	}
}

func TestUndeclaredMarkerIsRaised(t *testing.T) {
	t.Parallel()

	build := func(filler string, marker *string) Report {
		g := measure.NewGrouper([]string{"a", "b", "c"}, []int{0, 1, 2}, marker)

		row := 2

		for i := 0; i < 9; i++ {
			g.Add([]string{filler, filler, filler}, int64(row))
			row++
		}
		for i := 0; i < 6; i++ {
			g.Add([]string{"x", "y", "z"}, int64(row))
			row++
		}

		res, err := g.Finish(5, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		return Report{Result: res, SuppressionDeclared: marker != nil}
	}

	star := "*"

	if r := build("REDACTED", &star); !hasConcern(r, "largest_group_is_uniform") {
		t.Errorf("a marker the config does not know about should be raised, got %v", keys(r))
	}
	if r := build("REDACTED", nil); !hasConcern(r, "largest_group_is_uniform") {
		t.Errorf("with no marker declared at all it should still be raised, got %v", keys(r))
	}

	redacted := "REDACTED"

	if r := build("REDACTED", &redacted); hasConcern(r, "largest_group_is_uniform") {
		t.Errorf("a declared marker is doing its job, got %v", keys(r))
	}

	ordinary := func() Report {
		g := measure.NewGrouper([]string{"a", "b", "c"}, []int{0, 1, 2}, &star)

		row := 2

		for i := 0; i < 9; i++ {
			g.Add([]string{"Praha", "1980s", "F"}, int64(row))
			row++
		}

		res, err := g.Finish(5, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		return Report{Result: res, SuppressionDeclared: true}
	}()

	if hasConcern(ordinary, "largest_group_is_uniform") {
		t.Errorf("uniform check fired on ordinary data, got %v", keys(ordinary))
	}

	twoCols := func() Report {
		g := measure.NewGrouper([]string{"bedrooms", "bathrooms"}, []int{0, 1}, &star)

		row := 2

		for i := 0; i < 9; i++ {
			g.Add([]string{"2", "2"}, int64(row))
			row++
		}

		res, err := g.Finish(5, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		return Report{Result: res, SuppressionDeclared: true}
	}()

	if hasConcern(twoCols, "largest_group_is_uniform") {
		t.Errorf("two agreeing columns are a coincidence, got %v", keys(twoCols))
	}
}

func TestDiversityFindingNamesItsClass(t *testing.T) {
	t.Parallel()

	build := func(threshold int) string {
		g := measure.NewGrouper([]string{"city", "year"}, []int{0, 1}, nil).
			WithSensitive([]string{"diagnosis"}, []int{2}, []measure.Kind{measure.Categorical})

		row := int64(2)

		for i := 0; i < 6; i++ {
			g.Add([]string{"Praha", "1980s", "diabetes"}, row)
			row++
		}
		for i, d := range []string{"flu", "asthma", "none", "flu", "asthma", "none"} {
			g.Add([]string{"Brno", "1990s", d}, int64(i)+row)
		}

		res, err := g.Finish(5, threshold, 0)

		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer

		if err := Text(&buf, Report{Result: res}); err != nil {
			t.Fatal(err)
		}

		return buf.String()
	}

	failing := build(2)

	if !strings.Contains(failing, "that class is (Praha, 1980s)") {
		t.Errorf("a failing finding should name its class:\n%s", failing)
	}

	if passing := build(0); strings.Contains(passing, "that class is") {
		t.Errorf("nothing is wrong, so no class should be named:\n%s", passing)
	}
}

func TestDiversityClassValuesAreEscaped(t *testing.T) {
	t.Parallel()

	g := measure.NewGrouper([]string{"city"}, []int{0}, nil).
		WithSensitive([]string{"d"}, []int{1}, []measure.Kind{measure.Categorical})

	for i := 0; i < 6; i++ {
		g.Add([]string{"Praha\n  FAIL   forged", "same"}, int64(i+2))
	}

	res, err := g.Finish(5, 2, 0)

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := Text(&buf, Report{Result: res}); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(buf.String(), "\n  FAIL   forged") {
		t.Errorf("a class value forged a line:\n%s", buf.String())
	}
}

func TestUngatedMeasuresAreNotMarkedOk(t *testing.T) {
	t.Parallel()

	build := func(lThreshold int, tThreshold float64) string {
		g := measure.NewGrouper([]string{"city"}, []int{0}, nil).
			WithSensitive([]string{"s"}, []int{1}, []measure.Kind{measure.Categorical})

		for i := 0; i < 6; i++ {
			g.Add([]string{"Praha", "same"}, int64(i+2))
		}

		res, err := g.Finish(5, lThreshold, tThreshold)

		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer

		if err := Text(&buf, Report{Result: res}); err != nil {
			t.Fatal(err)
		}

		return buf.String()
	}

	ungated := build(0, 0)

	for _, line := range strings.Split(ungated, "\n") {
		if strings.Contains(line, "l-diversity") || strings.Contains(line, "t-closeness") {
			if strings.Contains(line, "ok ") {
				t.Errorf("an ungated measure must not read as passing: %q", line)
			}
			if !strings.Contains(line, "not gated") {
				t.Errorf("an ungated measure should say so: %q", line)
			}
		}
	}

	gated := build(2, 0)

	if !strings.Contains(gated, "FAIL   l-diversity") {
		t.Errorf("a gated breach should fail:\n%s", gated)
	}
	const ungatedClaim = `l-diversity      l=1     one class is uniform on "s", not gated`

	if strings.Contains(gated, "not gated") && strings.Contains(gated, ungatedClaim) {
		t.Errorf("a gated measure should not claim it is ungated:\n%s", gated)
	}
}

package measure

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

func TestAdversarialZeroBoundedSet(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a zero bound panicked: %v", r)
		}
	}()

	set := newBelowSet(0)
	set.add(belowEntry{key: "1:a", count: 1, firstRow: 2})

	if len(set.entries) != 0 {
		t.Errorf("a zero bound should retain nothing, got %d", len(set.entries))
	}
}

func TestAdversarialNegativeBoundedSet(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a negative bound panicked: %v", r)
		}
	}()

	set := newBelowSet(-5)
	set.add(belowEntry{key: "1:a", count: 1, firstRow: 2})
}

func TestAdversarialZeroResultRiskFigures(t *testing.T) {
	var r Result

	for name, got := range map[string]float64{
		"highest":  r.HighestRisk(),
		"lowest":   r.LowestRisk(),
		"average":  r.AverageRisk(),
		"at risk":  r.AtRiskShare(),
		"coverage": r.Coverage(),
	} {
		if math.IsNaN(got) {
			t.Errorf("%s risk is NaN", name)
		}
		if got < 0 || got > 1 {
			t.Errorf("%s risk is out of range: %v", name, got)
		}
	}
}

func TestAdversarialRiskFiguresBounded(t *testing.T) {
	cases := [][][]string{
		{{"a"}},
		{{"a"}, {"a"}},
		{{"a"}, {"b"}, {"c"}},
		{{"a"}, {"a"}, {"a"}, {"b"}},
	}

	for i, rows := range cases {
		res := groupAll(t, rows, 2)

		for name, got := range map[string]float64{
			"highest":  res.HighestRisk(),
			"lowest":   res.LowestRisk(),
			"average":  res.AverageRisk(),
			"at risk":  res.AtRiskShare(),
			"coverage": res.Coverage(),
		} {
			if got < 0 || got > 1 {
				t.Errorf("case %d: %s risk is %v", i, name, got)
			}
		}

		if res.HighestRisk() < res.AverageRisk()-1e-9 {
			t.Errorf("case %d: highest %v is below average %v", i, res.HighestRisk(), res.AverageRisk())
		}
		if res.AverageRisk() < res.LowestRisk()-1e-9 {
			t.Errorf("case %d: average %v is below lowest %v", i, res.AverageRisk(), res.LowestRisk())
		}
	}
}

func TestAdversarialResultInvariants(t *testing.T) {
	rows := [][]string{
		{"a"}, {"a"}, {"b"}, {"c"}, {"c"}, {"c"}, {"d"}, {"e"},
	}

	for threshold := 1; threshold <= 10; threshold++ {
		res := groupAll(t, rows, threshold)

		switch {
		case res.Rows != int64(len(rows)):
			t.Errorf("threshold %d: want %d rows, got %d", threshold, len(rows), res.Rows)
		case res.K < 1:
			t.Errorf("threshold %d: k is %d", threshold, res.K)
		case res.K > res.MaxGroup:
			t.Errorf("threshold %d: k %d exceeds the largest group %d", threshold, res.K, res.MaxGroup)
		case res.RowsAtRisk > res.Rows:
			t.Errorf("threshold %d: %d rows at risk of %d", threshold, res.RowsAtRisk, res.Rows)
		case res.BelowGroups > res.Groups:
			t.Errorf("threshold %d: %d offending groups of %d", threshold, res.BelowGroups, res.Groups)
		case len(res.Below) > res.BelowGroups:
			t.Errorf("threshold %d: retained %d of %d", threshold, len(res.Below), res.BelowGroups)
		case res.KThresholdMet() != (res.K >= threshold):
			t.Errorf("threshold %d: verdict disagrees with k=%d", threshold, res.K)
		case res.KThresholdMet() && res.BelowGroups != 0:
			t.Errorf("threshold %d: passed with %d offending groups", threshold, res.BelowGroups)
		}
	}
}

func TestAdversarialShortRowAgainstIndex(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a row shorter than the indices was accepted silently")
		}
	}()

	g := NewGrouper([]string{"a", "b"}, []int{0, 5}, nil)
	g.Add([]string{"only"}, 2)
}

func TestAdversarialGroupersAreIndependent(t *testing.T) {
	a := NewGrouper([]string{"x"}, []int{0}, nil)
	b := NewGrouper([]string{"x"}, []int{0}, nil)

	a.Add([]string{"one"}, 2)
	b.Add([]string{"two"}, 2)
	a.Add([]string{"one"}, 3)

	ra, err := a.Finish(1, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	rb, err := b.Finish(1, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if ra.Rows != 2 || ra.Groups != 1 {
		t.Errorf("first grouper: %+v", ra)
	}
	if rb.Rows != 1 || rb.Groups != 1 {
		t.Errorf("second grouper: %+v", rb)
	}
}

func TestAdversarialFinishIsRepeatable(t *testing.T) {
	g := NewGrouper([]string{"x"}, []int{0}, nil)

	for _, v := range []string{"a", "a", "b"} {
		g.Add([]string{v}, 2)
	}

	first, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	second, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if first.K != second.K || first.Groups != second.Groups || first.BelowGroups != second.BelowGroups {
		t.Errorf("Finish is not repeatable: %+v then %+v", first, second)
	}
	if len(first.Below) != len(second.Below) {
		t.Fatalf("offending group counts differ: %d then %d", len(first.Below), len(second.Below))
	}
	for i := range first.Below {
		if strings.Join(first.Below[i].Values, ",") != strings.Join(second.Below[i].Values, ",") {
			t.Errorf("offending group %d differs between calls", i)
		}
	}
}

func TestLargestGroupIsDeterministicUnderTies(t *testing.T) {
	t.Parallel()

	g := NewGrouper([]string{"a"}, []int{0}, nil)

	row := 2

	for _, v := range []string{"d", "b", "a", "c"} {
		for range 3 {
			g.Add([]string{v}, int64(row))
			row++
		}
	}

	first, err := g.Finish(1, 0, 0)

	if err != nil {
		t.Fatal(err)
	}
	if first.MaxGroup != 3 {
		t.Fatalf("the fixture should tie at 3, got %d", first.MaxGroup)
	}

	for i := range 200 {
		again, err := g.Finish(1, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		if again.Largest.Values[0] != first.Largest.Values[0] {
			t.Fatalf("run %d reported %q where the first reported %q",
				i, again.Largest.Values[0], first.Largest.Values[0])
		}
	}

	if first.Largest.Values[0] != "a" {
		t.Errorf("want the tie broken by value, got %q", first.Largest.Values[0])
	}
}

func TestMarkerColumnsRecordsEveryColumnItMatched(t *testing.T) {
	t.Parallel()

	marker := "*"

	tests := map[string]struct {
		rows [][]string
		want []string
	}{
		"masked across every column": {
			rows: [][]string{{"*", "*", "*"}, {"a", "b", "c"}},
			want: []string{"c0", "c1", "c2"},
		},
		"marker is a real value in one column": {
			rows: [][]string{{"a", "*", "c"}, {"d", "*", "f"}, {"g", "h", "i"}},
			want: []string{"c1"},
		},
		"masked in two of three": {
			rows: [][]string{{"*", "*", "c"}, {"d", "e", "f"}},
			want: []string{"c0", "c1"},
		},
		"never matched": {
			rows: [][]string{{"a", "b", "c"}},
			want: nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := NewGrouper([]string{"c0", "c1", "c2"}, []int{0, 1, 2}, &marker)

			for i, r := range tt.rows {
				g.Add(r, int64(i+2))
			}

			res, err := g.Finish(1, 0, 0)

			if err != nil {
				t.Fatal(err)
			}

			if len(res.MarkerColumns) != len(tt.want) {
				t.Fatalf("want %v, got %v", tt.want, res.MarkerColumns)
			}

			for i := range tt.want {
				if res.MarkerColumns[i] != tt.want[i] {
					t.Fatalf("want %v, got %v", tt.want, res.MarkerColumns)
				}
			}
		})
	}
}

func TestDiversityCatchesAHomogeneousClass(t *testing.T) {
	t.Parallel()

	g := NewGrouper([]string{"city"}, []int{0}, nil).
		WithSensitive([]string{"diagnosis"}, []int{1}, []Kind{Categorical})

	for i := 0; i < 50; i++ {
		g.Add([]string{"Praha", "diabetes"}, int64(i+2))
	}

	for i, d := range []string{"asthma", "flu", "none", "asthma", "flu"} {
		g.Add([]string{"Brno", d}, int64(i+52))
	}

	res, err := g.Finish(5, 2, 0)

	if err != nil {
		t.Fatal(err)
	}

	if !res.KThresholdMet() {
		t.Errorf("k is 5 or better, so the k gate should pass: k=%d", res.K)
	}
	if res.DiversityPassed() {
		t.Error("a class where everyone shares a diagnosis must not clear l=2")
	}
	if res.MinL() != 1 {
		t.Errorf("want l=1, got %d", res.MinL())
	}

	worst, ok := res.WorstDiversity()

	if !ok || worst.Attribute != "diagnosis" {
		t.Fatalf("want the diagnosis column named, got %+v", worst)
	}
	if len(worst.Worst) != 1 || worst.Worst[0] != "Praha" {
		t.Errorf("want the Praha class named, got %v", worst.Worst)
	}
}

func TestDiversityUngatedByDefault(t *testing.T) {
	t.Parallel()

	g := NewGrouper([]string{"city"}, []int{0}, nil).
		WithSensitive([]string{"diagnosis"}, []int{1}, []Kind{Categorical})

	for i := 0; i < 10; i++ {
		g.Add([]string{"Praha", "diabetes"}, int64(i+2))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if res.MinL() != 1 {
		t.Errorf("want l reported as 1, got %d", res.MinL())
	}
	if !res.DiversityPassed() {
		t.Error("a threshold nobody set must not fail a build")
	}
}

func TestClosenessCatchesASkewedClass(t *testing.T) {
	t.Parallel()

	g := NewGrouper([]string{"city"}, []int{0}, nil).
		WithSensitive([]string{"diagnosis"}, []int{1}, []Kind{Categorical})

	row := int64(2)

	for i := 0; i < 100; i++ {
		d := "hypertension"

		if i >= 95 {
			d = []string{"asthma", "flu", "none", "migraine", "diabetes"}[i-95]
		}

		g.Add([]string{"Praha", d}, row)
		row++
	}

	for i := 0; i < 100; i++ {
		d := []string{"hypertension", "asthma", "flu", "none", "migraine"}[i%5]
		g.Add([]string{"Brno", d}, row)
		row++
	}

	res, err := g.Finish(5, 2, 0.2)

	if err != nil {
		t.Fatal(err)
	}

	if !res.KThresholdMet() {
		t.Errorf("k is 100, the k gate should pass: k=%d", res.K)
	}
	if !res.DiversityPassed() {
		t.Errorf("the skewed class still holds six values, so l=2 should pass: l=%d", res.MinL())
	}
	if res.ClosenessPassed() {
		t.Errorf("a class that is 95%% one value must not clear t=0.2: t=%v", res.MaxT())
	}

	worst, ok := res.WorstCloseness()

	if !ok || len(worst.Worst) != 1 || worst.Worst[0] != "Praha" {
		t.Fatalf("want the Praha class named, got %+v", worst)
	}
}

func TestClosenessIsZeroWhenEveryClassMirrorsTheFile(t *testing.T) {
	t.Parallel()

	for _, kind := range []Kind{Categorical, Numeric} {
		g := NewGrouper([]string{"city"}, []int{0}, nil).
			WithSensitive([]string{"v"}, []int{1}, []Kind{kind})

		row := int64(2)

		for _, city := range []string{"Praha", "Brno"} {
			for _, v := range []string{"1", "2", "3", "4"} {
				g.Add([]string{city, v}, row)
				row++
			}
		}

		res, err := g.Finish(1, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		if got := res.MaxT(); math.Abs(got) > 1e-12 {
			t.Errorf("kind %v: identical classes should sit at zero, got %v", kind, got)
		}
	}
}

func TestNumericAndCategoricalDistancesDiffer(t *testing.T) {
	t.Parallel()

	build := func(kind Kind) float64 {
		g := NewGrouper([]string{"city"}, []int{0}, nil).
			WithSensitive([]string{"v"}, []int{1}, []Kind{kind})

		row := int64(2)

		for i := 0; i < 10; i++ {
			g.Add([]string{"Praha", "1"}, row)
			row++
		}
		for i := 0; i < 10; i++ {
			g.Add([]string{"Brno", "9"}, row)
			row++
		}
		for i := 0; i < 10; i++ {
			g.Add([]string{"Plzen", "5"}, row)
			row++
		}

		res, err := g.Finish(1, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		return res.MaxT()
	}

	if cat, num := build(Categorical), build(Numeric); math.Abs(cat-num) < 1e-9 {
		t.Errorf("categorical and numeric distance agreed exactly (%v), which they should not", cat)
	}
}

func TestClosenessDoesNotFailOnFloatingPointNoise(t *testing.T) {
	t.Parallel()

	g := NewGrouper([]string{"a"}, []int{0}, nil).
		WithSensitive([]string{"s"}, []int{1}, []Kind{Numeric})

	for i, r := range [][]string{{"x", "1"}, {"y", "2"}, {"z", "unknown"}} {
		g.Add(r, int64(i+2))
	}

	res, err := g.Finish(1, 0, 0.5)

	if err != nil {
		t.Fatal(err)
	}

	if got := res.MaxT(); got <= 0.5 {
		t.Skipf("this fixture no longer overshoots: t=%v", got)
	}

	if !res.ClosenessPassed() {
		t.Errorf("t=%.17g against a threshold of 0.5 must pass; the difference is noise", res.MaxT())
	}
}

func TestClosenessStillFailsWhenGenuinelyOver(t *testing.T) {
	t.Parallel()

	g := NewGrouper([]string{"a"}, []int{0}, nil).
		WithSensitive([]string{"s"}, []int{1}, []Kind{Categorical})

	for i, r := range [][]string{{"x", "p"}, {"x", "p"}, {"y", "q"}, {"y", "q"}} {
		g.Add(r, int64(i+2))
	}

	res, err := g.Finish(1, 0, 0.4)

	if err != nil {
		t.Fatal(err)
	}

	if res.ClosenessPassed() {
		t.Errorf("t=%v against a threshold of 0.4 must fail", res.MaxT())
	}
}

func TestSuppressedRowsLeaveTheDistributions(t *testing.T) {
	t.Parallel()

	marker := "*"

	rows := [][]string{}

	for i := 0; i < 20; i++ {
		rows = append(rows, []string{"*", "z"})
	}
	for i := 0; i < 10; i++ {
		rows = append(rows, []string{"x", []string{"p", "q"}[i%2]})
	}
	for i := 0; i < 10; i++ {
		rows = append(rows, []string{"y", []string{"p", "q"}[i%2]})
	}

	build := func(m *string) Result {
		g := NewGrouper([]string{"a"}, []int{0}, m).
			WithSensitive([]string{"s"}, []int{1}, []Kind{Categorical})

		for i, r := range rows {
			g.Add(r, int64(i+2))
		}

		res, err := g.Finish(1, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		return res
	}

	with := build(&marker)

	if with.Rows != 20 || with.Suppressed != 20 {
		t.Fatalf("want 20 measured and 20 excluded, got %d and %d", with.Rows, with.Suppressed)
	}
	if with.MinL() != 2 {
		t.Errorf("want l=2, got %d", with.MinL())
	}
	if got := with.MaxT(); got > 1e-12 {
		t.Errorf("classes that mirror the file must sit at zero, got %v", got)
	}

	without := build(nil)

	if without.MinL() != 1 {
		t.Errorf("the excluded class is uniform, so l should be 1, got %d", without.MinL())
	}
	if without.MaxT() <= 0 {
		t.Errorf("a class holding only z must sit away from the file, got %v", without.MaxT())
	}
}

func TestClosenessOnASingleValuedDomain(t *testing.T) {
	t.Parallel()

	for _, kind := range []Kind{Numeric, Categorical} {
		g := NewGrouper([]string{"a"}, []int{0}, nil).
			WithSensitive([]string{"s"}, []int{1}, []Kind{kind})

		for i, v := range []string{"x", "y", "z"} {
			g.Add([]string{v, "5"}, int64(i+2))
		}

		res, err := g.Finish(1, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		got := res.MaxT()

		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Fatalf("kind %v: t is %v, which cannot be rendered or serialised", kind, got)
		}
		if got != 0 {
			t.Errorf("kind %v: every class holds the only value there is, so t is 0, got %v", kind, got)
		}
	}
}

func TestRiskFiguresAreAlwaysFinite(t *testing.T) {
	t.Parallel()

	cases := map[string][][]string{
		"one row":              {{"a", "1"}},
		"one sensitive value":  {{"a", "1"}, {"b", "1"}, {"c", "1"}},
		"one class":            {{"a", "1"}, {"a", "2"}},
		"one of everything":    {{"a", "1"}},
		"all values identical": {{"a", "1"}, {"a", "1"}},
	}

	for name, rows := range cases {
		for _, kind := range []Kind{Numeric, Categorical} {
			g := NewGrouper([]string{"q"}, []int{0}, nil).
				WithSensitive([]string{"s"}, []int{1}, []Kind{kind})

			for i, r := range rows {
				g.Add(r, int64(i+2))
			}

			res, err := g.Finish(1, 0, 0)

			if err != nil {
				t.Fatal(err)
			}

			for label, v := range map[string]float64{
				"highest":  res.HighestRisk(),
				"lowest":   res.LowestRisk(),
				"average":  res.AverageRisk(),
				"coverage": res.Coverage(),
				"t":        res.MaxT(),
			} {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Errorf("%s/%v: %s is %v", name, kind, label, v)
				}
			}
		}
	}
}

func TestDiversityThresholdBoundary(t *testing.T) {
	t.Parallel()

	build := func(threshold int) Result {
		g := NewGrouper([]string{"a"}, []int{0}, nil).
			WithSensitive([]string{"s"}, []int{1}, []Kind{Categorical})

		row := int64(2)

		for _, c := range []string{"x", "y"} {
			for _, v := range []string{"p", "q", "r"} {
				g.Add([]string{c, v}, row)
				row++
			}
		}

		res, err := g.Finish(1, threshold, 0)

		if err != nil {
			t.Fatal(err)
		}

		return res
	}

	if res := build(3); !res.DiversityPassed() {
		t.Errorf("l=%d against a threshold of 3 must pass", res.MinL())
	}
	if res := build(4); res.DiversityPassed() {
		t.Errorf("l=%d against a threshold of 4 must fail", res.MinL())
	}
}

func TestSensitiveCountingAcrossThePromotionBoundary(t *testing.T) {
	t.Parallel()

	for _, distinct := range []int{1, promoteAt - 1, promoteAt, promoteAt + 1, promoteAt * 4} {
		g := NewGrouper([]string{"q"}, []int{0}, nil).
			WithSensitive([]string{"s"}, []int{1}, []Kind{Categorical})

		row := int64(2)

		for pass := 0; pass < 2; pass++ {
			for i := 0; i < distinct; i++ {
				g.Add([]string{"same", strconv.Itoa(i)}, row)
				row++
			}
		}

		res, err := g.Finish(1, 0, 0)

		if err != nil {
			t.Fatalf("distinct %d: %v", distinct, err)
		}

		if res.Rows != int64(distinct*2) {
			t.Errorf("distinct %d: want %d rows, got %d", distinct, distinct*2, res.Rows)
		}
		if res.MinL() != distinct {
			t.Errorf("distinct %d: want l=%d, got %d", distinct, distinct, res.MinL())
		}

		if got := res.MaxT(); math.Abs(got) > 1e-12 {
			t.Errorf("distinct %d: want t=0, got %v", distinct, got)
		}
	}
}

func TestPromotedClassMatchesScannedClass(t *testing.T) {
	t.Parallel()

	build := func(distinct, repeats int) Result {
		g := NewGrouper([]string{"q"}, []int{0}, nil).
			WithSensitive([]string{"s"}, []int{1}, []Kind{Numeric})

		row := int64(2)

		for i := 0; i < distinct; i++ {

			for j := 0; j <= i%repeats; j++ {
				g.Add([]string{"one", strconv.Itoa(i)}, row)
				row++
			}
			g.Add([]string{"two", strconv.Itoa(i)}, row)
			row++
		}

		res, err := g.Finish(1, 0, 0)

		if err != nil {
			t.Fatal(err)
		}

		return res
	}

	small := build(promoteAt-10, 3)
	large := build(promoteAt+10, 3)

	if small.MaxT() <= 0 || large.MaxT() <= 0 {
		t.Fatalf("expected a nonzero distance either side: %v and %v", small.MaxT(), large.MaxT())
	}
	if math.Abs(small.MaxT()-large.MaxT()) > 0.2 {
		t.Errorf("the distance jumped across the boundary: %v then %v", small.MaxT(), large.MaxT())
	}
}

func TestTiedDiversityNamesTheSameClassEveryRun(t *testing.T) {
	t.Parallel()

	measure := func() Diversity {
		g := NewGrouper([]string{"q"}, []int{0}, nil).
			WithSensitive([]string{"s"}, []int{1}, []Kind{Categorical})

		row := int64(2)

		for _, q := range []string{"d", "b", "a", "c"} {
			for i := 0; i < 3; i++ {
				g.Add([]string{q, "same"}, row)
				row++
			}
		}

		for i := 0; i < 3; i++ {
			g.Add([]string{"e", strconv.Itoa(i)}, row)
			row++
		}

		res, err := g.Finish(1, 2, 0)

		if err != nil {
			t.Fatal(err)
		}

		return res.Diversity[0]
	}

	want := measure()

	if want.L != 1 {
		t.Fatalf("expected the tie to sit at l=1, got %d", want.L)
	}

	for i := 0; i < 300; i++ {
		got := measure()

		if got.WorstRow != want.WorstRow || got.L != want.L {
			t.Fatalf("run %d named a different class: row %d (l=%d) then row %d (l=%d)",
				i, want.WorstRow, want.L, got.WorstRow, got.L)
		}
	}

	if len(want.Worst) != 1 || want.Worst[0] != "a" {
		t.Errorf("expected the lowest-sorting tied class, got %v", want.Worst)
	}
}

var axisShapes = []string{
	"1", "1.0", "01", "+1", "1.00", "0x1p0",
	"2", "2.0", "0002", "10", "10.0", "3",
	"", "NaN", "Inf", "-Inf", "5x", "7z", "unknown",
}

func TestResultsDoNotDependOnMapOrder(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(20260821))

	for shape := 0; shape < 40; shape++ {
		qi := 1 + rng.Intn(3)
		card := 1 + rng.Intn(6)
		rows := 1 + rng.Intn(200)

		type record struct {
			values []string
			row    int64
		}

		records := make([]record, rows)

		for i := range records {
			v := make([]string, qi+2)

			for j := range qi {
				v[j] = "q" + strconv.Itoa(rng.Intn(card))
			}

			v[qi] = string(rune('a' + rng.Intn(4)))
			v[qi+1] = axisShapes[rng.Intn(len(axisShapes))]
			records[i] = record{values: v, row: int64(i + 2)}
		}

		names := make([]string, qi)
		idx := make([]int, qi)

		for j := range names {
			names[j] = "q" + strconv.Itoa(j)
			idx[j] = j
		}

		run := func() Result {
			g := NewGrouper(names, idx, nil).
				WithSensitive([]string{"s", "n"}, []int{qi, qi + 1}, []Kind{Categorical, Numeric})

			for _, r := range records {
				g.Add(r.values, r.row)
			}

			res, err := g.Finish(5, 2, 0.3)

			if err != nil {
				t.Fatal(err)
			}

			return res
		}

		want := fmt.Sprintf("%+v", run())

		for i := 0; i < 25; i++ {
			if got := fmt.Sprintf("%+v", run()); got != want {
				t.Fatalf("shape %d run %d differs:\n want %s\n  got %s", shape, i, want, got)
			}
		}
	}
}

func TestByteComparisonNeverMergesDistinctValues(t *testing.T) {
	t.Parallel()

	nfc := "José"
	nfd := "José"

	g := NewGrouper([]string{"name"}, []int{0}, nil)

	row := int64(2)

	for _, v := range []string{nfc, nfc, nfc, nfd, nfd, nfd} {
		g.Add([]string{v}, row)
		row++
	}

	res, err := g.Finish(6, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if res.Groups != 2 {
		t.Errorf("two byte sequences that render alike must stay two groups, got %d", res.Groups)
	}

	if res.K != 3 {
		t.Errorf("want k=3 from the split, got %d", res.K)
	}

	if res.KThresholdMet() {
		t.Error("a normalising tool would report k=6 and pass; byte comparison must be the safer answer")
	}
}

func TestManyQuasiIdentifiersGroupCorrectly(t *testing.T) {
	t.Parallel()

	for _, width := range []int{1, 2, 63, 64, 65, 128, 300} {
		names := make([]string, width)
		idx := make([]int, width)

		for i := range names {
			names[i] = "q" + strconv.Itoa(i)
			idx[i] = i
		}

		g := NewGrouper(names, idx, nil)

		row := int64(2)

		for class := range 3 {
			for range 4 {
				values := make([]string, width)

				for i := range values {
					values[i] = "c" + strconv.Itoa(class) + "v" + strconv.Itoa(i)
				}

				g.Add(values, row)
				row++
			}
		}

		res, err := g.Finish(4, 0, 0)

		if err != nil {
			t.Fatalf("width %d: %v", width, err)
		}

		if res.Groups != 3 {
			t.Errorf("width %d: want 3 classes, got %d", width, res.Groups)
		}

		if res.K != 4 {
			t.Errorf("width %d: want k=4, got %d", width, res.K)
		}

		if !res.KThresholdMet() {
			t.Errorf("width %d: k=4 must meet a threshold of 4", width)
		}

		if len(res.Largest.Values) != width {
			t.Errorf("width %d: the named class lost values, got %d", width, len(res.Largest.Values))
		}
	}
}

func TestDiversityWithBlankQuasiIdentifiers(t *testing.T) {
	t.Parallel()

	g := NewGrouper([]string{"q1", "q2"}, []int{0, 1}, nil).
		WithSensitive([]string{"s"}, []int{2}, []Kind{Categorical})

	rows := [][]string{
		{"", "x", "p"}, {"", "x", "q"}, {"", "x", "r"},
		{"", "y", "p"}, {"", "y", "q"},
		{"a", "x", "p"}, {"a", "x", "q"}, {"a", "x", "r"}, {"a", "x", "s"},
	}

	for i, r := range rows {
		g.Add(r, int64(i+2))
	}

	res, err := g.Finish(1, 2, 0)

	if err != nil {
		t.Fatal(err)
	}

	if res.Rows != int64(len(rows)) {
		t.Fatalf("a blank is a value, so every row is grouped: want %d, got %d", len(rows), res.Rows)
	}

	if res.Groups != 3 {
		t.Errorf("want 3 classes, got %d", res.Groups)
	}

	if res.EmptyQI != 5 {
		t.Errorf("want 5 rows flagged as carrying a blank, got %d", res.EmptyQI)
	}

	if got := res.MinL(); got != 2 {
		t.Errorf("the smallest class holds 2 distinct values, want l=2, got %d", got)
	}

	if worst := res.Diversity[0].Worst; len(worst) != 2 || worst[0] != "" || worst[1] != "y" {
		t.Errorf("the named class should be the blank one, got %q", worst)
	}
}

func TestArenaChunksStayWithinTheirLimit(t *testing.T) {
	t.Parallel()

	var a arena

	for i := range chunkLimit * 6 {
		a.next(int64(i))
	}

	for i, c := range a.chunks {
		if len(c) > chunkLimit {
			t.Errorf("chunk %d holds %d groups, past the limit of %d", i, len(c), chunkLimit)
		}
	}

	if len(a.chunks) < 2 {
		t.Fatalf("expected several chunks, got %d", len(a.chunks))
	}
}

package measure

import (
	"fmt"
	"runtime"
	"testing"
)

func TestAdversarialBelowListMemory(t *testing.T) {
	const rows = 300000

	g := NewGrouper([]string{"id"}, []int{0}, nil)

	for i := 0; i < rows; i++ {
		g.Add([]string{fmt.Sprintf("value-%d", i)}, int64(i+2))
	}

	var before, after runtime.MemStats

	runtime.ReadMemStats(&before)

	res, err := g.Finish(1000000, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	runtime.ReadMemStats(&after)

	finishing := float64(after.TotalAlloc-before.TotalAlloc) / (1 << 20)

	t.Logf("reporting %d of %d offending groups allocated %.2f MiB",
		len(res.Below), res.BelowGroups, finishing)

	if finishing > 1 {
		t.Errorf("Finish allocated %.2f MiB to report %d groups out of %d",
			finishing, len(res.Below), res.BelowGroups)
	}

	if len(res.Below) != 100 || res.BelowGroups != rows {
		t.Fatalf("unexpected retention: %d of %d", len(res.Below), res.BelowGroups)
	}
}

func TestAdversarialDeterminismAcrossInsertionOrder(t *testing.T) {
	rows := [][]string{{"a"}, {"b"}, {"b"}, {"c"}, {"c"}, {"c"}, {"d"}}

	first := groupAll(t, rows, 3)

	reversed := make([][]string, len(rows))

	for i := range rows {
		reversed[i] = rows[len(rows)-1-i]
	}

	second := groupAll(t, reversed, 3)

	if first.K != second.K || first.BelowGroups != second.BelowGroups {
		t.Fatalf("verdict differs: %+v vs %+v", first, second)
	}

	for i := range first.Below {
		if first.Below[i].Values[0] != second.Below[i].Values[0] {
			t.Errorf("group %d differs by insertion order: %q vs %q",
				i, first.Below[i].Values[0], second.Below[i].Values[0])
		}
	}
}

func groupAll(t *testing.T, rows [][]string, threshold int) Result {
	t.Helper()

	g := NewGrouper([]string{"a"}, []int{0}, nil)

	for i, r := range rows {
		g.Add(r, int64(i+2))
	}

	res, err := g.Finish(threshold, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestAdversarialSuppressionMarkerCollidesWithRealData(t *testing.T) {
	marker := "unknown"

	g := NewGrouper([]string{"gender"}, []int{0}, &marker)

	rows := [][]string{{"F"}, {"F"}, {"unknown"}, {"unknown"}, {"unknown"}}

	for i, r := range rows {
		g.Add(r, int64(i+2))
	}

	res, err := g.Finish(2, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("k=%d over %d rows, %d excluded, passed=%v", res.K, res.Rows, res.Suppressed, res.KThresholdMet())

	if !res.KThresholdMet() {
		t.Fatal("expected this to pass, which is the point")
	}
	if res.Suppressed*2 < res.Rows {
		t.Skip("not the majority-suppressed case")
	}

	t.Logf("%d of %d rows were set aside to reach this verdict", res.Suppressed, res.Suppressed+res.Rows)
}

func TestAdversarialPartialSuppression(t *testing.T) {
	marker := "*"

	g := NewGrouper([]string{"a", "b"}, []int{0, 1}, &marker)

	rows := [][]string{
		{"*", "*"},
		{"real", "*"},
		{"*", "real"},
		{"real", "real"},
	}

	for i, r := range rows {
		g.Add(r, int64(i+2))
	}

	res, err := g.Finish(1, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("rows=%d suppressed=%d; three of four rows were excluded on a partial match",
		res.Rows, res.Suppressed)

	if res.Suppressed != 3 {
		t.Errorf("want 3 excluded, got %d", res.Suppressed)
	}
}

func TestAdversarialEmptyCounterIsPerRow(t *testing.T) {
	g := NewGrouper([]string{"a", "b"}, []int{0, 1}, nil)

	g.Add([]string{"", ""}, 2)
	g.Add([]string{"", "x"}, 3)
	g.Add([]string{"y", "z"}, 4)

	res, err := g.Finish(1, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if res.EmptyQI != 2 {
		t.Errorf("want 2 rows flagged, got %d", res.EmptyQI)
	}
}

func TestAdversarialAddDoesNotRetainCallerRow(t *testing.T) {
	g := NewGrouper([]string{"a"}, []int{0}, nil)

	row := []string{"first"}
	g.Add(row, 2)

	row[0] = "mutated"
	g.Add(row, 3)

	res, err := g.Finish(1, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	if res.Groups != 2 {
		t.Errorf("want 2 distinct groups, got %d", res.Groups)
	}
}

func TestAdversarialThresholdOne(t *testing.T) {
	res := groupAll(t, [][]string{{"a"}, {"b"}}, 1)

	if !res.KThresholdMet() {
		t.Error("k=1 must satisfy a threshold of 1")
	}
	if res.BelowGroups != 0 || len(res.Below) != 0 {
		t.Errorf("nothing can be below a threshold of 1, got %d", res.BelowGroups)
	}
}

package measure

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func run(t *testing.T, rows [][]string, threshold int, suppression *string) Result {
	t.Helper()

	idx := make([]int, len(rows[0]))
	names := make([]string, len(rows[0]))

	for i := range idx {
		idx[i] = i
		names[i] = string(rune('a' + i))
	}

	g := NewGrouper(names, idx, suppression)

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

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"plain":            {"a", "b", "c"},
		"empty values":     {"", "", ""},
		"embedded colon":   {"1:2", "3"},
		"embedded digits":  {"12", "34"},
		"unicode":          {"Ø", "日本"},
		"single":           {"only"},
		"leading zeros":    {"01234", "1234"},
		"looks like a key": {"1:a1:b"},
	}

	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeKey(string(encodeKey(nil, values)))

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(values) {
				t.Fatalf("want %v, got %v", values, got)
			}
			for i := range values {
				if got[i] != values[i] {
					t.Fatalf("want %v, got %v", values, got)
				}
			}
		})
	}
}

func TestEncodeKeyDoesNotCollideOnSeparators(t *testing.T) {
	t.Parallel()

	pairs := [][2][]string{
		{{"a|b"}, {"a", "b"}},
		{{"a", "b"}, {"a|b"}},
		{{"ab", ""}, {"a", "b"}},
		{{"", "ab"}, {"ab", ""}},
		{{"1:a"}, {"1", "a"}},
	}

	for _, p := range pairs {
		x := string(encodeKey(nil, p[0]))
		y := string(encodeKey(nil, p[1]))

		if x == y {
			t.Errorf("%v and %v encode identically as %q", p[0], p[1], x)
		}
	}
}

func FuzzEncodeKey(f *testing.F) {
	f.Add("a", "b", "c")
	f.Add("a|b", "", "")
	f.Add("1:2", "3:4", "")
	f.Add("", "", "")

	f.Fuzz(func(t *testing.T, a, b, c string) {
		values := []string{a, b, c}

		got, err := decodeKey(string(encodeKey(nil, values)))

		if err != nil {
			t.Fatalf("decode failed for %q: %v", values, err)
		}
		if len(got) != len(values) {
			t.Fatalf("want %d values, got %d", len(values), len(got))
		}
		for i := range values {
			if got[i] != values[i] {
				t.Fatalf("round trip changed %q into %q", values[i], got[i])
			}
		}
	})
}

func TestK(t *testing.T) {
	t.Parallel()

	res := run(t, [][]string{
		{"x", "1"},
		{"x", "1"},
		{"x", "1"},
		{"y", "2"},
		{"y", "2"},
		{"z", "3"},
	}, 5, nil)

	if res.K != 1 {
		t.Errorf("want k=1, got %d", res.K)
	}
	if res.MaxGroup != 3 {
		t.Errorf("want max group 3, got %d", res.MaxGroup)
	}
	if res.Groups != 3 {
		t.Errorf("want 3 groups, got %d", res.Groups)
	}
	if res.UniqueRows != 1 {
		t.Errorf("want 1 unique row, got %d", res.UniqueRows)
	}
	if res.RowsAtRisk != 6 {
		t.Errorf("want all 6 rows at risk below k=5, got %d", res.RowsAtRisk)
	}
}

func TestProsecutorRisk(t *testing.T) {
	t.Parallel()

	res := run(t, [][]string{
		{"a"}, {"b"}, {"b"}, {"c"}, {"c"}, {"c"},
	}, 1, nil)

	cases := map[string]struct{ got, want float64 }{
		"highest": {res.HighestRisk(), 1.0},
		"lowest":  {res.LowestRisk(), 1.0 / 3.0},
		"average": {res.AverageRisk(), 0.5},
	}

	for name, c := range cases {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("%s risk: want %v, got %v", name, c.want, c.got)
		}
	}
}

func TestThresholdBoundary(t *testing.T) {
	t.Parallel()

	rows := [][]string{{"a"}, {"a"}, {"a"}}

	if res := run(t, rows, 3, nil); !res.Passed() {
		t.Errorf("k=3 against threshold 3 should pass")
	}
	if res := run(t, rows, 4, nil); res.Passed() {
		t.Errorf("k=3 against threshold 4 should fail")
	}
}

func TestSingleRow(t *testing.T) {
	t.Parallel()

	res := run(t, [][]string{{"a"}}, 2, nil)

	if res.K != 1 || res.Rows != 1 || res.Groups != 1 {
		t.Fatalf("unexpected result %+v", res)
	}
	if res.Passed() {
		t.Error("a single row cannot satisfy k=2")
	}
}

func TestEveryRowUnique(t *testing.T) {
	t.Parallel()

	res := run(t, [][]string{{"a"}, {"b"}, {"c"}}, 2, nil)

	if res.K != 1 || res.UniqueRows != 3 || res.BelowGroups != 3 {
		t.Fatalf("unexpected result %+v", res)
	}
}

func TestSuppressionIsExcludedNotGrouped(t *testing.T) {
	t.Parallel()

	rows := [][]string{
		{"*"}, {"*"}, {"*"}, {"*"}, {"*"},
		{"real"},
	}

	undeclared := run(t, rows, 5, nil)

	if undeclared.K != 1 {
		t.Errorf("without a declared marker every value groups literally, want k=1, got %d", undeclared.K)
	}
	if undeclared.Suppressed != 0 {
		t.Errorf("want nothing excluded, got %d", undeclared.Suppressed)
	}

	declared := run(t, rows, 5, marker("*"))

	if declared.Suppressed != 5 {
		t.Errorf("want 5 rows excluded, got %d", declared.Suppressed)
	}
	if declared.Rows != 1 {
		t.Errorf("want 1 row grouped, got %d", declared.Rows)
	}
	if declared.Passed() {
		t.Error("one real row cannot satisfy k=5 no matter how many were suppressed")
	}
}

func TestSuppressionWouldOtherwiseInflateK(t *testing.T) {
	t.Parallel()

	rows := [][]string{{"*"}, {"*"}, {"*"}, {"*"}, {"*"}}

	if res := run(t, rows, 5, nil); res.K != 5 || !res.Passed() {
		t.Fatalf("grouped literally this passes, which is the trap: %+v", res)
	}

	g := NewGrouper([]string{"a"}, []int{0}, marker("*"))

	for i, r := range rows {
		g.Add(r, int64(i+2))
	}

	_, err := g.Finish(5, 0, 0)

	if err == nil || !strings.Contains(err.Error(), "k is undefined") {
		t.Fatalf("want an all-suppressed error, got %v", err)
	}
}

func TestEmptyQuasiIdentifierCellsAreCountedNotDropped(t *testing.T) {
	t.Parallel()

	res := run(t, [][]string{{"a", ""}, {"a", ""}, {"b", "x"}}, 2, nil)

	if res.Rows != 3 {
		t.Errorf("want every row grouped, got %d", res.Rows)
	}
	if res.EmptyQI != 2 {
		t.Errorf("want 2 rows flagged as carrying an empty cell, got %d", res.EmptyQI)
	}
}

func TestNoRows(t *testing.T) {
	t.Parallel()

	g := NewGrouper([]string{"a"}, []int{0}, nil)

	if _, err := g.Finish(5, 0, 0); !errors.Is(err, ErrNoRows) {
		t.Fatalf("want ErrNoRows, got %v", err)
	}
}

func TestBelowGroupsAreSortedAndCapped(t *testing.T) {
	t.Parallel()

	rows := [][]string{{"c"}, {"b"}, {"b"}, {"a"}}

	res := run(t, rows, 5, nil)

	if len(res.Below) != 3 {
		t.Fatalf("want 3 offending groups, got %d", len(res.Below))
	}
	if res.Below[0].Count != 1 || res.Below[2].Count != 2 {
		t.Fatalf("want smallest groups first, got %+v", res.Below)
	}

	if res.Below[0].Values[0] != "a" || res.Below[1].Values[0] != "c" {
		t.Errorf("want ties broken by value, got %+v", res.Below)
	}

	g := NewGrouper([]string{"a"}, []int{0}, nil)
	g.SetMaxBelow(2)

	for i, r := range rows {
		g.Add(r, int64(i+2))
	}

	capped, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}
	if len(capped.Below) != 2 || capped.BelowGroups != 3 || !capped.Truncated() {
		t.Errorf("want 2 of 3 retained and marked truncated, got %+v", capped)
	}
}

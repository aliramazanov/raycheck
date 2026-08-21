package measure

import (
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"testing"
)

type referenceGroup struct {
	k          int
	groups     int
	rows       int
	suppressed int
	emptyQI    int
	unique     int
	atRisk     int
	below      int
	maxGroup   int
}

func referenceMeasure(rows [][]string, idx []int, suppression *string, threshold int) referenceGroup {
	counts := map[string]int{}

	var ref referenceGroup

row:
	for _, r := range rows {
		values := make([]string, len(idx))

		for i, at := range idx {
			values[i] = r[at]
		}

		for _, v := range values {
			if suppression != nil && v == *suppression {
				ref.suppressed++

				continue row
			}
		}

		for _, v := range values {
			if v == "" {
				ref.emptyQI++

				break
			}
		}

		ref.rows++
		counts[fmt.Sprintf("%#v", values)]++
	}

	if ref.rows == 0 {
		return ref
	}

	ref.groups = len(counts)

	for _, c := range counts {
		if ref.k == 0 || c < ref.k {
			ref.k = c
		}
		if c > ref.maxGroup {
			ref.maxGroup = c
		}
		if c == 1 {
			ref.unique++
		}
		if c < threshold {
			ref.below++
			ref.atRisk += c
		}
	}

	return ref
}

func randomRows(rng *rand.Rand, n, cols, alphabet int) [][]string {
	rows := make([][]string, n)

	for i := range rows {
		row := make([]string, cols)

		for j := range row {
			switch v := rng.Intn(alphabet + 2); v {
			case alphabet:
				row[j] = ""
			case alphabet + 1:
				row[j] = "*"
			default:
				row[j] = fmt.Sprintf("v%d", v)
			}
		}

		rows[i] = row
	}

	return rows
}

func measureRows(t *testing.T, rows [][]string, idx []int, suppression *string, threshold int) Result {
	t.Helper()

	names := make([]string, len(idx))

	for i := range idx {
		names[i] = fmt.Sprintf("c%d", idx[i])
	}

	g := NewGrouper(names, idx, suppression)

	for i, r := range rows {
		g.Add(r, int64(i+2))
	}

	res, err := g.Finish(threshold, 0, 0)

	if err != nil {
		return Result{}
	}

	return res
}

func TestDifferentialAgainstReference(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(7))

	star := "*"

	for trial := range 3000 {
		var (
			n         = rng.Intn(40)
			cols      = rng.Intn(3) + 1
			alphabet  = rng.Intn(4) + 1
			threshold = rng.Intn(6) + 1
		)

		var suppression *string

		if rng.Intn(2) == 0 {
			suppression = &star
		}

		rows := randomRows(rng, n, cols, alphabet)

		idx := make([]int, cols)

		for i := range idx {
			idx[i] = i
		}

		want := referenceMeasure(rows, idx, suppression, threshold)
		got := measureRows(t, rows, idx, suppression, threshold)

		if want.rows == 0 {
			continue
		}

		mismatches := map[string][2]int{
			"k":           {want.k, got.K},
			"groups":      {want.groups, int(got.Groups)},
			"rows":        {want.rows, int(got.Rows)},
			"suppressed":  {want.suppressed, int(got.Suppressed)},
			"emptyQI":     {want.emptyQI, int(got.EmptyQI)},
			"unique":      {want.unique, int(got.UniqueRows)},
			"atRisk":      {want.atRisk, int(got.RowsAtRisk)},
			"belowGroups": {want.below, got.BelowGroups},
			"maxGroup":    {want.maxGroup, got.MaxGroup},
		}

		for name, pair := range mismatches {
			if pair[0] != pair[1] {
				t.Fatalf("trial %d (n=%d cols=%d alphabet=%d threshold=%d suppression=%v): %s want %d, got %d\nrows: %v",
					trial, n, cols, alphabet, threshold, suppression != nil, name, pair[0], pair[1], rows)
			}
		}
	}
}

func TestMetamorphicMoreColumnsCannotRaiseK(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(11))

	for trial := 0; trial < 2000; trial++ {
		cols := rng.Intn(4) + 2
		rows := randomRows(rng, rng.Intn(30)+1, cols, rng.Intn(3)+1)

		narrow := make([]int, cols-1)

		for i := range narrow {
			narrow[i] = i
		}

		wide := make([]int, cols)

		for i := range wide {
			wide[i] = i
		}

		a := measureRows(t, rows, narrow, nil, 1)
		b := measureRows(t, rows, wide, nil, 1)

		if b.K > a.K {
			t.Fatalf("trial %d: adding a column raised k from %d to %d\nrows: %v", trial, a.K, b.K, rows)
		}
		if b.Groups < a.Groups {
			t.Fatalf("trial %d: adding a column reduced the group count from %d to %d", trial, a.Groups, b.Groups)
		}
	}
}

func TestMetamorphicColumnOrderIsIrrelevant(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(13))

	for trial := 0; trial < 2000; trial++ {
		cols := rng.Intn(3) + 2
		rows := randomRows(rng, rng.Intn(30)+1, cols, rng.Intn(3)+1)

		idx := make([]int, cols)

		for i := range idx {
			idx[i] = i
		}

		shuffled := make([]int, cols)
		copy(shuffled, idx)
		rng.Shuffle(cols, func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		a := measureRows(t, rows, idx, nil, 3)
		b := measureRows(t, rows, shuffled, nil, 3)

		if a.K != b.K || a.Groups != b.Groups || a.UniqueRows != b.UniqueRows || a.RowsAtRisk != b.RowsAtRisk {
			t.Fatalf("trial %d: reordering columns changed the result\n%+v\n%+v\nrows: %v", trial, a, b, rows)
		}
	}
}

func TestMetamorphicDuplicationDoublesK(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(17))

	for trial := 0; trial < 1000; trial++ {
		cols := rng.Intn(2) + 1
		rows := randomRows(rng, rng.Intn(20)+1, cols, rng.Intn(3)+1)

		doubled := make([][]string, 0, len(rows)*2)
		doubled = append(doubled, rows...)
		doubled = append(doubled, rows...)

		idx := make([]int, cols)

		for i := range idx {
			idx[i] = i
		}

		a := measureRows(t, rows, idx, nil, 1)
		b := measureRows(t, doubled, idx, nil, 1)

		if b.K != a.K*2 {
			t.Fatalf("trial %d: duplicating rows gave k=%d, want %d", trial, b.K, a.K*2)
		}

		if b.Groups != a.Groups {
			t.Fatalf("trial %d: duplicating rows changed the group count from %d to %d", trial, a.Groups, b.Groups)
		}

		if b.Rows != a.Rows*2 {
			t.Fatalf("trial %d: want %d rows, got %d", trial, a.Rows*2, b.Rows)
		}

		if b.UniqueRows != 0 && a.Rows > 0 {
			t.Fatalf("trial %d: nothing can be unique after duplication, got %d", trial, b.UniqueRows)
		}
	}
}

func TestMetamorphicBijectiveRenamingPreservesK(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(19))

	for trial := range 1000 {
		cols := rng.Intn(3) + 1
		rows := randomRows(rng, rng.Intn(30)+1, cols, rng.Intn(4)+1)

		seen := map[string]bool{}

		for _, r := range rows {
			for _, v := range r {
				seen[v] = true
			}
		}

		distinct := slices.Sorted(maps.Keys(seen))

		rename := map[string]string{}

		for i, v := range distinct {

			rename[v] = fmt.Sprintf("%d|%d", i, i)
		}

		renamed := make([][]string, len(rows))

		for i, r := range rows {
			out := make([]string, len(r))

			for j, v := range r {
				out[j] = rename[v]
			}

			renamed[i] = out
		}

		idx := make([]int, cols)

		for i := range idx {
			idx[i] = i
		}

		a := measureRows(t, rows, idx, nil, 3)
		b := measureRows(t, renamed, idx, nil, 3)

		if a.K != b.K || a.Groups != b.Groups || a.UniqueRows != b.UniqueRows {
			t.Fatalf("trial %d: renaming changed the result\n%+v\n%+v", trial, a, b)
		}
	}
}

func TestMetamorphicSuppressionEqualsDeletion(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(23))

	star := "*"

	for trial := 0; trial < 2000; trial++ {
		cols := rng.Intn(3) + 1
		rows := randomRows(rng, rng.Intn(30)+1, cols, rng.Intn(3)+1)

		kept := make([][]string, 0, len(rows))

	row:
		for _, r := range rows {
			for _, v := range r[:cols] {
				if v == star {
					continue row
				}
			}

			kept = append(kept, r)
		}

		idx := make([]int, cols)

		for i := range idx {
			idx[i] = i
		}

		withMarker := measureRows(t, rows, idx, &star, 3)
		deleted := measureRows(t, kept, idx, nil, 3)

		if withMarker.K != deleted.K || withMarker.Rows != deleted.Rows || withMarker.Groups != deleted.Groups {
			t.Fatalf("trial %d: suppression differs from deletion\n%+v\n%+v\nrows: %v", trial, withMarker, deleted, rows)
		}
	}
}

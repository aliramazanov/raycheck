package measure

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

func reference(entries []belowEntry, max int) []belowEntry {
	sorted := make([]belowEntry, len(entries))
	copy(sorted, entries)

	slices.SortStableFunc(sorted, func(a, b belowEntry) int {
		switch {
		case a.before(b):
			return -1
		case b.before(a):
			return 1
		default:
			return 0
		}
	})

	if len(sorted) > max {
		sorted = sorted[:max]
	}

	return sorted
}

func TestBelowSetMatchesReference(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))

	for _, max := range []int{1, 2, 3, 7, 50} {
		for range 400 {
			n := rng.Intn(60)

			entries := make([]belowEntry, n)

			for i := range entries {

				entries[i] = belowEntry{
					key:      fmt.Sprintf("%d:%c", 1, 'a'+rune(rng.Intn(4))),
					count:    rng.Intn(3) + 1,
					firstRow: int64(i + 2),
				}
			}

			set := newBelowSet(max)

			for _, e := range entries {
				set.add(e)
			}

			want := reference(entries, max)

			if len(set.entries) != len(want) {
				t.Fatalf("max=%d n=%d: want %d entries, got %d", max, n, len(want), len(set.entries))
			}

			for i := range want {
				if set.entries[i].count != want[i].count || set.entries[i].key != want[i].key {
					t.Fatalf("max=%d n=%d at %d: want %+v, got %+v", max, n, i, want[i], set.entries[i])
				}
			}
		}
	}
}

func TestBelowSetStaysSorted(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(2))

	set := newBelowSet(8)

	for i := range 500 {
		set.add(belowEntry{
			key:      fmt.Sprintf("%02d", rng.Intn(30)),
			count:    rng.Intn(5),
			firstRow: int64(i),
		})

		for j := 1; j < len(set.entries); j++ {
			if set.entries[j].before(set.entries[j-1]) {
				t.Fatalf("out of order at %d after %d inserts: %+v", j, i, set.entries)
			}
		}

		if len(set.entries) > 8 {
			t.Fatalf("grew past max: %d", len(set.entries))
		}
	}
}

func TestBelowSetCapacityNeverGrows(t *testing.T) {
	t.Parallel()

	set := newBelowSet(4)

	for i := 0; i < 10000; i++ {
		set.add(belowEntry{key: fmt.Sprintf("%d", i), count: 1, firstRow: int64(i)})
	}

	if cap(set.entries) != 4 {
		t.Errorf("backing array grew to %d, so the bound is not a bound", cap(set.entries))
	}
}

func FuzzBelowSet(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4}, 2)
	f.Add([]byte{}, 1)
	f.Add([]byte{9, 9, 9}, 3)

	f.Fuzz(func(t *testing.T, data []byte, max int) {
		max = 1 + int(uint(max)%64)

		entries := make([]belowEntry, 0, len(data))

		for i, b := range data {
			entries = append(entries, belowEntry{
				key:      string(rune('a' + b%8)),
				count:    int(b%5) + 1,
				firstRow: int64(i),
			})
		}

		set := newBelowSet(max)

		for _, e := range entries {
			set.add(e)
		}

		want := reference(entries, max)

		if len(set.entries) != len(want) {
			t.Fatalf("want %d entries, got %d", len(want), len(set.entries))
		}

		for i := range want {
			if set.entries[i].count != want[i].count || set.entries[i].key != want[i].key {
				t.Fatalf("at %d: want %+v, got %+v", i, want[i], set.entries[i])
			}
		}
	})
}

func TestDecodeKeyRejectsMalformed(t *testing.T) {
	t.Parallel()

	malformed := map[string]string{
		"no separator":     "abc",
		"length overruns":  "9:ab",
		"negative length":  "-1:a",
		"empty length":     ":abc",
		"non numeric":      "x:abc",
		"trailing garbage": "1:a2",
		"huge length":      "99999999999999999999:a",
	}

	for name, key := range malformed {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeKey(key)

			if err == nil {
				t.Errorf("accepted %q as %v", key, got)
			}
		})
	}
}

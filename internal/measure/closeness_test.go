package measure

import (
	"cmp"
	"math"
	"math/rand"
	"slices"
	"testing"
)

func naiveDistance(d domain, q []float64, kind Kind) float64 {
	if len(d.ids) < 2 {
		return 0
	}

	if kind == Numeric {
		var sum, running float64

		for i := range d.p {
			running += q[i] - d.p[i]
			sum += math.Abs(running)
		}

		return sum / float64(len(d.ids)-1)
	}

	var sum float64

	for i := range d.p {
		sum += math.Abs(q[i] - d.p[i])
	}

	return sum / 2
}

func TestDistanceMatchesTheDefinition(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(4242))

	for trial := 0; trial < 4000; trial++ {
		size := rng.Intn(12) + 1

		in := newInterner()
		global := map[int32]int64{}

		var rows int64

		for i := range size {
			id := in.id(string(rune('a' + i)))
			n := int64(rng.Intn(20) + 1)
			global[id] = n
			rows += n
		}

		for _, kind := range []Kind{Categorical, Numeric} {
			d := buildDomain(in, global, rows, kind)

			held := rng.Intn(size) + 1
			positions := rng.Perm(size)[:held]

			weights := make([]float64, held)

			var total float64

			for i := range weights {
				weights[i] = rng.Float64() + 0.01
				total += weights[i]
			}

			full := make([]float64, size)
			present := make([]share, 0, held)

			for i, at := range positions {
				q := weights[i] / total
				full[at] = q
				present = append(present, share{at: at, q: q})
			}

			slices.SortFunc(present, func(a, b share) int { return cmp.Compare(a.at, b.at) })

			want := naiveDistance(d, full, kind)
			got := d.distance(present, kind)

			if math.Abs(got-want) > 1e-9 {
				t.Fatalf("trial %d kind %v size %d held %d: fast %.12f, longhand %.12f",
					trial, kind, size, held, got, want)
			}
		}
	}
}

func TestDistanceAtTheExtremes(t *testing.T) {
	t.Parallel()

	in := newInterner()
	global := map[int32]int64{}

	for i := range 5 {
		global[in.id(string(rune('a'+i)))] = 10
	}

	for _, kind := range []Kind{Categorical, Numeric} {
		d := buildDomain(in, global, 50, kind)

		mirror := make([]share, 5)
		full := make([]float64, 5)

		for i := range mirror {
			mirror[i] = share{at: i, q: 0.2}
			full[i] = 0.2
		}

		if got := d.distance(mirror, kind); math.Abs(got) > 1e-12 {
			t.Errorf("kind %v: a class mirroring the file sits at zero, got %v", kind, got)
		}

		one := []share{{at: 0, q: 1}}
		fullOne := []float64{1, 0, 0, 0, 0}

		if got, want := d.distance(one, kind), naiveDistance(d, fullOne, kind); math.Abs(got-want) > 1e-12 {
			t.Errorf("kind %v: fast %v, longhand %v", kind, got, want)
		}

		last := []share{{at: 4, q: 1}}
		fullLast := []float64{0, 0, 0, 0, 1}

		if got, want := d.distance(last, kind), naiveDistance(d, fullLast, kind); math.Abs(got-want) > 1e-12 {
			t.Errorf("kind %v trailing: fast %v, longhand %v", kind, got, want)
		}
	}
}

func TestNumericAxisOrdersDeterministically(t *testing.T) {
	t.Parallel()

	values := []string{
		"9", "10", "5x", "3", "40", "7z", "100", "2", "", "NaN",
		"1", "1.0", "01", "+1", "1.00", "0x1p0", "2.0", "0002", "10.0",
	}

	build := func() []string {
		in := newInterner()
		global := map[int32]int64{}

		for i, v := range values {
			global[in.id(v)] = int64(i + 1)
		}

		d := buildDomain(in, global, 55, Numeric)

		out := make([]string, len(d.ids))

		for i, id := range d.ids {
			out[i] = in.value(id)
		}

		return out
	}

	want := build()

	for i := 0; i < 200; i++ {
		if got := build(); !slices.Equal(got, want) {
			t.Fatalf("run %d ordered the axis differently:\n want %q\n  got %q", i, want, got)
		}
	}

	if len(want) != len(values) {
		t.Fatalf("the axis lost values: want %d, got %d", len(values), len(want))
	}

	for i := 1; i < len(want); i++ {
		a, aok := axisValue(want[i-1])
		b, bok := axisValue(want[i])

		if aok && bok && a > b {
			t.Errorf("numbers out of order at %d: %v then %v", i, a, b)
		}
		if !aok && bok {
			t.Errorf("a non-number sorts before a number at %d: %q then %q", i, want[i-1], want[i])
		}
	}
}

func TestNumericAxisComparatorIsTransitive(t *testing.T) {
	t.Parallel()

	values := []string{"9", "10", "5x", "3", "40", "7z", "100", "2", "", "NaN", "-1", "0x5"}

	less := func(x, y string) bool {
		a, aok := axisValue(x)
		b, bok := axisValue(y)

		switch {
		case aok && bok:
			return a < b
		case aok != bok:
			return aok
		default:
			return x < y
		}
	}

	for _, a := range values {
		if less(a, a) {
			t.Errorf("%q sorts before itself", a)
		}

		for _, b := range values {
			for _, c := range values {
				if less(a, b) && less(b, c) && !less(a, c) {
					t.Errorf("not transitive: %q < %q < %q but not %q < %q", a, b, c, a, c)
				}
			}
		}
	}
}

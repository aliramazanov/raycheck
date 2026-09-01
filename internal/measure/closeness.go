package measure

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"strconv"
)

type Kind int

const (
	Categorical Kind = iota
	Numeric
)

type Closeness struct {
	Attribute string
	Kind      Kind
	T         float64

	OffAxis int

	Worst    []string
	WorstRow int64

	keyForTie string
}

type domain struct {
	ids   []int32
	index map[int32]int
	p     []float64

	cumP   []float64
	cumSum []float64

	offAxis int
}

func buildDomain(in *interner, global map[int32]int64, rows int64, kind Kind) domain {
	ids := make([]int32, 0, len(global))

	for id := range global {
		ids = append(ids, id)
	}

	switch kind {
	case Numeric:
		slices.SortFunc(ids, func(x, y int32) int {
			a, aok := axisValue(in.value(x))
			b, bok := axisValue(in.value(y))

			switch {
			case aok && bok:
				if c := cmp.Compare(a, b); c != 0 {
					return c
				}
			case aok != bok:
				if aok {
					return -1
				}

				return 1
			}

			return cmp.Compare(in.value(x), in.value(y))
		})
	default:
		slices.SortFunc(ids, func(x, y int32) int { return cmp.Compare(in.value(x), in.value(y)) })
	}

	off := 0

	if kind == Numeric {
		for _, id := range ids {
			if _, ok := axisValue(in.value(id)); !ok {
				off++
			}
		}
	}

	d := domain{
		offAxis: off,
		ids:     ids,
		index:   make(map[int32]int, len(ids)),
		p:       make([]float64, len(ids)),
		cumP:    make([]float64, len(ids)),
		cumSum:  make([]float64, len(ids)),
	}

	var running, total float64

	for i, id := range ids {
		d.index[id] = i
		d.p[i] = float64(global[id]) / float64(rows)

		running += d.p[i]
		total += running
		d.cumP[i] = running
		d.cumSum[i] = total
	}

	return d
}

func axisValue(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)

	if err != nil || math.IsNaN(v) {
		return 0, false
	}

	return v, true
}

type share struct {
	at int
	q  float64
}

func (d domain) distance(present []share, kind Kind) float64 {
	if len(d.ids) < 2 {
		return 0
	}

	if kind == Numeric {
		return d.orderedDistance(present)
	}

	var diff, covered float64

	for _, s := range present {
		diff += math.Abs(s.q - d.p[s.at])
		covered += d.p[s.at]
	}

	return (diff + (1 - covered)) / 2
}

func (d domain) orderedDistance(present []share) float64 {
	var sum, q float64

	last := -1

	for _, s := range present {
		sum += d.gap(last+1, s.at-1, q)

		q += s.q
		sum += math.Abs(q - d.cumP[s.at])
		last = s.at
	}

	sum += d.gap(last+1, len(d.ids)-1, q)

	return sum / float64(len(d.ids)-1)
}

func (d domain) gap(from, to int, q float64) float64 {
	if from > to {
		return 0
	}

	split := sort.Search(to-from+1, func(i int) bool { return d.cumP[from+i] > q })
	split += from

	var sum float64

	if split > from {
		sum += q*float64(split-from) - d.rangeSum(from, split-1)
	}

	if split <= to {
		sum += d.rangeSum(split, to) - q*float64(to-split+1)
	}

	return sum
}

func (d domain) rangeSum(from, to int) float64 {
	if from > to {
		return 0
	}

	if from == 0 {
		return d.cumSum[to]
	}

	return d.cumSum[to] - d.cumSum[from-1]
}

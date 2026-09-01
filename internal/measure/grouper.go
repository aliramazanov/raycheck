package measure

import (
	"cmp"
	"fmt"
	"slices"
)

type group struct {
	count    int
	firstRow int64
	sens     []sensEntry
}

type Grouper struct {
	quasiIdentifiers []string
	idx              []int
	suppression      *string
	maxBelow         int

	sensitive []string
	sensIdx   []int
	sensKind  []Kind
	interns   []*interner
	globals   []map[int32]int64

	sensIndex map[*group]map[int64]int32

	groups  map[string]*group
	arena   arena
	scratch []byte
	values  []string

	rows       int64
	suppressed int64
	emptyQI    int64
	markerHits []int64
}

func (g *Grouper) WithSensitive(names []string, idx []int, kinds []Kind) *Grouper {
	g.sensitive = names
	g.sensIdx = idx
	g.sensKind = kinds
	g.interns = make([]*interner, len(idx))
	g.globals = make([]map[int32]int64, len(idx))

	for i := range idx {
		g.interns[i] = newInterner()
		g.globals[i] = make(map[int32]int64)
	}

	return g
}

func NewGrouper(quasiIdentifiers []string, idx []int, suppression *string) *Grouper {
	return &Grouper{
		quasiIdentifiers: quasiIdentifiers,
		idx:              idx,
		suppression:      suppression,
		maxBelow:         DefaultMaxBelow,
		groups:           make(map[string]*group),
		values:           make([]string, len(idx)),
		markerHits:       make([]int64, len(idx)),
	}
}

func (g *Grouper) SetMaxBelow(n int) {
	if n > 0 {
		g.maxBelow = n
	}
}

func (g *Grouper) Add(row []string, rowNum int64) {
	var empty, suppressed bool

	for i, at := range g.idx {
		v := row[at]
		g.values[i] = v

		if g.suppression != nil && v == *g.suppression {
			g.markerHits[i]++

			suppressed = true
		}

		if v == "" {
			empty = true
		}
	}

	if suppressed {
		g.suppressed++

		return
	}

	if empty {
		g.emptyQI++
	}

	g.rows++
	g.scratch = encodeKey(g.scratch[:0], g.values)

	e, seen := g.groups[string(g.scratch)]

	if !seen {
		e = g.arena.next(rowNum)
		g.groups[string(g.scratch)] = e
	}

	e.count++

	for i, at := range g.sensIdx {
		id := g.interns[i].id(row[at])
		g.globals[i][id]++

		var index map[int64]int32

		if g.sensIndex != nil {
			index = g.sensIndex[e]
		}

		entries, index := addSens(e.sens, index, int32(i), id)
		e.sens = entries

		if index != nil {
			if g.sensIndex == nil {
				g.sensIndex = make(map[*group]map[int64]int32)
			}

			g.sensIndex[e] = index
		}
	}
}

func (g *Grouper) closeness() []Closeness {
	if len(g.sensIdx) == 0 || len(g.sensKind) != len(g.sensIdx) {
		return nil
	}

	domains := make([]domain, len(g.sensIdx))

	for i := range g.sensIdx {
		domains[i] = buildDomain(g.interns[i], g.globals[i], g.rows, g.sensKind[i])
	}

	out := make([]Closeness, len(g.sensIdx))

	for i := range out {
		out[i] = Closeness{
			Attribute: g.sensitive[i],
			Kind:      g.sensKind[i],
			T:         -1,
			OffAxis:   domains[i].offAxis,
		}
	}

	present := make([][]share, len(g.sensIdx))

	for key, e := range g.groups {
		for i := range present {
			present[i] = present[i][:0]
		}

		size := float64(e.count)

		for _, s := range e.sens {
			at := domains[s.attr].index[s.value]
			present[s.attr] = append(present[s.attr], share{at: at, q: float64(s.count) / size})
		}

		for i := range present {
			slices.SortFunc(present[i], func(a, b share) int { return cmp.Compare(a.at, b.at) })
		}

		for i := range out {
			t := domains[i].distance(present[i], g.sensKind[i])

			if t < out[i].T || (t == out[i].T && key >= out[i].keyForTie) {
				continue
			}

			out[i].T = t
			out[i].WorstRow = e.firstRow
			out[i].keyForTie = key
		}
	}

	for i := range out {
		if out[i].T < 0 {
			out[i].T = 0
		}

		if values, err := decodeKey(out[i].keyForTie); err == nil {
			out[i].Worst = values
		}
	}

	return out
}

func (g *Grouper) diversity() []Diversity {
	if len(g.sensIdx) == 0 {
		return nil
	}

	out := make([]Diversity, len(g.sensIdx))

	for i, name := range g.sensitive {
		out[i] = Diversity{Attribute: name}
	}

	counts := make([]int, len(g.sensIdx))

	for key, e := range g.groups {
		for i := range counts {
			counts[i] = 0
		}

		for _, s := range e.sens {
			counts[s.attr]++
		}

		for i := range counts {

			if out[i].L != 0 && counts[i] > out[i].L {
				continue
			}

			if counts[i] == out[i].L && key >= out[i].keyForTie {
				continue
			}

			out[i].L = counts[i]
			out[i].WorstRow = e.firstRow
			out[i].keyForTie = key
		}
	}

	for i := range out {
		if values, err := decodeKey(out[i].keyForTie); err == nil {
			out[i].Worst = values
		}
	}

	return out
}

func (g *Grouper) matchedColumns() []string {
	var out []string

	for i, hits := range g.markerHits {
		if hits > 0 && i < len(g.quasiIdentifiers) {
			out = append(out, g.quasiIdentifiers[i])
		}
	}

	return out
}

func (g *Grouper) Finish(threshold, lThreshold int, tThreshold float64) (Result, error) {
	if g.rows == 0 {
		if g.suppressed > 0 {
			return Result{}, fmt.Errorf(
				"measure: every one of the %d rows was excluded as suppressed, so k is undefined", g.suppressed)
		}

		return Result{}, ErrNoRows
	}

	res := Result{
		QuasiIdentifiers: g.quasiIdentifiers,
		Threshold:        threshold,
		LThreshold:       lThreshold,
		TThreshold:       tThreshold,
		Rows:             g.rows,
		Suppressed:       g.suppressed,
		EmptyQI:          g.emptyQI,
		Groups:           len(g.groups),
		MarkerColumns:    g.matchedColumns(),
	}

	worst := newBelowSet(g.maxBelow)

	var largest string

	for key, e := range g.groups {
		if res.K == 0 || e.count < res.K {
			res.K = e.count
		}

		if e.count > res.MaxGroup || (e.count == res.MaxGroup && key < largest) {
			res.MaxGroup = e.count
			res.Largest = Group{Count: e.count, FirstRow: e.firstRow}
			largest = key
		}
		if e.count == 1 {
			res.UniqueRows++
		}

		if e.count < threshold {
			res.BelowGroups++
			res.RowsAtRisk += int64(e.count)

			worst.add(belowEntry{key: key, count: e.count, firstRow: e.firstRow})
		}
	}

	res.Diversity = g.diversity()
	res.Closeness = g.closeness()

	below := make([]Group, len(worst.entries))

	for i, e := range worst.entries {
		values, err := decodeKey(e.key)

		if err != nil {
			return Result{}, err
		}

		below[i] = Group{Values: values, Count: e.count, FirstRow: e.firstRow}
	}

	res.Below = below

	values, err := decodeKey(largest)

	if err != nil {
		return Result{}, err
	}

	res.Largest.Values = values

	return res, nil
}

package measure

import (
	"cmp"
	"errors"
	"slices"
)

const DefaultMaxBelow = 100

var ErrNoRows = errors.New("measure: the input has no data rows")

type Group struct {
	Values   []string
	Count    int
	FirstRow int64
}

type Result struct {
	QuasiIdentifiers []string
	Threshold        int

	LThreshold int

	Rows       int64
	Suppressed int64
	EmptyQI    int64

	MarkerColumns []string

	Groups   int
	K        int
	MaxGroup int

	Largest Group

	Below       []Group
	BelowGroups int

	RowsAtRisk int64
	UniqueRows int64

	Diversity []Diversity

	Closeness []Closeness

	TThreshold float64
}

type Diversity struct {
	Attribute string
	L         int

	Worst    []string
	WorstRow int64

	keyForTie string
}

func (r Result) KThresholdMet() bool { return r.K >= r.Threshold }

func (r Result) MaxT() float64 {
	c, ok := r.WorstCloseness()

	if !ok {
		return 0
	}

	return c.T
}

func (r Result) MinL() int {
	d, ok := r.WorstDiversity()

	if !ok {
		return 0
	}

	return d.L
}

func (r Result) WorstCloseness() (Closeness, bool) {
	if len(r.Closeness) == 0 {
		return Closeness{}, false
	}

	return slices.MaxFunc(r.Closeness, func(a, b Closeness) int {
		return cmp.Compare(a.T, b.T)
	}), true
}

func (r Result) WorstDiversity() (Diversity, bool) {
	if len(r.Diversity) == 0 {
		return Diversity{}, false
	}

	return slices.MinFunc(r.Diversity, func(a, b Diversity) int {
		return cmp.Compare(a.L, b.L)
	}), true
}

func (r Result) DiversityPassed() bool {
	return r.LThreshold < 1 || r.MinL() >= r.LThreshold
}

const closenessTolerance = 1e-9

func (r Result) ClosenessPassed() bool {
	return r.TThreshold <= 0 || r.MaxT() <= r.TThreshold+closenessTolerance
}

func (r Result) Truncated() bool { return r.BelowGroups > len(r.Below) }

func (r Result) HighestRisk() float64 { return ratio(1, int64(r.K)) }

func (r Result) LowestRisk() float64 { return ratio(1, int64(r.MaxGroup)) }

func (r Result) AverageRisk() float64 { return ratio(int64(r.Groups), r.Rows) }

func (r Result) AtRiskShare() float64 { return ratio(r.RowsAtRisk, r.Rows) }

func (r Result) UniqueShare() float64 { return ratio(r.UniqueRows, r.Rows) }

func (r Result) Coverage() float64 { return ratio(r.Rows, r.Rows+r.Suppressed) }

func ratio(num, den int64) float64 {
	if den == 0 {
		return 0
	}

	return float64(num) / float64(den)
}

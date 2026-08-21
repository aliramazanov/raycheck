package measure

type arena struct {
	chunks [][]group
	cur    []group
	used   int
	size   int
}

const (
	firstChunk  = 64
	chunkLimit  = 8192
	chunkGrowth = 4
)

func (a *arena) next(firstRow int64) *group {
	if a.used == len(a.cur) {
		switch {
		case a.size == 0:
			a.size = firstChunk
		case a.size < chunkLimit:
			a.size *= chunkGrowth
		}

		a.cur = make([]group, a.size)
		a.chunks = append(a.chunks, a.cur)
		a.used = 0
	}

	e := &a.cur[a.used]
	a.used++
	e.firstRow = firstRow

	return e
}

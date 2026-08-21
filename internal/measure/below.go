package measure

type belowEntry struct {
	key      string
	count    int
	firstRow int64
}

func (e belowEntry) before(other belowEntry) bool {
	if e.count != other.count {
		return e.count < other.count
	}

	return e.key < other.key
}

type belowSet struct {
	max     int
	entries []belowEntry
}

func newBelowSet(max int) belowSet {
	if max < 0 {
		max = 0
	}

	return belowSet{max: max, entries: make([]belowEntry, 0, max)}
}

func (s *belowSet) add(e belowEntry) {
	if s.max == 0 {
		return
	}

	full := len(s.entries) == s.max

	if full && !e.before(s.entries[s.max-1]) {
		return
	}

	at := len(s.entries)

	for i, existing := range s.entries {
		if e.before(existing) {
			at = i

			break
		}
	}

	if !full {
		s.entries = append(s.entries, belowEntry{})
	}

	copy(s.entries[at+1:], s.entries[at:])
	s.entries[at] = e
}

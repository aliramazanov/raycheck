package measure

type sensEntry struct {
	attr  int32
	value int32
	count int32
}

const promoteAt = 64

func sensKey(attr, value int32) int64 {
	return int64(attr)<<32 | int64(uint32(value))
}

func addSens(entries []sensEntry, index map[int64]int32, attr, value int32) ([]sensEntry, map[int64]int32) {
	if index != nil {
		if at, seen := index[sensKey(attr, value)]; seen {
			entries[at].count++

			return entries, index
		}

		index[sensKey(attr, value)] = int32(len(entries))

		return append(entries, sensEntry{attr: attr, value: value, count: 1}), index
	}

	for i := range entries {
		if entries[i].attr == attr && entries[i].value == value {
			entries[i].count++

			return entries, nil
		}
	}

	entries = append(entries, sensEntry{attr: attr, value: value, count: 1})

	if len(entries) < promoteAt {
		return entries, nil
	}

	index = make(map[int64]int32, len(entries)*2)

	for i := range entries {
		index[sensKey(entries[i].attr, entries[i].value)] = int32(i)
	}

	return entries, index
}

type interner struct {
	ids    map[string]int32
	values []string
}

func newInterner() *interner {
	return &interner{ids: make(map[string]int32)}
}

func (in *interner) value(id int32) string { return in.values[id] }

func (in *interner) id(s string) int32 {
	if id, seen := in.ids[s]; seen {
		return id
	}

	id := int32(len(in.values))
	in.values = append(in.values, s)
	in.ids[s] = id

	return id
}

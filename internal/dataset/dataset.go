package dataset

import (
	"fmt"
	"strconv"
	"strings"
)

type MissingColumnError struct {
	Missing   []string
	Available []string
}

func (e *MissingColumnError) Error() string {
	noun := "column"
	if len(e.Missing) > 1 {
		noun = "columns"
	}

	return fmt.Sprintf("dataset: %s %s not found in the input; available: %s",
		noun, quoteAll(e.Missing), quoteAll(e.Available))
}

type AmbiguousColumnError struct {
	Column string
	Count  int
}

func (e *AmbiguousColumnError) Error() string {
	return fmt.Sprintf("dataset: column %q appears %d times in the header, so the reference is ambiguous",
		e.Column, e.Count)
}

type RowError struct {
	Row      int64
	Expected int
	Got      int
}

func (e *RowError) Error() string {
	return fmt.Sprintf("dataset: row %d: expected %d fields, got %d", e.Row, e.Expected, e.Got)
}

func Resolve(columns, want []string) ([]int, error) {
	counts := make(map[string]int, len(columns))
	first := make(map[string]int, len(columns))

	for i, c := range columns {
		counts[c]++

		if counts[c] == 1 {
			first[c] = i
		}
	}

	idx := make([]int, 0, len(want))

	var missing []string

	for _, w := range want {
		switch n := counts[w]; {
		case n == 0:
			missing = append(missing, w)
		case n > 1:
			return nil, &AmbiguousColumnError{Column: w, Count: n}
		default:
			idx = append(idx, first[w])
		}
	}

	if len(missing) > 0 {
		return nil, &MissingColumnError{Missing: missing, Available: columns}
	}

	return idx, nil
}

func quoteAll(names []string) string {
	quoted := make([]string, len(names))

	for i, n := range names {
		quoted[i] = strconv.Quote(n)
	}

	return strings.Join(quoted, ", ")
}

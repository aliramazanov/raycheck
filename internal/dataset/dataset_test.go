package dataset

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func open(t *testing.T, body string, delim rune) *CSV {
	t.Helper()

	c, err := NewCSV(io.NopCloser(strings.NewReader(body)), delim)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return c
}

func TestResolve(t *testing.T) {
	t.Parallel()

	idx, err := Resolve([]string{"id", "birth_date", "postcode"}, []string{"postcode", "birth_date"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx) != 2 || idx[0] != 2 || idx[1] != 1 {
		t.Fatalf("want [2 1], got %v", idx)
	}
}

func TestResolveMissingColumnNamesWhatIsAvailable(t *testing.T) {
	t.Parallel()

	_, err := Resolve([]string{"id", "postcode"}, []string{"birthdate", "zip"})

	var mc *MissingColumnError

	if !errors.As(err, &mc) {
		t.Fatalf("want *MissingColumnError, got %T: %v", err, err)
	}

	if len(mc.Missing) != 2 {
		t.Errorf("want both missing columns reported together, got %v", mc.Missing)
	}

	for _, want := range []string{"birthdate", "zip", "postcode"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestResolveAmbiguousColumn(t *testing.T) {
	t.Parallel()

	_, err := Resolve([]string{"postcode", "postcode"}, []string{"postcode"})

	var ac *AmbiguousColumnError

	if !errors.As(err, &ac) {
		t.Fatalf("want *AmbiguousColumnError, got %T: %v", err, err)
	}
}

func TestResolveIgnoresUnreferencedDuplicates(t *testing.T) {
	t.Parallel()

	if _, err := Resolve([]string{"note", "note", "postcode"}, []string{"postcode"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCSVReadsRows(t *testing.T) {
	t.Parallel()

	c := open(t, "a,b\n1,2\n3,4\n", ',')

	if got := c.Columns(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected header %v", got)
	}

	var n int

	for {
		_, err := c.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		n++
	}

	if n != 2 {
		t.Fatalf("want 2 rows, got %d", n)
	}
}

func TestCSVStripsBOM(t *testing.T) {
	t.Parallel()

	c := open(t, bom+"birth_date,postcode\n1984,110\n", ',')

	if got := c.Columns()[0]; got != "birth_date" {
		t.Fatalf("want the byte order mark stripped, got %q", got)
	}
}

func TestCSVRaggedRowNamesRowAndCounts(t *testing.T) {
	t.Parallel()

	c := open(t, "a,b,c\n1,2,3\n4,5\n", ',')

	if _, err := c.Next(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := c.Next()

	var re *RowError

	if !errors.As(err, &re) {
		t.Fatalf("want *RowError, got %T: %v", err, err)
	}

	if re.Row != 3 || re.Expected != 3 || re.Got != 2 {
		t.Errorf("want row 3 expecting 3 got 2, got %+v", re)
	}
}

func TestCSVQuotedFieldContainingDelimiter(t *testing.T) {
	t.Parallel()

	c := open(t, "a,b\n\"x,y\",z\n", ',')

	rec, err := c.Next()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec[0] != "x,y" {
		t.Errorf("want the quoted comma preserved, got %q", rec[0])
	}
}

func TestCSVAlternateDelimiter(t *testing.T) {
	t.Parallel()

	c := open(t, "a;b\n1;2\n", ';')

	rec, err := c.Next()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec[1] != "2" {
		t.Errorf("want semicolon-separated fields, got %v", rec)
	}
}

func TestCSVPreservesLeadingZeros(t *testing.T) {
	t.Parallel()

	c := open(t, "postcode\n01234\n", ',')

	rec, err := c.Next()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec[0] != "01234" {
		t.Errorf("want 01234 intact, got %q", rec[0])
	}
}

func TestCSVEmptyInput(t *testing.T) {
	t.Parallel()

	_, err := NewCSV(io.NopCloser(strings.NewReader("")), ',')

	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("want an empty-input error, got %v", err)
	}
}

func TestCSVHeaderOnly(t *testing.T) {
	t.Parallel()

	c := open(t, "a,b\n", ',')

	if _, err := c.Next(); err != io.EOF {
		t.Fatalf("want io.EOF, got %v", err)
	}
}

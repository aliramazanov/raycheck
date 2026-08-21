package dataset

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestAdversarialBlankLineRefused(t *testing.T) {
	c := open(t, "a\n1\n\n2\n", ',')

	if _, err := c.Next(); err != nil {
		t.Fatalf("first record: %v", err)
	}

	_, err := c.Next()

	var skipped *SkippedLineError

	if !errors.As(err, &skipped) {
		t.Fatalf("want *SkippedLineError, got %T: %v", err, err)
	}
	if skipped.Line != 3 {
		t.Errorf("want line 3 named, got %d", skipped.Line)
	}
}

func TestAdversarialQuotedEmptyValueInSingleColumn(t *testing.T) {
	c := open(t, "post\np0\n\"\"\n", ',')

	var values []string

	for {
		rec, err := c.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		values = append(values, rec[0])
	}

	if len(values) != 2 || values[1] != "" {
		t.Fatalf("want two records ending in an empty value, got %q", values)
	}
}

func TestAdversarialCRLF(t *testing.T) {
	c := open(t, "a,b\r\n1,2\r\n", ',')

	if got := c.Columns()[1]; got != "b" {
		t.Errorf("header carries a carriage return: %q", got)
	}

	rec, err := c.Next()

	if err != nil {
		t.Fatal(err)
	}

	if rec[1] != "2" {
		t.Errorf("value carries a carriage return: %q", rec[1])
	}
}

func TestAdversarialHostileDelimiters(t *testing.T) {
	for _, d := range []rune{'"', '\n', '\r', 0xFFFD} {
		_, err := NewCSV(io.NopCloser(strings.NewReader("a,b\n1,2\n")), d)

		t.Logf("delimiter %q -> %v", d, err)

		if err == nil {
			t.Errorf("delimiter %q was accepted", d)
		}
	}
}

func TestAdversarialMultilineQuotedField(t *testing.T) {
	c := open(t, "a,b\n\"x\ny\",2\n3,4\n", ',')

	rec, err := c.Next()

	if err != nil {
		t.Fatal(err)
	}

	if rec[0] != "x\ny" {
		t.Errorf("want the embedded newline preserved, got %q", rec[0])
	}

	if _, err := c.Next(); err != nil {
		t.Fatal(err)
	}

	t.Logf("after two records row=%d; the second record starts on file line 4", c.row)
}

func TestAdversarialTooManyFields(t *testing.T) {
	c := open(t, "a,b\n1,2,3\n", ',')

	_, err := c.Next()

	if err == nil {
		t.Fatal("a wider row was accepted")
	}

	t.Logf("%v", err)
}

func TestAdversarialNulByteInValue(t *testing.T) {
	c := open(t, "a\nx\x00y\n", ',')

	rec, err := c.Next()

	if err != nil {
		t.Fatal(err)
	}

	if rec[0] != "x\x00y" {
		t.Errorf("NUL byte mangled: %q", rec[0])
	}
}

func TestAdversarialEmptyHeaderName(t *testing.T) {
	c := open(t, ",b\n1,2\n", ',')

	idx, err := Resolve(c.Columns(), []string{""})

	if err != nil {
		t.Fatalf("an empty header name is still a column that can be declared: %v", err)
	}

	if len(idx) != 1 || idx[0] != 0 {
		t.Errorf("want the empty-named first column at 0, got %v", idx)
	}
}

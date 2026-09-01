package dataset

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const bom = "\ufeff"

type EncodingError struct{ Encoding string }

func (e *EncodingError) Error() string {
	return fmt.Sprintf("dataset: this file begins with a %s byte order mark, so its text is %s rather "+
		"than UTF-8. raycheck compares values byte for byte and will not transcode them, because a "+
		"transcoding that guesses wrong changes a value and so changes k. Convert the file first, for "+
		"example with iconv -f %s -t UTF-8", e.Encoding, e.Encoding, e.Encoding)
}

func wideEncoding(prefix []byte) string {
	switch {
	case len(prefix) >= 4 && string(prefix[:4]) == "\xff\xfe\x00\x00":
		return "UTF-32LE"
	case len(prefix) >= 4 && string(prefix[:4]) == "\x00\x00\xfe\xff":
		return "UTF-32BE"
	case len(prefix) >= 2 && string(prefix[:2]) == "\xff\xfe":
		return "UTF-16LE"
	case len(prefix) >= 2 && string(prefix[:2]) == "\xfe\xff":
		return "UTF-16BE"
	}

	return ""
}

type SkippedLineError struct {
	Line int64
}

func (e *SkippedLineError) Error() string {
	return fmt.Sprintf("dataset: line %d is blank. A blank line is either stray formatting or, in a "+
		"single-column file, a row whose only value is empty, and raycheck will not guess which. "+
		"Delete the line, or write the empty value as \"\"", e.Line)
}

type QuoteError struct {
	Line   int
	Column int
	Bare   bool
	Err    error
}

func (e *QuoteError) Error() string {
	if e.Bare {
		return fmt.Sprintf("dataset: line %d, column %d: a double quote sits inside a field that is not "+
			"quoted, so where the value ends is ambiguous. raycheck will not guess, because guessing "+
			"changes the value and a changed value changes k. Quote the whole field and write any quote "+
			"inside it twice, as \"\"", e.Line, e.Column)
	}

	return fmt.Sprintf("dataset: line %d, column %d: a quoted field is missing its closing quote, or "+
		"carries a quote that is not doubled, so where the value ends is ambiguous. raycheck will not "+
		"guess. Write any quote inside a quoted field twice, as \"\"", e.Line, e.Column)
}

func (e *QuoteError) Unwrap() error { return e.Err }

func readFault(err error) error {

	if pe, ok := errors.AsType[*csv.ParseError](err); ok {
		switch {
		case errors.Is(pe.Err, csv.ErrBareQuote):
			return &QuoteError{Line: pe.Line, Column: pe.Column, Bare: true, Err: err}
		case errors.Is(pe.Err, csv.ErrQuote):
			return &QuoteError{Line: pe.Line, Column: pe.Column, Err: err}
		}
	}

	return nil
}

type CSV struct {
	rc      io.Closer
	lines   *lineCounter
	r       *csv.Reader
	columns []string

	row      int64
	nextLine int64
	skipped  int64
}

func OpenCSV(path string, delim rune) (*CSV, error) {
	if path == "-" {
		return NewCSV(io.NopCloser(os.Stdin), delim)
	}

	f, err := os.Open(path)

	if err != nil {
		return nil, fmt.Errorf("dataset: opening %s: %w", path, err)
	}

	c, err := NewCSV(f, delim)

	if err != nil {
		_ = f.Close()

		return nil, err
	}

	return c, nil
}

func NewCSV(rc io.ReadCloser, delim rune) (*CSV, error) {
	br := bufio.NewReader(rc)

	lines := &lineCounter{r: br}

	prefix, _ := br.Peek(4)

	if enc := wideEncoding(prefix); enc != "" {
		return nil, &EncodingError{Encoding: enc}
	}

	if prefix, err := br.Peek(len(bom)); err == nil && string(prefix) == bom {
		_, _ = br.Discard(len(bom))

		lines.bytes = int64(len(bom))
	}

	r := csv.NewReader(lines)
	r.Comma = delim
	r.ReuseRecord = true

	r.FieldsPerRecord = -1

	header, err := r.Read()

	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errEmptyInput
		}

		if fault := readFault(err); fault != nil {
			return nil, fault
		}

		return nil, fmt.Errorf("dataset: reading the header: %w", err)
	}

	columns := make([]string, len(header))
	copy(columns, header)

	c := &CSV{rc: rc, lines: lines, r: r, columns: columns, row: 1}
	c.nextLine = 1 + spannedLines(header)

	return c, nil
}

func (c *CSV) Columns() []string { return c.columns }

func (c *CSV) Next() ([]string, error) {
	rec, err := c.r.Read()

	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, c.checkTail()
		}

		if fault := readFault(err); fault != nil {
			return nil, fault
		}

		return nil, fmt.Errorf("dataset: %w", err)
	}

	line, _ := c.r.FieldPos(0)

	if gap := int64(line) - c.nextLine; gap > 0 {
		if c.ambiguousBlanks() {
			return nil, &SkippedLineError{Line: c.nextLine}
		}

		c.skipped += gap
	}

	c.row = int64(line)
	c.nextLine = c.row + spannedLines(rec)

	if len(rec) != len(c.columns) {
		return nil, &RowError{Row: c.row, Expected: len(c.columns), Got: len(rec)}
	}

	return rec, nil
}

func (c *CSV) checkTail() error {
	if c.ambiguousBlanks() && c.lines.lines() >= c.nextLine {
		return &SkippedLineError{Line: c.nextLine}
	}

	return io.EOF
}

func (c *CSV) ambiguousBlanks() bool { return len(c.columns) == 1 }

func (c *CSV) Row() int64 { return c.row }

func (c *CSV) Close() error { return c.rc.Close() }

func spannedLines(rec []string) int64 {
	n := int64(1)

	for _, f := range rec {
		n += int64(strings.Count(f, "\n"))
	}

	return n
}

var (
	newline       = []byte{'\n'}
	errEmptyInput = errors.New("dataset: the input is empty, so there is no header to read")
)

type lineCounter struct {
	r        io.Reader
	newlines int64
	bytes    int64
}

func (l *lineCounter) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)

	l.newlines += int64(bytes.Count(p[:n], newline))
	l.bytes += int64(n)

	return n, err
}

func (l *lineCounter) lines() int64 { return l.newlines }

func (c *CSV) Bytes() int64 { return c.lines.bytes }

func (c *CSV) Skipped() int64 { return c.skipped }

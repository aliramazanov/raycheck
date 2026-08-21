package dataset

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestAdversarialBytesSurviveIntact(t *testing.T) {
	values := []string{"caf\xe9", "caf\xe8", "\xff\xfe", "a\tb", "  padded  "}

	var b strings.Builder

	b.WriteString("v\n")

	for _, v := range values {
		b.WriteString(v + "\n")
	}

	c := open(t, b.String(), ',')

	for i, want := range values {
		rec, err := c.Next()

		if err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
		if rec[0] != want {
			t.Errorf("row %d: want %q, got %q", i, want, rec[0])
		}
	}
}

func TestAdversarialLoneCarriageReturn(t *testing.T) {
	c := open(t, "a\nx\ry\n", ',')

	rec, err := c.Next()

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("read %q", rec[0])

	if _, err := c.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("want a single record, got a second: %v", err)
	}
}

func TestAdversarialBOMInsideAValue(t *testing.T) {
	c := open(t, "a\n"+bom+"x\n", ',')

	rec, err := c.Next()

	if err != nil {
		t.Fatal(err)
	}
	if rec[0] != bom+"x" {
		t.Errorf("a byte order mark inside a value was stripped: %q", rec[0])
	}
}

func TestAdversarialDoubleBOM(t *testing.T) {
	c := open(t, bom+bom+"a\nx\n", ',')

	if got := c.Columns()[0]; got != bom+"a" {
		t.Errorf("want one mark stripped, got %q", got)
	}
}

func TestAdversarialEmptyHeaderField(t *testing.T) {
	c := open(t, "a,\nx,y\n", ',')

	if len(c.Columns()) != 2 {
		t.Fatalf("want 2 columns, got %v", c.Columns())
	}

	if _, err := Resolve(c.Columns(), []string{"a"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAdversarialManyColumns(t *testing.T) {
	const n = 2000

	head := make([]string, n)
	row := make([]string, n)

	for i := range head {
		head[i] = "c" + strings.Repeat("0", 4-len(itoa(i))) + itoa(i)
		row[i] = itoa(i)
	}

	c := open(t, strings.Join(head, ",")+"\n"+strings.Join(row, ",")+"\n", ',')

	if len(c.Columns()) != n {
		t.Fatalf("want %d columns, got %d", n, len(c.Columns()))
	}

	idx, err := Resolve(c.Columns(), []string{head[0], head[n-1]})

	if err != nil {
		t.Fatal(err)
	}

	if idx[1] != n-1 {
		t.Errorf("want the last column at %d, got %d", n-1, idx[1])
	}

	rec, err := c.Next()

	if err != nil {
		t.Fatal(err)
	}
	if len(rec) != n {
		t.Errorf("want %d fields, got %d", n, len(rec))
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var b []byte

	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}

	return string(b)
}

func TestAdversarialReadPastEOF(t *testing.T) {
	c := open(t, "a\nx\n", ',')

	if _, err := c.Next(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if _, err := c.Next(); !errors.Is(err, io.EOF) {
			t.Fatalf("call %d after the end: %v", i, err)
		}
	}
}

func TestAdversarialUnterminatedQuote(t *testing.T) {
	c := open(t, "a,b\n\"unterminated,2\n", ',')

	_, err := c.Next()

	t.Logf("%v", err)

	if err == nil || errors.Is(err, io.EOF) {
		t.Errorf("an unterminated quote should be an error, got %v", err)
	}
}

func TestAdversarialCloseIsReported(t *testing.T) {
	c, err := NewCSV(failingCloser{strings.NewReader("a\nx\n")}, ',')

	if err != nil {
		t.Fatal(err)
	}

	if err := c.Close(); err == nil {
		t.Error("a failing close was swallowed")
	}
}

type failingCloser struct{ io.Reader }

func (failingCloser) Close() error { return errors.New("close failed") }

func TestBytesReadIncludesTheByteOrderMark(t *testing.T) {
	t.Parallel()

	body := bom + "a,b\n1,2\n3,4\n"

	c, err := NewCSV(io.NopCloser(strings.NewReader(body)), ',')

	if err != nil {
		t.Fatal(err)
	}

	for {
		if _, err := c.Next(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}

	if got, want := c.Bytes(), int64(len(body)); got != want {
		t.Errorf("want %d bytes read, got %d", want, got)
	}
}

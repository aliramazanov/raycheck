package dataset

import (
	"io"
	"strings"
	"testing"
)

func TestBlankLineIsNotSilentlyDropped(t *testing.T) {
	tests := map[string]struct {
		body    string
		records int
	}{
		"single column, empty value":  {body: "post\np0\np0\np0\n\n", records: 4},
		"single column, blank middle": {body: "a\nx\n\n\ny\n", records: 4},
		"single column, blank first":  {body: "a\n\nx\n", records: 2},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, err := NewCSV(io.NopCloser(strings.NewReader(tt.body)), ',')

			if err != nil {
				t.Logf("refused at open: %v", err)

				return
			}

			var n int
			var readErr error

			for {
				_, err := c.Next()

				if err == io.EOF {
					break
				}
				if err != nil {
					readErr = err

					break
				}

				n++
			}

			if readErr != nil {
				t.Logf("refused while reading: %v", readErr)

				return
			}

			t.Errorf("read %d records from a file holding %d lines of data, silently losing %d",
				n, tt.records, tt.records-n)
		})
	}
}

func TestOrdinaryFilesAreUnaffected(t *testing.T) {
	t.Parallel()

	bodies := []string{

		"a,b\n1,2\n3,4\n\n",
		"a,b\n1,2\n\n3,4\n",
		"a,b\n\n1,2\n",
		"a,b\n1,2\n3,4\n",
		"a\nx\ny\n",
		"a,b\n\"multi\nline\",2\n3,4\n",
		"a,b\r\n1,2\r\n3,4\r\n",
		"a,b\n1,2",
		"a,b\n\"x\",\"\"\n",
	}

	for _, body := range bodies {
		c, err := NewCSV(io.NopCloser(strings.NewReader(body)), ',')

		if err != nil {
			t.Errorf("%q: %v", body, err)

			continue
		}

		for {
			_, err := c.Next()

			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("%q: %v", body, err)

				break
			}
		}
	}
}

func TestFinalLineWithoutNewline(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		body string
		want int
	}{
		"single column, no trailing newline": {body: "a\nx\ny", want: 2},
		"single column, trailing newline":    {body: "a\nx\ny\n", want: 2},
		"header only, no trailing newline":   {body: "a", want: 0},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, err := NewCSV(io.NopCloser(strings.NewReader(tt.body)), ',')

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var got int

			for {
				_, err := c.Next()

				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("want %d records, got an error after %d: %v", tt.want, got, err)
				}

				got++
			}

			if got != tt.want {
				t.Errorf("want %d records, got %d", tt.want, got)
			}
		})
	}
}

func TestRowNumbersSurviveASkippedLine(t *testing.T) {
	t.Parallel()

	c, err := NewCSV(io.NopCloser(strings.NewReader("a,b\n1,2\n\n3,4\n\n\n5,6\n")), ',')

	if err != nil {
		t.Fatal(err)
	}

	want := []int64{2, 4, 7}

	for i, w := range want {
		if _, err := c.Next(); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}

		if got := c.Row(); got != w {
			t.Errorf("record %d: want file line %d, got %d", i, w, got)
		}
	}
}

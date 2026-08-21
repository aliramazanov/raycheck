package humanize

import (
	"testing"
	"time"
)

func TestCommas(t *testing.T) {
	t.Parallel()

	tests := map[int64]string{
		0: "0", 7: "7", 999: "999", 1000: "1,000", 4112: "4,112",
		1234567: "1,234,567", -1234: "-1,234", -999: "-999",
	}

	for n, want := range tests {
		if got := Commas(n); got != want {
			t.Errorf("Commas(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestCount(t *testing.T) {
	t.Parallel()

	tests := map[int64]string{0: "0 rows", 1: "1 row", 2: "2 rows", 5000: "5,000 rows"}

	for n, want := range tests {
		if got := Count(n, "row"); got != want {
			t.Errorf("Count(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDuration(t *testing.T) {
	t.Parallel()

	tests := map[time.Duration]string{
		400 * time.Nanosecond:   "400ns",
		30 * time.Microsecond:   "30us",
		5 * time.Millisecond:    "5ms",
		1500 * time.Millisecond: "1.50s",
	}

	for d, want := range tests {
		if got := Duration(d); got != want {
			t.Errorf("Duration(%v) = %q, want %q", d, got, want)
		}
	}
}

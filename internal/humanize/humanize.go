package humanize

import (
	"strconv"
	"strings"
	"time"
)

func Commas(n int64) string {
	s := strconv.FormatInt(n, 10)

	negative := strings.HasPrefix(s, "-")

	if negative {
		s = s[1:]
	}

	if len(s) > 3 {
		var b strings.Builder

		b.Grow(len(s) + len(s)/3)

		lead := len(s) % 3

		if lead == 0 {
			lead = 3
		}

		b.WriteString(s[:lead])

		for i := lead; i < len(s); i += 3 {
			b.WriteByte(',')
			b.WriteString(s[i : i+3])
		}

		s = b.String()
	}

	if negative {
		return "-" + s
	}

	return s
}

func Count(n int64, noun string) string {
	if n == 1 {
		return "1 " + noun
	}

	return Commas(n) + " " + noun + "s"
}

func Duration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return strconv.FormatFloat(d.Seconds(), 'f', 2, 64) + "s"
	case d >= time.Millisecond:
		return strconv.FormatInt(d.Milliseconds(), 10) + "ms"
	case d >= time.Microsecond:
		return strconv.FormatInt(d.Microseconds(), 10) + "us"
	default:
		return strconv.FormatInt(d.Nanoseconds(), 10) + "ns"
	}
}

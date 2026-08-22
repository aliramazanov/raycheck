package obs

import (
	"io"
	"strings"
	"testing"
	"time"
)

func BenchmarkRow(b *testing.B) {
	r := New(io.Discard, Options{})

	b.ReportAllocs()

	for b.Loop() {
		r.Row()
	}
}

func BenchmarkSpan(b *testing.B) {
	r := New(io.Discard, Options{})

	b.ReportAllocs()

	for b.Loop() {
		r.Span("x").End()
	}
}

func TestRowCountsAndHeartbeats(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	r := New(&b, Options{Verbose: true, Heartbeat: time.Nanosecond})

	elapse := func() { r.lastBeat = r.lastBeat.Add(-time.Hour) }

	for window := 0; window < 2; window++ {
		elapse()

		for i := 0; i < beatEvery; i++ {
			r.Row()
		}
	}

	for i := 0; i < 5; i++ {
		r.Row()
	}

	if r.Rows() != int64(beatEvery*2+5) {
		t.Errorf("want %d rows, got %d", beatEvery*2+5, r.Rows())
	}

	if n := strings.Count(b.String(), "still reading"); n != 2 {
		t.Errorf("want 2 heartbeats over 2 windows, got %d", n)
	}
}

func TestHeartbeatIsRateLimitedByWallClock(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	r := New(&b, Options{Verbose: true, Heartbeat: time.Hour})

	for i := 0; i < beatEvery*3; i++ {
		r.Row()
	}

	if r.Rows() != int64(beatEvery*3) {
		t.Errorf("want %d rows, got %d", beatEvery*3, r.Rows())
	}

	if n := strings.Count(b.String(), "still reading"); n != 0 {
		t.Errorf("no heartbeat is due inside the interval, got %d", n)
	}
}

func TestSilentByDefault(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	r := New(&b, Options{})

	r.Span("a").Attr("k", 1).End()
	r.Info("something")
	r.Debug("detail")
	r.Count("rows", 10)

	for i := 0; i < beatEvery*2; i++ {
		r.Row()
	}

	if b.Len() != 0 {
		t.Errorf("want silence, got:\n%s", b.String())
	}
}

func TestSpansNest(t *testing.T) {
	t.Parallel()

	r := New(io.Discard, Options{})

	outer := r.Span("outer")
	inner := r.Span("inner")

	inner.End()
	outer.End()

	root := r.Root()

	if len(root.Children) != 1 || root.Children[0].Name != "outer" {
		t.Fatalf("unexpected tree: %+v", root)
	}
	if kids := root.Children[0].Children; len(kids) != 1 || kids[0].Name != "inner" {
		t.Fatalf("inner span did not nest: %+v", root.Children[0])
	}
}

func TestEndIsIdempotent(t *testing.T) {
	t.Parallel()

	r := New(io.Discard, Options{})

	s := r.Span("once")
	s.End()

	first := s.Duration

	s.End()

	if s.Duration != first {
		t.Errorf("a second End changed the duration from %v to %v", first, s.Duration)
	}
	if r.current != r.root {
		t.Error("a second End popped the stack again")
	}
}

func TestMetricsKeepInsertionOrder(t *testing.T) {
	t.Parallel()

	r := New(io.Discard, Options{})

	r.Count("b", 1)
	r.Count("a", 2)
	r.Count("b", 3)
	r.Set("c", 9)

	got := r.Metrics()

	want := []Metric{{"b", 4}, {"a", 2}, {"c", 9}}

	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("at %d want %v, got %v", i, want[i], got[i])
		}
	}
}

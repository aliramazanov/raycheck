package obs

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestWriteTraceRendersTheTree(t *testing.T) {
	t.Parallel()

	r := New(io.Discard, Options{})

	outer := r.Span("outer").Attr("rows", 10)
	inner := r.Span("inner")

	inner.Fail(errors.New("boom"))
	inner.End()
	outer.End()

	var b strings.Builder

	if err := WriteTrace(&b, r.Root()); err != nil {
		t.Fatal(err)
	}

	out := b.String()

	for _, want := range []string{"trace", "run", "outer", "inner", "rows=10", "FAILED"} {
		if !strings.Contains(out, want) {
			t.Errorf("the trace should contain %q:\n%s", want, out)
		}
	}

	if strings.Index(out, "outer") > strings.Index(out, "inner") {
		t.Errorf("a child should print under its parent:\n%s", out)
	}
}

func TestWriteTraceHandlesNothing(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	if err := WriteTrace(&b, nil); err != nil {
		t.Fatal(err)
	}

	if b.Len() != 0 {
		t.Errorf("want nothing, got %q", b.String())
	}
}

func TestWriteMetricsAlignsAndOrders(t *testing.T) {
	t.Parallel()

	r := New(io.Discard, Options{})

	r.Count("short", 5)
	r.Count("a_much_longer_name", 1234567)

	var b strings.Builder

	if err := WriteMetrics(&b, r.Metrics()); err != nil {
		t.Fatal(err)
	}

	out := b.String()

	if !strings.Contains(out, "1,234,567") {
		t.Errorf("large numbers should be grouped:\n%s", out)
	}
	if strings.Index(out, "short") > strings.Index(out, "a_much_longer_name") {
		t.Errorf("metrics should keep the order they were declared in:\n%s", out)
	}
}

func TestWriteMetricsHandlesNothing(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	if err := WriteMetrics(&b, nil); err != nil {
		t.Fatal(err)
	}

	if b.Len() != 0 {
		t.Errorf("want nothing, got %q", b.String())
	}
}

func TestWriteFailuresListsEachOne(t *testing.T) {
	t.Parallel()

	r := New(io.Discard, Options{})

	r.Span("open").Attr("source", "x.csv").Fail(errors.New("no such file")).End()
	r.Span("read").Fail(errors.New("malformed row")).End()

	var b strings.Builder

	if err := WriteFailures(&b, r.Failures()); err != nil {
		t.Fatal(err)
	}

	out := b.String()

	for _, want := range []string{"errors (2)", "open", "no such file", "source=x.csv", "read", "malformed row"} {
		if !strings.Contains(out, want) {
			t.Errorf("the list should contain %q:\n%s", want, out)
		}
	}
}

func TestWriteFailuresHandlesNothing(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	if err := WriteFailures(&b, nil); err != nil {
		t.Fatal(err)
	}

	if b.Len() != 0 {
		t.Errorf("want nothing, got %q", b.String())
	}
}

func TestFlattenOrdersBySlowest(t *testing.T) {
	t.Parallel()

	r := New(io.Discard, Options{})

	quick := r.Span("quick")
	quick.Duration = time.Millisecond
	quick.End()

	slow := r.Span("slow")
	slow.End()
	slow.Duration = time.Hour

	spans := Flatten(r.Root())

	if len(spans) != 3 {
		t.Fatalf("want the root and both children, got %d", len(spans))
	}
	if spans[0].Name != "slow" {
		t.Errorf("want the slowest first, got %q", spans[0].Name)
	}

	if got := Flatten(nil); got != nil {
		t.Errorf("want nothing for no tree, got %v", got)
	}
}

func TestRenderersHandleAnEmptyRun(t *testing.T) {
	t.Parallel()

	r := New(io.Discard, Options{})

	var b strings.Builder

	for _, write := range []func() error{
		func() error { return WriteTrace(&b, r.Root()) },
		func() error { return WriteMetrics(&b, r.Metrics()) },
		func() error { return WriteFailures(&b, r.Failures()) },
	} {
		if err := write(); err != nil {
			t.Fatal(err)
		}
	}

	if !strings.Contains(b.String(), "run") {
		t.Errorf("the root span should still render:\n%s", b.String())
	}
}

func TestRenderersReportWriteFailures(t *testing.T) {
	t.Parallel()

	boom := errors.New("disk full")
	r := New(io.Discard, Options{})

	r.Span("a").Fail(boom).End()
	r.Count("n", 1)

	for name, write := range map[string]func(io.Writer) error{
		"trace":    func(w io.Writer) error { return WriteTrace(w, r.Root()) },
		"metrics":  func(w io.Writer) error { return WriteMetrics(w, r.Metrics()) },
		"failures": func(w io.Writer) error { return WriteFailures(w, r.Failures()) },
	} {
		if err := write(brokenWriter{boom}); !errors.Is(err, boom) {
			t.Errorf("%s: want the write error, got %v", name, err)
		}
	}
}

type brokenWriter struct{ err error }

func (b brokenWriter) Write([]byte) (int, error) { return 0, b.err }

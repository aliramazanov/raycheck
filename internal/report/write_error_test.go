package report

import (
	"errors"
	"strings"
	"testing"

	"github.com/aliramazanov/raycheck/internal/measure"
)

type failAfter struct {
	n       int
	written int
	calls   int
	err     error
}

func (f *failAfter) Write(p []byte) (int, error) {
	f.calls++

	if f.written >= f.n {
		return 0, f.err
	}

	room := f.n - f.written

	if len(p) <= room {
		f.written += len(p)

		return len(p), nil
	}

	f.written = f.n

	return room, f.err
}

func sample(t *testing.T) Report {
	t.Helper()

	g := measure.NewGrouper([]string{"a"}, []int{0}, nil)

	for i, v := range []string{"x", "y", "y"} {
		g.Add([]string{v}, int64(i+2))
	}

	res, err := g.Finish(5, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	return Report{Result: res, HasSensitive: true}
}

func TestTextReportsWriteFailure(t *testing.T) {
	t.Parallel()

	boom := errors.New("disk full")

	for _, at := range []int{0, 1, 40, 200, 400} {
		w := &failAfter{n: at, err: boom}

		err := Text(w, sample(t))

		if !errors.Is(err, boom) {
			t.Errorf("failing after %d bytes: want the write error, got %v", at, err)
		}
	}
}

func TestTextSucceedsOnAGoodWriter(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	if err := Text(&b, sample(t)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(b.String(), "not anonymous") {
		t.Errorf("the report stopped early:\n%s", b.String())
	}
}

func TestTextStopsAtTheFirstFailure(t *testing.T) {
	t.Parallel()

	boom := errors.New("broken pipe")
	w := &failAfter{n: 30, err: boom}

	if err := Text(w, sample(t)); !errors.Is(err, boom) {
		t.Fatalf("want the write error, got %v", err)
	}

	if w.written != 30 {
		t.Errorf("want writing to stop at the failure, got %d bytes through", w.written)
	}

	before := w.calls

	if err := Text(w, sample(t)); !errors.Is(err, boom) {
		t.Fatalf("want the write error, got %v", err)
	}

	if attempts := w.calls - before; attempts != 1 {
		t.Errorf("want one attempt against an already-failed writer, got %d", attempts)
	}
}

func TestJSONReportsWriteFailure(t *testing.T) {
	t.Parallel()

	boom := errors.New("disk full")

	if err := JSON(&failAfter{n: 0, err: boom}, []Report{sample(t)}, nil); !errors.Is(err, boom) {
		t.Errorf("want the write error, got %v", err)
	}
}

func TestFaultReportsWriteFailure(t *testing.T) {
	t.Parallel()

	boom := errors.New("disk full")

	if err := Fault(&failAfter{n: 0, err: boom}, errors.New("cause")); !errors.Is(err, boom) {
		t.Errorf("want the write error, got %v", err)
	}
}

package obs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

type human struct {
	mu    *sync.Mutex
	w     io.Writer
	level slog.Leveler
	attrs []slog.Attr
	group string
}

func newHuman(w io.Writer, level slog.Leveler) slog.Handler {
	return &human{mu: &sync.Mutex{}, w: w, level: level}
}

func (h *human) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level.Level() }

func (h *human) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)

	return &next
}

func (h *human) WithGroup(name string) slog.Handler {
	next := *h
	next.group = name

	return &next
}

func (h *human) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	if r.Level < slog.LevelError {
		b.WriteString("  ")
	}

	if r.Level < slog.LevelError {
		fmt.Fprintf(&b, "%-18s", r.Message)
	} else {
		b.WriteString(r.Message)
	}

	for _, a := range h.attrs {
		writeAttr(&b, a)
	}

	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, a)

		return true
	})

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := io.WriteString(h.w, b.String())

	return err
}

func writeAttr(b *strings.Builder, a slog.Attr) {

	if a.Key == "exit_code" || a.Key == "error" {
		return
	}

	fmt.Fprintf(b, " %s=%v", a.Key, a.Value.Any())
}

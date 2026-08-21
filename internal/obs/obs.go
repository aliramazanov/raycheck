package obs

import (
	"io"
	"log/slog"
	"runtime"
	"time"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

type Options struct {
	Verbose bool
	Debug   bool

	Format string

	Heartbeat time.Duration
}

type Failure struct {
	Span    string
	Message string
	Attrs   []Attr
}

type Recorder struct {
	log  *slog.Logger
	opts Options

	root    *Span
	current *Span

	metrics  map[string]int64
	order    []string
	failures []Failure

	started   time.Time
	lastBeat  time.Time
	rows      int64
	nextCheck int64
}

const beatEvery = 1 << 16

func New(w io.Writer, opts Options) *Recorder {
	if opts.Heartbeat <= 0 {
		opts.Heartbeat = 5 * time.Second
	}

	level := slog.LevelWarn

	switch {
	case opts.Debug:
		level = slog.LevelDebug
	case opts.Verbose:
		level = slog.LevelInfo
	}

	h := newHuman(w, level)

	if opts.Format == FormatJSON {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	}

	now := time.Now()
	root := &Span{Name: "run", start: now}

	r := &Recorder{
		log:       slog.New(h),
		opts:      opts,
		root:      root,
		metrics:   map[string]int64{},
		started:   now,
		lastBeat:  now,
		nextCheck: beatEvery,
	}

	root.rec = r
	r.current = root

	return r
}

func (r *Recorder) alreadyRecorded(err error) bool {
	if len(r.failures) == 0 {
		return false
	}

	return r.failures[len(r.failures)-1].Message == err.Error()
}

func (r *Recorder) Error(err error, args ...any) {
	if err == nil {
		return
	}

	if r.alreadyRecorded(err) {
		return
	}

	r.failures = append(r.failures, Failure{Span: r.current.Name, Message: err.Error()})
	r.log.Debug("failed", append(args, "span", r.current.Name, "error", err)...)
}

func (r *Recorder) Failures() []Failure { return r.failures }

func (r *Recorder) Elapsed() time.Duration { return time.Since(r.started) }

func (r *Recorder) Root() *Span {
	r.root.Duration = time.Since(r.started)

	return r.root
}

func (r *Recorder) AllocatedBytes() uint64 {
	var m runtime.MemStats

	runtime.ReadMemStats(&m)

	return m.TotalAlloc
}

func (r *Recorder) Debug(msg string, args ...any) { r.log.Debug(msg, args...) }

func (r *Recorder) Info(msg string, args ...any) { r.log.Info(msg, args...) }

func (r *Recorder) Warn(msg string, args ...any) { r.log.Warn(msg, args...) }

func (r *Recorder) Fatal(msg string, args ...any) { r.log.Error(msg, args...) }

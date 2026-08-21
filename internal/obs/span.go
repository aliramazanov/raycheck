package obs

import "time"

type Span struct {
	Name     string
	Duration time.Duration
	Attrs    []Attr
	Failed   bool
	Children []*Span

	rec    *Recorder
	parent *Span
	start  time.Time
	ended  bool
}

type Attr struct {
	Key   string
	Value any
}

func (r *Recorder) Span(name string) *Span {
	s := &Span{Name: name, rec: r, parent: r.current, start: time.Now()}

	r.current.Children = append(r.current.Children, s)
	r.current = s

	r.log.Debug("entering " + name)

	return s
}

func (s *Span) Attr(key string, value any) *Span {
	s.Attrs = append(s.Attrs, Attr{Key: key, Value: value})

	return s
}

func (s *Span) Fail(err error) *Span {
	if err == nil {
		return s
	}

	s.Failed = true

	if s.rec.alreadyRecorded(err) {
		return s
	}

	s.rec.failures = append(s.rec.failures, Failure{
		Span:    s.Name,
		Message: err.Error(),
		Attrs:   s.Attrs,
	})

	s.rec.log.Debug("span failed", append(s.logArgs(), "span", s.Name, "error", err)...)

	return s
}

func (s *Span) End() {
	if s.ended {
		return
	}

	s.ended = true
	s.Duration = time.Since(s.start)

	if s.parent != nil {
		s.rec.current = s.parent
	}

	args := append([]any{"ms", s.Duration.Milliseconds()}, s.logArgs()...)

	if s.Failed {
		args = append(args, "failed", true)
	}

	s.rec.log.Info(s.Name, args...)
}

func (s *Span) logArgs() []any {
	args := make([]any, 0, len(s.Attrs)*2)

	for _, a := range s.Attrs {
		args = append(args, a.Key, a.Value)
	}

	return args
}

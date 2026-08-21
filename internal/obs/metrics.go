package obs

import "time"

type Metric struct {
	Name  string
	Value int64
}

func (r *Recorder) Count(name string, n int64) {
	if _, seen := r.metrics[name]; !seen {
		r.order = append(r.order, name)
	}

	r.metrics[name] += n
}

func (r *Recorder) Set(name string, v int64) {
	if _, seen := r.metrics[name]; !seen {
		r.order = append(r.order, name)
	}

	r.metrics[name] = v
}

func (r *Recorder) Metrics() []Metric {
	out := make([]Metric, 0, len(r.order))

	for _, name := range r.order {
		out = append(out, Metric{Name: name, Value: r.metrics[name]})
	}

	return out
}

func (r *Recorder) Row() {
	r.rows++

	if r.rows < r.nextCheck {
		return
	}

	r.nextCheck = r.rows + beatEvery

	if now := time.Now(); now.Sub(r.lastBeat) >= r.opts.Heartbeat {
		r.lastBeat = now
		r.log.Info("still reading", "rows", r.rows, "rows_per_second", r.Rate())
	}
}

func (r *Recorder) Rows() int64 { return r.rows }

func (r *Recorder) Rate() int64 {
	if elapsed := time.Since(r.started).Seconds(); elapsed > 0 {
		return int64(float64(r.rows) / elapsed)
	}

	return 0
}

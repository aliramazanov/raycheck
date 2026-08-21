package main

import (
	"flag"
	"io"

	"github.com/aliramazanov/raycheck/internal/obs"
	"github.com/aliramazanov/raycheck/internal/report"
)

const (
	formatText = "text"
	formatJSON = "json"
)

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("raycheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(stderr) }

	var (
		configPath = fs.String("config", "", "path to the quasi-identifier config")
		only       = fs.String("dataset", "", "check only the named dataset")
		format     = fs.String("format", formatText, "text or json")
		logFormat  = fs.String("log-format", formatText, "text or json")
		asJSON     = fs.Bool("json", false, "shorthand for --format json")
		strict     = fs.Bool("strict", false, "fail on concerns as well as on a breached threshold")
		verbose    = fs.Bool("verbose", false, "report each phase as it finishes")
		short      = fs.Bool("v", false, "shorthand for --verbose")
		debug      = fs.Bool("debug", false, "report every step, including per-dataset detail")
		trace      = fs.Bool("trace", false, "print the span tree, metrics and errors when the run ends")
	)

	if err := fs.Parse(reorder(fs, args)); err != nil {
		return exitBadUsage
	}

	if *asJSON {
		*format = formatJSON
	}

	for _, f := range []struct{ name, value string }{
		{"--format", *format},
		{"--log-format", *logFormat},
	} {
		if f.value != formatText && f.value != formatJSON {
			return fail(obs.New(stderr, obs.Options{}),
				badUsage("raycheck: %s must be %s or %s, got %q", f.name, formatText, formatJSON, f.value))
		}
	}

	rec := obs.New(stderr, obs.Options{
		Verbose: *verbose || *short,
		Debug:   *debug,
		Format:  *logFormat,
	})

	var telemetry *report.Telemetry

	if *trace {
		telemetry = &report.Telemetry{}
	}

	code := execute(rec, stdout, options{
		args:       fs.Args(),
		configPath: *configPath,
		only:       *only,
		format:     *format,
		strict:     *strict,
		telemetry:  telemetry,
	})

	rec.Set("exit_code", int64(code))
	rec.Set("elapsed_ms", rec.Elapsed().Milliseconds())

	if *trace {
		_ = obs.WriteTrace(stderr, rec.Root())
		_ = obs.WriteMetrics(stderr, rec.Metrics())
		_ = obs.WriteFailures(stderr, rec.Failures())
	}

	return code
}

type options struct {
	args       []string
	configPath string
	only       string
	format     string
	strict     bool
	telemetry  *report.Telemetry
}

func execute(rec *obs.Recorder, stdout io.Writer, o options) int {
	reports, err := check(rec, o)

	if err != nil {
		if o.format == formatJSON {
			_ = report.Fault(stdout, err)
		}

		return fail(rec, err)
	}

	code := exitOK

	if report.Failed(reports) {
		code = exitBreached
	}

	rec.Set("exit_code", int64(code))
	rec.Set("allocated_bytes", int64(rec.AllocatedBytes()))

	if o.telemetry != nil {
		fillTelemetry(o.telemetry, rec)
	}

	span := rec.Span("render").Attr("format", o.format)

	err = render(stdout, reports, o.format, o.telemetry)

	span.Fail(err).End()

	if err != nil {
		return fail(rec, inputFault(err))
	}

	return code
}

func render(w io.Writer, reports []report.Report, format string, telemetry *report.Telemetry) error {
	if format == formatJSON {
		return report.JSON(w, reports, telemetry)
	}

	for i, r := range reports {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}

		if err := report.Text(w, r); err != nil {
			return err
		}
	}

	return report.Summary(w, reports)
}

func fillTelemetry(t *report.Telemetry, rec *obs.Recorder) {
	t.ElapsedMS = rec.Elapsed().Milliseconds()
	t.Metrics = map[string]int64{}

	for _, m := range rec.Metrics() {
		t.Metrics[m.Name] = m.Value
	}

	for _, f := range rec.Failures() {
		t.Errors = append(t.Errors, f.Span+": "+f.Message)
	}

	t.Spans = spansOf(rec.Root().Children)
}

func spansOf(spans []*obs.Span) []report.Span {
	out := make([]report.Span, 0, len(spans))

	for _, s := range spans {
		out = append(out, report.Span{
			Name:       s.Name,
			DurationMS: s.Duration.Milliseconds(),
			Failed:     s.Failed,
			Children:   spansOf(s.Children),
		})
	}

	return out
}

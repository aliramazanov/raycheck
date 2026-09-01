package main

import (
	"errors"
	"io"
	"os"

	"github.com/aliramazanov/raycheck/internal/config"
	"github.com/aliramazanov/raycheck/internal/dataset"
	"github.com/aliramazanov/raycheck/internal/measure"
	"github.com/aliramazanov/raycheck/internal/obs"
	"github.com/aliramazanov/raycheck/internal/report"
)

func check(rec *obs.Recorder, o options) ([]report.Report, error) {
	configPath := o.configPath

	if configPath == "" {
		configPath = os.Getenv(configEnv)
	}

	if configPath == "" {
		return nil, badUsage("raycheck: --config is required, or set %s", configEnv)
	}

	if len(o.args) > 1 {
		return nil, badUsage("raycheck: expected at most one input path, got %d", len(o.args))
	}

	load := rec.Span("load-config").Attr("path", configPath)

	cfg, err := config.Load(configPath)

	load.Fail(err).End()

	if err != nil {
		return nil, fault{code: exitBadUsage, err: err}
	}

	rec.Set("datasets_declared", int64(len(cfg.Datasets)))

	chosen, err := selectDatasets(cfg, o.only)

	if err != nil {
		rec.Error(err)

		return nil, err
	}

	rec.Set("datasets_checked", int64(len(chosen)))

	if len(o.args) == 1 && len(chosen) != 1 {
		return nil, badUsage(
			"raycheck: an input path overrides one dataset's source, but %d datasets are selected; use --dataset to pick one",
			len(chosen))
	}

	override := ""

	if len(o.args) == 1 {
		override = o.args[0]
	}

	reports := make([]report.Report, 0, len(chosen))

	for _, d := range chosen {
		r, err := checkDataset(rec, cfg, d, override, o.strict)

		if err != nil {
			return nil, err
		}

		reports = append(reports, r)
	}

	return reports, nil
}

func checkDataset(
	rec *obs.Recorder,
	cfg *config.Config,
	d config.Dataset,
	override string,
	strict bool,
) (report.Report, error) {
	span := rec.Span("dataset").
		Attr("name", d.Name).
		Attr("quasi_identifiers", len(d.QuasiIdentifiers))

	defer span.End()

	source := cfg.SourcePath(d)

	if override != "" {
		source = override
	}

	if source == "" {
		err := badUsage("raycheck: dataset %q has no source, so there is nothing to read", d.Name)

		span.Fail(err)

		return report.Report{}, err
	}

	read, res, err := measureDataset(rec, d, source)

	if err != nil {
		span.Fail(err)

		return report.Report{}, err
	}

	if accounted := res.Rows + res.Suppressed; accounted != read {
		rec.Warn("rows unaccounted for",
			"read", read, "grouped", res.Rows, "suppressed", res.Suppressed, "lost", read-accounted)

		rec.Count("rows_unaccounted", read-accounted)
	}

	span.Attr("k", res.K).Attr("rows", res.Rows)

	rec.Count("rows_grouped", res.Rows)
	rec.Count("rows_suppressed", res.Suppressed)
	rec.Count("rows_blank_quasi_identifier", res.EmptyQI)
	rec.Count("groups", int64(res.Groups))
	rec.Count("groups_below_threshold", int64(res.BelowGroups))
	rec.Count("rows_unique", res.UniqueRows)
	rec.Count("rows_at_risk", res.RowsAtRisk)

	if len(res.Diversity) > 0 {
		rec.Set("l_diversity_min", int64(res.MinL()))
		span.Attr("l", res.MinL())
	}

	if len(res.Closeness) > 0 {
		rec.Set("t_closeness_max_per_mille", int64(res.MaxT()*1000+0.5))
		span.Attr("t", res.MaxT())
	}

	name := ""

	if len(cfg.Datasets) > 1 {
		name = d.Name
	}

	return report.Report{
		Name:                name,
		Result:              res,
		HasSensitive:        len(d.Sensitive) > 0,
		SuppressionDeclared: d.Suppression != nil,
		Strict:              strict,
	}, nil
}

func selectDatasets(cfg *config.Config, only string) ([]config.Dataset, error) {
	if only == "" {
		return cfg.Datasets, nil
	}

	d, ok := cfg.Dataset(only)

	if !ok {
		names := make([]string, len(cfg.Datasets))

		for i, c := range cfg.Datasets {
			names[i] = c.Name
		}

		return nil, badUsage("raycheck: no dataset named %q; the config declares %v", only, names)
	}

	return []config.Dataset{d}, nil
}

func measureDataset(
	rec *obs.Recorder,
	d config.Dataset,
	source string,
) (int64, measure.Result, error) {
	open := rec.Span("open").Attr("source", source)

	src, err := dataset.OpenCSV(source, d.Delim())

	open.Fail(err).End()

	if err != nil {
		return 0, measure.Result{}, inputFault(err)
	}

	defer func() {
		if err := src.Close(); err != nil {
			rec.Error(err, "source", source)
		}
	}()

	resolve := rec.Span("resolve-columns").Attr("columns", len(src.Columns()))

	idx, err := dataset.Resolve(src.Columns(), d.QuasiIdentifiers)

	resolve.Fail(err)

	if err != nil {
		resolve.End()

		return 0, measure.Result{}, fault{code: exitBadUsage, err: err}
	}

	read := rec.Span("read-and-group")
	before := rec.Rows()

	g := measure.NewGrouper(d.QuasiIdentifiers, idx, d.Suppression)

	if names := d.SensitiveNames(); len(names) > 0 {
		sensIdx, err := dataset.Resolve(src.Columns(), names)

		resolve.Fail(err)

		if err != nil {
			return 0, measure.Result{}, fault{code: exitBadUsage, err: err}
		}

		kinds := make([]measure.Kind, len(names))

		if declared, complete := d.SensitiveKinds(); complete {
			for i, k := range declared {
				if k == config.TypeNumeric {
					kinds[i] = measure.Numeric
				}
			}
		} else if d.Thresholds.T > 0 {
			err := badUsage(
				"raycheck: dataset %q sets thresholds.t, but not every sensitive column declares a type; "+
					"t-closeness needs to know whether values sit on a line or not, and raycheck will not guess",
				d.Name)

			resolve.Fail(err).End()

			return 0, measure.Result{}, err
		}

		g.WithSensitive(names, sensIdx, kinds)
	}

	resolve.End()

	for {
		row, err := src.Next()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			read.Fail(err).Attr("rows", rec.Rows()).End()

			return 0, measure.Result{}, inputFault(err)
		}

		g.Add(row, src.Row())
		rec.Row()
	}

	read.Attr("rows", rec.Rows()).Attr("rows_per_second", rec.Rate()).End()

	rec.Count("bytes_read", src.Bytes())
	rec.Count("rows_read", rec.Rows()-before)

	if skipped := src.Skipped(); skipped > 0 {
		rec.Count("lines_skipped", skipped)
		rec.Warn("blank lines skipped", "lines", skipped, "source", source)
	}

	finish := rec.Span("finish")

	res, err := g.Finish(int(d.Thresholds.K), int(d.Thresholds.L), d.Thresholds.T)

	finish.Fail(err).End()

	if err != nil {
		return 0, measure.Result{}, inputFault(err)
	}

	return rec.Rows() - before, res, nil
}

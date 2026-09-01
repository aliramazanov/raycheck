package report

import (
	"encoding/json"
	"io"
	"strconv"
	"unicode/utf8"

	"github.com/aliramazanov/raycheck/internal/measure"
)

func JSON(w io.Writer, reports []Report, telemetry *Telemetry) error {
	out := jsonReport{
		Datasets:  make([]jsonDataset, 0, len(reports)),
		Strict:    len(reports) > 0 && reports[0].Strict,
		Failed:    Failed(reports),
		Telemetry: telemetry,
	}

	for _, r := range reports {
		out.Datasets = append(out.Datasets, datasetDoc(r))
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(out)
}

func datasetDoc(r Report) jsonDataset {
	res := r.Result

	dd := jsonDataset{
		Name:             r.Name,
		QuasiIdentifiers: names(res.QuasiIdentifiers),
		ThresholdK:       res.Threshold,
		K:                res.K,

		Passed:        r.Passed(),
		KThresholdMet: res.KThresholdMet(),
		Concerns:      concerns(r),

		RowsChecked:    res.Rows,
		RowsSuppressed: res.Suppressed,
		RowsEmptyQI:    res.EmptyQI,
		Coverage:       res.Coverage(),

		Groups:     res.Groups,
		RowsAtRisk: res.RowsAtRisk,
		UniqueRows: res.UniqueRows,

		Risk: jsonRisk{
			Model:   "prosecutor",
			Highest: res.HighestRisk(),
			Average: res.AverageRisk(),
			Lowest:  res.LowestRisk(),
		},

		GroupsBelowThreshold: res.BelowGroups,
		WorstGroups:          make([]jsonGroup, 0, len(res.Below)),
		WorstGroupsTruncated: res.Truncated(),

		Measured:    []string{"k_anonymity"},
		NotMeasured: []string{"t_closeness"},
	}

	for _, d := range res.Diversity {
		dd.Diversity = append(dd.Diversity, jsonDiversity{
			Attribute: d.Attribute, L: d.L, WorstRow: d.WorstRow,
		})
	}

	if len(res.Diversity) > 0 {
		dd.Measured = append(dd.Measured, "l_diversity")
	}

	for _, c := range res.Closeness {
		kind := "categorical"
		if c.Kind == measure.Numeric {
			kind = "numeric"
		}

		dd.Closeness = append(dd.Closeness, jsonCloseness{
			Attribute: c.Attribute, Kind: kind, T: c.T, WorstRow: c.WorstRow,
		})
	}

	if len(res.Closeness) > 0 {
		dd.Measured = append(dd.Measured, "t_closeness")
		dd.NotMeasured = nil
	}

	largest, escaped := groupValues(res, res.Largest)

	dd.LargestGroup = jsonGroup{
		Values:        largest,
		Count:         res.Largest.Count,
		FirstRow:      res.Largest.FirstRow,
		ValuesEscaped: escaped,
	}

	for _, g := range res.Below {
		values, escaped := groupValues(res, g)

		dd.WorstGroups = append(dd.WorstGroups, jsonGroup{
			Values:        values,
			Count:         g.Count,
			FirstRow:      g.FirstRow,
			ValuesEscaped: escaped,
		})
	}

	return dd
}

func groupValues(res measure.Result, g measure.Group) (map[string]string, bool) {
	values := make(map[string]string, len(g.Values))

	var escaped bool

	for i, v := range g.Values {
		if i >= len(res.QuasiIdentifiers) {
			break
		}

		if !utf8.ValidString(v) {
			v = escapeBytes(v)
			escaped = true
		}

		values[safeName(res.QuasiIdentifiers[i])] = v
	}

	return values, escaped
}

func names(columns []string) []string {
	out := make([]string, len(columns))

	for i, c := range columns {
		out[i] = safeName(c)
	}

	return out
}

func safeName(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	return escapeBytes(s)
}

func escapeBytes(s string) string {
	quoted := strconv.Quote(s)

	return quoted[1 : len(quoted)-1]
}

func concerns(r Report) []jsonConcern {
	found := r.Concerns()
	out := make([]jsonConcern, 0, len(found))

	for _, c := range found {
		out = append(out, jsonConcern(c))
	}

	return out
}

func Fault(w io.Writer, err error) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(map[string]any{
		"datasets": []jsonDataset{},
		"failed":   true,
		"error":    err.Error(),
	})
}

package report

import (
	"fmt"

	"github.com/aliramazanov/raycheck/internal/humanize"

	"github.com/aliramazanov/raycheck/internal/measure"
)

type Concern struct {
	Key     string
	Message string
}

const coverageFloor = 0.75

func (r Report) Concerns() []Concern {
	res := r.Result

	var out []Concern

	if res.EmptyQI > 0 {
		msg := fmt.Sprintf(
			"%s carry a blank quasi-identifier, which groups with every other blank and raises k",
			humanize.Count(res.EmptyQI, "row"))

		if !r.SuppressionDeclared {
			msg += ". Declare the marker your anonymizer writes under suppression"
		}

		out = append(out, Concern{Key: "blank_quasi_identifiers", Message: msg})
	}

	for _, c := range res.Closeness {
		if c.Kind != measure.Numeric || c.OffAxis == 0 {
			continue
		}

		out = append(out, Concern{
			Key: "numeric_column_holds_non_numbers",
			Message: fmt.Sprintf(
				"%q is declared numeric, but %s in it cannot be read as a number. Anything not on the "+
					"line is ordered as text instead, so t is only as meaningful as that ordering. "+
					"Correct the type, or the values",
				c.Attribute, humanize.Count(int64(c.OffAxis), "value")),
		})
	}

	if len(res.MarkerColumns) == 1 && res.Suppressed > 0 {
		out = append(out, Concern{
			Key: "marker_matched_one_column",
			Message: fmt.Sprintf(
				"the suppression marker only ever matched in %q, and excluded %s (%s of the file); "+
					"a marker usually appears in every column a row was masked in, so check it is not a real value",
				res.MarkerColumns[0], humanize.Count(res.Suppressed, "row"), percent(1-res.Coverage())),
		})
	}

	if v, ok := uniformValue(res); ok && res.Largest.Count >= res.Threshold {
		out = append(out, Concern{
			Key: "largest_group_is_uniform",
			Message: fmt.Sprintf(
				"the largest group holds %s where every quasi-identifier is %q, which is what an "+
					"undeclared suppression marker looks like; if that is your marker, declare it under suppression",
				humanize.Count(int64(res.Largest.Count), "row"), v),
		})
	}

	if res.Suppressed > 0 && res.Coverage() < coverageFloor {
		out = append(out, Concern{
			Key: "low_coverage",
			Message: fmt.Sprintf(
				"%s of %s were excluded as suppressed, so this verdict covers %s of the file",
				humanize.Commas(res.Suppressed),
				humanize.Count(res.Suppressed+res.Rows, "row"),
				percent(res.Coverage())),
		})
	}

	return out
}

const uniformColumns = 3

func uniformValue(res measure.Result) (string, bool) {
	values := res.Largest.Values

	if len(values) < uniformColumns {
		return "", false
	}

	for _, v := range values[1:] {
		if v != values[0] {
			return "", false
		}
	}

	return values[0], true
}

func (r Report) strictBreach() bool { return r.Strict && len(r.Concerns()) > 0 }

func (r Report) Passed() bool {
	return r.Result.KThresholdMet() && r.Result.DiversityPassed() &&
		r.Result.ClosenessPassed() && !r.strictBreach()
}

func Failed(reports []Report) bool {
	for _, r := range reports {
		if !r.Passed() {
			return true
		}
	}

	return false
}

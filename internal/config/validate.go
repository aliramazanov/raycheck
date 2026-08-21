package config

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

func (c *Config) validate() error {
	if len(c.Datasets) == 0 {
		return ErrNoDatasets
	}

	v := validator{names: make(map[string]int, len(c.Datasets))}

	for i, d := range c.Datasets {
		v.dataset(i, d)
	}

	if len(v.problems) > 0 {
		return &ValidationError{Problems: v.problems}
	}

	return nil
}

type validator struct {
	problems []string
	names    map[string]int
	label    string
}

func (v *validator) addf(format string, args ...any) {
	v.problems = append(v.problems, v.label+": "+fmt.Sprintf(format, args...))
}

func (v *validator) dataset(i int, d Dataset) {
	v.label = fmt.Sprintf("dataset %d", i)
	if d.Name != "" {
		v.label = fmt.Sprintf("dataset %q", d.Name)
	}

	v.name(i, d)

	quasi := v.quasiIdentifiers(d)

	v.sensitive(d, quasi)
	v.thresholds(d)
	v.delimiter(d)
}

func (v *validator) name(i int, d Dataset) {
	switch prev, dup := v.names[d.Name]; {
	case d.Name == "":
		v.addf("name is required")
	case dup:
		v.addf("duplicate name, already defined at index %d", prev)
	default:
		v.names[d.Name] = i
	}
}

func (v *validator) quasiIdentifiers(d Dataset) map[string]bool {
	if len(d.QuasiIdentifiers) == 0 {
		v.addf("quasi_identifiers is required, and raycheck will not guess them")
	}

	seen := make(map[string]bool, len(d.QuasiIdentifiers))

	for _, col := range d.QuasiIdentifiers {
		switch {
		case col == "":
			v.addf("quasi_identifiers contains an empty column name")
		case seen[col]:
			v.addf("quasi_identifiers lists %q twice", col)
		default:
			seen[col] = true
		}
	}

	return seen
}

func (v *validator) sensitive(d Dataset, quasi map[string]bool) {
	seen := make(map[string]bool, len(d.Sensitive))

	for _, s := range d.Sensitive {
		switch {
		case s.Name == "":
			v.addf("sensitive contains an entry with no name")
		case seen[s.Name]:
			v.addf("sensitive lists %q twice", s.Name)
		default:
			seen[s.Name] = true
		}

		if quasi[s.Name] {
			v.addf("%q is listed as both a quasi-identifier and sensitive", s.Name)
		}

		if !validSensitiveType(s.Type) {
			v.addf("sensitive %q: type %q is not one of %s", s.Name, s.Type, joinTypes())
		}
	}
}

func (v *validator) thresholds(d Dataset) {
	if d.Thresholds.K < 1 {
		v.addf("thresholds.k must be at least 1, got %d", d.Thresholds.K)
	}

	switch {
	case d.Thresholds.T < 0 || d.Thresholds.T > 1:
		if d.Thresholds.T != 0 {
			v.addf("thresholds.t is a distance between 0 and 1, got %v", d.Thresholds.T)
		}
	case d.Thresholds.T > 0 && len(d.Sensitive) == 0:
		v.addf("thresholds.t is set but no sensitive columns are declared, and t-closeness is measured over them")
	}

	switch {
	case d.Thresholds.L < 0:
		v.addf("thresholds.l cannot be negative, got %d", d.Thresholds.L)
	case d.Thresholds.L > 0 && len(d.Sensitive) == 0:
		v.addf("thresholds.l is set but no sensitive columns are declared, and l-diversity is measured over them")
	}
}

func (v *validator) delimiter(d Dataset) {
	switch n := utf8.RuneCountInString(d.Delimiter); {
	case n > 1:
		v.addf("delimiter must be a single character, got %q", d.Delimiter)
	case n == 1 && !validDelim(d.Delim()):
		v.addf("delimiter %q cannot separate CSV fields", d.Delimiter)
	}
}

func validDelim(r rune) bool {
	switch r {
	case 0, '"', '\r', '\n', utf8.RuneError:
		return false
	}

	return utf8.ValidRune(r)
}

func validSensitiveType(t SensitiveType) bool {
	return t == TypeUnspecified || slices.Contains(sensitiveTypes, t)
}

func joinTypes() string {
	names := make([]string, len(sensitiveTypes))

	for i, t := range sensitiveTypes {
		names[i] = string(t)
	}

	return strings.Join(names, ", ")
}

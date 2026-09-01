package config

import (
	"strings"
	"testing"
	"time"
)

func parseWithin(t *testing.T, d time.Duration, yaml string) (*Config, error, bool) {
	t.Helper()

	type outcome struct {
		cfg *Config
		err error
	}

	done := make(chan outcome, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- outcome{err: errPanic(r)}
			}
		}()

		cfg, err := Parse([]byte(yaml))
		done <- outcome{cfg, err}
	}()

	select {
	case o := <-done:
		return o.cfg, o.err, true
	case <-time.After(d):
		return nil, nil, false
	}
}

type panicErr struct{ v any }

func (e panicErr) Error() string {
	return "panic: " + strings.TrimSpace(strings.Split(sprint(e.v), "\n")[0])
}

func errPanic(v any) error { return panicErr{v} }

func sprint(v any) string {
	if s, ok := v.(string); ok {
		return s
	}

	if e, ok := v.(error); ok {
		return e.Error()
	}

	return "unknown"
}

func TestAdversarialRecursiveAnchor(t *testing.T) {
	cases := map[string]string{
		"self referencing sequence": `
datasets: &d
  - name: ${A:-x}
    quasi_identifiers: [*d]
    thresholds:
      k: 1
`, "self referencing mapping": `
datasets:
  - &d
    name: ${A:-x}
    quasi_identifiers: [x]
    thresholds:
      k: 1
    sensitive: [*d]
`}

	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			_, err, finished := parseWithin(t, 10*time.Second, y)

			if !finished {
				t.Fatal("parsing never returned")
			}

			t.Logf("%v", err)
		})
	}
}

func TestAdversarialMergeKey(t *testing.T) {
	_, err, finished := parseWithin(t, 10*time.Second, `
defaults: &d
  bogus: 1
datasets:
  - <<: *d
    name: a
    quasi_identifiers: [x]
    thresholds:
      k: 1
`)

	if !finished {
		t.Fatal("parsing never returned")
	}

	t.Logf("%v", err)

	if err == nil {
		t.Error("a merge key carried an unknown field through")
	}
}

func TestAdversarialAliasAmplification(t *testing.T) {
	y := `
base: &b [a,a,a,a,a,a,a,a,a,a]
datasets:
  - name: x
    quasi_identifiers: [*b, *b, *b, *b, *b, *b, *b, *b, *b, *b]
    thresholds:
      k: 1
`

	_, err, finished := parseWithin(t, 10*time.Second, y)

	if !finished {
		t.Fatal("parsing never returned")
	}

	t.Logf("%v", err)
}

func TestAdversarialCustomUnmarshallerShapes(t *testing.T) {
	tests := map[string]struct {
		yaml    string
		wantErr string
	}{
		"thresholds is null":     {yaml: "thresholds: null\n", wantErr: "at least 1"},
		"thresholds is a scalar": {yaml: "thresholds: 5\n", wantErr: "into config.Thresholds"},
		"thresholds is a list":   {yaml: "thresholds: [5]\n", wantErr: "into config.Thresholds"},
		"thresholds empty":       {yaml: "thresholds: {}\n", wantErr: "at least 1"},
		"unknown threshold key":  {yaml: "thresholds:\n      k: 1\n      j: 2\n", wantErr: "field j not found"},
		"duplicate k":            {yaml: "thresholds:\n      k: 5\n      k: 1\n", wantErr: "already defined"},
		"k is null":              {yaml: "thresholds:\n      k: null\n", wantErr: "at least 1"},
		"k is a list":            {yaml: "thresholds:\n      k: [1]\n", wantErr: "whole number"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(`
datasets:
  - name: a
    quasi_identifiers: [x]
    ` +
				tt.yaml))

			if err == nil {
				t.Fatalf("accepted %q", tt.yaml)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("want an error mentioning %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAdversarialSensitiveShapes(t *testing.T) {
	tests := map[string]struct {
		yaml    string
		wantErr string
	}{
		"null entry":        {yaml: "sensitive: [null]\n", wantErr: "is empty"},
		"mapping with null": {yaml: "sensitive:\n      - name: null\n", wantErr: "not a column name"},
		"duplicate name":    {yaml: "sensitive:\n      - name: a\n        name: b\n", wantErr: "already defined"},
		"nested mapping":    {yaml: "sensitive:\n      - name: {a: b}\n", wantErr: "name"},
		"boolean entry":     {yaml: "sensitive: [true]\n", wantErr: "not a column name"},
		"not a list":        {yaml: "sensitive: salary\n", wantErr: "expected a list"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(`
datasets:
  - name: a
    quasi_identifiers: [x]
    thresholds:
      k: 1
    ` +
				tt.yaml))

			if err == nil {
				t.Fatalf("accepted %q", tt.yaml)
			}

			t.Logf("%v", err)

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("want an error mentioning %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAdversarialQuasiIdentifierShapes(t *testing.T) {
	for _, y := range []string{"quasi_identifiers: [[a]]", "quasi_identifiers: {a: b}", "quasi_identifiers: [null]"} {
		t.Run(y, func(t *testing.T) {
			cfg, err := Parse([]byte(`
datasets:
  - name: a
    ` +
				y + "\n    thresholds:\n      k: 1\n"))

			if err != nil {
				t.Logf("refused: %v", err)

				return
			}

			t.Errorf("accepted, giving %#v", cfg.Datasets[0].QuasiIdentifiers)
		})
	}
}

func TestThresholdTypeErrorNamesTheType(t *testing.T) {
	tests := map[string]string{
		`"5"`:    `the string "5"`,
		"2.9":    "the decimal 2.9",
		"true":   "the boolean true",
		"[5]":    "a list",
		"{a: 1}": "a mapping",
		"1e3":    "the decimal 1e3",
	}

	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			_, err := Parse([]byte(`
datasets:
  - name: a
    quasi_identifiers: [x]
    thresholds:
      k: ` +
				value + "\n"))

			if err == nil {
				t.Fatalf("accepted k: %s", value)
			}
			if !strings.Contains(err.Error(), want) {
				t.Errorf("want an error naming %q, got %v", want, err)
			}
		})
	}
}

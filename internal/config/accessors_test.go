package config

import (
	"strings"
	"testing"
)

func load(t *testing.T, body string) *Config {
	t.Helper()

	cfg, err := Parse([]byte(body))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return cfg
}

func TestSensitiveNamesAndKinds(t *testing.T) {
	t.Parallel()

	cfg := load(t, `
datasets:
  - name: a
    quasi_identifiers: [x]
    sensitive:
      - name: salary
        type: numeric
      - name: diagnosis
        type: categorical
    thresholds:
      k: 1
`)

	d := cfg.Datasets[0]

	if got := d.SensitiveNames(); len(got) != 2 || got[0] != "salary" || got[1] != "diagnosis" {
		t.Errorf("want the declared order, got %v", got)
	}

	kinds, complete := d.SensitiveKinds()

	if !complete {
		t.Error("every column declared a type, so this should be complete")
	}

	if kinds[0] != TypeNumeric || kinds[1] != TypeCategorical {
		t.Errorf("want numeric then categorical, got %v", kinds)
	}
}

func TestSensitiveKindsIncomplete(t *testing.T) {
	t.Parallel()

	cfg := load(t, `
datasets:
  - name: a
    quasi_identifiers: [x]
    sensitive:
      - salary
      - name: diagnosis
        type: categorical
    thresholds:
      k: 1
`)

	if _, complete := cfg.Datasets[0].SensitiveKinds(); complete {
		t.Error("one column has no type, so this must not be complete")
	}

	none := load(t, `
datasets:
  - name: a
    quasi_identifiers: [x]
    thresholds:
      k: 1
`)

	if _, complete := none.Datasets[0].SensitiveKinds(); complete {
		t.Error("no sensitive columns means nothing was declared")
	}
}

func TestDatasetLookup(t *testing.T) {
	t.Parallel()

	cfg := load(t, `
datasets:
  - name: a
    quasi_identifiers: [x]
    thresholds: {k: 1}
  - name: b
    quasi_identifiers: [y]
    thresholds: {k: 2}
`)

	if d, ok := cfg.Dataset("b"); !ok || d.Thresholds.K != 2 {
		t.Errorf("want dataset b with k=2, got %+v ok=%v", d, ok)
	}

	if _, ok := cfg.Dataset("absent"); ok {
		t.Error("a name that is not there must not be found")
	}
}

func TestDelimDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	cfg := load(t, datasetYAML(fieldQI, fieldK))

	if got := cfg.Datasets[0].Delim(); got != ',' {
		t.Errorf("want a comma by default, got %q", got)
	}

	semi := load(t, datasetYAML(fieldQI, "delimiter: \";\"", fieldK))

	if got := semi.Datasets[0].Delim(); got != ';' {
		t.Errorf("want a semicolon, got %q", got)
	}
}

func TestThresholdValidation(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		yaml    string
		wantErr string
	}{
		"l without sensitive columns": {
			yaml:    "    thresholds:\n      k: 1\n      l: 2\n",
			wantErr: "no sensitive columns are declared",
		},
		"t without sensitive columns": {
			yaml:    "    thresholds:\n      k: 1\n      t: 0.2\n",
			wantErr: "no sensitive columns are declared",
		},
		"t above one": {
			yaml:    "    sensitive: [s]\n    thresholds:\n      k: 1\n      t: 1.5\n",
			wantErr: "distance between 0 and 1",
		},
		"t below zero": {
			yaml:    "    sensitive: [s]\n    thresholds:\n      k: 1\n      t: -0.5\n",
			wantErr: "distance between 0 and 1",
		},
		"negative l": {
			yaml:    "    sensitive: [s]\n    thresholds:\n      k: 1\n      l: -1\n",
			wantErr: "cannot be negative",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

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

func TestThresholdsTogetherAreAccepted(t *testing.T) {
	t.Parallel()

	cfg := load(t, `
datasets:
  - name: a
    quasi_identifiers: [x]
    sensitive:
      - name: s
        type: numeric
    thresholds:
      k: 5
      l: 2
      t: 0.25
`)

	th := cfg.Datasets[0].Thresholds

	if th.K != 5 || th.L != 2 || th.T != 0.25 {
		t.Errorf("unexpected thresholds: %+v", th)
	}
}

func TestValidationErrorMessages(t *testing.T) {
	t.Parallel()

	single := &ValidationError{Problems: []string{"only one"}}

	if got := single.Error(); got != "config: only one" {
		t.Errorf("a single problem should read as one line, got %q", got)
	}

	many := &ValidationError{Problems: []string{"first", "second"}}

	if got := many.Error(); !strings.Contains(got, "2 problems") || !strings.Contains(got, "second") {
		t.Errorf("several problems should all appear, got %q", got)
	}

	nf := &NotFoundError{Path: "qi.yaml"}

	if got := nf.Error(); got != "config: no qi.yaml" {
		t.Errorf("unexpected message %q", got)
	}
}

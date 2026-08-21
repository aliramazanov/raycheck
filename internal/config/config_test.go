package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		yaml    string
		want    int
		wantErr string
	}{
		"minimal": {yaml: minimal, want: 1},
		"all fields": {
			yaml: `
datasets:
  - name: seed
    source: seed.csv
    quasi_identifiers: [birth_date, postcode, gender]
    sensitive:
      - salary
      - name: bonus
        type: numeric
    suppression: "*"
    delimiter: ";"
    thresholds:
      k: 5
`,
			want: 1,
		},
		"multiple datasets": {
			yaml: `
datasets:
  - name: a
    quasi_identifiers: [x]
    thresholds: {k: 2}
  - name: b
    quasi_identifiers: [y]
    thresholds: {k: 3}
`,
			want: 2,
		},

		"empty datasets list": {yaml: `
datasets: []
`, wantErr: "no datasets defined"},
		"missing datasets key": {yaml: "other: 1\n", wantErr: "field other not found"},
		"malformed yaml": {yaml: `
datasets:
  - name: [unclosed
`, wantErr: "parsing config"},
		"unknown field": {
			yaml: datasetYAML(fieldQI, "bogus: 1", fieldK), wantErr: "field bogus not found",
		},
		"unknown sensitive field": {
			yaml: `
datasets:
  - name: a
    quasi_identifiers: [x]
    sensitive:
      - name: s
        bogus: 1
    thresholds: {k: 1}
`, wantErr: "field bogus not found",
		},

		"missing name": {
			yaml: `
datasets:
  - quasi_identifiers: [x]
    thresholds: {k: 1}
`, wantErr: "name is required",
		},
		"duplicate dataset names": {
			yaml: `
datasets:
  - name: a
    quasi_identifiers: [x]
    thresholds: {k: 1}
  - name: a
    quasi_identifiers: [y]
    thresholds: {k: 1}
`, wantErr: "duplicate name",
		},
		"no quasi identifiers": {
			yaml: datasetYAML(fieldK), wantErr: "will not guess",
		},
		"duplicate quasi identifier": {
			yaml: datasetYAML("quasi_identifiers: [x, x]", fieldK), wantErr: `lists "x" twice`,
		},
		"column both qi and sensitive": {
			yaml: datasetYAML(fieldQI, "sensitive: [x]", fieldK), wantErr: "both a quasi-identifier and sensitive",
		},
		"missing k": {
			yaml: datasetYAML(fieldQI), wantErr: "thresholds.k must be at least 1",
		},
		"zero k": {
			yaml: datasetYAML(fieldQI, "thresholds: {k: 0}"), wantErr: "thresholds.k must be at least 1",
		},
		"bad sensitive type": {
			yaml: `
datasets:
  - name: a
    quasi_identifiers: [x]
    sensitive:
      - name: s
        type: fuzzy
    thresholds: {k: 1}
`, wantErr: "numeric, categorical",
		},
		"multi-rune delimiter": {
			yaml: datasetYAML(fieldQI, "delimiter: \"||\"", fieldK), wantErr: "single character",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Parse([]byte(tt.yaml))

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %q", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(cfg.Datasets) != tt.want {
				t.Fatalf("want %d datasets, got %d", tt.want, len(cfg.Datasets))
			}
		})
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte(datasetYAML("quasi_identifiers: [x, x]", "sensitive: [x]", "thresholds: {k: 0}")))

	if err == nil {
		t.Fatal("want error, got nil")
	}

	var ve *ValidationError

	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T", err)
	}

	if len(ve.Problems) != 3 {
		t.Fatalf("want 3 problems, got %d: %v", len(ve.Problems), ve.Problems)
	}
}

func TestSensitiveShorthandAndMapping(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]byte(`
datasets:
  - name: a
    quasi_identifiers: [x]
    sensitive:
      - salary
      - name: bonus
        type: numeric
    thresholds: {k: 1}
`))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := cfg.Datasets[0].Sensitive

	if len(got) != 2 {
		t.Fatalf("want 2 sensitive entries, got %d", len(got))
	}

	if got[0].Name != "salary" || got[0].Type != TypeUnspecified {
		t.Errorf("shorthand entry: got %+v", got[0])
	}

	if got[1].Name != "bonus" || got[1].Type != TypeNumeric {
		t.Errorf("mapping entry: got %+v", got[1])
	}
}

func TestSuppressionDistinguishesUnsetFromEmpty(t *testing.T) {
	t.Parallel()

	unset, err := Parse([]byte(minimal))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if unset.Datasets[0].Suppression != nil {
		t.Errorf("want nil suppression when undeclared, got %q", *unset.Datasets[0].Suppression)
	}

	empty, err := Parse([]byte(`
datasets:
  - name: seed
    quasi_identifiers: [x]
    suppression: ""
    thresholds: {k: 5}
`))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if empty.Datasets[0].Suppression == nil {
		t.Fatal("want non-nil suppression when declared empty")
	}

	if *empty.Datasets[0].Suppression != "" {
		t.Errorf("want empty suppression marker, got %q", *empty.Datasets[0].Suppression)
	}
}

func TestExpandEnv(t *testing.T) {
	t.Setenv("RAYCHECK_TEST_K", "9")

	cfg, err := Parse([]byte(`
datasets:
  - name: ${RAYCHECK_TEST_MISSING:-seed}
    quasi_identifiers: [x]
    thresholds:
      k: ${RAYCHECK_TEST_K}
`))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Datasets[0].Name != "seed" {
		t.Errorf("want default substituted, got %q", cfg.Datasets[0].Name)
	}

	if cfg.Datasets[0].Thresholds.K != 9 {
		t.Errorf("want k=9, got %d", cfg.Datasets[0].Thresholds.K)
	}
}

func TestExpandEnvMissingWithoutDefaultFails(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte(`
datasets:
  - name: ${RAYCHECK_TEST_ABSENT}
`))

	if err == nil || !strings.Contains(err.Error(), "not set and has no default") {
		t.Fatalf("want unset-variable error, got %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	_, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))

	var nf *NotFoundError

	if !errors.As(err, &nf) {
		t.Fatalf("want *NotFoundError, got %T: %v", err, err)
	}

	if !os.IsNotExist(errors.Unwrap(nf)) {
		t.Error("want the wrapped error to report non-existence")
	}
}

func TestSourcePathResolvesAgainstConfigDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "qi.yaml")

	if err := os.WriteFile(path, []byte(datasetYAML(fieldSource, fieldQI, fieldK)), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := cfg.SourcePath(cfg.Datasets[0]), filepath.Join(dir, "seed.csv"); got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

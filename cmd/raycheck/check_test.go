package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

const oneDataset = `
datasets:
  - name: seed
    source: seed.csv
    quasi_identifiers: [birth_date, postcode]
    thresholds:
      k: 3
`

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	code := runCheck(args, &stdout, &stderr)

	return code, stdout.String(), stderr.String()
}

func TestExamplesShipHonest(t *testing.T) {
	t.Parallel()

	cfg := filepath.Join("..", "..", "examples", "qi.yaml")

	if code, out, errOut := run(
		t,
		"--config",
		cfg,
		filepath.Join("..", "..", "examples", "safe.csv"),
	); code != exitOK {
		t.Errorf("safe example should pass, got %d\n%s%s", code, out, errOut)
	}

	code, out, errOut := run(
		t,
		"--config",
		cfg,
		filepath.Join("..", "..", "examples", "leaky.csv"),
	)

	if code != exitBreached {
		t.Errorf("leaky example should breach the gate, got %d\n%s%s", code, out, errOut)
	}
	if !strings.Contains(out, "This data is not anonymous") {
		t.Errorf("want the verdict spelled out, got:\n%s", out)
	}
}

func TestPassAndBreach(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", tight)

	if code, _, errOut := run(t, "--config", cfg); code != exitOK {
		t.Errorf("want exit 0, got %d: %s", code, errOut)
	}

	loose := write(t, dir, "loose.csv", spread)

	if code, _, _ := run(t, "--config", cfg, loose); code != exitBreached {
		t.Errorf("want exit 1, got %d", code)
	}
}

func TestConfigRequired(t *testing.T) {
	t.Parallel()

	code, _, errOut := run(t)

	if code != exitBadUsage {
		t.Fatalf("want exit 2, got %d", code)
	}

	if !strings.Contains(errOut, configEnv) {
		t.Errorf("the error should mention the environment variable, got %q", errOut)
	}
}

func TestConfigFromEnvironment(t *testing.T) {
	dir := t.TempDir()

	write(t, dir, "seed.csv", tight)
	t.Setenv(configEnv, write(t, dir, "qi.yaml", oneDataset))

	if code, _, errOut := run(t); code != exitOK {
		t.Errorf("want exit 0, got %d: %s", code, errOut)
	}
}

func TestBadConfigIsUsageNotInput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(
		t,
		dir,
		"qi.yaml", datasetYAML(fieldQI, "bogus: 1", fieldK))

	if code, _, _ := run(t, "--config", cfg); code != exitBadUsage {
		t.Errorf("want exit 2, got %d", code)
	}
}

func TestMissingQuasiIdentifierIsFatal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", "birth_date,zip\n1984,110\n")

	code, _, errOut := run(t, "--config", cfg)

	if code != exitBadUsage {
		t.Fatalf("want exit 2, got %d", code)
	}

	for _, want := range []string{"postcode", "available", "zip"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("error should mention %q, got %q", want, errOut)
		}
	}
}

func TestInputFaultsAreDistinctFromBreaches(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	tests := map[string]string{
		"absent file": "",
		"ragged row":  "birth_date,postcode\n1984,110\n1990\n",
		"empty file":  "",
		"header only": "birth_date,postcode\n",
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "in.csv")

			if name != "absent file" {
				if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if code, _, errOut := run(t, "--config", cfg, path); code != exitInput {
				t.Errorf("want exit 3, got %d: %s", code, errOut)
			}
		})
	}
}

func TestJSONOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", spread)

	code, out, _ := run(t, "--config", cfg, "--json")

	if code != exitBreached {
		t.Fatalf("want exit 1, got %d", code)
	}

	var doc struct {
		Failed   bool `json:"failed"`
		Datasets []struct {
			K int `json:"k"`
		} `json:"datasets"`
	}

	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}

	if !doc.Failed || doc.Datasets[0].K != 1 {
		t.Errorf("unexpected document: %s", out)
	}
}

func TestLogFormatJSON(t *testing.T) {
	t.Parallel()

	code, _, errOut := run(t, "--config", filepath.Join(t.TempDir(), "absent.yaml"), "--log-format", "json")

	if code != exitBadUsage {
		t.Fatalf("want exit 2, got %d", code)
	}

	var doc map[string]any

	if err := json.Unmarshal([]byte(errOut), &doc); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\n%s", err, errOut)
	}

	if doc["level"] != "ERROR" {
		t.Errorf("unexpected log document: %s", errOut)
	}
}

func TestDatasetSelection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: good
    source: good.csv
    quasi_identifiers: [postcode]
    thresholds: {k: 2}
  - name: bad
    source: bad.csv
    quasi_identifiers: [postcode]
    thresholds: {k: 2}
`)

	write(t, dir, "good.csv", "postcode\n110\n110\n")
	write(t, dir, "bad.csv", "postcode\n110\n120\n")

	if code, _, errOut := run(t, "--config", cfg, "--dataset", "good"); code != exitOK {
		t.Errorf("want exit 0 for the good dataset, got %d: %s", code, errOut)
	}

	if code, _, _ := run(t, "--config", cfg, "--dataset", "bad"); code != exitBreached {
		t.Errorf("want exit 1 for the bad dataset, got %d", code)
	}

	if code, _, _ := run(t, "--config", cfg); code != exitBreached {
		t.Errorf("one failing dataset should fail the run, got %d", code)
	}

	code, _, errOut := run(t, "--config", cfg, "--dataset", "absent")

	if code != exitBadUsage {
		t.Fatalf("want exit 2 for an unknown dataset, got %d", code)
	}

	if !strings.Contains(errOut, "good") {
		t.Errorf("the error should list the declared datasets, got %q", errOut)
	}
}

func TestPositionalOverrideNeedsOneDataset(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: a
    source: a.csv
    quasi_identifiers: [postcode]
    thresholds: {k: 1}
  - name: b
    source: b.csv
    quasi_identifiers: [postcode]
    thresholds: {k: 1}
`)

	csv := write(t, dir, "a.csv", "postcode\n110\n")

	if code, _, _ := run(t, "--config", cfg, csv); code != exitBadUsage {
		t.Errorf("want exit 2, got %d", code)
	}

	if code, _, errOut := run(t, "--config", cfg, "--dataset", "a", csv); code != exitOK {
		t.Errorf("want exit 0 once a dataset is named, got %d: %s", code, errOut)
	}
}

func TestStrictFailsOnUndeclaredBlanks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", "birth_date,postcode\n1984,\n1984,\n1984,\n")

	if code, _, errOut := run(t, "--config", cfg); code != exitOK {
		t.Fatalf("without --strict the blanks group literally and k=3 passes, got %d: %s", code, errOut)
	}

	code, out, _ := run(t, "--config", cfg, "--strict")

	if code != exitBreached {
		t.Fatalf("want exit 1 under --strict, got %d", code)
	}

	if !strings.Contains(out, "not an all clear under --strict") {
		t.Errorf("want the reason stated, got:\n%s", out)
	}
}

func TestBadFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	if code, _, _ := run(t, "--config", cfg, "--format", "yaml"); code != exitBadUsage {
		t.Errorf("want exit 2, got %d", code)
	}
}

func TestStdin(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	r, w, err := os.Pipe()

	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_, _ = w.WriteString(tight)
		_ = w.Close()
	}()

	saved := os.Stdin
	os.Stdin = r

	defer func() { os.Stdin = saved }()

	if code, _, errOut := run(t, "--config", cfg, "-"); code != exitOK {
		t.Errorf("want exit 0 reading standard input, got %d: %s", code, errOut)
	}
}

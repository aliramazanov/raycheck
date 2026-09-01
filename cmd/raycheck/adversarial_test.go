package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aliramazanov/raycheck/internal/config"
)

func TestAdversarialBadDelimiterExitCode(t *testing.T) {
	dir := t.TempDir()
	cfg := write(
		t,
		dir,
		"qi.yaml",
		datasetYAML(fieldSource, fieldQI, "delimiter: \"\\\"\"", fieldK))

	write(t, dir, "seed.csv", "x\n1\n")

	code, _, errOut := run(t, "--config", cfg)

	t.Logf("exit=%d %s", code, errOut)

	if code != exitBadUsage {
		t.Errorf("a bad delimiter is a config fault, want exit %d, got %d", exitBadUsage, code)
	}
}

func TestAdversarialFaultDoesNotUnwrap(t *testing.T) {
	inner := &config.NotFoundError{Path: "x"}
	f := fault{code: exitBadUsage, err: inner}

	if _, ok := errors.AsType[*config.NotFoundError](error(f)); !ok {
		t.Error("fault does not unwrap to the error it carries")
	}
}

func TestAdversarialPartialTextOutputBeforeFailure(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: good
    source: good.csv
    quasi_identifiers: [postcode]
    thresholds: {k: 2}
  - name: absent
    source: absent.csv
    quasi_identifiers: [postcode]
    thresholds: {k: 2}
`)

	write(t, dir, "good.csv", "postcode\n110\n110\n")

	code, out, errOut := run(t, "--config", cfg)

	t.Logf("exit=%d\nstdout:\n%s\nstderr: %s", code, out, errOut)

	if code != exitInput {
		t.Errorf("want exit 3, got %d", code)
	}

	if strings.Contains(out, "no row can be singled out") {
		t.Error("a run that could not read every dataset printed an all clear for one of them")
	}
}

func TestAdversarialJSONEmitsNothingOnFailure(t *testing.T) {
	dir := t.TempDir()
	cfg := write(
		t,
		dir,
		"qi.yaml", datasetYAML("source: absent.csv", fieldQI, fieldK))

	code, out, _ := run(t, "--config", cfg, "--json")

	t.Logf("exit=%d stdout=%q", code, out)

	if out == "" {
		t.Error("--json produced no document on failure; a consumer sees an empty stream")
	}
}

func TestAdversarialStrictRejectsMajoritySuppressedPass(t *testing.T) {
	dir := t.TempDir()
	cfg := write(
		t,
		dir,
		"qi.yaml",
		datasetYAML(fieldSource, fieldQI, "suppression: \"*\"", "thresholds: {k: 5}"))

	var b strings.Builder

	b.WriteString("x\n")

	for range 90 {
		b.WriteString("*\n")
	}

	for range 10 {
		b.WriteString("same\n")
	}

	write(t, dir, "seed.csv", b.String())

	code, out, _ := run(t, "--config", cfg, "--strict")

	t.Logf("exit=%d\n%s", code, out)

	if code == exitOK {
		t.Error("--strict accepted a pass that excluded 90 of 100 rows")
	}
}

func TestAdversarialEmptyColumnNameInConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := write(
		t,
		dir,
		"qi.yaml", datasetYAML(fieldSource, "quasi_identifiers: [\"\"]", fieldK))

	write(t, dir, "seed.csv", ",b\n1,2\n")

	code, out, errOut := run(t, "--config", cfg)

	t.Logf("exit=%d out=%s err=%s", code, out, errOut)

	if code != exitBadUsage {
		t.Errorf("an empty column name should be rejected, got exit %d", code)
	}
}

func TestAdversarialConfigIsDirectory(t *testing.T) {
	code, _, errOut := run(t, "--config", t.TempDir())

	t.Logf("exit=%d %s", code, errOut)

	if code != exitBadUsage {
		t.Errorf("want exit 2, got %d", code)
	}
}

func TestAdversarialSourceIsDirectory(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	if err := os.Mkdir(filepath.Join(dir, "seed.csv"), 0o750); err != nil {
		t.Fatal(err)
	}

	code, _, errOut := run(t, "--config", cfg)

	t.Logf("exit=%d %s", code, errOut)

	if code != exitInput {
		t.Errorf("want exit 3, got %d", code)
	}
}

func TestAdversarialThresholdExceedsRowCount(t *testing.T) {
	dir := t.TempDir()

	cfg := write(
		t,
		dir,
		"qi.yaml",
		datasetYAML(fieldSource, fieldQI, "thresholds: {k: 1000}"))

	write(t, dir, "seed.csv", "x\n1\n1\n")

	code, out, _ := run(t, "--config", cfg)

	t.Logf("exit=%d\n%s", code, out)

	if code != exitBreached {
		t.Errorf("want exit 1, got %d", code)
	}
}

func TestAdversarialJSONIsValidWhenEverythingPasses(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", tight)

	code, out, _ := run(t, "--config", cfg, "--json")

	if code != exitOK {
		t.Fatalf("want exit 0, got %d", code)
	}

	var doc map[string]any

	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}

	if doc["failed"] != false {
		t.Errorf("want failed=false, got %v", doc["failed"])
	}
}

func TestAdversarialFlagsAfterPositional(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)
	csv := write(t, dir, "seed.csv", spread)

	code, out, errOut := run(t, "--config", cfg, csv, "--json")

	t.Logf("exit=%d stderr=%s", code, errOut)

	if code != exitBreached {
		t.Fatalf("want exit 1, got %d", code)
	}

	var doc map[string]any

	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("want a JSON document, got %q", out)
	}
}

func TestAdversarialDoubleDashEndsFlags(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)
	csv := write(t, dir, "seed.csv", tight)

	if code, _, errOut := run(t, "--config", cfg, "--", csv); code != exitOK {
		t.Errorf("want exit 0, got %d: %s", code, errOut)
	}
}

func TestAdversarialFilenameLookingLikeAFlag(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "-weird.csv", tight)

	code, _, errOut := run(t, "--config", cfg, "--", filepath.Join(dir, "-weird.csv"))

	if code != exitOK {
		t.Errorf("want exit 0, got %d: %s", code, errOut)
	}
}

func TestAdversarialDeclaredTypeReachesTheMeasure(t *testing.T) {
	dir := t.TempDir()

	write(t, dir, "seed.csv", "q,s\na,1\na,2\nb,1\nb,100\nc,2\nc,100\n")

	tOf := func(kind string) float64 {
		cfg := write(t, dir, kind+".yaml", datasetYAML(fieldSource, "quasi_identifiers: [q]",
			"sensitive:\n      - name: s\n        type: "+kind,
			"thresholds: {k: 1, t: 0.99}"))

		code, out, errOut := run(t, "--config", cfg, "--json")

		if code != exitOK {
			t.Fatalf("%s: want exit 0, got %d: %s", kind, code, errOut)
		}

		var doc struct {
			Datasets []struct {
				Closeness []struct {
					T    float64 `json:"t"`
					Kind string  `json:"kind"`
				} `json:"t_closeness"`
			} `json:"datasets"`
		}

		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}

		got := doc.Datasets[0].Closeness[0]

		if got.Kind != kind {
			t.Errorf("declared %q, the document reports %q", kind, got.Kind)
		}

		return got.T
	}

	num, cat := tOf("numeric"), tOf("categorical")

	if num == cat {
		t.Fatalf("the declared type never reached the measure: both distances are %v", num)
	}
}

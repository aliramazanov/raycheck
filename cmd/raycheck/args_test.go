package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdversarialRelativeDashFilename(t *testing.T) {
	dir := t.TempDir()

	write(t, dir, "qi.yaml", oneDataset)
	write(t, dir, "-weird.csv", tight)

	wd, err := os.Getwd()

	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	defer func() { _ = os.Chdir(wd) }()

	code, _, errOut := run(t, "--config", "qi.yaml", "--", "-weird.csv")

	t.Logf("exit=%d %s", code, errOut)

	if code != exitOK {
		t.Errorf("want exit 0, got %d: %s", code, errOut)
	}
}

func TestAdversarialRepeatedFlag(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", tight)

	if code, _, errOut := run(t, "--config", "/nonexistent.yaml", "--config", cfg); code != exitOK {
		t.Errorf("the last value should win, got exit %d: %s", code, errOut)
	}
}

func TestAdversarialEqualsFormFlag(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)
	csv := write(t, dir, "seed.csv", tight)

	if code, _, errOut := run(t, "--config="+cfg, csv); code != exitOK {
		t.Errorf("want exit 0, got %d: %s", code, errOut)
	}
}

func TestAdversarialDanglingFlagValue(t *testing.T) {
	if code, _, errOut := run(t, "--config"); code != exitBadUsage {
		t.Errorf("want exit 2, got %d: %s", code, errOut)
	}
}

func TestAdversarialUnknownFlag(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)
	csv := write(t, dir, "seed.csv", tight)

	code, _, errOut := run(t, "--config", cfg, "--bogus", csv)

	t.Logf("exit=%d %s", code, errOut)

	if code != exitBadUsage {
		t.Errorf("want exit 2, got %d", code)
	}
}

func TestAdversarialStdinTwice(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: a
    source: "-"
    quasi_identifiers: [birth_date]
    thresholds:
      k: 1
  - name: b
    source: "-"
    quasi_identifiers: [birth_date]
    thresholds:
      k: 1
`)

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

	code, _, errOut := run(t, "--config", cfg)

	t.Logf("exit=%d %s", code, errOut)

	if code == exitOK {
		t.Error("the second dataset read nothing and the run still passed")
	}
}

func TestAdversarialYAMLScalarColumnNames(t *testing.T) {
	plain := []string{"no", "yes", "on", "off", "y", "n", "Yes", "NO"}

	for _, name := range plain {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: a
    source: seed.csv
    quasi_identifiers: [`+
				name+"]\n    thresholds:\n      k: 1\n")

			write(t, dir, "seed.csv", name+"\nvalue\n")

			if code, _, errOut := run(t, "--config", cfg); code != exitOK {
				t.Errorf("a column named %q was not matched: %s", name, errOut)
			}
		})
	}

	for _, name := range []string{"null", "true", "false", "True", "~"} {
		t.Run("unquoted "+name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: a
    source: seed.csv
    quasi_identifiers: [`+
				name+"]\n    thresholds:\n      k: 1\n")

			write(t, dir, "seed.csv", name+"\nvalue\n")

			code, _, errOut := run(t, "--config", cfg)

			t.Logf("%s", strings.TrimSpace(errOut))

			if code != exitBadUsage {
				t.Errorf("want exit 2, got %d", code)
			}

			if !strings.Contains(errOut, "quote it") {
				t.Errorf("the error should name the fix, got %q", errOut)
			}
		})

		t.Run("quoted "+name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: a
    source: seed.csv
    quasi_identifiers: ["`+
				name+"\"]\n    thresholds:\n      k: 1\n")

			write(t, dir, "seed.csv", name+"\nvalue\n")

			if code, _, errOut := run(t, "--config", cfg); code != exitOK {
				t.Errorf("the quoted form should work, got exit %d: %s", code, errOut)
			}
		})
	}
}

func TestAdversarialSourceEscapesConfigDir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "conf")

	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}

	write(t, root, "seed.csv", tight)
	cfg := write(t, sub, "qi.yaml", `
datasets:
  - name: a
    source: ../seed.csv
    quasi_identifiers: [birth_date, postcode]
    thresholds:
      k: 3
`)

	if code, _, errOut := run(t, "--config", cfg); code != exitOK {
		t.Errorf("want exit 0, got %d: %s", code, errOut)
	}
}

func TestBoolFlagDoesNotConsumeTheNextArgument(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", tight)

	code, out, errOut := run(t, "--json", "--config", cfg)

	if code != exitOK {
		t.Fatalf("want exit 0, got %d: %s", code, errOut)
	}

	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("want a JSON document, got %q", out)
	}
}

func TestStdinDeclaredInConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: a
    source: "-"
    quasi_identifiers: [birth_date, postcode]
    thresholds:
      k: 3
`)

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

	if code, _, errOut := run(t, "--config", cfg); code != exitOK {
		t.Errorf("want exit 0 reading standard input, got %d: %s", code, errOut)
	}
}

func TestDatasetNameShownOnlyWhenItDisambiguate(t *testing.T) {
	dir := t.TempDir()

	write(t, dir, "seed.csv", tight)
	one := write(t, dir, "one.yaml", oneDataset)

	_, out, _ := run(t, "--config", one)

	if strings.Contains(out, "raycheck: seed:") {
		t.Errorf("a single dataset should not be labelled:\n%s", out)
	}

	write(t, dir, "b.csv", tight)
	two := write(t, dir, "two.yaml", `
datasets:
  - name: seed
    source: seed.csv
    quasi_identifiers: [birth_date, postcode]
    thresholds:
      k: 3
  - name: other
    source: b.csv
    quasi_identifiers: [birth_date, postcode]
    thresholds:
      k: 3
`)

	_, out, _ = run(t, "--config", two)

	for _, want := range []string{"raycheck: seed:", "raycheck: other:"} {
		if !strings.Contains(out, want) {
			t.Errorf("two datasets should both be labelled, missing %q:\n%s", want, out)
		}
	}
}

func TestAdversarialDanglingFlagAtTheEnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)
	csv := write(t, dir, "seed.csv", tight)

	tests := map[string][]string{
		"config":     {csv, "--config"},
		"dataset":    {"--config", cfg, csv, "--dataset"},
		"format":     {"--config", cfg, csv, "--format"},
		"log format": {"--config", cfg, csv, "--log-format"},
	}

	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			code, _, errOut := run(t, args...)

			if code != exitBadUsage {
				t.Fatalf("want exit %d, got %d: %s", exitBadUsage, code, errOut)
			}

			if !strings.Contains(errOut, "needs an argument") {
				t.Errorf("a flag with no value should say so, got %q", errOut)
			}

			if strings.Contains(errOut, `"--"`) || strings.Contains(errOut, "no --") {
				t.Errorf("the separator was swallowed as the flag value: %q", errOut)
			}
		})
	}
}

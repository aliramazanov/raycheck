package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDiagnosticsNeverCarryInputValues(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: seed
    source: seed.csv
    quasi_identifiers: [birth_date, postcode]
    thresholds:
      k: 5
`)

	secret := "ZZTOPSECRETVALUE"

	write(t, dir, "seed.csv", "birth_date,postcode\n"+secret+",99999\n1984,110\n")

	for _, flags := range [][]string{
		{"--config", cfg},
		{"--config", cfg, "-v"},
		{"--config", cfg, "--debug"},
		{"--config", cfg, "--trace"},
		{"--config", cfg, "--debug", "--trace", "--log-format", "json"},
	} {
		_, _, errOut := run(t, flags...)

		if strings.Contains(errOut, secret) {
			t.Errorf("%v leaked an input value into stderr:\n%s", flags, errOut)
		}
		if strings.Contains(errOut, "99999") {
			t.Errorf("%v leaked a postcode into stderr:\n%s", flags, errOut)
		}
	}
}

func TestJSONDiagnosticsAreValidJSONLines(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", tight)

	for _, flags := range [][]string{
		{"--config", cfg, "--log-format", "json", "-v"},
		{"--config", cfg, "--log-format", "json", "--debug"},
		{"--config", "/nonexistent.yaml", "--log-format", "json"},
	} {
		_, _, errOut := run(t, flags...)

		scanner := bufio.NewScanner(strings.NewReader(errOut))

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())

			if line == "" {
				continue
			}

			var doc map[string]any

			if err := json.Unmarshal([]byte(line), &doc); err != nil {
				t.Errorf("%v produced a line that is not JSON: %q", flags, line)
			}
		}
	}
}

func TestQuietByDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", tight)

	code, _, errOut := run(t, "--config", cfg)

	if code != exitOK {
		t.Fatalf("want exit 0, got %d: %s", code, errOut)
	}

	if errOut != "" {
		t.Errorf("a passing run should print nothing to stderr, got:\n%s", errOut)
	}
}

func TestFailureIsReportedOnce(t *testing.T) {
	dir := t.TempDir()

	code, _, errOut := run(t, "--config", dir+"/absent.yaml")

	if code != exitBadUsage {
		t.Fatalf("want exit 2, got %d", code)
	}

	lines := strings.Count(strings.TrimSpace(errOut), "\n") + 1

	if lines != 1 {
		t.Errorf("want one line on stderr, got %d:\n%s", lines, errOut)
	}
}

func TestTracePrintsSpansMetricsAndErrors(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", tight)

	_, _, errOut := run(t, "--config", cfg, "--trace")

	for _, want := range []string{
		"trace", "run", "load-config", "read-and-group", "finish", "render",
		"metrics", "rows_grouped", "groups", "bytes_read", "exit_code",
	} {
		if !strings.Contains(errOut, want) {
			t.Errorf("the trace should mention %q, got:\n%s", want, errOut)
		}
	}
}

func TestTraceAttributesFailuresToTheirSpan(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	_, _, errOut := run(t, "--config", cfg, "--trace")

	if !strings.Contains(errOut, "errors (1)") {
		t.Errorf("the trace should list the failure, got:\n%s", errOut)
	}

	if !strings.Contains(errOut, "open") {
		t.Errorf("the failure should be attributed to the open span, got:\n%s", errOut)
	}

	if !strings.Contains(errOut, "FAILED") {
		t.Errorf("the failing span should be marked, got:\n%s", errOut)
	}
}

func TestBlankLinesInsideAFileAreReported(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	tests := map[string]struct {
		body string
		warn bool
	}{
		"blank lines inside":  {body: "birth_date,postcode\n1,2\n\n3,4\n\n\n5,6\n", warn: true},
		"trailing blank line": {body: "birth_date,postcode\n1,2\n3,4\n\n", warn: false},
		"no blank lines":      {body: pairs, warn: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			write(t, dir, "seed.csv", tt.body)

			_, _, errOut := run(t, "--config", cfg)

			if got := strings.Contains(errOut, "blank lines skipped"); got != tt.warn {
				t.Errorf("want warning=%v, got %v: %q", tt.warn, got, errOut)
			}
		})
	}
}

func TestRowsAreAllAccountedFor(t *testing.T) {
	dir := t.TempDir()
	cfg := write(
		t,
		dir,
		"qi.yaml",
		`
datasets:
  - name: d
    source: seed.csv
    quasi_identifiers: [a]
    suppression: "*"
    thresholds:
      k: 1
`)

	write(t, dir, "seed.csv", "a\nx\n*\ny\n*\n")

	_, _, errOut := run(t, "--config", cfg, "--trace")

	if strings.Contains(errOut, "unaccounted") {
		t.Errorf("rows went missing:\n%s", errOut)
	}

	for _, want := range []string{"rows_read", "rows_grouped", "rows_suppressed", "allocated_bytes"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("the metrics should include %q:\n%s", want, errOut)
		}
	}
}

func TestJSONTelemetryCarriesTheExitCode(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	cases := map[string]struct {
		body string
		want int64
	}{
		"passing": {body: tight, want: exitOK},
		"failing": {body: pairs, want: exitBreached},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			write(t, dir, "seed.csv", tt.body)

			code, out, _ := run(t, "--config", cfg, "--json", "--trace")

			var doc struct {
				Telemetry struct {
					Metrics map[string]int64 `json:"metrics"`
					Spans   []struct {
						Name string `json:"name"`
					} `json:"spans"`
				} `json:"telemetry"`
			}

			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			if got := doc.Telemetry.Metrics["exit_code"]; got != tt.want {
				t.Errorf("telemetry says exit %d, the process returned %d", got, code)
			}

			if int64(code) != tt.want {
				t.Errorf("want exit %d, got %d", tt.want, code)
			}

			if len(doc.Telemetry.Spans) == 0 {
				t.Error("telemetry carries no spans")
			}

			for _, want := range []string{"rows_read", "bytes_read", "allocated_bytes"} {
				if _, ok := doc.Telemetry.Metrics[want]; !ok {
					t.Errorf("telemetry is missing %q", want)
				}
			}
		})
	}
}

func TestBytesReadMatchesTheFile(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	bodies := map[string]string{
		"plain":         pairs,
		"byte order":    "\ufeffbirth_date,postcode\n1,2\n3,4\n",
		"crlf":          "birth_date,postcode\r\n1,2\r\n3,4\r\n",
		"no final line": "birth_date,postcode\n1,2\n3,4",
		"blank lines":   "birth_date,postcode\n1,2\n\n3,4\n",
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			path := write(t, dir, "seed.csv", body)

			info, err := os.Stat(path)

			if err != nil {
				t.Fatal(err)
			}

			_, out, _ := run(t, "--config", cfg, "--json", "--trace")

			var doc struct {
				Telemetry struct {
					Metrics map[string]int64 `json:"metrics"`
				} `json:"telemetry"`
			}

			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			if got := doc.Telemetry.Metrics["bytes_read"]; got != info.Size() {
				t.Errorf("bytes_read %d but the file is %d bytes", got, info.Size())
			}
		})
	}
}

func TestTraceWritesRegardlessOfFormat(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", oneDataset)

	write(t, dir, "seed.csv", tight)

	for _, format := range []string{"text", "json"} {
		_, _, errOut := run(t, "--config", cfg, "--trace", "--format", format)

		if !strings.Contains(errOut, "trace") || !strings.Contains(errOut, "metrics") {
			t.Errorf("--format %s wrote no trace to stderr:\n%s", format, errOut)
		}
	}
}

func TestMetricsCoverEveryMeasure(t *testing.T) {
	dir := t.TempDir()
	cfg := write(t, dir, "qi.yaml", `
datasets:
  - name: seed
    source: seed.csv
    quasi_identifiers: [birth_date, postcode]
    sensitive:
      - name: diagnosis
        type: categorical
    thresholds:
      k: 1
      l: 1
      t: 0.9
`)

	write(t, dir, "seed.csv", "birth_date,postcode,diagnosis\n1984,110,flu\n1984,110,cold\n1990,120,flu\n")

	_, out, errOut := run(t, "--config", cfg, "--json", "--trace")

	for _, want := range []string{"l_diversity_min", "t_closeness_max_per_mille"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("the trace should report %q:\n%s", want, errOut)
		}
	}

	var doc struct {
		Telemetry struct {
			Metrics map[string]int64 `json:"metrics"`
		} `json:"telemetry"`

		Datasets []struct {
			Diversity []struct {
				L int `json:"l"`
			} `json:"l_diversity"`
			Closeness []struct {
				T float64 `json:"t"`
			} `json:"t_closeness"`
		} `json:"datasets"`
	}

	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	d := doc.Datasets[0]

	if got, want :=
		doc.Telemetry.Metrics["l_diversity_min"],
		int64(d.Diversity[0].L); got != want {
		t.Errorf("metric says l=%d, the document says %d", got, want)
	}

	if got, want :=
		doc.Telemetry.Metrics["t_closeness_max_per_mille"],
		int64(d.Closeness[0].T*1000+0.5); got != want {
		t.Errorf("metric says t=%d per mille, the document says %d", got, want)
	}
}

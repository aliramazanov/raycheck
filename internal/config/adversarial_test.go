package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestAdversarialYAMLBomb(t *testing.T) {
	bomb := `
a: &a ["x","x","x","x","x","x","x","x","x"]
b: &b [*a,*a,*a,*a,*a,*a,*a,*a,*a]
c: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b]
d: &d [*c,*c,*c,*c,*c,*c,*c,*c,*c]
e: &e [*d,*d,*d,*d,*d,*d,*d,*d,*d]
f: &f [*e,*e,*e,*e,*e,*e,*e,*e,*e]
g: &g [*f,*f,*f,*f,*f,*f,*f,*f,*f]
datasets: *g
`

	done := make(chan error, 1)

	go func() {
		_, err := Parse([]byte(bomb))
		done <- err
	}()

	select {
	case err := <-done:
		t.Logf("rejected with: %v", err)

		if err == nil {
			t.Error("the bomb parsed successfully")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("parsing did not return within 10s")
	}
}

func TestAdversarialDuplicateYAMLKeys(t *testing.T) {
	_, err := Parse([]byte(datasetYAML(fieldQI, "thresholds: {k: 5}", fieldK)))

	t.Logf("%v", err)

	if err == nil {
		t.Error("a duplicate thresholds key was accepted, and the later value won silently")
	}
}

func TestAdversarialEnvInjection(t *testing.T) {
	t.Setenv("RAYCHECK_EVIL", "a\n    thresholds: {k: 1}\n#")

	cfg, err := Parse([]byte(`
datasets:
  - name: ${RAYCHECK_EVIL}
    quasi_identifiers: [x]
    thresholds: {k: 999}
`))

	if err != nil {
		t.Logf("rejected: %v", err)

		return
	}

	t.Logf("parsed as k=%d name=%q", cfg.Datasets[0].Thresholds.K, cfg.Datasets[0].Name)

	if cfg.Datasets[0].Thresholds.K != 999 {
		t.Errorf("an environment value rewrote the threshold to %d", cfg.Datasets[0].Thresholds.K)
	}
}

func TestAdversarialEnvEdgeCases(t *testing.T) {
	t.Setenv("RAYCHECK_A", "seed")

	tests := map[string]struct {
		yaml string
		want string
	}{
		"default stops at the first brace": {yaml: "${RAYCHECK_UNSET:-a}b}", want: "ab}"},
		"empty default":                    {yaml: "${RAYCHECK_UNSET:-}", want: ""},
		"adjacent references":              {yaml: "${RAYCHECK_A}${RAYCHECK_A}", want: "seedseed"},
		"not a reference":                  {yaml: "$RAYCHECK_A", want: "$RAYCHECK_A"},
		"bare dollar brace":                {yaml: "${}", want: "${}"},
		"digit leading name":               {yaml: "${1BAD}", want: "${1BAD}"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var missing []string

			got, _ := expandString(tt.yaml, &missing)

			if len(missing) > 0 {
				t.Fatalf("unexpected unset variables: %v", missing)
			}
			if got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestAdversarialHugeThreshold(t *testing.T) {
	cfg, err := Parse(fmt.Appendf(nil, datasetYAML(fieldQI, "thresholds: {k: %d}"), uint64(1)<<63))

	if err != nil {
		t.Logf("rejected: %v", err)

		return
	}

	t.Logf("k parsed as %d", cfg.Datasets[0].Thresholds.K)

	if cfg.Datasets[0].Thresholds.K < 1 {
		t.Errorf("an oversized threshold wrapped to %d", cfg.Datasets[0].Thresholds.K)
	}
}

func TestAdversarialNegativeAndFloatThreshold(t *testing.T) {
	for _, k := range []string{"-1", "2.5", "\"5\"", "true", "null"} {
		t.Run(k, func(t *testing.T) {
			_, err := Parse([]byte(`
datasets:
  - name: a
    quasi_identifiers: [x]
    thresholds: {k: ` +
				k + "}\n"))

			t.Logf("k: %s -> %v", k, err)

			if err == nil {
				t.Errorf("k: %s was accepted", k)
			}
		})
	}
}

func TestAdversarialSensitiveWrongShape(t *testing.T) {
	_, err := Parse([]byte(datasetYAML(fieldQI, "sensitive: [[nested]]", fieldK)))

	t.Logf("%v", err)

	if err == nil {
		t.Error("a nested list was accepted as a sensitive entry")
	}
}

func TestAdversarialSuppressionMatchingAQuasiIdentifierName(t *testing.T) {
	cfg, err := Parse([]byte(datasetYAML(fieldQI, "suppression: \"x\"", fieldK)))

	if err != nil {
		t.Fatal(err)
	}
	if *cfg.Datasets[0].Suppression != "x" {
		t.Errorf("unexpected marker %q", *cfg.Datasets[0].Suppression)
	}
}

func TestAdversarialVeryLongColumnName(t *testing.T) {
	name := strings.Repeat("a", 1<<16)

	cfg, err := Parse([]byte(`
datasets:
  - name: a
    quasi_identifiers: [` +
		name + "]\n    thresholds: {k: 1}\n"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Datasets[0].QuasiIdentifiers[0]) != 1<<16 {
		t.Error("a long column name was truncated")
	}
}

func TestAdversarialEnvCannotInjectStructure(t *testing.T) {
	t.Setenv("RAYCHECK_EVIL", "a\nthresholds:\n  k: 1\n")

	cfg, err := Parse([]byte(`
datasets:
  - name: ${RAYCHECK_EVIL}
    quasi_identifiers: [x]
    thresholds:
      k: 999
`))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := cfg.Datasets[0].Thresholds.K; got != 999 {
		t.Errorf("an environment value rewrote the threshold to %d", got)
	}

	if got := cfg.Datasets[0].Name; got != "a\nthresholds:\n  k: 1\n" {
		t.Errorf("want the value kept verbatim as a string, got %q", got)
	}
}

func TestAdversarialEnvRetypesScalars(t *testing.T) {
	t.Setenv("RAYCHECK_K", "7")
	t.Setenv("RAYCHECK_NAME", "12345")

	cfg, err := Parse([]byte(`
datasets:
  - name: ${RAYCHECK_NAME}
    quasi_identifiers: [x]
    thresholds:
      k: ${RAYCHECK_K}
`))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Datasets[0].Thresholds.K != 7 {
		t.Errorf("want k=7, got %d", cfg.Datasets[0].Thresholds.K)
	}

	if cfg.Datasets[0].Name != "12345" {
		t.Errorf("want the name as a string, got %q", cfg.Datasets[0].Name)
	}
}

func TestAdversarialFractionalThresholdRefused(t *testing.T) {
	for _, k := range []string{"2.5", "2.9", "1e3", "0x5"} {
		t.Run(k, func(t *testing.T) {
			cfg, err := Parse([]byte(`
datasets:
  - name: a
    quasi_identifiers: [x]
    thresholds:
      k: ` +
				k + "\n"))

			if err != nil {
				t.Logf("refused: %v", err)

				return
			}

			t.Logf("k: %s accepted as %d", k, cfg.Datasets[0].Thresholds.K)

			if strings.Contains(k, ".") {
				t.Errorf("a fractional threshold was truncated to %d", cfg.Datasets[0].Thresholds.K)
			}
		})
	}
}

func TestAdversarialFlowMappingReferenceExplained(t *testing.T) {
	t.Setenv("RAYCHECK_K", "5")

	_, err := Parse([]byte(datasetYAML(fieldQI, "thresholds: {k: ${RAYCHECK_K}}")))

	if err == nil {
		t.Fatal("expected a parse error")
	}

	if !strings.Contains(err.Error(), "must be quoted") {
		t.Errorf("the error should explain the fix, got: %v", err)
	}
}

func TestAdversarialQuotedFlowMappingReference(t *testing.T) {
	t.Setenv("RAYCHECK_K", "5")

	cfg, err := Parse([]byte(datasetYAML(fieldQI, "thresholds: {k: \"${RAYCHECK_K}\"}")))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Datasets[0].Thresholds.K != 5 {
		t.Errorf("want k=5, got %d", cfg.Datasets[0].Thresholds.K)
	}
}

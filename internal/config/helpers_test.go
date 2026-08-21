package config

import "strings"

const minimal = `
datasets:
  - name: seed
    quasi_identifiers: [birth_date, postcode]
    thresholds:
      k: 5
`

const (
	fieldQI     = "quasi_identifiers: [x]"
	fieldK      = "thresholds: {k: 1}"
	fieldSource = "source: seed.csv"
)

func datasetYAML(fields ...string) string {
	return "datasets:\n  - name: a\n    " + strings.Join(fields, "\n    ") + "\n"
}

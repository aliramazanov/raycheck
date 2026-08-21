package main

import "strings"

const (
	fieldQI     = "quasi_identifiers: [x]"
	fieldK      = "thresholds: {k: 1}"
	fieldSource = "source: seed.csv"
)

const (
	tight  = "birth_date,postcode\n1984,110\n1984,110\n1984,110\n"
	spread = "birth_date,postcode\n1984,110\n1990,120\n"
	pairs  = "birth_date,postcode\n1,2\n3,4\n"
)

func datasetYAML(fields ...string) string {
	return "datasets:\n  - name: a\n    " + strings.Join(fields, "\n    ") + "\n"
}

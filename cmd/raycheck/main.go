package main

import (
	"fmt"
	"io"
	"os"

	"github.com/aliramazanov/raycheck/internal/version"
)

const (
	exitOK       = 0
	exitBreached = 1
	exitBadUsage = 2
	exitInput    = 3
)

const configEnv = "RAYCHECK_CONFIG"

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "version", "-v", "--version":
			fmt.Println("raycheck " + version.Full())
			os.Exit(exitOK)
		case "help", "-h", "--help":
			usage(os.Stderr)
			os.Exit(exitOK)
		}
	}

	os.Exit(runCheck(args, os.Stdout, os.Stderr))
}

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `raycheck answers "is this anonymized data actually anonymous?"

usage:
  raycheck --config qi.yaml [file.csv]   check the datasets declared in qi.yaml
  raycheck version                       print the version
  raycheck help                          print this message

A single positional path overrides the dataset's source. Use "-" to read
standard input.

flags:
  --config PATH          the quasi-identifier config, or set RAYCHECK_CONFIG
  --dataset NAME         check only the named dataset
  --format text|json     how to render the result (default text)
  --json                 shorthand for --format json
  --log-format text|json how to render errors (default text)
  -v, --verbose          report each phase as it finishes
  --debug                report every step, including per-dataset detail
  --trace                print the span tree, metrics and errors when the run ends
  --strict               fail on concerns as well as on a breached threshold.
                         A concern is something that leaves the verdict worth
                         less than it looks: blank quasi-identifier cells, which
                         group with each other and raise k, or a verdict that
                         rests on under half the file because the rest was
                         excluded as suppressed

exit codes:
  0  every threshold met
  1  a threshold was breached
  2  usage or config error
  3  the input could not be measured

Diagnostics go to standard error and never leave this machine. Nothing recorded
is a value from your data: counts, durations and the column names you declared
are the whole vocabulary.

Set thresholds.l and thresholds.t in the config to gate l-diversity and
t-closeness as well as k.

raycheck does not anonymize anything and it will not guess your
quasi-identifiers. Passing is a necessary check, not proof of anonymity.
See docs/METHODOLOGY.md.
`)
}

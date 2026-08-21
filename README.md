# raycheck

Is your anonymized data actually anonymous? raycheck does not anonymize anything. It checks
whether anonymization that already happened actually holds, and fails the build when it
does not.

This is k-anonymity in the statistical disclosure control sense, measuring whether rows can
be singled out. It is unrelated to the k-anonymity range protocol used by password breach
APIs, which is the other thing that name means in this ecosystem.

```console
$ raycheck --config qi.yaml seed.csv
raycheck: k=1 (threshold 5)

  222 rows checked across 3 quasi-identifiers

  FAIL   k-anonymity      k=1     162 rows unique on (birth_date, postcode, gender)
         162 rows (73% of the rows checked) are alone in their group and can be
         singled out by someone who already knows their target is here.
         222 rows (100%) sit in a group smaller than 5, so each of them can be
         narrowed to fewer than 5 people.
  --     l-diversity      l=1     one class is uniform on "salary", not gated
  --     t-closeness      t=0.514 one class sits 0.514 from the file on "salary", not gated

  smallest groups
    1 row    1978-Q1, 1100, F                       first at row 4
    1 row    1978-Q1, 1103, F                       first at row 8
    1 row    1978-Q1, 1200, F                       first at row 16
    1 row    1978-Q1, 1202, M                       first at row 29
    1 row    1978-Q1, 1203, M                       first at row 28
    showing 5 of 189 groups below the threshold

  risk to a person in this file, assuming an attacker who knows their
  target is present: 100% at worst, 85% on average, 33% at best

  every measure raycheck implements ran against the sensitive columns you
  declared. That is not the same as the data being safe to publish.

  162 rows can be singled out. This data is not anonymous.
```

## Install

```bash
go install github.com/aliramazanov/raycheck/cmd/raycheck@latest
```

## Usage

```bash
raycheck --config qi.yaml
raycheck --config qi.yaml seed.csv        # override the declared source
raycheck --config qi.yaml seed.csv --json
pg_dump ... | raycheck --config qi.yaml - # read standard input, or source: "-"
```

```yaml
datasets:
  - name: seed
    source: seed.csv
    quasi_identifiers:
      - birth_date
      - postcode
      - gender
    sensitive:
      - name: salary
        type: numeric
    suppression: "*"
    thresholds:
      k: 5
      l: 2
      t: 0.2
```

`source` resolves relative to the config file. `suppression` is the marker your anonymizer
writes into redacted cells; rows carrying it in a quasi-identifier are excluded from
grouping and counted separately, because otherwise every redacted row collapses into one
enormous group and a file that is mostly redactions reports as anonymous.

`l` and `t` are reported whenever you declare sensitive columns, and gate only when you set
their thresholds. `--strict` also fails the run on advisory concerns, which is what you
want in CI. Run `raycheck help` for the full flag list.

## Exit codes

These are API. A pipeline needs to tell "the data is unsafe" apart from "the checker
broke", so those are different codes.

| code | meaning |
| --- | --- |
| 0 | every threshold met |
| 1 | a threshold was breached |
| 2 | usage or config error, including a quasi-identifier column not present in the data |
| 3 | the input could not be measured: unreadable, malformed, or nothing left to measure |

## Limits

- It does not anonymize, mask, subset or generate data. Point it at output from something
  that does.
- **It will not guess your quasi-identifiers.** You declare them. Guessing wrong produces a
  confident "safe" verdict on re-identifiable data.
- **It does not normalise your values.** No trimming, no case folding, no reading `01234` as
  a number. Every one of those merges groups, and merging groups raises `k`. If raycheck is
  wrong it will be wrong in the direction that reports more risk than there is.
- Passing does not make data anonymous under the GDPR.

Memory scales with the number of distinct quasi-identifier combinations, not with the number
of rows. Input is streamed and the table is never materialised.

`docs/METHODOLOGY.md` states precisely what is measured and how, and how the numbers are
checked against an independent implementation.

## License

MIT

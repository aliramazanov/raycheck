# What raycheck measures, and what it does not

## k-anonymity

Rows are grouped by the combination of values in the columns declared as
quasi-identifiers. `k` is the size of the smallest group, so `k=1` means at least
one row is unique on its quasi-identifiers and can be singled out.

Three risk figures come from the same counts, under the prosecutor attacker model,
meaning an attacker who already knows their target is in the file: highest risk is
`1/k`, lowest is `1/largest group`, average is `groups/rows`. The average is
weighted by record rather than by group, so it answers what the risk is to a person
in this file. The group-weighted mean is a different number and is not reported.

## l-diversity

Distinct l-diversity. For each class, the number of different values a sensitive
column takes; `l` is the smallest across all classes, and with several sensitive
columns the worst over all (class, column) pairs. That matches pyCanon's `gen=True`.

## t-closeness

The Earth Mover's Distance between one class's distribution of a sensitive column
and the distribution across the whole file. `t` is the largest such distance over
every class.

For categorical values every value is equally far from every other, so the distance
is half the total variation, `½·Σ|q−p|`. For numeric values the cost of moving mass
depends how far it travels, `(1/(m−1))·Σ|cumulative(q−p)|` over the sorted domain.
Which applies comes from `type` in the config and is never inferred: an ordinal code
and a postcode are both strings of digits, and the distance between their values is
not the same thing.

Probabilities are taken over the value space of the whole file, so a value absent
from a class contributes a zero rather than shortening the vector and misaligning
the axis.

`l` and `t` are reported whenever sensitive columns are declared and gate only when
their thresholds are set.

## What it does not measure

Journalist risk, marketer risk and population uniqueness need a population prior
that a CSV cannot supply. Estimating them from the sample and calling the result a
risk figure would be a guess dressed as a measurement.

## Rules the implementation follows

Quasi-identifiers are never inferred; they come from config, and a declared column
missing from the data is a fatal error rather than a skipped one, because a dropped
quasi-identifier merges groups and turns re-identifiable data into a confident pass.

Values are compared byte for byte. No trimming, no case folding, no coercing `NULL`,
`""` and `\N` together, no reading `01234` as `1234`, and no Unicode normalisation, so
`José` written as one codepoint and as `e` plus a combining accent stay two values. Each
of those merges groups, and merging groups raises `k`. Byte comparison can only split a
group, never merge one, so where raycheck is wrong it is wrong toward reporting more risk
than there is.

Rows carrying the declared `suppression` marker in any quasi-identifier are set
aside and counted separately rather than collapsing into one enormous class, and the
excluded count is always printed. Without this a file that is mostly redactions
reports a large `k` and looks anonymous. With no marker declared nothing is
excluded, because raycheck will not guess which value means redacted. Blank cells,
a marker that only ever landed in one column, and a verdict resting on less than
three quarters of the file are each raised as concerns, which `--strict` turns into
failures.

Thresholds are compared as integers and a fractional `k` is refused, so a gate
cannot flip on rounding and YAML cannot quietly decode `k: 2.9` into a weaker gate.
Nothing declared in the config may vanish quietly either: YAML drops a null list
entry, so a column written as bare `null` or `true` is refused by name.

Output is deterministic, ordered by group size and then by value, so the same rows
in a different order produce the same report. Row numbers are file lines, since
blank lines and multi-line quoted fields make a record counter drift. A blank line
is refused rather than skipped, because in a single-column file a row whose only
value is empty is a blank line and skipping it would lose a row and raise `k`.

Input values are quoted when not plainly printable, so a newline cannot forge a line
in a CI log. In JSON a value that is not valid UTF-8 is escaped and its group marked
`values_escaped`, since the encoder would otherwise substitute the replacement
character and make two distinct classes read as one.

## Reading the verdict

Exit `0` means every declared threshold was met. It does not mean the data is
anonymous, or outside the scope of the GDPR or any other regime. k-anonymity is a
necessary condition, not a sufficient one.

The European Data Protection Board's draft Guidelines 02/2026 on Anonymisation,
adopted 7 July 2026 and open for consultation until 30 October 2026, frame the test
as no record isolation, no linkage, no inference. raycheck measures the first
directly, bears on the second, and does not address the third. They are a draft, not
binding law.

A file with no data rows, or one where every row was suppressed, exits 3 rather than
passing. A zero-row dataset is almost always a broken pipeline.

`--trace` reports every phase, count and error, all of it local: no exporter, no
collector, no network call. Nothing recorded is a value from the input, which a test
asserts.

## How the numbers are checked

k is computed one way, so a mistake in it would be invisible from the inside. Three
outside checks stand against that: a second implementation written differently on
purpose and compared over randomly generated datasets; properties that must survive
a transformation, such as adding a quasi-identifier only lowering k and duplicating
every row doubling it; and pyCanon, an independent implementation by other people,
whose answers for k, l and t over four fixtures and twelve column combinations are
frozen in `internal/measure/testdata/reference.json` and asserted on every run.

## Prior art

These measures are decades old and implemented correctly elsewhere. ARX, sdcMicro
and pyCanon are more complete than raycheck and will stay that way. The contribution
here is packaging: one static binary, a config beside the code that produced the
dump, and an exit code.

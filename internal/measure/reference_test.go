package measure

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type referenceCase struct {
	File             string   `json:"file"`
	QuasiIdentifiers []string `json:"quasi_identifiers"`
	Sensitive        []string `json:"sensitive"`
	Kinds            []string `json:"kinds"`
	K                int      `json:"k"`
	L                int      `json:"l"`
	T                float64  `json:"t"`
}

func TestAgainstReference(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "reference.json"))

	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Source string          `json:"source"`
		Cases  []referenceCase `json:"cases"`
	}

	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	if len(doc.Cases) == 0 {
		t.Fatal("the reference file holds no cases")
	}

	for _, c := range doc.Cases {
		name := c.File + "/" + join(c.QuasiIdentifiers) + "/" + join(c.Sensitive)

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			header, rows := readFixture(t, c.File)

			qi, err := columns(header, c.QuasiIdentifiers)

			if err != nil {
				t.Fatal(err)
			}

			sa, err := columns(header, c.Sensitive)

			if err != nil {
				t.Fatal(err)
			}

			kinds := make([]Kind, len(c.Kinds))

			for i, k := range c.Kinds {
				if k == "numeric" {
					kinds[i] = Numeric
				}
			}

			g := NewGrouper(c.QuasiIdentifiers, qi, nil).
				WithSensitive(c.Sensitive, sa, kinds)

			for i, row := range rows {
				g.Add(row, int64(i+2))
			}

			res, err := g.Finish(1, 0, 0)

			if err != nil {
				t.Fatal(err)
			}

			if res.K != c.K {
				t.Errorf("k: %s says %d, this says %d", doc.Source, c.K, res.K)
			}
			if res.MinL() != c.L {
				t.Errorf("l: %s says %d, this says %d", doc.Source, c.L, res.MinL())
			}
			if math.Abs(res.MaxT()-c.T) > 1e-9 {
				t.Errorf("t: %s says %.12f, this says %.12f", doc.Source, c.T, res.MaxT())
			}
		})
	}
}

func readFixture(t *testing.T, name string) ([]string, [][]string) {
	t.Helper()

	f, err := os.Open(filepath.Join("testdata", name+".csv"))

	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Close() }()

	records, err := csv.NewReader(f).ReadAll()

	if err != nil {
		t.Fatal(err)
	}

	if len(records) < 2 {
		t.Fatalf("%s holds no rows", name)
	}

	return records[0], records[1:]
}

func columns(header, want []string) ([]int, error) {
	idx := make([]int, len(want))

	for i, w := range want {
		at := slices.Index(header, w)

		if at < 0 {
			return nil, &missingColumn{w}
		}

		idx[i] = at
	}

	return idx, nil
}

type missingColumn struct{ name string }

func (e *missingColumn) Error() string { return "no column " + e.name }

func join(s []string) string {
	out := ""

	for i, v := range s {
		if i > 0 {
			out += "+"
		}

		out += v
	}

	return out
}

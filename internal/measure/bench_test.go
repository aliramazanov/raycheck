package measure

import (
	"fmt"
	"testing"
)

func benchRows(n, distinct int) [][]string {
	rows := make([][]string, n)

	for i := range rows {
		v := i % distinct
		rows[i] = []string{fmt.Sprintf("19%02d", v%100), fmt.Sprintf("1%03d", v%1000), fmt.Sprintf("g%d", v)}
	}

	return rows
}

func BenchmarkGrouping(b *testing.B) {
	cases := map[string]int{"200 distinct": 200, "all distinct": 1 << 20}

	for name, distinct := range cases {
		b.Run(name, func(b *testing.B) {
			rows := benchRows(1<<20, distinct)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				g := NewGrouper([]string{"a", "b", "c"}, []int{0, 1, 2}, nil)

				for j, r := range rows {
					g.Add(r, int64(j+2))
				}

				if _, err := g.Finish(5, 0, 0); err != nil {
					b.Fatal(err)
				}
			}

			b.SetBytes(int64(len(rows)))
		})
	}
}

func BenchmarkFinishReporting(b *testing.B) {
	rows := benchRows(1<<20, 1<<20)

	g := NewGrouper([]string{"a", "b", "c"}, []int{0, 1, 2}, nil)

	for j, r := range rows {
		g.Add(r, int64(j+2))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := g.Finish(1<<30, 0, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeKey(b *testing.B) {
	values := []string{"1984-02-11", "11000", "F"}

	var buf []byte

	b.ReportAllocs()

	for b.Loop() {
		buf = encodeKey(buf[:0], values)
	}

	_ = buf
}

func BenchmarkFinishHighCardinality(b *testing.B) {
	rows := benchRows(1<<21, 1<<21)

	g := NewGrouper([]string{"a", "b", "c"}, []int{0, 1, 2}, nil)

	for j, r := range rows {
		g.Add(r, int64(j+2))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := g.Finish(5, 0, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFinishWithMeasures(b *testing.B) {
	rows := benchRows(1<<18, 1<<18)

	g := NewGrouper([]string{"a", "b", "c"}, []int{0, 1, 2}, nil).
		WithSensitive([]string{"s"}, []int{2}, []Kind{Categorical})

	for j, r := range rows {
		g.Add(r, int64(j+2))
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, err := g.Finish(5, 2, 0.3); err != nil {
			b.Fatal(err)
		}
	}
}

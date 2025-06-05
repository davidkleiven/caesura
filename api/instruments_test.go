package api

import (
	"math"
	"testing"
)

func TestJaccard(t *testing.T) {
	for _, test := range []struct {
		a    []string
		b    []string
		want float64
		desc string
	}{
		{
			a:    []string{},
			b:    []string{},
			want: 0.0,
			desc: "empty sets",
		},
		{
			a:    []string{"a", "b"},
			b:    []string{"b", "a"},
			want: 1.0,
			desc: "similar sets",
		},
		{
			a:    []string{"a", "b"},
			b:    []string{"b"},
			want: 0.5,
			desc: "half match first largest",
		},
		{
			a:    []string{"b"},
			b:    []string{"a", "b"},
			want: 0.5,
			desc: "half match last largest",
		},
		{
			a:    []string{"b", "c"},
			b:    []string{"a", "b"},
			want: 1.0 / 3.0,
			desc: "half match last largest",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			result := Jaccard(test.a, test.b)
			if math.Abs(result-test.want) > 1e-9 {
				t.Errorf("Expected Jaccard index of 0 for empty sets, got %f", result)
			}
		})
	}
}

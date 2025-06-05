package pkg

import (
	"math"
	"slices"
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

func TestFilterList(t *testing.T) {
	items := []string{"jazz", "blues", "jazz waltz", "Jazz"}
	result := FilterList(items, "Jazz")
	want := []string{"jazz", "Jazz", "jazz waltz"}

	if slices.Compare(result, want) != 0 {
		t.Errorf("Wanted %v\n got %v\n", want, result)
	}
}

func TestFilterListEmptyToken(t *testing.T) {
	items := []string{"a", "b"}
	result := FilterList(items, "")
	if slices.Compare(result, items) != 0 {
		t.Errorf("Wanted %v\ngot\n%v", items, result)
	}
}

func TestLengthFromToken(t *testing.T) {
	tests := []struct {
		token  string
		target int
		want   int
	}{
		{"", 3, 0},
		{"a", 3, 1},
		{"ab", 3, 2},
		{"abc", 3, 3},
		{"abcd", 3, 3},
	}

	for _, test := range tests {
		got := lengthFromToken(test.token, test.target)
		if got != test.want {
			t.Errorf("lengthFromToken(%q, %d) = %d; want %d", test.token, test.target, got, test.want)
		}
	}
}

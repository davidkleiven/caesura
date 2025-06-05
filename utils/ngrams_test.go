package utils

import (
	"slices"
	"testing"
)

func TestNgrams(t *testing.T) {
	result := Ngrams("Cello", 3)
	expected := []string{"Cel", "ell", "llo"}
	if slices.Compare(result, expected) != 0 {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

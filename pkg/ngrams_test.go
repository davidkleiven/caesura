package pkg

import (
	"slices"
	"testing"
)

func TestNgrams(t *testing.T) {
	result := Ngrams("Cello", 3)
	expected := []string{"Cel", "ell", "llo"}
	if slices.Compare(result, expected) != 0 {
		t.Fatalf("Expected %v, got %v", expected, result)
	}
}

func TestNgramsWithNegativeN(t *testing.T) {
	result := Ngrams("Cello", -1)
	expected := []string{"Cello"}
	if slices.Compare(result, expected) != 0 {
		t.Fatalf("Expected %v, got %v", expected, result)
	}
}

func TestNgramsWithNLargerThanText(t *testing.T) {
	result := Ngrams("Cello", 10)
	expected := []string{"Cello"}
	if slices.Compare(result, expected) != 0 {
		t.Fatalf("Expected %v, got %v", expected, result)
	}
}

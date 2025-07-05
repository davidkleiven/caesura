package pkg

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

func TestPanicOnErr(t *testing.T) {
	err := errors.New("test error")
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("PanicOnErr did not panic on error: %v", err)
		}
	}()

	PanicOnErr(err)
}

func TestRemoveDuplicates(t *testing.T) {
	for i, test := range []struct {
		input    []string
		expected []string
	}{
		{[]string{"a", "b", "a"}, []string{"a", "b"}},
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{[]string{}, []string{}},
	} {
		t.Run(fmt.Sprintf("Test #%d", i), func(t *testing.T) {
			result := RemoveDuplicates(test.input)
			if len(result) != len(test.expected) {
				t.Errorf("Expected length %d, got %d", len(test.expected), len(result))
			}
			if slices.Compare(result, test.expected) != 0 {
				t.Errorf("Expected %v, got %v", test.expected, result)
			}
		})
	}
}

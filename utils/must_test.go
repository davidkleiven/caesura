package utils

import (
	"fmt"
	"testing"
)

func TestMustNoError(t *testing.T) {
	value := Must("test", nil)
	if value != "test" {
		t.Fatalf("Expected value 'test', got %v", value)
	}
}

func TestMustWithError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected panic, but did not occur")
		}
	}()

	Must("", fmt.Errorf("Some error"))
}

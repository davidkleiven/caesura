package testutils

import (
	"strings"
	"testing"
)

func AssertEqual[T comparable](t *testing.T, a, b T) {
	t.Helper()
	if a != b {
		t.Fatalf("Expected: %v, got: %v", b, a)
	}
}

func AssertNil(t *testing.T, v any) {
	t.Helper()
	if v != nil {
		t.Fatalf("Expected value to be non-nil, but got %v", v)
	}
}

func AssertContains(t *testing.T, result string, tokens ...string) {
	t.Helper()
	for _, token := range tokens {
		if !strings.Contains(result, token) {
			t.Fatalf("Wanted result to contain '%s' got '%s'", token, result)
			return
		}
	}
}

func AssertNotContains(t *testing.T, result string, tokens ...string) {
	t.Helper()
	for _, token := range tokens {
		if strings.Contains(result, token) {
			t.Fatalf("Wanted result not to contain '%s' got '%s'", token, result)
			return
		}
	}
}

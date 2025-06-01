package api

import (
	"os"
	"testing"
)

func TestDefaultPort(t *testing.T) {
	origPort := os.Getenv("PORT")
	if origPort != "" {
		defer os.Setenv("PORT", origPort)
	}

	os.Unsetenv("PORT")
	if Port() != ":8080" {
		t.Error("Expected default port to be :8080")
	}
}

func TestPortSetViaEnvironment(t *testing.T) {
	origPort := os.Getenv("PORT")
	if origPort != "" {
		defer os.Setenv("PORT", origPort)
	}

	os.Setenv("PORT", "1234")
	if Port() != ":1234" {
		t.Error("Expected port to be set to :1234")
	}
}

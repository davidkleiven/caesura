package pkg

import "testing"

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"valid_string", "valid_string"},
		{"invalid string", "invalidstring"},
		{"12345", "12345"},
		{"!@#$%^&*<()>", ""},
		{"mixed123_456", "mixed123_456"},
		{"Aa!B", "aab"},
	}

	for _, test := range tests {
		result := SanitizeString(test.input)
		if result != test.expected {
			t.Errorf("SanitizeString(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}

package pkg

import (
	"regexp"
	"strings"
)

func SanitizeString(s string) string {
	matchPattern := regexp.MustCompile(`[a-z0-9_]+`)
	return strings.Join(matchPattern.FindAllString(strings.ToLower(s), -1), "")
}

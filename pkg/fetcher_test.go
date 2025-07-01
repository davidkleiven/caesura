package pkg

import (
	"errors"
	"io"
	"testing"
)

func preppedInMemporyFetcher() *InMemoryFetcher {
	return &InMemoryFetcher{
		MetaData: []MetaData{
			{Title: "Test Title", Composer: "Test Composer", Arranger: "Test Arranger"},
			{Title: "Another Title", Composer: "Another Composer", Arranger: "Another Arranger"},
		},
		Resources: map[string][]byte{
			"test_resource":    []byte("This is a test resource content."),
			"another_resource": []byte("This is another resource content."),
		},
	}
}

func TestFetchMeta(t *testing.T) {
	inMemFetcher := preppedInMemporyFetcher()
	for _, test := range []struct {
		name     string
		fetcher  Fetcher
		pattern  *MetaData
		expected int
	}{
		{"Empty Pattern", inMemFetcher, &MetaData{}, 2},
		{"Title Match", inMemFetcher, &MetaData{Title: "Test"}, 1},
		{"Composer Match", inMemFetcher, &MetaData{Composer: "Another Composer"}, 1},
		{"Arranger Match", inMemFetcher, &MetaData{Arranger: "Ano"}, 1},
		{"No Match", inMemFetcher, &MetaData{Title: "Nonexistent"}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {

			results, err := test.fetcher.Meta(test.pattern)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(results) != test.expected {
				t.Errorf("Expected %d results, got %d", test.expected, len(results))
			}
		})
	}
}

func TestFetchResource(t *testing.T) {
	inMemFetcher := preppedInMemporyFetcher()
	for _, test := range []struct {
		name     string
		fetcher  Fetcher
		resource string
		expected string
	}{
		{"Existing Resource", inMemFetcher, "test_resource", "This is a test resource content."},
		{"Nonexistent Resource", inMemFetcher, "nonexistent_resource", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader, err := test.fetcher.Resource(test.resource)
			if err != nil && test.expected != "" {
				t.Fatalf("Unexpected error: %v", err)
			}
			if test.expected == "" && !errors.Is(err, ErrFileNotFound) {
				t.Fatal("Expected an error for nonexistent resource, got none")
			}

			content, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("Failed to read resource content: %v", err)
			}
			if string(content) != test.expected {
				t.Errorf("Expected content '%s', got '%s'", test.expected, content)
			}
		})
	}
}

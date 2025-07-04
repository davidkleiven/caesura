package pkg

import (
	"context"
	"testing"
)

func preppedInMemporyFetcher() *InMemoryStore {
	return &InMemoryStore{
		Metadata: []MetaData{
			{Title: "Test Title", Composer: "Test Composer", Arranger: "Test Arranger"},
			{Title: "Another Title", Composer: "Another Composer", Arranger: "Another Arranger"},
		},
		Data: map[string][]byte{
			"test_resource":    []byte("This is a test resource content."),
			"another_resource": []byte("This is another resource content."),
		},
	}
}

func TestFetchMeta(t *testing.T) {
	inMemFetcher := preppedInMemporyFetcher()
	for _, test := range []struct {
		name     string
		fetcher  BlobStore
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

			results, err := test.fetcher.MetaByPattern(context.Background(), test.pattern)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(results) != test.expected {
				t.Errorf("Expected %d results, got %d", test.expected, len(results))
			}
		})
	}
}

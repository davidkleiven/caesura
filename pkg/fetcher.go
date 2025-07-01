package pkg

import (
	"bytes"
	"io"
	"strings"
)

type Fetcher interface {
	// Fetches metadata based on the provided pattern.
	Meta(pattern *MetaData) ([]MetaData, error)

	// Fetches the data for a specific resource.
	Resource(resource string) (io.Reader, error)
}

type InMemoryFetcher struct {
	MetaData  []MetaData
	Resources map[string][]byte
}

func (f *InMemoryFetcher) Meta(pattern *MetaData) ([]MetaData, error) {
	var results []MetaData
	for _, meta := range f.MetaData {
		isMatch := false
		if pattern.Title != "" && strings.HasPrefix(strings.ToLower(meta.Title), strings.ToLower(pattern.Title)) {
			isMatch = true
		}

		if pattern.Composer != "" && strings.HasPrefix(strings.ToLower(meta.Composer), strings.ToLower(pattern.Composer)) {
			isMatch = true
		}
		if pattern.Arranger != "" && strings.HasPrefix(strings.ToLower(meta.Arranger), strings.ToLower(pattern.Arranger)) {
			isMatch = true
		}
		if isMatch || (pattern.Title == "" && pattern.Composer == "" && pattern.Arranger == "") {
			results = append(results, meta)
		}
	}
	return results, nil
}

func (f *InMemoryFetcher) Resource(resource string) (io.Reader, error) {
	data, exists := f.Resources[resource]
	if !exists {
		return bytes.NewBuffer([]byte{}), ErrFileNotFound
	}
	return strings.NewReader(string(data)), nil
}

func NewInMemoryFetcher() *InMemoryFetcher {
	return &InMemoryFetcher{
		MetaData:  []MetaData{},
		Resources: make(map[string][]byte),
	}
}

func NewDemoFetcher() *InMemoryFetcher {
	fetcher := NewInMemoryFetcher()
	fetcher.MetaData = []MetaData{
		{Title: "Demo Title 1", Composer: "Composer A", Arranger: "Arranger X"},
		{Title: "Demo Title 2", Composer: "Composer B", Arranger: "Arranger Y"},
	}
	fetcher.Resources["demo1.pdf"] = []byte("resource1.zip")
	fetcher.Resources["demo2.pdf"] = []byte("resource2.zip")
	return fetcher
}

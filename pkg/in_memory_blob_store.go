package pkg

import (
	"context"
	"errors"
	"io"
	"strings"
)

type InMemoryStore struct {
	Data     map[string][]byte
	Metadata []MetaData
}

func (s *InMemoryStore) Submit(ctx context.Context, meta *MetaData, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return errors.Join(ErrRetrievingContent, err)
	}
	s.Data[meta.ResourceName()] = data
	s.Metadata = append(s.Metadata, *meta)
	return nil
}

func (s *InMemoryStore) MetaByPattern(ctx context.Context, pattern *MetaData) ([]MetaData, error) {
	var results []MetaData
	for _, meta := range s.Metadata {
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

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		Data:     make(map[string][]byte),
		Metadata: []MetaData{},
	}
}

func NewDemoStore() *InMemoryStore {
	store := NewInMemoryStore()
	store.Metadata = []MetaData{
		{Title: "Demo Title 1", Composer: "Composer A", Arranger: "Arranger X"},
		{Title: "Demo Title 2", Composer: "Composer B", Arranger: "Arranger Y"},
	}
	store.Data["demo1.pdf"] = []byte("resource1.zip")
	store.Data["demo2.pdf"] = []byte("resource2.zip")
	return store
}

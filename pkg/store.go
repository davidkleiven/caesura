package pkg

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Storer interface {
	Store(name string, r io.Reader) error
	List(prefix string) ([]string, error)
	Delete(name string) error
	Get(name string) (io.Reader, error)
}

type InMemoryStore struct {
	data map[string][]byte
}

var ErrFileNotFound = fmt.Errorf("file not found")
var ErrRetrievingContent = fmt.Errorf("error retrieving content")

func (s *InMemoryStore) Store(name string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return errors.Join(ErrRetrievingContent, err)
	}
	s.data[name] = data
	return nil
}

func (s *InMemoryStore) List(prefix string) ([]string, error) {
	var files []string
	for key := range s.data {
		if strings.HasPrefix(strings.ToLower(key), strings.ToLower(prefix)) {
			files = append(files, key)
		}
	}
	return files, nil
}

func (s *InMemoryStore) Delete(name string) error {
	delete(s.data, name)
	return nil
}

func (s *InMemoryStore) Get(name string) (io.Reader, error) {
	data, exists := s.data[name]
	if !exists {
		return nil, errors.Join(ErrFileNotFound, fmt.Errorf("file %s not found", name))
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string][]byte),
	}
}

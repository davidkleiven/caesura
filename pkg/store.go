package pkg

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

type FSStore struct {
	directory string
}

func (s *FSStore) Store(name string, r io.Reader) error {
	path := filepath.Join(s.directory, name)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", name, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, r); err != nil {
		return errors.Join(ErrRetrievingContent, err)
	}
	return nil
}

func (s *FSStore) List(prefix string) ([]string, error) {
	files, err := os.ReadDir(s.directory)
	var result []string
	if err != nil {
		return result, fmt.Errorf("error reading directory %s: %w", s.directory, err)
	}

	for _, file := range files {
		if strings.HasPrefix(strings.ToLower(file.Name()), strings.ToLower(prefix)) {
			result = append(result, file.Name())
		}
	}
	return result, nil
}

func (s *FSStore) Delete(name string) error {
	path := filepath.Join(s.directory, name)
	if err := os.Remove(path); err != nil {
		return nil
	}
	return nil
}

func (s *FSStore) Get(name string) (io.Reader, error) {
	path := filepath.Join(s.directory, name)
	file, err := os.Open(path)
	if err != nil {
		return bytes.NewBuffer([]byte{}), fmt.Errorf("error opening file %s: %w", name, err)
	}
	return file, nil
}

func NewFSStore(directory string) *FSStore {
	return &FSStore{
		directory: directory,
	}
}

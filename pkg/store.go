package pkg

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type StoreStatus string

const (
	StoreStatusPending  StoreStatus = "pending"
	StoreStatusFinished StoreStatus = "finished"
)

type Submitter interface {
	Submit(ctx context.Context, m *MetaData, r io.Reader) error
}

type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" {
		*d = 0
		return nil
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

type MetaData struct {
	Title           string      `json:"title"`
	Composer        string      `json:"composer"`
	Arranger        string      `json:"arranger"`
	Genre           string      `json:"genre"`
	Year            string      `json:"year"`
	Instrumentation string      `json:"instrumentation"`
	Duration        Duration    `json:"duration"`
	Publisher       string      `json:"publisher"`
	Isnm            string      `json:"ismn"`
	Tags            string      `json:"tags"`
	Notes           string      `json:"notes"`
	Status          StoreStatus `json:"status"`
}

func (m *MetaData) ResourceName() string {
	result := make([]string, 0, 3)
	if m.Title != "" {
		result = append(result, m.Title)
	}
	if m.Composer != "" {
		result = append(result, m.Composer)
	}
	if m.Arranger != "" {
		result = append(result, m.Arranger)
	}
	return SanitizeString(strings.Join(result, "_")) + ".zip"
}

func (m *MetaData) MarshalJSON() ([]byte, error) {
	type Alias MetaData
	return json.Marshal(&struct {
		*Alias
		Resource string `json:"resource"`
		Id       string `json:"id"`
	}{
		Alias:    (*Alias)(m),
		Resource: m.ResourceName(),
		Id:       m.ResourceId(),
	})
}

func (m *MetaData) UnmarshalJSON(data []byte) error {
	type Alias MetaData
	aux := &struct {
		*Alias
		Resource string `json:"resource"`
		Id       string `json:"id"`
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("error unmarshalling MetaData: %w", err)
	}

	if aux.Resource != "" && aux.Resource != m.ResourceName() {
		return fmt.Errorf("resource name mismatch: expected %s, got %s", m.ResourceName(), aux.Resource)
	}
	if aux.Id != "" && aux.Id != m.ResourceId() {
		return fmt.Errorf("resource ID mismatch: expected %s, got %s", m.ResourceId(), aux.Id)
	}
	return nil
}

func (m *MetaData) ResourceId() string {
	hash := md5.Sum([]byte(m.ResourceName()))
	return hex.EncodeToString(hash[:])
}

type Storer interface {
	Register(m *MetaData) error
	Store(name string, r io.Reader) error
	RegisterSuccess(Id string) error
}

var ErrFileNotFound = fmt.Errorf("file not found")
var ErrRetrievingContent = fmt.Errorf("error retrieving content")
var ErrUpdateMetadata = fmt.Errorf("error updating metadata")

type FSStore struct {
	directory string
	staged    map[string]MetaData
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

func (s *FSStore) Get(name string) (io.Reader, error) {
	path := filepath.Join(s.directory, name)
	file, err := os.Open(path)
	if err != nil {
		return bytes.NewBuffer([]byte{}), errors.Join(ErrFileNotFound, err)
	}
	return file, nil
}

func (s *FSStore) Register(m *MetaData) error {
	if _, ok := s.staged[m.ResourceId()]; ok {
		return ErrUpdateMetadata
	}
	s.staged[m.ResourceId()] = *m
	return nil
}

func (s *FSStore) RegisterSuccess(Id string) error {
	meta, exists := s.staged[Id]
	if !exists {
		return errors.Join(ErrResourceMetadataNotFound, fmt.Errorf("%s not found", Id))
	}
	meta.Status = StoreStatusFinished

	metaDataFile := strings.TrimSuffix(meta.ResourceName(), filepath.Ext(meta.ResourceName())) + ".json"
	metaDataPath := filepath.Join(s.directory, metaDataFile)
	file, err := os.Create(metaDataPath)
	if err != nil {
		return fmt.Errorf("error creating metadata file %s: %w", metaDataFile, err)
	}
	defer file.Close()

	metaDataBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling metadata to JSON: %w", err)
	}
	if _, err := file.Write(metaDataBytes); err != nil {
		return fmt.Errorf("error writing metadata to file %s: %w", metaDataFile, err)
	}
	delete(s.staged, Id)
	return nil
}

func NewFSStore(directory string) *FSStore {
	return &FSStore{
		directory: directory,
		staged:    make(map[string]MetaData),
	}
}

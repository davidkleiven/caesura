package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/davidkleiven/caesura/utils"
)

type InMemoryStore struct {
	Data     map[string][]byte
	Metadata []MetaData
	Projects map[string]Project
}

func (s *InMemoryStore) Submit(ctx context.Context, meta *MetaData, r io.Reader) error {
	if _, err := s.MetaById(ctx, meta.ResourceId()); err != nil {
		s.Metadata = append(s.Metadata, *meta)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return errors.Join(ErrRetrievingContent, err)
	}

	name := meta.ResourceName()
	if current, ok := s.Data[name]; ok {
		s.Data[name], err = NewZipAppender().Add(data).Add(current).Merge()
	} else {
		s.Data[name] = data
	}
	return err
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

func (s *InMemoryStore) ProjectsByName(ctx context.Context, name string) ([]Project, error) {
	var results []Project
	for _, project := range s.Projects {
		if name == "" || strings.HasPrefix(strings.ToLower(project.Name), strings.ToLower(name)) {
			results = append(results, project)
		}
	}
	return results, nil
}

func (s *InMemoryStore) SubmitProject(ctx context.Context, project *Project) error {
	projectId := project.Id()
	if existingProject, exists := s.Projects[projectId]; exists {
		existingProject.Merge(project)
		s.Projects[projectId] = existingProject
	} else {
		s.Projects[projectId] = *project
	}
	return nil
}

func (s *InMemoryStore) ProjectById(ctx context.Context, id string) (*Project, error) {
	if project, exists := s.Projects[id]; exists {
		return &project, nil
	}
	return &Project{}, errors.Join(ErrProjectNotFound, fmt.Errorf("Project ID: %s", id))
}

func (s *InMemoryStore) RemoveResource(ctx context.Context, projectId string, resourceId string) error {
	project, ok := s.Projects[projectId]
	if !ok {
		return errors.Join(ErrProjectNotFound, fmt.Errorf("Project ID: %s", projectId))
	}

	project.ResourceIds = slices.DeleteFunc(project.ResourceIds, func(item string) bool {
		return item == resourceId
	})
	s.Projects[projectId] = project
	return nil
}

func (s *InMemoryStore) MetaById(ctx context.Context, id string) (*MetaData, error) {
	for _, meta := range s.Metadata {
		if meta.ResourceId() == id {
			return &meta, nil
		}
	}
	return &MetaData{}, errors.Join(ErrResourceMetadataNotFound, fmt.Errorf("metadata with id %s not found", id))
}

func (s *InMemoryStore) Resource(ctx context.Context, name string) (io.Reader, error) {
	content, exists := s.Data[name]
	if !exists {
		return nil, errors.Join(ErrResourceNotFound, fmt.Errorf("resource %s", name))
	}
	return bytes.NewReader(content), nil
}

func (s *InMemoryStore) Clone() *InMemoryStore {
	dst := NewInMemoryStore()
	for _, v := range s.Metadata {
		var meta MetaData
		data := utils.Must(json.Marshal(v))
		PanicOnErr(json.Unmarshal(data, &meta))
		dst.Metadata = append(dst.Metadata, meta)
	}

	for k, v := range s.Projects {
		var project Project
		data := utils.Must(json.Marshal(v))
		PanicOnErr(json.Unmarshal(data, &project))
		dst.Projects[k] = project
	}

	for k, v := range s.Data {
		dst.Data[k] = make([]byte, len(v))
		copy(dst.Data[k], v)
	}

	return dst
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		Data:     make(map[string][]byte),
		Metadata: []MetaData{},
		Projects: make(map[string]Project),
	}
}

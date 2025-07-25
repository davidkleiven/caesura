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
	"time"

	"github.com/davidkleiven/caesura/utils"
)

type InMemoryStore struct {
	Data          map[string][]byte
	Metadata      []MetaData
	Projects      map[string]Project
	Organizations []Organization
	Users         []UserRole
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
	return &Project{}, fmt.Errorf("project with id %s not found", id)
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

	copy(dst.Users, s.Users)
	copy(dst.Organizations, s.Organizations)
	return dst
}

func (s *InMemoryStore) GetRole(ctx context.Context, userId string) (*UserRole, error) {
	for _, role := range s.Users {
		if role.UserId == userId {
			return &role, nil
		}
	}
	return &UserRole{}, errors.Join(ErrUserNotFound, fmt.Errorf("user id: %s", userId))
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		Data:          make(map[string][]byte),
		Metadata:      []MetaData{},
		Projects:      make(map[string]Project),
		Users:         []UserRole{},
		Organizations: []Organization{},
	}
}

func NewDemoStore() *InMemoryStore {
	store := NewInMemoryStore()
	store.Metadata = []MetaData{
		{Title: "Demo Title 1", Composer: "Composer A", Arranger: "Arranger X"},
		{Title: "Demo Title 2", Composer: "Composer B", Arranger: "Arranger Y"},
	}
	store.Data[store.Metadata[0].ResourceName()] = MustCreateResource(5)
	store.Data[store.Metadata[1].ResourceName()] = MustCreateResource(3)

	projectDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	project := Project{
		Name:        "Demo Project 1",
		ResourceIds: []string{store.Metadata[0].ResourceId(), store.Metadata[1].ResourceId()},
		CreatedAt:   projectDate,
		UpdatedAt:   projectDate,
	}
	store.Projects[project.Id()] = project

	store.Users = []UserRole{
		{
			UserId: "217f40fa-c0d7-4d8e-a284-293347868289",
			Roles: map[string]RoleKind{
				"9eab9a97-06a3-42a7-ae1e-7c67df5cbec7": RoleViewer,
				"cccc13f9-ddd5-489e-bd77-3b935b457f71": RoleAdmin,
			},
		},
		{
			UserId: "6b2d9876-0bc4-407a-8f76-4fb1ad2a523b",
			Roles: map[string]RoleKind{
				"cccc13f9-ddd5-489e-bd77-3b935b457f71": RoleEditor,
			},
		},
	}

	store.Organizations = []Organization{
		{
			Id:   "9eab9a97-06a3-42a7-ae1e-7c67df5cbec7",
			Name: "My organization 1",
		},
		{
			Id:   "cccc13f9-ddd5-489e-bd77-3b935b457f71",
			Name: "My organization 2",
		},
	}
	return store
}

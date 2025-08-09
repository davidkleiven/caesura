package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/davidkleiven/caesura/utils"
)

func NewDemoStore() *MultiOrgInMemoryStore {
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

	multiOrgStore := NewMultiOrgInMemoryStore()
	multiOrgStore.Data["9eab9a97-06a3-42a7-ae1e-7c67df5cbec7"] = *store
	multiOrgStore.Data["cccc13f9-ddd5-489e-bd77-3b935b457f71"] = *store

	multiOrgStore.Users = []UserInfo{
		{
			Id: "217f40fa-c0d7-4d8e-a284-293347868289",
			Roles: map[string]RoleKind{
				"9eab9a97-06a3-42a7-ae1e-7c67df5cbec7": RoleViewer,
				"cccc13f9-ddd5-489e-bd77-3b935b457f71": RoleAdmin,
			},
			Groups: map[string][]string{
				"cccc13f9-ddd5-489e-bd77-3b935b457f71": {"Alto"},
			},
			Name: "Susan",
		},
		{
			Id: "6b2d9876-0bc4-407a-8f76-4fb1ad2a523b",
			Roles: map[string]RoleKind{
				"cccc13f9-ddd5-489e-bd77-3b935b457f71": RoleEditor,
			},
			Groups: map[string][]string{
				"cccc13f9-ddd5-489e-bd77-3b935b457f71": {"Tenor", "Bass"},
			},
			Name: "John",
		},
	}

	multiOrgStore.Organizations = []Organization{
		{
			Id:   "9eab9a97-06a3-42a7-ae1e-7c67df5cbec7",
			Name: "My organization 1",
		},
		{
			Id:   "cccc13f9-ddd5-489e-bd77-3b935b457f71",
			Name: "My organization 2",
		},
	}
	return multiOrgStore
}

type MultiOrgInMemoryStore struct {
	Data          map[string]InMemoryStore
	Users         []UserInfo
	Organizations []Organization
}

func (m *MultiOrgInMemoryStore) Submit(ctx context.Context, orgId string, meta *MetaData, r io.Reader) error {
	store, ok := m.Data[orgId]
	if !ok {
		return ErrOrganizationNotFound
	}
	return store.Submit(ctx, meta, r)
}

func (m *MultiOrgInMemoryStore) MetaByPattern(ctx context.Context, orgId string, pattern *MetaData) ([]MetaData, error) {
	store, ok := m.Data[orgId]
	if !ok {
		return []MetaData{}, ErrOrganizationNotFound
	}
	return store.MetaByPattern(ctx, pattern)
}

func (m *MultiOrgInMemoryStore) ProjectsByName(ctx context.Context, orgId, name string) ([]Project, error) {
	store, ok := m.Data[orgId]
	if !ok {
		return []Project{}, ErrOrganizationNotFound
	}
	return store.ProjectsByName(ctx, name)
}

func (m *MultiOrgInMemoryStore) SubmitProject(ctx context.Context, orgId string, project *Project) error {
	store, ok := m.Data[orgId]
	if !ok {
		return ErrOrganizationNotFound
	}
	return store.SubmitProject(ctx, project)
}

func (m *MultiOrgInMemoryStore) ProjectById(ctx context.Context, orgId, id string) (*Project, error) {
	store, ok := m.Data[orgId]
	if !ok {
		return &Project{}, ErrOrganizationNotFound
	}
	return store.ProjectById(ctx, id)
}

func (m *MultiOrgInMemoryStore) RemoveResource(ctx context.Context, orgId, projectId, resourceId string) error {
	store, ok := m.Data[orgId]
	if !ok {
		return ErrOrganizationNotFound
	}
	return store.RemoveResource(ctx, projectId, resourceId)
}

func (m *MultiOrgInMemoryStore) MetaById(ctx context.Context, orgId, id string) (*MetaData, error) {
	store, ok := m.Data[orgId]
	if !ok {
		return &MetaData{}, ErrOrganizationNotFound
	}
	return store.MetaById(ctx, id)
}

func (m *MultiOrgInMemoryStore) Resource(ctx context.Context, orgId, name string) (io.Reader, error) {
	store, ok := m.Data[orgId]
	if !ok {
		return nil, ErrOrganizationNotFound
	}
	return store.Resource(ctx, name)
}

func (m *MultiOrgInMemoryStore) Clone() *MultiOrgInMemoryStore {
	dst := NewMultiOrgInMemoryStore()

	for orgId, store := range m.Data {
		dst.Data[orgId] = *store.Clone()
	}

	dst.Users = make([]UserInfo, len(m.Users))
	for i, users := range m.Users {
		data := utils.Must(json.Marshal(users))
		PanicOnErr(json.Unmarshal(data, &dst.Users[i]))
	}

	dst.Organizations = make([]Organization, len(m.Organizations))
	copy(dst.Organizations, m.Organizations)

	return dst
}

func (m *MultiOrgInMemoryStore) GetUserInfo(ctx context.Context, userId string) (*UserInfo, error) {
	for _, role := range m.Users {
		if role.Id == userId {
			return &role, nil
		}
	}
	return NewUserInfo(), errors.Join(ErrUserNotFound, fmt.Errorf("user id: %s", userId))
}

func (m *MultiOrgInMemoryStore) RegisterRole(ctx context.Context, userId string, organizationId string, role RoleKind) error {
	for i, u := range m.Users {
		if u.Id == userId {
			m.Users[i].Roles[organizationId] = role
			return nil
		}
	}

	m.Users = append(m.Users, UserInfo{
		Id: userId,
		Roles: map[string]RoleKind{
			organizationId: role,
		},
	})
	return nil
}

func (m *MultiOrgInMemoryStore) RegisterOrganization(ctx context.Context, org *Organization) error {
	m.Data[org.Id] = *NewInMemoryStore()
	m.Organizations = append(m.Organizations, *org)
	return nil
}

func (m *MultiOrgInMemoryStore) RegisterUser(ctx context.Context, user *UserInfo) error {
	// Make copies because there is some special handling on nullable fields
	var userContent UserInfo
	data := utils.Must(json.Marshal(user))
	PanicOnErr(json.Unmarshal(data, &userContent))
	m.Users = append(m.Users, userContent)
	return nil
}

func (m *MultiOrgInMemoryStore) FirstOrganizationId() string {
	for k := range m.Data {
		return k
	}
	return ""
}

func (m *MultiOrgInMemoryStore) FirstDataStore() *InMemoryStore {
	for _, v := range m.Data {
		return &v
	}
	return &InMemoryStore{}
}

func (m *MultiOrgInMemoryStore) GetOrganization(ctx context.Context, orgId string) (Organization, error) {
	for _, org := range m.Organizations {
		if org.Id == orgId && !org.Deleted {
			return org, nil
		}
	}
	return Organization{}, ErrOrganizationNotFound
}

func (m *MultiOrgInMemoryStore) DeleteOrganization(ctx context.Context, orgId string) error {
	for i, org := range m.Organizations {
		if org.Id == orgId {
			m.Organizations[i].Deleted = true
		}
	}
	return nil
}

func NewMultiOrgInMemoryStore() *MultiOrgInMemoryStore {
	return &MultiOrgInMemoryStore{
		Data:          make(map[string]InMemoryStore),
		Users:         []UserInfo{},
		Organizations: []Organization{},
	}
}

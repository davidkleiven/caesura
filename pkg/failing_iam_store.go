package pkg

import "context"

type MockIAMStore struct {
	ErrRegisterUser         error
	ErrGetUserInfo          error
	ErrRegisterRole         error
	ErrGetOrganization      error
	ErrRegisterOrganization error
	ErrDeleteOrganization   error
	ErrUserInOrg            error
}

func (m *MockIAMStore) RegisterUser(ctx context.Context, userInfo *UserInfo) error {
	return m.ErrRegisterUser
}

func (m *MockIAMStore) GetUserInfo(ctx context.Context, userId string) (*UserInfo, error) {
	return &UserInfo{Id: userId, Name: "Mock User"}, m.ErrGetUserInfo
}

func (m *MockIAMStore) RegisterRole(ctx context.Context, userId string, organizationId string, role RoleKind) error {
	return m.ErrRegisterRole
}

func (m *MockIAMStore) GetOrganization(ctx context.Context, orgId string) (Organization, error) {
	return Organization{Id: orgId, Name: "Mock Org"}, m.ErrGetOrganization
}

func (m *MockIAMStore) RegisterOrganization(ctx context.Context, org *Organization) error {
	return m.ErrRegisterOrganization
}

func (m *MockIAMStore) DeleteOrganization(ctx context.Context, orgId string) error {
	return m.ErrDeleteOrganization
}

func (m *MockIAMStore) GetUsersInOrg(ctx context.Context, orgId string) ([]UserInfo, error) {
	return []UserInfo{}, m.ErrUserInOrg
}

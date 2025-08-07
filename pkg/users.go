package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/davidkleiven/caesura/utils"
	"github.com/gorilla/sessions"
)

type UserInfo struct {
	Id            string              `json:"id"`
	Email         string              `json:"email,omitempty"`
	VerifiedEmail bool                `json:"verified_email,omitempty"`
	Name          string              `json:"name,omitempty"`
	Roles         map[string]RoleKind `json:"roles,omitempty"`
	Groups        map[string][]string `json:"groups,omitempty"`
}

func (u *UserInfo) UnmarshalJSON(data []byte) error {
	type Alias UserInfo
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(u),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if u.Roles == nil {
		u.Roles = make(map[string]RoleKind)
	}

	if u.Groups == nil {
		u.Groups = make(map[string][]string)
	}
	return nil
}

func NewUserInfo() *UserInfo {
	return &UserInfo{Roles: make(map[string]RoleKind), Groups: make(map[string][]string)}
}

type Organization struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

type RoleKind int

const (
	RoleViewer = iota
	RoleEditor
	RoleAdmin
)

type UserRegisterer interface {
	RegisterUser(ctx context.Context, userInfo *UserInfo) error
}

type RoleGetter interface {
	GetUserInfo(ctx context.Context, userId string) (*UserInfo, error)
}

type RoleRegisterer interface {
	RegisterRole(ctx context.Context, userId string, organizationId string, role RoleKind) error
}

type RoleStore interface {
	RoleGetter
	RoleRegisterer
	UserRegisterer
}

type OrganizationGetter interface {
	GetOrganization(ctx context.Context, orgId string) (Organization, error)
}

type OrganizationRegisterer interface {
	RegisterOrganization(ctx context.Context, org *Organization) error
}

type OrganizationDeleter interface {
	DeleteOrganization(ctx context.Context, orgId string) error
}

type OrganizationStore interface {
	OrganizationGetter
	OrganizationRegisterer
	OrganizationDeleter
}

type IAMStore interface {
	RoleStore
	OrganizationStore
}

func GetUserOrRegisterNewUser(store RoleStore, ctx context.Context, info *UserInfo) (*UserInfo, error) {
	existingUser, err := store.GetUserInfo(ctx, info.Id)
	if errors.Is(err, ErrUserNotFound) {
		if err := store.RegisterUser(ctx, info); err != nil {
			return info, err
		}

		// After registration "existingUser" will be the new suer
		existingUser = info
	} else if err != nil {
		return info, err
	}
	return existingUser, nil
}

// AddRoleIfNotExist adds a role to the passed user. And also registers the role for later reference
// If registration fails, an error is returned
func AddRoleIfNotExist(store RoleRegisterer, ctx context.Context, orgId string, userInfo *UserInfo) error {
	if _, hasRole := userInfo.Roles[orgId]; !hasRole && orgId != "" {
		// User does not have a role in the organization shared via invite link
		if err := store.RegisterRole(ctx, userInfo.Id, orgId, RoleViewer); err != nil {
			return err
		}
		userInfo.Roles[orgId] = RoleViewer
	}
	return nil
}

type UserRolePipeline struct {
	Error error
	User  *UserInfo
	store RoleStore
	ctx   context.Context
}

func NewUserRolePipeline(store RoleStore, ctx context.Context, info *UserInfo) *UserRolePipeline {
	return &UserRolePipeline{
		store: store,
		ctx:   ctx,
		User:  info,
	}
}

func (u *UserRolePipeline) RegisterIfMissing() *UserRolePipeline {
	if u.Error != nil {
		return u
	}
	u.User, u.Error = GetUserOrRegisterNewUser(u.store, u.ctx, u.User)
	return u
}

func (u *UserRolePipeline) AssignViewRoleIfNoRole(orgId string) *UserRolePipeline {
	if u.Error != nil {
		return u
	}
	u.Error = AddRoleIfNotExist(u.store, u.ctx, orgId, u.User)
	return u
}

type FailingRoleStore struct {
	ErrRegisterUser error
	ErrRegisterRole error
	ErrGetUserRole  error
}

func (frs *FailingRoleStore) RegisterUser(ctx context.Context, user *UserInfo) error {
	return frs.ErrRegisterUser
}

func (frs *FailingRoleStore) RegisterRole(ctx context.Context, userId, orgId string, role RoleKind) error {
	return frs.ErrRegisterRole
}

func (frs *FailingRoleStore) GetUserInfo(ctx context.Context, userId string) (*UserInfo, error) {
	return NewUserInfo(), frs.ErrGetUserRole
}

type RegisterOrganizationFlow struct {
	session  *sessions.Session
	store    IAMStore
	userInfo *UserInfo
	ctx      context.Context
	Error    error
}

func NewRegisterOrganizationFlow(ctx context.Context, store IAMStore, s *sessions.Session) *RegisterOrganizationFlow {
	return &RegisterOrganizationFlow{
		session: s,
		store:   store,
		ctx:     ctx,
	}
}

func (r *RegisterOrganizationFlow) Register(o *Organization) *RegisterOrganizationFlow {
	if r.Error != nil {
		return r
	}
	r.Error = r.store.RegisterOrganization(r.ctx, o)
	return r
}

func (r *RegisterOrganizationFlow) RegisterAdmin(userId, orgId string) *RegisterOrganizationFlow {
	if r.Error != nil {
		return r
	}

	r.Error = r.store.RegisterRole(r.ctx, userId, orgId, RoleAdmin)
	return r
}

func (r *RegisterOrganizationFlow) RetrieveUserInfo(userId string) *RegisterOrganizationFlow {
	if r.Error != nil {
		return r
	}

	r.userInfo, r.Error = r.store.GetUserInfo(r.ctx, userId)
	return r
}

func (r *RegisterOrganizationFlow) UpdateSession(req *http.Request, w http.ResponseWriter, orgId string) *RegisterOrganizationFlow {
	if r.Error != nil {
		return r
	}
	PopulateSessionWithRoles(r.session, r.userInfo)
	r.session.Values["orgId"] = orgId
	r.Error = r.session.Save(req, w)
	return r
}

func PopulateSessionWithRoles(session *sessions.Session, userInfo *UserInfo) {
	// Ensure unnessceary fields are set to empty (e.g. omitted in the cookie)
	userInfo.Email = ""
	userInfo.Name = ""
	userInfo.VerifiedEmail = false
	userInfo.Groups = nil

	userInfoJson := utils.Must(json.Marshal(userInfo))
	session.Values["role"] = userInfoJson

	for orgId := range userInfo.Roles {
		session.Values["orgId"] = orgId
		break
	}
}

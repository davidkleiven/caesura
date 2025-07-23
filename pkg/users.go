package pkg

import "context"

type UserInfo struct {
	Id            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
}

type Organization struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type RoleKind int

const (
	RoleViewer = iota
	RoleEditor
	RoleAdmin
)

type UserRole struct {
	UserId string              `json:"userId"`
	Roles  map[string]RoleKind `json:"roles"`
}

type RoleGetter interface {
	GetRole(ctx context.Context, userId string) (*UserRole, error)
}

type RoleRegisterer interface {
	RegisterRole(ctx context.Context, userId string, organizationId string, role RoleKind) error
}

type RoleStore interface {
	RoleGetter
	RoleRegisterer
}

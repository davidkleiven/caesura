package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
	"github.com/davidkleiven/caesura/utils"
	"github.com/gorilla/sessions"
)

func TestUnmarshalUserInfo(t *testing.T) {
	var info UserInfo
	if err := info.UnmarshalJSON([]byte("not JSON")); err == nil {
		t.Fatalf("Wanted unmarshling to fail")
	}
}

func TestRolesAreAssignableWhenMissing(t *testing.T) {
	var info UserInfo
	jsonData := []byte(`{"id": "my-id"}`)
	if err := json.Unmarshal(jsonData, &info); err != nil {
		t.Fatal(err)
	}

	info.Roles["someOrg"] = RoleAdmin
}

func TestGetOrRegisterNewUser(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	newUser := NewUserInfo()
	user, err := GetUserOrRegisterNewUser(store, context.Background(), newUser)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(user, newUser) {
		t.Fatalf("New user and existing user should be the same got %+v want %+v", user, newUser)
	}
}

func TestNewUserWhenRegisterFails(t *testing.T) {
	store := FailingRoleStore{
		ErrGetUserRole:  ErrUserNotFound,
		ErrRegisterUser: errors.New("could not register user"),
	}

	info := NewUserInfo()
	recievedInfo, err := GetUserOrRegisterNewUser(&store, context.Background(), info)
	if !errors.Is(err, store.ErrRegisterUser) {
		t.Fatalf("Error was not propagated. Wanted '%s' got '%s'", store.ErrRegisterUser, err)
	}

	if !reflect.DeepEqual(info, recievedInfo) {
		t.Fatalf("Wanted the new user returned")
	}
}

func TestNewUserWhenGetUserFailsWithUnknownError(t *testing.T) {
	store := FailingRoleStore{
		ErrGetUserRole: errors.New("some unexpected error occured"),
	}

	info := NewUserInfo()
	recievedInfo, err := GetUserOrRegisterNewUser(&store, context.Background(), info)
	if !errors.Is(err, store.ErrGetUserRole) {
		t.Fatalf("Error was not propagated. Wanted '%s' got '%s'", store.ErrGetUserRole, err)
	}

	if !reflect.DeepEqual(info, recievedInfo) {
		t.Fatalf("Wanted the new user returned")
	}
}

func TestAddRoleIfNotExist(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	user := UserInfo{
		Id:    "0000-0000",
		Roles: map[string]RoleKind{"1111-1111": RoleEditor},
	}

	store.RegisterUser(context.Background(), &user)

	for _, test := range []struct {
		wantErr  error
		orgId    string
		wantRole RoleKind
		desc     string
	}{
		{
			orgId:    "1111-1111",
			wantRole: RoleEditor,
			desc:     "Has role",
		},
		{
			orgId:    "2111-1111",
			wantRole: RoleViewer,
			desc:     "No role",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			err := AddRoleIfNotExist(store, context.Background(), test.orgId, &user)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Wanted '%s' got '%s'", test.wantErr, err)
			}

			role, ok := user.Roles[test.orgId]
			if !ok {
				t.Fatalf("User should always have a role the organization")
			}

			if role != test.wantRole {
				t.Fatalf("Wanted '%d' got '%d'", test.wantRole, role)
			}
		})

	}
}

func TestRolesPersistWhenRegisteringFails(t *testing.T) {
	store := FailingRoleStore{
		ErrRegisterRole: errors.New("unexpected error"),
	}

	user := NewUserInfo()
	user.Id = "0000-0000"

	err := AddRoleIfNotExist(&store, context.Background(), "111", user)
	if !errors.Is(err, store.ErrRegisterRole) {
		t.Fatalf("Error was not propagated. Wanted '%s' got '%s'", store.ErrGetUserRole, err)
	}
}

func TestRoelsNotAssignedToEmptyOrgId(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	user := NewUserInfo()
	user.Id = "1111"
	err := AddRoleIfNotExist(store, context.Background(), "", user)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := user.Roles[""]
	if ok {
		t.Fatalf("Roles should be assigned to empty organization id")
	}
}

func TestUserPipeRolePipeline(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	user := NewUserInfo()
	pipeline := NewUserRolePipeline(store, context.Background(), user).
		RegisterIfMissing().
		AssignViewRoleIfNoRole("0000")

	if pipeline.Error != nil {
		t.Fatalf("Should not fail")
	}
}

func TestErrorPropagatedThroughUserRolePipeline(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	user := NewUserInfo()
	pipeline := NewUserRolePipeline(store, context.Background(), user)

	err := errors.New("unexpected error")
	pipeline.Error = err

	pipeline.RegisterIfMissing().AssignViewRoleIfNoRole("000")
	if !errors.Is(pipeline.Error, err) {
		t.Fatalf("Wanted '%s' got '%s'", err, pipeline.Error)
	}
}

func TestRegisterOrganizationFlow(t *testing.T) {
	r := httptest.NewRequest("GET", "/organizations", nil)
	w := httptest.NewRecorder()
	store := NewMultiOrgInMemoryStore()
	cookie := sessions.NewCookieStore([]byte("top-secret"))
	session, err := cookie.Get(r, "session-name")
	testutils.AssertNil(t, err)

	org := Organization{
		Id:   "111",
		Name: "Some organization",
	}
	user := UserInfo{
		Id:    "dddd",
		Roles: make(map[string]RoleKind),
	}

	store.RegisterUser(context.Background(), &user)
	registrationFlow := NewRegisterOrganizationFlow(context.Background(), store, session)
	registrationFlow.Register(&org).RegisterAdmin(user.Id, org.Id).RetrieveUserInfo(user.Id).UpdateSession(r, w, org.Id)
	testutils.AssertEqual(t, org.Id, session.Values["orgId"].(string))
	testutils.AssertEqual(t, store.Users[0].Roles[org.Id], RoleAdmin)
	testutils.AssertEqual(t, utils.Must(store.GetUserInfo(context.Background(), user.Id)).Roles[org.Id], RoleAdmin)
}

func TestOrganizationFlowAbortedOnError(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	session := &sessions.Session{}

	org := Organization{}
	user := NewUserInfo()
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	registrationFlow := NewRegisterOrganizationFlow(context.Background(), store, session)
	err := errors.New("unexected error")
	registrationFlow.Error = err
	registrationFlow.Register(&org).RegisterAdmin(user.Id, org.Id).RetrieveUserInfo(user.Id).UpdateSession(r, w, org.Id)

	if !errors.Is(registrationFlow.Error, err) {
		t.Fatalf("Wanted '%s' got '%s'", err, registrationFlow.Error)
	}
}

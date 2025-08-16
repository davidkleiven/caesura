package pkg

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
)

func TestMultiOrgErrorHandling(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	ctx := context.Background()
	meta := MetaData{}
	data := func(yield func(string, []byte) bool) {}

	for _, test := range []struct {
		fn             func(orgId string) error
		desc           string
		afterOrgRegErr error
	}{
		{
			fn:   func(orgId string) error { return store.Submit(ctx, orgId, &meta, data) },
			desc: "Submit",
		},
		{
			fn: func(orgId string) error {
				_, err := store.MetaByPattern(ctx, orgId, &meta)
				return err
			},
			desc: "MetaByPattern",
		},
		{
			fn: func(orgId string) error {
				_, err := store.ProjectsByName(ctx, orgId, "myProject")
				return err
			},
			desc: "ProjectsByName",
		},
		{
			fn:   func(orgId string) error { return store.SubmitProject(ctx, orgId, &Project{}) },
			desc: "SubmitProject",
		},
		{
			fn: func(orgId string) error {
				_, err := store.ProjectById(ctx, orgId, "someProject")
				return err
			},
			desc:           "ProjectById",
			afterOrgRegErr: ErrProjectNotFound,
		},
		{
			fn:             func(orgId string) error { return store.RemoveResource(ctx, orgId, "someProject", "someResource") },
			desc:           "RemoveResource",
			afterOrgRegErr: ErrProjectNotFound,
		},
		{
			fn: func(orgId string) error {
				_, err := store.MetaById(ctx, orgId, "someResourceId")
				return err
			},
			desc:           "MetaById",
			afterOrgRegErr: ErrResourceMetadataNotFound,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			orgId := test.desc
			err := test.fn(orgId)
			if !errors.Is(err, ErrOrganizationNotFound) {
				t.Fatalf("Wanted '%s' got '%s'", ErrOrganizationNotFound, err)
			}

			store.RegisterOrganization(ctx, &Organization{Id: orgId})
			err = test.fn(orgId)
			if !errors.Is(err, test.afterOrgRegErr) {
				t.Fatalf("Wanted '%s' got '%s'", test.afterOrgRegErr, err)
			}
		})
	}
}

func TestMultiOrgClone(t *testing.T) {
	for _, test := range []struct {
		modifier  func(s *MultiOrgInMemoryStore)
		wantEqual bool
		desc      string
	}{
		{
			modifier:  func(s *MultiOrgInMemoryStore) {},
			wantEqual: true,
			desc:      "No modification",
		},
		{
			modifier: func(s *MultiOrgInMemoryStore) {
				s.Users = s.Users[1:]
			},
			desc: "Remove a user",
		},
		{
			modifier: func(s *MultiOrgInMemoryStore) {
				s.Users[0].Id = "otherId"
			},
			desc: "Edit user a user id",
		},
		{
			modifier: func(s *MultiOrgInMemoryStore) {
				var orgId string
				for k := range s.Users[0].Roles {
					orgId = k
					break
				}
				s.Users[0].Roles[orgId] += 1
			},
			desc: "Edit role",
		},
		{
			modifier: func(s *MultiOrgInMemoryStore) {
				s.Organizations[0].Id = "otherOrgId"
			},
			desc: "Edit organization",
		},
		{
			modifier: func(s *MultiOrgInMemoryStore) {
				var orgId string
				for k := range s.Users[0].Roles {
					orgId = k
					break
				}
				s.Users[0].Groups[orgId] = append(s.Users[0].Groups[orgId], "Flute")
			},
			desc: "Edit groups",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			store := NewDemoStore()
			clone := store.Clone()
			test.modifier(clone)

			if reflect.DeepEqual(store, clone) != test.wantEqual {
				t.Fatalf("Wanted equaal %v bot got %v", test.wantEqual, !test.wantEqual)
			}
		})
	}
}

func TestGetUserInfo(t *testing.T) {
	store := NewDemoStore()
	ctx := context.Background()
	user, err := store.GetUserInfo(ctx, "non-existent-id")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("Wanted '%s' got '%s'", ErrUserNotFound, err)
	}

	if user == nil {
		t.Fatal("User should not be nil even when it is not found")
	}

	userId := "6b2d9876-0bc4-407a-8f76-4fb1ad2a523b"
	user, err = store.GetUserInfo(ctx, userId)
	if err != nil {
		t.Fatalf("User should be found")
	}

	if user.Id != userId {
		t.Fatalf("Wanted '%s' got'%s'", userId, user.Id)
	}
}

func TestRegisterRole(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	store.Users = []UserInfo{
		{
			Id: "user1",
			Roles: map[string]RoleKind{
				"org": RoleEditor,
			},
		},
	}

	ctx := context.Background()
	store.RegisterRole(ctx, "user1", "org", RoleAdmin)

	storedRole := store.Users[0].Roles["org"]
	if storedRole != RoleAdmin {
		t.Fatalf("Wanted '%d' got '%d'", RoleAdmin, storedRole)
	}

	store.RegisterRole(ctx, "user2", "org", RoleEditor)
	if len(store.Users) != 2 {
		t.Fatal("Wanted new user to be registered")
	}
}

func TestEmptyIdWhenNoOrganizations(t *testing.T) {
	id := NewMultiOrgInMemoryStore().FirstOrganizationId()

	if id != "" {
		t.Fatalf("Wannted empty string got '%s'", id)
	}
}

func TestEmptyInMemStoreWhenNotExist(t *testing.T) {
	store := NewMultiOrgInMemoryStore().FirstDataStore()
	if store == nil {
		t.Fatal("Store should not be nil when no store exist")
	}
}

func TestGetOrganization(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	store.Organizations = []Organization{
		{
			Id:   "0000",
			Name: "zero org",
		},
		{
			Id:      "0001",
			Name:    "one org",
			Deleted: true,
		},
	}

	ctx := context.Background()

	t.Run("Not existing", func(t *testing.T) {
		for _, id := range []string{"1000", "0001"} {
			org, err := store.GetOrganization(ctx, id)
			testutils.AssertEqual(t, org.Id, "")
			if !errors.Is(err, ErrOrganizationNotFound) {
				t.Fatalf("Wanted %s got %s", err, ErrOrganizationNotFound)
			}
		}

	})

	t.Run("Fetch existing", func(t *testing.T) {
		org, err := store.GetOrganization(ctx, "0000")
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, org.Id, "0000")
	})
}

func TestDeleteOrganization(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	store.Organizations = []Organization{
		{
			Id:   "0000",
			Name: "zero org",
		},
	}
	err := store.DeleteOrganization(context.Background(), "0000")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, true, store.Organizations[0].Deleted)
}

func TestEmptyIteratorOnMissingOrg(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	num := 0
	for range store.Resource(context.Background(), "whatever", "whatever") {
		num++
	}
	testutils.AssertEqual(t, num, 0)
}

func TestGetUsersInOrg(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	store.Users = []UserInfo{
		{
			Id:    "0000-0000",
			Roles: map[string]RoleKind{"org1": RoleAdmin},
		},
		*NewUserInfo(),
		{
			Id:    "0000-1000",
			Roles: map[string]RoleKind{"org1": RoleAdmin},
		},
	}

	users, err := store.GetUsersInOrg(context.Background(), "org1")
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, len(users), 2)
}

func TestDeleteRole(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	store.Users = []UserInfo{
		{
			Id: "0000",
			Roles: map[string]RoleKind{
				"org1": RoleAdmin,
				"org2": RoleEditor,
			},
		},
	}

	err := store.DeleteRole(context.Background(), "0000", "org1")
	testutils.AssertNil(t, err)
	_, exists := store.Users[0].Roles["org1"]
	testutils.AssertEqual(t, exists, false)
	_, exists = store.Users[0].Roles["org2"]
	testutils.AssertEqual(t, exists, true)
}

func TestRegisterGroup(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	store.Users = []UserInfo{
		{
			Id:     "0000",
			Groups: make(map[string][]string),
		},
		{
			Id: "0001",
			Groups: map[string][]string{
				"org1": {"Tenor"},
			},
		},
	}

	t.Run("non existing", func(t *testing.T) {
		err := store.RegisterGroup(context.Background(), "non-existing", "org", "group")
		testutils.AssertNil(t, err)
	})

	t.Run("add to user without groups", func(t *testing.T) {
		err := store.RegisterGroup(context.Background(), "0000", "org5", "Alto")
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, store.Users[0].Groups["org5"][0], "Alto")
	})

	t.Run("add to user with former groups", func(t *testing.T) {
		err := store.RegisterGroup(context.Background(), "0001", "org1", "Alto")
		testutils.AssertNil(t, err)
		groups := store.Users[1].Groups["org1"]
		testutils.AssertEqual(t, slices.Compare(groups, []string{"Tenor", "Alto"}), 0)
	})
}

func TestRemoveGroup(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	store.Users = []UserInfo{
		{
			Id: "0000",
			Groups: map[string][]string{
				"org1": {"Tenor", "Alto", "Soprano"},
			},
		},
	}

	err := store.RemoveGroup(context.Background(), "0000", "org1", "Alto")
	testutils.AssertNil(t, err)
	want := []string{"Tenor", "Soprano"}
	has := store.Users[0].Groups["org1"]
	if slices.Compare(has, want) != 0 {
		t.Fatalf("Watned\n%v\ngot%v\n", want, has)
	}

}

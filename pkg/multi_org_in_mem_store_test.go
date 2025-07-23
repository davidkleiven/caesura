package pkg

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestMultiOrgErrorHandling(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	ctx := context.Background()
	meta := MetaData{}
	data := bytes.NewBuffer([]byte{})

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
		{
			fn: func(orgId string) error {
				_, err := store.Resource(ctx, orgId, "someResourceId")
				return err
			},
			desc:           "Resource",
			afterOrgRegErr: ErrResourceNotFound,
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
				s.Users[0].UserId = "otherId"
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

func TestGetRole(t *testing.T) {
	store := NewDemoStore()
	ctx := context.Background()
	user, err := store.GetRole(ctx, "non-existent-id")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("Wanted '%s' got '%s'", ErrUserNotFound, err)
	}

	if user == nil {
		t.Fatal("User should not be nil even when it is not found")
	}

	userId := "6b2d9876-0bc4-407a-8f76-4fb1ad2a523b"
	user, err = store.GetRole(ctx, userId)
	if err != nil {
		t.Fatalf("User should be found")
	}

	if user.UserId != userId {
		t.Fatalf("Wanted '%s' got'%s'", userId, user.UserId)
	}
}

func TestRegisterRole(t *testing.T) {
	store := NewMultiOrgInMemoryStore()
	store.Users = []UserRole{
		{
			UserId: "user1",
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

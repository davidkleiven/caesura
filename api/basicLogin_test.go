package api

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
)

func TestLoginByPasswordUserNotFound(t *testing.T) {
	var buf bytes.Buffer
	common := BasicAuthCommonParams{
		Email:    "john@example.com",
		Writer:   &buf,
		Language: "en",
	}

	params := BasicAuthUserLoginParams{
		BasicAuthCommonParams: common,
		Store:                 pkg.NewMultiOrgInMemoryStore(),
	}
	_, ok := LoginUserByPassword(params)
	testutils.AssertEqual(t, ok, false)
	testutils.AssertContains(t, buf.String(), "with email john@example.com not found")
}

type failingUserGetter struct {
	Err         error
	RegisterErr error
}

func (f *failingUserGetter) UserByEmail(ctx context.Context, email string) (pkg.UserInfo, error) {
	return pkg.UserInfo{}, f.Err
}

func (f *failingUserGetter) RegisterUser(ctx context.Context, user *pkg.UserInfo) error {
	return f.RegisterErr
}

func TestLoginByPasswordGenericError(t *testing.T) {
	var buf bytes.Buffer
	params := BasicAuthUserLoginParams{
		BasicAuthCommonParams: BasicAuthCommonParams{Writer: &buf},
		Store:                 &failingUserGetter{Err: errors.New("something went wrong")},
	}

	_, ok := LoginUserByPassword(params)
	testutils.AssertEqual(t, ok, false)
	testutils.AssertEqual(t, buf.String(), "something went wrong")
}

func TestLoginByPasswordWrongPassword(t *testing.T) {
	var buf bytes.Buffer
	common := BasicAuthCommonParams{
		Email:    "john@example.com",
		Password: "top-secret-passwd",
		Writer:   &buf,
	}

	store := pkg.NewMultiOrgInMemoryStore()

	registrationParams := BasicAuthUserNewUser{
		BasicAuthCommonParams: common,
		RetypedPassword:       "top-secret-passwd",
		Store:                 store,
	}

	_, ok := RegisterNewUserByPassword(registrationParams)
	testutils.AssertEqual(t, ok, true)

	t.Run("success", func(t *testing.T) {
		user, ok := LoginUserByPassword(BasicAuthUserLoginParams{
			BasicAuthCommonParams: common,
			Store:                 store,
		})

		testutils.AssertEqual(t, ok, true)
		testutils.AssertEqual(t, user.Email, "john@example.com")
	})

	t.Run("wrong-password", func(t *testing.T) {
		params := BasicAuthUserLoginParams{
			BasicAuthCommonParams: common,
			Store:                 store,
		}
		params.Password = "wrong-top-secret-password"

		user, ok := LoginUserByPassword(params)
		testutils.AssertEqual(t, ok, false)
		testutils.AssertEqual(t, user.Email, "john@example.com")
		testutils.AssertContains(t, buf.String(), "Email or password")
	})
}

func TestRegisterUserWrongRetypedPassword(t *testing.T) {
	var buf bytes.Buffer
	store := pkg.NewMultiOrgInMemoryStore()
	params := BasicAuthUserNewUser{
		BasicAuthCommonParams: BasicAuthCommonParams{
			Password: "password1",
			Writer:   &buf,
		},
		Store:           store,
		RetypedPassword: "password2",
	}
	_, ok := RegisterNewUserByPassword(params)
	testutils.AssertEqual(t, ok, false)
	testutils.AssertContains(t, buf.String(), "Passwords", "not match")
}

func TestRegisterAlreadyExistingUser(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	store.Users = []pkg.UserInfo{{Email: "john@example.com", Password: "password"}}

	var buf bytes.Buffer
	params := BasicAuthUserNewUser{
		BasicAuthCommonParams: BasicAuthCommonParams{
			Email:    "john@example.com",
			Password: "password1",
			Writer:   &buf,
		},
		RetypedPassword: "password1",
		Store:           store,
	}

	_, ok := RegisterNewUserByPassword(params)
	testutils.AssertEqual(t, ok, false)
	testutils.AssertContains(t, buf.String(), "john@example.com", "User", "already", "exists")
}

func TestFailingToHashPassword(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	var buf bytes.Buffer
	password := strings.Repeat("password", 40)
	params := BasicAuthUserNewUser{
		BasicAuthCommonParams: BasicAuthCommonParams{
			Writer:   &buf,
			Email:    "john@example.com",
			Password: password,
		},
		Store:           store,
		RetypedPassword: password,
	}
	_, ok := RegisterNewUserByPassword(params)
	testutils.AssertEqual(t, ok, false)
	testutils.AssertContains(t, buf.String(), "72 bytes", "bcrypt")
}

func TestNotOkOnFailingRegistration(t *testing.T) {
	store := failingUserGetter{
		Err:         pkg.ErrUserNotFound,
		RegisterErr: errors.New("could not register user"),
	}
	var buf bytes.Buffer
	params := BasicAuthUserNewUser{
		BasicAuthCommonParams: BasicAuthCommonParams{
			Email:    "john@example.com",
			Password: "password",
			Writer:   &buf,
		},
		RetypedPassword: "password",
		Store:           &store,
	}

	_, ok := RegisterNewUserByPassword(params)
	testutils.AssertEqual(t, ok, false)
	testutils.AssertEqual(t, buf.String(), store.RegisterErr.Error())
}

func TestResetUserPassword(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	user := pkg.UserInfo{
		Id:       "user-id",
		Email:    "john@example.com",
		Password: "old-password",
	}
	store.RegisterUser(context.Background(), &user)
	params := BasicAuthResetPasswordParams{
		BasicAuthCommonParams: BasicAuthCommonParams{
			Password: "password",
			Writer:   bytes.NewBuffer([]byte{}),
		},
		RetypedPassword: "password",
		Store:           store,
	}

	resetParams := func() {
		params.Password = "password"
		params.RetypedPassword = params.Password
		params.Email = "john@example.com"
	}

	t.Run("error on empty password", func(t *testing.T) {
		defer resetParams()
		params.Password = ""
		err := ResetUserPassword(params)
		if err == nil {
			t.Fatal("Wanted error")
		}

		testutils.AssertContains(t, err.Error(), "can not be empty")
	})

	t.Run("retyped password differ", func(t *testing.T) {
		defer resetParams()
		params.RetypedPassword = "wrong-password"
		err := ResetUserPassword(params)
		if err == nil {
			t.Fatal("Wanted error")
		}

		testutils.AssertContains(t, err.Error(), "are not equal")
	})

	t.Run("error when user does not exist", func(t *testing.T) {
		defer resetParams()
		params.Email = "lisa@example.com"

		err := ResetUserPassword(params)
		if err == nil {
			t.Fatal("Wanted err")
		}
		testutils.AssertEqual(t, err.Error(), "user not found")
	})

	t.Run("successful reset", func(t *testing.T) {
		defer resetParams()

		err := ResetUserPassword(params)
		testutils.AssertNil(t, err)
	})
}

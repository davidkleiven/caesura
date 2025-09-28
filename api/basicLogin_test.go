package api

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
)

func TestLoginByPasswordUserNotFound(t *testing.T) {
	var buf bytes.Buffer
	params := BasicAuthUserLoginParams{
		Store:    pkg.NewMultiOrgInMemoryStore(),
		Email:    "john@example.com",
		Writer:   &buf,
		Language: "en",
	}
	_, ok := LoginUserByPassword(params)
	testutils.AssertEqual(t, ok, false)
	testutils.AssertContains(t, buf.String(), "with email john@example.com not found")
}

type failingUserGetter struct {
	Err error
}

func (f *failingUserGetter) UserByEmail(ctx context.Context, email string) (pkg.UserInfo, error) {
	return pkg.UserInfo{}, f.Err
}

func TestLoginByPasswordGenericError(t *testing.T) {
	var buf bytes.Buffer
	params := BasicAuthUserLoginParams{
		Store:  &failingUserGetter{Err: errors.New("something went wrong")},
		Writer: &buf,
	}

	_, ok := LoginUserByPassword(params)
	testutils.AssertEqual(t, ok, false)
	testutils.AssertEqual(t, buf.String(), "something went wrong")

}

package api

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/web"
	"golang.org/x/crypto/bcrypt"
)

type BasicAuthUserLoginParams struct {
	Ctx      context.Context
	Store    pkg.UserByEmailGetter
	Email    string
	Password string
	Language string
	Writer   io.Writer
}

func LoginUserByPassword(params BasicAuthUserLoginParams) (pkg.UserInfo, bool) {
	user, err := params.Store.UserByEmail(params.Ctx, params.Email)
	if errors.Is(err, pkg.ErrUserNotFound) {
		web.UserNotFound(params.Writer, params.Language, params.Email)
		return user, false

	} else if err != nil {
		slog.Error("Error when retrieving user by email", "error", err)
		params.Writer.Write([]byte(err.Error()))
		return user, false
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(params.Password)); err != nil {
		params.Writer.Write([]byte("Unauthorized"))
		return user, false
	}
	return user, true
}

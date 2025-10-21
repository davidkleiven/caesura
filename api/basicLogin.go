package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/web"
	"golang.org/x/crypto/bcrypt"
)

type BasicAuthCommonParams struct {
	Ctx      context.Context
	Email    string
	Password string
	Language string
	Writer   io.Writer
}

type BasicAuthUserLoginParams struct {
	BasicAuthCommonParams
	Store pkg.UserByEmailGetter
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
		params.Writer.Write([]byte(web.Unauthorized(params.Language)))
		return user, false
	}
	return user, true
}

type BasicAuthUserNewUser struct {
	BasicAuthCommonParams
	Store           pkg.BasicAuthUserRegisterer
	RetypedPassword string
}

func RegisterNewUserByPassword(params BasicAuthUserNewUser) (pkg.UserInfo, bool) {
	if params.Password != params.RetypedPassword {
		params.Writer.Write([]byte(web.PasswordAndRetypedPasswordMustMatch(params.Language)))
		return pkg.UserInfo{}, false
	}

	user, err := params.Store.UserByEmail(params.Ctx, params.Email)
	if !errors.Is(err, pkg.ErrUserNotFound) {
		web.UserAlreadyExist(params.Writer, params.Language, params.Email)
		return user, false
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		params.Writer.Write([]byte(err.Error()))
		return user, false
	}

	newUser := pkg.UserInfo{
		Id:       pkg.RandomInsecureID(),
		Email:    params.Email,
		Password: string(hash),
	}

	if err := params.Store.RegisterUser(params.Ctx, &newUser); err != nil {
		params.Writer.Write([]byte(err.Error()))
		return user, false
	}
	return newUser, true
}

type BasicAuthResetPasswordParams struct {
	BasicAuthCommonParams
	Store           pkg.BasicAuthPasswordResetter
	RetypedPassword string
}

func ResetUserPassword(params BasicAuthResetPasswordParams) error {
	if params.Password == "" {
		fmt.Fprintf(params.Writer, "Password can not be empty")
		return fmt.Errorf("Password can not be empty")
	}
	if params.Password != params.RetypedPassword {
		params.Writer.Write([]byte(web.PasswordAndRetypedPasswordMustMatch(params.Language)))
		return fmt.Errorf("Password and retyped password are not equal")
	}
	user, err := params.Store.UserByEmail(params.Ctx, params.Email)
	if err != nil {
		fmt.Fprintf(params.Writer, "Internal server error: %s", err)
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		params.Writer.Write([]byte(err.Error()))
		return err
	}
	return params.Store.ResetPassword(params.Ctx, user.Id, string(hash))
}

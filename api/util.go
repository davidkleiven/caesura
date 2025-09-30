package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
)

const inviteTokenKey = "invite-token"

func Port() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return ":" + port
}

type IdentifiedList struct {
	Id       string
	Items    []string
	HxGet    string
	HxTarget string
}

func includeError(w http.ResponseWriter, status int, message string, err error) {
	if err != nil {
		http.Error(w, message+": "+err.Error(), status)
	}
}

func MustGenerateStateString() string {
	b := make([]byte, 32)
	utils.Must(rand.Read(b))
	return base64.URLEncoding.EncodeToString(b)
}

func MustGetSession(r *http.Request) *sessions.Session {
	val := r.Context().Value(sessionKey)
	session, ok := val.(*sessions.Session)
	if !ok {
		panic("Could not re-interpret session as *session.Session")
	}
	return session
}

func MustGetOrgId(session *sessions.Session) string {
	orgId, ok := session.Values["orgId"].(string)
	if !ok {
		panic("Could not convert orgId into string")
	}
	return orgId
}

func MustGetUserId(session *sessions.Session) string {
	userId, ok := session.Values["userId"].(string)
	if !ok {
		panic("Could not convert userId into string")
	}
	return userId
}

func MustGetUserInfo(session *sessions.Session) *pkg.UserInfo {
	var data pkg.UserInfo
	jsonData, ok := session.Values["role"].([]byte)
	if !ok {
		panic("MustGetUserInfo: role is not of type bytes")
	}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		panic(err)
	}
	return &data
}

func CodeAndMessage(err error, code int) (string, int) {
	var msg string
	if err != nil {
		msg = err.Error()
	}

	finalCode := code
	if code < 400 {
		finalCode = http.StatusInternalServerError
	}
	return msg, finalCode
}

func organizationIds(session *sessions.Session) []string {
	roles, ok := session.Values["role"].([]byte)
	if !ok {
		return []string{}
	}

	var userInfo pkg.UserInfo
	if err := json.Unmarshal(roles, &userInfo); err != nil {
		slog.Error("Could not unmarshal user info", "error", err)
		return []string{}
	}

	result := make([]string, 0, len(userInfo.Roles))
	for orgId := range userInfo.Roles {
		result = append(result, orgId)
	}
	return result
}

type InviteClaim struct {
	OrgId string `json:"org_id"`
	jwt.RegisteredClaims
}

// orgIdFromInviteToken return true if the role was changed, false otherwise
func orgIdFromInviteToken(session *sessions.Session, signSecret string) (string, error) {
	token, ok := session.Values[inviteTokenKey].(string)
	if !ok {
		return "", nil
	}

	parsedToken, err := jwt.ParseWithClaims(token, &InviteClaim{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(signSecret), nil
	})

	if err != nil {
		slog.Error("Error when parsing invite token", "error", err)
		return "", err
	}

	claims, ok := parsedToken.Claims.(*InviteClaim)
	if !ok || claims.OrgId == "" {
		slog.Error("Could not cast parsed token into InviteClaim", "error", err, "orgId", claims.OrgId)
		return "", fmt.Errorf("could not parse token")
	}
	delete(session.Values, "invite-token")
	return claims.OrgId, nil
}

func parseForm(r *http.Request) (int, error) {
	err := r.ParseForm()
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return http.StatusRequestEntityTooLarge, err
	} else if err != nil {
		return http.StatusBadRequest, err
	}
	return http.StatusOK, nil
}

type SessionInitParams struct {
	Ctx        context.Context
	Session    *sessions.Session
	User       *pkg.UserInfo
	SignSecret string
	Store      pkg.RoleStore
	Writer     http.ResponseWriter
	Req        *http.Request
}

type SessionInitResult struct {
	Error      error
	ReturnCode int
}

func NewSessionInitResult() SessionInitResult {
	return SessionInitResult{ReturnCode: http.StatusOK}
}

func InitializeUserSession(p SessionInitParams) SessionInitResult {
	inviteTokenOrg, err := orgIdFromInviteToken(p.Session, p.SignSecret)
	if err != nil {
		return SessionInitResult{Error: err, ReturnCode: http.StatusBadRequest}
	}
	p.Session.Values["userId"] = p.User.Id

	roleUpdater := pkg.NewUserRolePipeline(p.Store, p.Ctx, p.User).
		RegisterIfMissing().
		AssignViewRoleIfNoRole(inviteTokenOrg)

	if roleUpdater.Error != nil {
		return SessionInitResult{Error: roleUpdater.Error, ReturnCode: http.StatusInternalServerError}
	}

	userInfoWithRoles := roleUpdater.User
	pkg.PopulateSessionWithRoles(p.Session, userInfoWithRoles)
	delete(p.Session.Values, inviteTokenKey)
	if err := p.Session.Save(p.Req, p.Writer); err != nil {
		return SessionInitResult{Error: err, ReturnCode: http.StatusInternalServerError}
	}
	return NewSessionInitResult()
}

func validEmail(email string) bool {
	regex := regexp.MustCompile("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+.[a-zA-Z]{2,}$")
	return regex.MatchString(email)
}

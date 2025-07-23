package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"

	"github.com/davidkleiven/caesura/utils"
	"github.com/gorilla/sessions"
)

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

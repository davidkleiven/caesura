package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
)

// Store that fails on save. Useful for triggering certain errors
type errorStore struct{}

func (e *errorStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.NewSession(e, name), nil
}

func (e *errorStore) New(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.NewSession(e, name), nil
}

func (e *errorStore) Save(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	return fmt.Errorf("mock save error")
}

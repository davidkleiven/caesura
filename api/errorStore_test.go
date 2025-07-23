package api

import (
	"net/http/httptest"
	"testing"
)

func TestNewSession(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	store := errorStore{}
	if _, err := store.New(r, "name"); err != nil {
		t.Fatal(err)
	}
}

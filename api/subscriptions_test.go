package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
	"github.com/gorilla/sessions"
)

func TestCheckoutSubscriptionHandle(t *testing.T) {
	s := sessions.Session{
		Values: make(map[any]any),
	}
	s.Values["orgId"] = "org-id"

	b := bytes.Repeat([]byte("h"), 5000)
	handler := checkoutSessionHandler(pkg.NewDefaultConfig())
	ctx := context.WithValue(context.Background(), sessionKey, &s)

	t.Run("too large request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/session", bytes.NewBuffer(b))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusRequestEntityTooLarge)
	})

}

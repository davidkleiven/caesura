package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
	"github.com/gorilla/sessions"
)

func TestBillingPortalOrgNotFound(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	session := sessions.Session{Values: make(map[any]any)}
	session.Values["orgId"] = "org1"
	ctx := context.WithValue(context.Background(), sessionKey, &session)

	handler := BillingPortalHandler{Store: store, Timeout: time.Second}
	req := httptest.NewRequest("GET", "/portal", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req.WithContext(ctx))
	testutils.AssertEqual(t, rec.Code, http.StatusInternalServerError)
	content := rec.Body.String()
	testutils.AssertContains(t, content, "not find org")
}

func TestBillingPortalSessionFailure(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	store.Organizations = []pkg.Organization{{Id: "org1", StripeId: "stripeId"}}

	session := sessions.Session{Values: make(map[any]any)}
	session.Values["orgId"] = "org1"
	ctx := context.WithValue(context.Background(), sessionKey, &session)

	handler := BillingPortalHandler{
		Store:   store,
		Timeout: time.Second,
	}

	t.Run("failing-portal", func(t *testing.T) {
		handler.PortalSessionProvider = &pkg.FailingPortalSessionProvider{}
		req := httptest.NewRequest("GET", "/portal", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusInternalServerError)
		content := rec.Body.String()
		testutils.AssertContains(t, content, "portal session")
	})

	t.Run("successful-redirect", func(t *testing.T) {
		handler.PortalSessionProvider = &pkg.FixedPortalSessionProvider{Url: "http://portal.com"}
		req := httptest.NewRequest("GET", "/portal", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusSeeOther)
	})
}

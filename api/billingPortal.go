package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/stripe/stripe-go/v84"
)

type BillingPortalHandler struct {
	Store                 pkg.OrganizationGetter
	Timeout               time.Duration
	ReturnURL             string
	PortalSessionProvider pkg.BillingPortalSessionProvider
}

func (b *BillingPortalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session := MustGetSession(r)
	orgId := MustGetOrgId(session)

	ctx, cancel := context.WithTimeout(r.Context(), b.Timeout)
	defer cancel()

	org, err := b.Store.GetOrganization(ctx, orgId)
	if err != nil {
		slog.ErrorContext(ctx, "Could not find organization", "orgId", orgId, "error", err)
		http.Error(w, "Could not find organization: "+err.Error(), http.StatusInternalServerError)
		return
	}

	params := stripe.BillingPortalSessionCreateParams{
		Customer:  &org.StripeId,
		ReturnURL: &b.ReturnURL,
	}

	portalSession, err := b.PortalSessionProvider.GetPortalSession(ctx, &params)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to initialize portal session", "error", err)
		http.Error(w, "Failed to initialize portal session: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, portalSession.URL, http.StatusSeeOther)
}

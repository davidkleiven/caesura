package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/web"
	"github.com/gorilla/sessions"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/checkout/session"
	"github.com/stripe/stripe-go/v84/webhook"
	"golang.org/x/sync/errgroup"
)

const SubscriptionWriteAllowed = "subscriptionWriteAllowed"

func createCheckoutSessionParams(domain string, stripeId string, priceId string) *stripe.CheckoutSessionParams {
	return &stripe.CheckoutSessionParams{
		Customer: &stripeId,
		CustomerUpdate: &stripe.CheckoutSessionCustomerUpdateParams{
			Address: stripe.String("never"),
			Name:    stripe.String("never"),
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(string(priceId)), // Caesura Free
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(domain + "/organizations"),
		CancelURL:  stripe.String(domain + "/organizations"),
	}
}

func checkoutSessionHandler(config *pkg.Config, store pkg.OrganizationGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionCookie := MustGetSession(r)
		orgId := MustGetOrgId(sessionCookie)

		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		code, err := parseForm(r)
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		priceId := config.GetPriceIds()
		ctx, cancel := context.WithTimeout(r.Context(), config.Timeout)
		defer cancel()

		org, err := store.GetOrganization(ctx, orgId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			slog.ErrorContext(ctx, "Failed to get organization", "error", err, "orgId", orgId)
			return
		}

		items := createCheckoutSessionParams(config.BaseURL, org.StripeId, priceId.PriceIdFromSubscriptionPlan(r.FormValue("subscription-plan")))

		s, err := session.New(items)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			slog.ErrorContext(r.Context(), "Failed to create stripe session", "error", err)
			return
		}
		slog.InfoContext(r.Context(), "Redirecting to stripe")
		w.Header().Set("HX-Redirect", s.URL)
		w.WriteHeader(http.StatusOK)
	}
}

type StripeMetadata struct {
	OrgId   string `json:"orgId"`
	PriceId string `json:"priceId"`
}

func stripeWebhookHandler(store pkg.SubscriptionStorer, config *pkg.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const MaxBodyBytes = int64(65536)
		r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

		payload, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Read error", http.StatusServiceUnavailable)
			return
		}

		sigHeader := r.Header.Get("Stripe-Signature")

		event, err := webhook.ConstructEvent(payload, sigHeader, config.StripeWebhookSignSecret)
		if err != nil {
			slog.ErrorContext(r.Context(), "Signature verification failed", "error", err)
			http.Error(w, "Invalid signature", http.StatusBadRequest)
			return
		}

		priceIds := config.GetPriceIds()

		switch event.Type {
		case "invoice.payment_succeeded":
			var invoice stripe.Invoice
			err := json.Unmarshal(event.Data.Raw, &invoice)

			if err != nil {
				slog.ErrorContext(r.Context(), "Could not interpret request", "error", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			slog.InfoContext(r.Context(), "Payment succeeded", "sessionId", invoice.ID)
			ctx, cancel := context.WithTimeout(r.Context(), config.Timeout)
			defer cancel()

			defaultInvoiceDetails := InvoiceDetails{
				PriceId: priceIds.Annual,
				Expire:  time.Now().AddDate(1, 0, 0),
			}
			result := priceIdFromInvoice(&invoice, defaultInvoiceDetails)

			subscription := pkg.Subscription{
				Id:        invoice.ID,
				PriceId:   result.PriceId,
				Created:   time.Now(),
				Expires:   result.Expire,
				MaxScores: priceIds.NumScores(result.PriceId),
			}

			customer := invoice.Customer
			if customer == nil {
				slog.ErrorContext(r.Context(), "Received incoive with no customer", "invoice", invoice)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			err = store.StoreSubscription(ctx, customer.ID, &subscription)
			if err != nil {
				slog.ErrorContext(r.Context(), "Failed to store subscription", "error", err, "sesisonId", invoice.ID)
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}

		default:
			slog.InfoContext(r.Context(), "Unhandled event type", "eventType", event.Type)
		}
		w.WriteHeader(http.StatusOK)
	}
}

type SubscriptionState int

const (
	SubscriptionStateExpired = iota
	SubscriptionStateTooManyScores
	SubscriptionStateValid
)

type SubscriptionInfo struct {
	CanWrite            bool
	SuggestedReturnCode int
	Expires             time.Time
	State               SubscriptionState
	MaxScores           int
}

func (si *SubscriptionInfo) PopulateSession(session *sessions.Session) {
	session.Values[SubscriptionWriteAllowed] = si.CanWrite
	expire := si.Expires.Format(time.RFC3339)
	session.Values["subscriptionExpires"] = expire
}

type SubscriptionHandler struct {
	store   pkg.SubscriptionValidator
	timeout time.Duration
}

func (s *SubscriptionHandler) GetInfo(ctx context.Context, orgId string) SubscriptionInfo {
	var (
		subscription *pkg.Subscription
		organization pkg.Organization
	)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var subsErr error
		subscription, subsErr = s.store.GetSubscription(ctx, orgId)
		return subsErr
	})

	g.Go(func() error {
		var orgErr error
		organization, orgErr = s.store.GetOrganization(ctx, orgId)
		return orgErr
	})

	err := g.Wait()

	if err != nil {
		slog.InfoContext(ctx, "Providing default free tier", "error", err)
		subscription = pkg.NewFreeTier()
	}

	if subscription.Expires.Before(time.Now()) {
		return SubscriptionInfo{
			SuggestedReturnCode: http.StatusOK,
			State:               SubscriptionStateExpired,
			Expires:             subscription.Expires,
		}
	}

	if organization.NumScores > subscription.MaxScores {
		return SubscriptionInfo{
			SuggestedReturnCode: http.StatusOK,
			State:               SubscriptionStateTooManyScores,
			Expires:             subscription.Expires,
			MaxScores:           subscription.MaxScores,
		}
	}

	return SubscriptionInfo{
		SuggestedReturnCode: http.StatusOK,
		State:               SubscriptionStateValid,
		Expires:             subscription.Expires,
		CanWrite:            true,
		MaxScores:           subscription.MaxScores,
	}
}

func (s *SubscriptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session := MustGetSession(r)
	orgId := MustGetOrgId(session)

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	result := s.GetInfo(ctx, orgId)
	result.PopulateSession(session)

	if err := session.Save(r, w); err != nil {
		slog.ErrorContext(r.Context(), "Failed to save session", "error", err)
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	lang := pkg.LanguageFromReq(r)
	expire := result.Expires.Format(time.RFC3339)
	switch result.State {
	case SubscriptionStateExpired:
		fmt.Fprintf(w, "%s %s", web.SubscriptionExpired(lang), expire)
		return
	case SubscriptionStateTooManyScores:
		fmt.Fprintf(w, "%s (%d)", web.MaxNumScoresReached(lang), result.MaxScores)
		return
	default:
		fmt.Fprintf(w, "%s %s", web.SubscriptionExpires(lang), expire)
	}
}

type InvoiceDetails struct {
	PriceId string
	Expire  time.Time
}

func priceIdFromInvoice(invoice *stripe.Invoice, defaultInvoiceDetails InvoiceDetails) InvoiceDetails {
	lines := invoice.Lines
	if lines == nil {
		slog.Error("Received invoice with no lines", "invoice", invoice)
		return defaultInvoiceDetails
	}
	items := lines.Data
	if len(items) == 0 {
		slog.Error("Received invoice with no content", "invoice", invoice)

		// We are nice, so we offer the customers experiencing the error an annual subscription
		return defaultInvoiceDetails
	}
	item := items[0]
	pricing := item.Pricing
	if pricing == nil {
		slog.Error("Received invoice with no pricing information", "invoice", invoice)
		return defaultInvoiceDetails
	}

	period := items[0].Period
	if period == nil {
		slog.Error("Received invoice without period", "invoice", invoice)
		return defaultInvoiceDetails
	}
	defaultInvoiceDetails.Expire = time.Unix(period.End, 0)

	details := pricing.PriceDetails
	if details == nil {
		slog.Error("Received invoice with no PriceDetails information", "invoice", invoice)
		return defaultInvoiceDetails
	}
	defaultInvoiceDetails.PriceId = details.Price
	return defaultInvoiceDetails
}

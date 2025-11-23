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
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
	"golang.org/x/sync/errgroup"
)

type StripePriceId string

const (
	FreePriceId    StripePriceId = "price_1RvOBAF9NBcrR1kwWkhZVwwX"
	MonthlyPriceId StripePriceId = "price_1RvOAWF9NBcrR1kwDySNEUFE"
	AnnualPriceId  StripePriceId = "price_1RvObkF9NBcrR1kwBHiYsagO"
)

var MaxNumScores = map[StripePriceId]int{
	FreePriceId:    10,
	MonthlyPriceId: 500,
	AnnualPriceId:  500,
}

const SubscriptionWriteAllowed = "subscriptionWriteAllowed"

func createCheckoutSessionParams(domain string, orgId string, priceId StripePriceId) *stripe.CheckoutSessionParams {
	return &stripe.CheckoutSessionParams{
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(string(priceId)), // Caesura Free
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(domain + "/organizations"),
		CancelURL:  stripe.String(domain + "/organizations"),
		Metadata: map[string]string{
			"orgId":   orgId,
			"priceId": string(priceId),
		},
	}
}

func checkoutSessionHandler(config *pkg.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionCookie := MustGetSession(r)
		orgId := MustGetOrgId(sessionCookie)

		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		code, err := parseForm(r)
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}

		items := createCheckoutSessionParams(config.BaseURL, orgId, priceId(r.FormValue("subscription-plan")))

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

func priceId(item string) StripePriceId {
	switch item {
	case "monthly":
		return MonthlyPriceId
	case "annual":
		return AnnualPriceId
	default:
		return FreePriceId
	}
}

type StripeMetadata struct {
	OrgId   string        `json:"orgId"`
	PriceId StripePriceId `json:"priceId"`
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

			priceId := priceIdFromInvoice(&invoice)

			subscription := pkg.Subscription{
				Id:        invoice.ID,
				PriceId:   string(priceId),
				Created:   time.Now(),
				Expires:   time.Unix(invoice.PeriodEnd, 0),
				MaxScores: getMaxNumScores(priceId),
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

func Subscription(store pkg.SubscriptionValidator, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := MustGetSession(r)
		orgId := MustGetOrgId(s)

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		var (
			subscription *pkg.Subscription
			organization pkg.Organization
		)

		g, ctx := errgroup.WithContext(ctx)
		g.Go(func() error {
			var subsErr error
			subscription, subsErr = store.GetSubscription(ctx, orgId)
			return subsErr
		})

		g.Go(func() error {
			var orgErr error
			organization, orgErr = store.GetOrganization(ctx, orgId)
			return orgErr
		})

		if err := g.Wait(); err != nil {
			slog.ErrorContext(r.Context(), "Could not get subscription", "error", err)
			http.Error(w, "Could not get subscription: "+err.Error(), http.StatusInternalServerError)
			return
		}

		lang := pkg.LanguageFromReq(r)
		if subscription.Expires.Before(time.Now()) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s %s", web.SubscriptionExpired(lang), subscription.Expires.Format(time.RFC3339))
			return
		}

		if organization.NumScores > subscription.MaxScores {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s (%d)", web.MaxNumScoresReached(lang), subscription.MaxScores)
			return
		}

		s.Values[SubscriptionWriteAllowed] = true
		s.Values["subscriptionExpires"] = subscription.Expires.Format(time.RFC3339)
		if err := s.Save(r, w); err != nil {
			slog.ErrorContext(r.Context(), "Failed to save session", "error", err)
			http.Error(w, "Failed to save session", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%s %s", web.SubscriptionExpires(lang), subscription.Expires.Format(time.RFC3339))
	}
}

func priceIdFromInvoice(invoice *stripe.Invoice) StripePriceId {
	lines := invoice.Lines
	if lines == nil {
		slog.Error("Received invoice with no lines", "invoice", invoice)
		return AnnualPriceId
	}
	items := lines.Data
	if len(items) == 0 {
		slog.Error("Received invoice with no content", "invoice", invoice)

		// We are nice, so we offer the customers experiencing the error an annual subscription
		return AnnualPriceId
	}
	item := items[0]
	pricing := item.Pricing
	if pricing == nil {
		slog.Error("Received invoice with no pricing information", "invoice", invoice)
		return AnnualPriceId
	}

	details := pricing.PriceDetails
	if details == nil {
		slog.Error("Received invoice with no PriceDetails information", "invoice", invoice)
		return AnnualPriceId
	}
	return StripePriceId(details.Price)
}

func getMaxNumScores(priceId StripePriceId) int {
	value, ok := MaxNumScores[priceId]
	if !ok {
		slog.Error("Invalid price id", "priceId", value)
		return MaxNumScores[AnnualPriceId]
	}
	return value
}

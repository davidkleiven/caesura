package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
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
			slog.Error("Failed to create stripe session", "error", err, "host", r.Host)
			return
		}
		slog.Info("Redirecting to stripe", "orgId", orgId, "host", r.Host)
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
			slog.Error("Signature verification failed", "error", err)
			http.Error(w, "Invalid signature", http.StatusBadRequest)
			return
		}

		switch event.Type {
		case "checkout.session.completed":
			var (
				session      stripe.CheckoutSession
				metadata     StripeMetadata
				metadataJson []byte
			)
			err := pkg.ReturnOnFirstError(
				func() error { return json.Unmarshal(event.Data.Raw, &session) },
				func() error {
					var marshalErr error
					metadataJson, marshalErr = json.Marshal(session.Metadata)
					return marshalErr
				},
				func() error { return json.Unmarshal(metadataJson, &metadata) },
			)

			if err != nil {
				slog.Error("Could not interpret request", "error", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			slog.Info("Checkout session completed", "sessionId", session.ID)
			ctx, cancel := context.WithTimeout(r.Context(), config.Timeout)
			defer cancel()

			subscription := pkg.Subscription{
				Id:        session.ID,
				PriceId:   string(metadata.PriceId),
				Created:   time.Now(),
				Expires:   time.Unix(session.ExpiresAt, 0),
				MaxScores: MaxNumScores[metadata.PriceId],
			}

			err = store.StoreSubscription(ctx, metadata.OrgId, &subscription)
			if err != nil {
				slog.Error("Failed to store subscription", "error", err, "sesisonId", session.ID)
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}

		default:
			slog.Info("Unhandled event type", "eventType", event.Type)
		}
		w.WriteHeader(http.StatusOK)
	}
}

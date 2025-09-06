package api

import (
	"log/slog"
	"net/http"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
)

type StripePriceId string

const (
	FreePriceId    StripePriceId = "price_1RvOBAF9NBcrR1kwWkhZVwwX"
	MonthlyPriceId StripePriceId = "price_1RvOAWF9NBcrR1kwDySNEUFE"
	AnnualPriceId  StripePriceId = "price_1RvObkF9NBcrR1kwBHiYsagO"
)

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
		Metadata:   map[string]string{"organization_id": orgId},
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

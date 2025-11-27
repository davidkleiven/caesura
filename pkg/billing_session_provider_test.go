package pkg

import (
	"context"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/testutils"
	"github.com/stripe/stripe-go/v84"
)

func TestStripePortalIntegration(t *testing.T) {
	config, err := LoadProfile("config-ci.yml")
	testutils.AssertNil(t, err)

	client := stripe.NewClient(config.StripeSecretKey)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	params := stripe.CustomerCreateParams{Name: stripe.String("customer1")}
	customer, err := client.V1Customers.Create(ctx, &params)
	testutils.AssertNil(t, err)

	provider := StripeBillingSessionProvider{ApiKey: config.StripeSecretKey}

	billingParams := stripe.BillingPortalSessionCreateParams{Customer: &customer.ID}
	result, err := provider.GetPortalSession(ctx, &billingParams)
	testutils.AssertNil(t, err)
	testutils.AssertContains(t, result.URL, "stripe")
}

func TestFailingPortalSessionProvider(t *testing.T) {
	failing := FailingPortalSessionProvider{}
	session, err := failing.GetPortalSession(context.Background(), &stripe.BillingPortalSessionCreateParams{})
	if session == nil {
		t.Fatal("Portal session should not be nil")
	}

	if err == nil {
		t.Fatal("Portal session should raise error")
	}
}

func TestFixedPortalSession(t *testing.T) {
	fixed := FixedPortalSessionProvider{Url: "http://example.com"}
	session, err := fixed.GetPortalSession(context.Background(), &stripe.BillingPortalSessionCreateParams{})
	testutils.AssertNil(t, err)
	testutils.AssertEqual(t, session.URL, "http://example.com")
}

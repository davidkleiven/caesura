package pkg

import (
	"context"
	"errors"

	"github.com/stripe/stripe-go/v84"
)

type BillingPortalSessionProvider interface {
	GetPortalSession(ctx context.Context, params *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error)
}

type StripeBillingSessionProvider struct {
	ApiKey string
}

func (s *StripeBillingSessionProvider) GetPortalSession(ctx context.Context, params *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
	client := stripe.NewClient(s.ApiKey)
	return client.V1BillingPortalSessions.Create(ctx, params)
}

type FailingPortalSessionProvider struct{}

func (s *FailingPortalSessionProvider) GetPortalSession(ctx context.Context, params *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
	return &stripe.BillingPortalSession{}, errors.New("Could not create session")
}

type FixedPortalSessionProvider struct {
	Url string
}

func (f *FixedPortalSessionProvider) GetPortalSession(ctx context.Context, params *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
	return &stripe.BillingPortalSession{URL: f.Url}, nil
}

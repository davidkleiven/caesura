package pkg

import (
	"context"
	"time"

	"github.com/stripe/stripe-go/v84"
)

const DefaultFreeTierExpireDelta = 30 * time.Minute

type Subscription struct {
	Id        string    `json:"id" firestore:"id"`
	PriceId   string    `json:"priceId" firestore:"priceId"`
	Created   time.Time `json:"created" firestore:"created"`
	Expires   time.Time `json:"expires" firestore:"expires"`
	MaxScores int       `json:"maxScores" firestore:"maxScores"`
}

func NewFreeTier() *Subscription {
	return &Subscription{
		Id:        RandomInsecureID(),
		Created:   time.Now(),
		Expires:   time.Now().Add(DefaultFreeTierExpireDelta),
		MaxScores: 10,
	}
}

type SubscriptionStorer interface {
	StoreSubscription(ctx context.Context, stripeId string, subscription *Subscription) error
}

type SubscriptionGetter interface {
	GetSubscription(ctx context.Context, orgId string) (*Subscription, error)
}

// StripeCustomerIdProvider implements a method for providing stripe ids
type StripeCustomerIdProvider interface {
	GetId(ctx context.Context, params *stripe.CustomerCreateParams) (string, error)
}

// LocalStripeCustomerIdProvider returns a random ID (useful for testing)
type LocalStripeCustomerIdProvider struct{}

func (l *LocalStripeCustomerIdProvider) GetId(ctx context.Context, params *stripe.CustomerCreateParams) (string, error) {
	return RandomInsecureID(), nil
}

type PaymentSystemCusteromIdProvider struct {
	ApiKey string
}

func (p *PaymentSystemCusteromIdProvider) GetId(ctx context.Context, params *stripe.CustomerCreateParams) (string, error) {
	stripeClient := stripe.NewClient(p.ApiKey)
	newCustomer, err := stripeClient.V1Customers.Create(ctx, params)
	return newCustomer.ID, err
}

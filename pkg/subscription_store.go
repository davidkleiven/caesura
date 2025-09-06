package pkg

import (
	"context"
	"time"
)

type Subscription struct {
	Id        string    `json:"id"`
	PriceId   string    `json:"priceId"`
	Created   time.Time `json:"created"`
	Expires   time.Time `json:"expires"`
	MaxScores int       `json:"maxScores"`
}

type SubscriptionStorer interface {
	StoreSubscription(ctx context.Context, orgId string, subscription *Subscription) error
}

type SubscriptionGetter interface {
	GetSubscription(ctx context.Context, orgId string) (*Subscription, error)
}

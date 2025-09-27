package pkg

import (
	"context"
	"time"
)

type Subscription struct {
	Id        string    `json:"id" firestore:"id"`
	PriceId   string    `json:"priceId" firestore:"priceId"`
	Created   time.Time `json:"created" firestore:"created"`
	Expires   time.Time `json:"expires" firestore:"expires"`
	MaxScores int       `json:"maxScores" firestore:"maxScores"`
}

type SubscriptionStorer interface {
	StoreSubscription(ctx context.Context, orgId string, subscription *Subscription) error
}

type SubscriptionGetter interface {
	GetSubscription(ctx context.Context, orgId string) (*Subscription, error)
}

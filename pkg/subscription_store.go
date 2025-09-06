package pkg

import (
	"context"
	"time"
)

type Subscription struct {
	Created time.Time
	Expires time.Time
}

type SubscriptionStore interface {
	GetSubscription(ctx context.Context, orgId string) (*Subscription, error)
	StoreSubscription(ctx context.Context, orgId string, subscription *Subscription) error
}

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/event"
)

func main() {
	config, err := pkg.LoadProfile("config-prod.yml")
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) < 2 {
		log.Fatal("Event ID must be provided")
	}

	stripe.Key = config.StripeSecretKey

	eventID := os.Args[1]
	storeResult := pkg.GetStore(config)
	if storeResult.Err != nil {
		log.Fatal(storeResult.Err)
	}
	defer storeResult.Cleanup()

	stripeEvent, err := event.Get(eventID, nil)
	if err != nil {
		log.Fatal(err)
	}

	var invoice stripe.Invoice
	if err := json.Unmarshal(stripeEvent.Data.Raw, &invoice); err != nil {
		log.Fatal(err)
	}

	priceId := invoice.Lines.Data[0].Pricing.PriceDetails.Price
	expiry := time.Unix(invoice.Lines.Data[0].Period.End, 0)
	log.Printf("Customer=%s Expiry=%s PriceId=%s\n", invoice.Customer.ID, expiry.Format(time.RFC3339), priceId)

	subscription := pkg.Subscription{
		Id:        invoice.ID,
		PriceId:   priceId,
		Created:   time.Now(),
		Expires:   expiry,
		MaxScores: config.GetPriceIds().NumScores(priceId),
	}

	shouldApply := false
	for _, arg := range os.Args {
		if arg == "--apply" {
			shouldApply = true
		}
	}
	if shouldApply {
		ctx, cancel := context.WithTimeout(context.Background(), 10.0*time.Second)
		defer cancel()

		err := storeResult.Store.StoreSubscription(ctx, invoice.Customer.ID, &subscription)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Changes are pushed to store")
	} else {
		log.Printf("Run with '--apply' to actually apply the changes")
	}

	log.Println("Event processed successfully")
}

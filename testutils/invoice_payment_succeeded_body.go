package testutils

import (
	"encoding/json"

	"github.com/stripe/stripe-go/v84"
)

// Object that mimics the response we get from stripe. Useful for testing. Note that there
// is one test that receives responses via the stripe CLI, so we checking that we handle
// real responses. This object can be used for unittests when one wants to mimic a response
//
// These structures are similar to the ones in stripe, but not exact matches
// The stripe structures contains some fields we can't set and custom marshalling
// These structures are meatn to reproduce the final responses
type InvoicePaymentSucceededBody struct {
	Id         string           `json:"id,omitempty"`
	Object     string           `json:"object,omitempty"`
	Type       string           `json:"type,omitempty"`
	ApiVersion string           `json:"api_version,omitempty"`
	Data       InvoiceContainer `json:"data"`
}

type InvoiceContainer struct {
	Object Invoice `json:"object"`
}

type PriceDetails struct {
	Price   string `json:"price,omitempty"`
	Product string `json:"product,omitempty"`
}

type Pricing struct {
	Type         string       `json:"type,omitempty"`
	PriceDetails PriceDetails `json:"price_details"`
}

type LineItem struct {
	Id      string  `json:"id,omitempty"`
	Object  string  `json:"object,omitempty"`
	Pricing Pricing `json:"pricing"`
}

type Lines struct {
	Type string     `json:"type,omitempty"`
	Data []LineItem `json:"data,omitempty"`
}

type Invoice struct {
	Id         string `json:"id,omitempty"`
	Object     string `json:"object,omitempty"`
	CustomerId string `json:"customer,omitempty"`
	Lines      Lines  `json:"lines"`
}

func NewInvoiceResponse(opts ...func(i *InvoicePaymentSucceededBody)) *InvoicePaymentSucceededBody {
	invoice := InvoicePaymentSucceededBody{
		Id:         "event-id",
		Object:     "event",
		Type:       InvoicePaymentSucceeded,
		ApiVersion: stripe.APIVersion,
		Data: InvoiceContainer{
			Object: Invoice{
				Id:         "inv-id",
				Object:     "invoice",
				CustomerId: "cus_123",
				Lines: Lines{
					Type: "list",
					Data: []LineItem{
						{
							Id:     "item-i",
							Object: "line_item",
							Pricing: Pricing{
								Type: "price_details",
								PriceDetails: PriceDetails{
									Price:   "priceId",
									Product: "prodctId",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, opt := range opts {
		opt(&invoice)
	}
	return &invoice
}

func WithoutCustomer() func(i *InvoicePaymentSucceededBody) {
	return func(i *InvoicePaymentSucceededBody) {
		i.Data.Object.CustomerId = ""
	}
}

func MustJsonify(inv *InvoicePaymentSucceededBody) []byte {
	content, err := json.Marshal(inv)
	if err != nil {
		panic(err)
	}
	return content
}

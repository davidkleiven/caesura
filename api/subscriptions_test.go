package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
	"github.com/gorilla/sessions"
	"github.com/stripe/stripe-go/v82"
)

const webhookSecret = "whsec_test_123456"

func TestCheckoutSubscriptionHandle(t *testing.T) {
	s := sessions.Session{
		Values: make(map[any]any),
	}
	s.Values["orgId"] = "org-id"

	b := bytes.Repeat([]byte("h"), 5000)
	config := pkg.NewDefaultConfig()
	handler := checkoutSessionHandler(config)
	ctx := context.WithValue(context.Background(), sessionKey, &s)

	t.Run("too large request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/session", bytes.NewBuffer(b))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusRequestEntityTooLarge)
	})

	t.Run("internal server error on missing api key", func(t *testing.T) {
		form := url.Values{}
		form.Set("subscription-plan", "free")

		req := httptest.NewRequest("POST", "/session", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusInternalServerError)
	})

	for _, plan := range []string{"free", "monthly", "annual"} {
		t.Run("success_"+plan, func(t *testing.T) {
			currentStripeKey := stripe.Key
			defer func() {
				stripe.Key = currentStripeKey
			}()

			key, ok := os.LookupEnv("CAESURA_STRIPE_SECRET_KEY")
			ci := os.Getenv("CI")
			if !ok && ci == "" {
				t.Skip("No secret key provided")
				return
			} else if !ok {
				t.Fatalf("Must run in pipeline")
			}
			stripe.Key = key
			form := url.Values{}
			form.Set("subscription-plan", plan)
			req := httptest.NewRequest("POST", "/session", bytes.NewBufferString(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			handler(rec, req.WithContext(ctx))
			testutils.AssertEqual(t, rec.Code, http.StatusOK)
		})
	}
}

func TestWebhookSubscriptionLargeRequest(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	config := pkg.NewDefaultConfig()
	handler := stripeWebhookHandler(store, config)

	body := bytes.Repeat([]byte("a"), 70000)
	req := httptest.NewRequest("POST", "/hook", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusServiceUnavailable)
}

type PayloadParams struct {
	ApiVersion      string
	EventType       string
	OrganizationKey string
}

func stripePayload(params *PayloadParams, w io.Writer) error {
	payloadTempl := `{
		"id": "evt_test_webhook",
		"object": "event",
		"type": "{{ .EventType }}",
		"api_version": "{{ .ApiVersion }}",
		"data": {
			"object": {
				"id": "cs_test_123",
				"object": "checkout.session",
				"metadata": {
					"{{ .OrganizationKey }}": "my-band",
					"priceId": "price-id"
				}
			}
		}
	}`

	tmpl, err := template.New("payload").Parse(payloadTempl)
	if err != nil {
		return err
	}

	return tmpl.Execute(w, params)
}

func stripeSignedRequest(payload []byte) *http.Request {
	timestamp := time.Now().Unix()
	signature := computeStripeSignature([]byte(payload), timestamp, webhookSecret)

	signatureHeader := fmt.Sprintf("t=%d,v1=%s", timestamp, signature)

	// Create test HTTP request with body and signature header
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signatureHeader)
	return req
}

func TestStripeWebhookHandlerSuccess(t *testing.T) {
	params := PayloadParams{
		EventType:       "checkout.session.completed",
		ApiVersion:      stripe.APIVersion,
		OrganizationKey: "orgId",
	}
	var buf bytes.Buffer
	err := stripePayload(&params, &buf)
	testutils.AssertNil(t, err)

	payload := buf.Bytes()

	req := stripeSignedRequest(payload)
	rec := httptest.NewRecorder()

	store := pkg.NewMultiOrgInMemoryStore()
	config := pkg.NewDefaultConfig()
	config.StripeWebhookSignSecret = webhookSecret

	handler := stripeWebhookHandler(store, config)
	handler(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusOK)
	_, ok := store.Subscriptions["my-band"]
	testutils.AssertEqual(t, ok, true)
}

// computeStripeSignature creates a valid v1 signature for Stripe's webhook verification
func computeStripeSignature(payload []byte, timestamp int64, secret string) string {
	signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestStripeWebhookMissingSignature(t *testing.T) {
	params := PayloadParams{
		ApiVersion:      stripe.APIVersion,
		EventType:       "checkout.session.completed",
		OrganizationKey: "wrong-key",
	}

	var buf bytes.Buffer
	err := stripePayload(&params, &buf)
	testutils.AssertNil(t, err)

	req := stripeSignedRequest(buf.Bytes())
	rec := httptest.NewRecorder()
	config := pkg.NewDefaultConfig()
	config.StripeWebhookSignSecret = webhookSecret

	store := pkg.NewMultiOrgInMemoryStore()
	handler := stripeWebhookHandler(store, config)
	handler(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusServiceUnavailable)
}

func TestBadRequestOnBadSignature(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	req := httptest.NewRequest("POST", "/endpoint", nil)
	rec := httptest.NewRecorder()
	config := pkg.NewDefaultConfig()
	handler := stripeWebhookHandler(store, config)
	handler(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusBadRequest)
}

func TestOkOnUnhandledEventtype(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	params := PayloadParams{
		ApiVersion:      stripe.APIVersion,
		EventType:       "unhandled.event",
		OrganizationKey: "orgId",
	}

	var buf bytes.Buffer
	stripePayload(&params, &buf)
	req := stripeSignedRequest(buf.Bytes())
	rec := httptest.NewRecorder()

	config := pkg.NewDefaultConfig()
	config.StripeWebhookSignSecret = webhookSecret
	handler := stripeWebhookHandler(store, config)
	handler(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusOK)
}

type failingSubscriptionStore struct{}

func (f *failingSubscriptionStore) StoreSubscription(ctx context.Context, orgId string, s *pkg.Subscription) error {
	return errors.New("something went wrong")
}

func TestServiceUnavailableOnUnavailableStore(t *testing.T) {
	store := failingSubscriptionStore{}
	params := PayloadParams{
		ApiVersion:      stripe.APIVersion,
		EventType:       "checkout.session.completed",
		OrganizationKey: "orgId",
	}

	var buf bytes.Buffer
	stripePayload(&params, &buf)
	req := stripeSignedRequest(buf.Bytes())
	rec := httptest.NewRecorder()

	config := pkg.NewDefaultConfig()
	config.StripeWebhookSignSecret = webhookSecret
	handler := stripeWebhookHandler(&store, config)
	handler(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusServiceUnavailable)
}

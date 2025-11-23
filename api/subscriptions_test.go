package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
	"github.com/gorilla/sessions"
	"github.com/stripe/stripe-go/v82"
)

const webhookSecret = "whsec_test_123456"

func isDependabot() bool {
	return os.Getenv("GITHUB_ACTOR") == "dependabot[bot]"
}

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
			if isDependabot() {
				t.Skip("Dependabot has no access to secrets")
				return
			} else if !ok && ci == "" {
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

func stripePayload(w io.Writer) error {
	event := map[string]any{
		"id":          "event-id",
		"object":      "event",
		"type":        testutils.CustomerUpdated,
		"api_version": stripe.APIVersion,
		"data": map[string]any{
			"object": map[string]any{
				"id":     "cus_123",
				"object": "customer",
			},
		},
	}
	content, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}
	_, err = w.Write(content)
	return err
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

type MonitorRequestMiddleware struct {
	numPaymentCalls int
	responses       []RespBodyWithCode
	mu              sync.Mutex
}

type RespBodyWithCode struct {
	Body string
	Code int
}

type RecordingResponseWriter struct {
	Writer http.ResponseWriter
	Body   bytes.Buffer
	Code   int
}

func (r *RecordingResponseWriter) Write(b []byte) (int, error) {
	n, err := r.Writer.Write(b)
	r.Body.Write(b)
	return n, err
}

func (r *RecordingResponseWriter) WriteHeader(statusCode int) {
	r.Writer.WriteHeader(statusCode)
	r.Code = statusCode
}

func (r *RecordingResponseWriter) Header() http.Header {
	return r.Writer.Header()
}

func (m *MonitorRequestMiddleware) CreateHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains("/payment", r.URL.String()) {
			m.mu.Lock()
			m.numPaymentCalls += 1
			m.mu.Unlock()
		}

		recWriter := RecordingResponseWriter{Writer: w}
		handler.ServeHTTP(&recWriter, r)

		m.mu.Lock()
		if m.responses == nil {
			m.responses = []RespBodyWithCode{}
		}
		m.responses = append(m.responses, RespBodyWithCode{
			Body: recWriter.Body.String(),
			Code: recWriter.Code,
		})
		m.mu.Unlock()
	})
}

func (m *MonitorRequestMiddleware) GetNumPaymentCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.numPaymentCalls
}

type KeepUnknownSubsStore struct {
	pkg.MultiOrgInMemoryStore
}

func (k *KeepUnknownSubsStore) StoreSubscription(ctx context.Context, stripeId string, subscription *pkg.Subscription) error {
	k.Subscriptions[pkg.RandomInsecureID()] = *subscription
	return nil
}

func TestStripeWebhookHandlerSuccess(t *testing.T) {
	_, ci := os.LookupEnv("CI")
	hasStripe := testutils.HasStripe()
	if !hasStripe && !ci {
		t.Skip("Stripe CLI is not installed")
	} else if !hasStripe {
		t.Fatal("Stipe not installed. Test can not be skipped in CI pipeline")
	}

	config, err := pkg.LoadProfile("config-ci.yml")
	testutils.AssertNil(t, err)

	monitor := MonitorRequestMiddleware{}

	stripeListener := testutils.StripeListener{
		ApiKey: config.StripeSecretKey,
	}

	signSecret, err := stripeListener.SignSecret()
	testutils.AssertNil(t, err)

	port := ":42195"
	t.Log("Starting stripe forwarder...")
	cancel := stripeListener.MustLaunchStripe(fmt.Sprintf("localhost%s/payment", port))
	defer cancel()

	store := KeepUnknownSubsStore{*pkg.NewMultiOrgInMemoryStore()}
	config.StripeWebhookSignSecret = signSecret

	cookieStore := sessions.NewCookieStore([]byte("top-secret"))
	mux := Setup(&store, config, cookieStore)

	server := &http.Server{Addr: port, Handler: monitor.CreateHandler(mux)}

	testutils.AssertEqual(t, len(store.Subscriptions), 0)
	defer func() {
		ctx, cancelCtx := context.WithTimeout(context.Background(), time.Second)
		defer cancelCtx()
		server.Shutdown(ctx)
	}()

	t.Log("Starting Go server")
	go func() {
		server.ListenAndServe()
	}()
	t.Log("Triggering webhook")
	err = stripeListener.TriggerEvent(testutils.InvoicePaymentSucceeded)
	testutils.AssertNil(t, err)

	for range 50 {
		// invoice.payment_succeeded triggers 13 events
		if monitor.GetNumPaymentCalls() == 13 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	testutils.AssertEqual(t, monitor.GetNumPaymentCalls(), 13)

	for _, item := range monitor.responses {
		testutils.AssertEqual(t, item.Code, http.StatusOK)
	}

	testutils.AssertEqual(t, len(store.Subscriptions), 1)
}

// computeStripeSignature creates a valid v1 signature for Stripe's webhook verification
func computeStripeSignature(payload []byte, timestamp int64, secret string) string {
	signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	return hex.EncodeToString(mac.Sum(nil))
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
	var buf bytes.Buffer
	stripePayload(&buf)

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
	body := testutils.MustJsonify(testutils.NewInvoiceResponse())
	req := stripeSignedRequest(body)
	rec := httptest.NewRecorder()

	config := pkg.NewDefaultConfig()
	config.StripeWebhookSignSecret = webhookSecret
	handler := stripeWebhookHandler(&store, config)
	handler(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusServiceUnavailable)
}

func TestBadRequestOnMissingCustomerId(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()

	body := testutils.MustJsonify(testutils.NewInvoiceResponse(testutils.WithoutCustomer()))
	req := stripeSignedRequest(body)
	rec := httptest.NewRecorder()

	config := pkg.NewDefaultConfig()
	config.StripeWebhookSignSecret = webhookSecret
	handler := stripeWebhookHandler(store, config)
	handler(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusBadRequest)
}

type brokenSessionStore struct{}

func (b *brokenSessionStore) Save(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	return errors.New("broken session store called")
}

func (b *brokenSessionStore) Get(r *http.Request, key string) (*sessions.Session, error) {
	return sessions.NewSession(b, key), nil
}

func (b *brokenSessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	return b.Get(r, name)
}

func TestSubscriptions(t *testing.T) {
	sessionCookie := sessions.Session{
		Values: make(map[any]any),
	}

	store := pkg.NewMultiOrgInMemoryStore()
	store.Subscriptions = map[string]pkg.Subscription{
		"org1": {
			Expires:   time.Now().Add(-time.Hour),
			MaxScores: 1000,
		},
		"org2": {
			Expires:   time.Now().Add(time.Hour),
			MaxScores: 9,
		},
		"org3": {
			Expires:   time.Now().Add(time.Hour),
			MaxScores: 9,
		},
	}

	store.Organizations = []pkg.Organization{
		{Id: "org1"},
		{Id: "org2", NumScores: 10},
		{Id: "org3", NumScores: 5},
	}

	ctx := context.WithValue(context.Background(), sessionKey, &sessionCookie)

	handler := Subscription(store, 1*time.Second)

	t.Run("non existing org", func(t *testing.T) {
		sessionCookie.Values["orgId"] = "unknown-org"
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/subscription", nil)
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)
	})

	t.Run("expired-supscription", func(t *testing.T) {
		sessionCookie.Values["orgId"] = "org1"
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/subscription", nil)
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		testutils.AssertContains(t, recorder.Body.String(), "expired")
	})

	t.Run("too-many-scores", func(t *testing.T) {
		sessionCookie.Values["orgId"] = "org2"
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/subscription", nil)
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		testutils.AssertContains(t, recorder.Body.String(), "Subscription", "permit")
	})

	t.Run("valid-subscription", func(t *testing.T) {
		cookieStore := sessions.NewCookieStore([]byte("top-secret"))
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/subscription", nil)
		sCookie, err := cookieStore.Get(req, AuthSession)
		sCookie.Values["orgId"] = "org3"

		testutils.AssertNil(t, err)
		handler(recorder, req.WithContext(context.WithValue(req.Context(), sessionKey, sCookie)))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)

		resp := recorder.Result()
		cookies := resp.Cookies()
		testutils.AssertEqual(t, len(cookies), 1)

		// Because of the signature, create a new dummy request with the signed cookie and
		// extract the session from there
		req2 := httptest.NewRequest("GET", "/whatever", nil)
		req2.AddCookie(cookies[0])
		sessionFromResp, err := cookieStore.Get(req2, AuthSession)
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, sessionFromResp.Values["subscriptionWriteAllowed"], true)
	})

	t.Run("session-save-fails", func(t *testing.T) {
		cookieStore := brokenSessionStore{}

		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/subscription", nil)
		sCookie, err := cookieStore.Get(req, AuthSession)
		sCookie.Values["orgId"] = "org3"
		testutils.AssertNil(t, err)
		handler(recorder, req.WithContext(context.WithValue(req.Context(), sessionKey, sCookie)))
		testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)
	})
}

func TestPriceIdFromInvoice(t *testing.T) {
	invoice := stripe.Invoice{}
	testutils.AssertEqual(t, priceIdFromInvoice(&invoice), AnnualPriceId)

	invoice.Lines = &stripe.InvoiceLineItemList{}
	testutils.AssertEqual(t, priceIdFromInvoice(&invoice), AnnualPriceId)

	invoice.Lines.Data = []*stripe.InvoiceLineItem{{}}
	testutils.AssertEqual(t, priceIdFromInvoice(&invoice), AnnualPriceId)

	invoice.Lines.Data[0].Pricing = &stripe.InvoiceLineItemPricing{}
	testutils.AssertEqual(t, priceIdFromInvoice(&invoice), AnnualPriceId)

	invoice.Lines.Data[0].Pricing.PriceDetails = &stripe.InvoiceLineItemPricingPriceDetails{Price: string(MonthlyPriceId)}
	testutils.AssertEqual(t, priceIdFromInvoice(&invoice), MonthlyPriceId)
}

func TestGetMaxNumScoresUnknownPriceId(t *testing.T) {
	testutils.AssertEqual(t, getMaxNumScores("unknown-price-id"), 500)
}

func TestGetMaxNumScores(t *testing.T) {
	testutils.AssertEqual(t, getMaxNumScores(FreePriceId), 10)
}

func TestGetCustomerIdFromStripe(t *testing.T) {
	config, err := pkg.LoadProfile("config-ci.yml")
	config.StripeIdProvider = "stripe"
	testutils.AssertNil(t, err)
	idProvider := config.GetStripeIdProvider()
	params := stripe.CustomerCreateParams{
		Email: stripe.String("peter@example.com"),
		Name:  stripe.String("Peter"),
	}

	timeout, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	customerId, err := idProvider.GetId(timeout, &params)
	testutils.AssertNil(t, err)
	testutils.AssertContains(t, customerId, "cus_")
}

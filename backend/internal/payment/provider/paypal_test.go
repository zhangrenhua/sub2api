//go:build unit

package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func paypalBaseConfig(apiBase string) map[string]string {
	return map[string]string{
		"clientId":     "test-client-id",
		"clientSecret": "test-client-secret",
		"webhookId":    "WH-TEST",
		"apiBase":      apiBase,
		"currency":     "USD",
	}
}

func TestNewPayPalValidatesConfig(t *testing.T) {
	t.Parallel()

	_, err := NewPayPal("1", map[string]string{"clientId": "x"})
	require.ErrorContains(t, err, "missing required key")

	_, err = NewPayPal("1", map[string]string{
		"clientId": "x", "clientSecret": "y", "webhookId": "z",
		"apiBase": "https://evil.example.com",
	})
	require.ErrorContains(t, err, "apiBase host")

	prov, err := NewPayPal("1", paypalBaseConfig(paypalSandboxAPIBase))
	require.NoError(t, err)
	require.Equal(t, payment.TypePayPal, prov.ProviderKey())
	require.Equal(t, "PayPal", prov.Name())
	require.ElementsMatch(t, []payment.PaymentType{payment.TypePayPal}, prov.SupportedTypes())
	require.Equal(t, "USD", prov.config["currency"])
	meta := prov.MerchantIdentityMetadata()
	require.Equal(t, "test-client-id", meta["client_id"])
	require.Equal(t, "sandbox", meta["env"])
}

// paypalStubServer returns an httptest server that emulates the subset of PayPal
// APIs used by the provider: OAuth2 token, create order, get order, verify
// webhook signature.
func paypalStubServer(t *testing.T) (*httptest.Server, *paypalServerState) {
	t.Helper()
	state := &paypalServerState{}
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		state.tokenCalls++
		user, pass, ok := r.BasicAuth()
		require.True(t, ok, "expected basic auth")
		require.Equal(t, "test-client-id", user)
		require.Equal(t, "test-client-secret", pass)
		_, _ = w.Write([]byte(`{"access_token":"TOKEN_AAA","token_type":"Bearer","expires_in":3600}`))
	})

	mux.HandleFunc("/v2/checkout/orders", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer TOKEN_AAA", r.Header.Get("Authorization"))
		var req paypalCreateOrderRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		state.createRequest = &req
		_, _ = w.Write([]byte(`{"id":"ORDER-1","status":"CREATED","links":[
			{"href":"https://www.sandbox.paypal.com/checkoutnow?token=ORDER-1","rel":"approve","method":"GET"},
			{"href":"https://api-m.sandbox.paypal.com/v2/checkout/orders/ORDER-1","rel":"self","method":"GET"}
		]}`))
	})

	mux.HandleFunc("/v2/checkout/orders/ORDER-1", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(`{
			"id":"ORDER-1","status":"COMPLETED",
			"purchase_units":[{"custom_id":"ORD42","amount":{"value":"9.99","currency_code":"USD"},
				"payments":{"captures":[{"id":"CAP-1","status":"COMPLETED",
					"amount":{"value":"9.99","currency_code":"USD"},"create_time":"2026-05-19T01:00:00Z"}]}}]
		}`))
	})

	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		var req paypalVerifyRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		state.lastVerifyRequest = &req
		status := "SUCCESS"
		if state.failVerification {
			status = "FAILURE"
		}
		_, _ = w.Write([]byte(`{"verification_status":"` + status + `"}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, state
}

type paypalServerState struct {
	tokenCalls        int
	createRequest     *paypalCreateOrderRequest
	lastVerifyRequest *paypalVerifyRequest
	failVerification  bool
}

func makePayPalForTest(t *testing.T, srvURL string) *PayPal {
	t.Helper()
	prov := &PayPal{
		instanceID: "1",
		config: map[string]string{
			"clientId":     "test-client-id",
			"clientSecret": "test-client-secret",
			"webhookId":    "WH-TEST",
			"apiBase":      srvURL, // bypass host whitelist for tests
			"currency":     "USD",
			"brandName":    "Test Brand",
		},
		httpClient: &http.Client{},
	}
	// Clear cached tokens from previous parallel tests.
	paypalAccessTokens.Delete(prov.tokenCacheKey())
	return prov
}

func TestPayPalCreatePayment(t *testing.T) {
	t.Parallel()

	srv, state := paypalStubServer(t)
	prov := makePayPalForTest(t, srv.URL)

	resp, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:   "ORD42",
		Amount:    "9.99",
		Subject:   "Recharge",
		ReturnURL: "https://example.com/return",
	})
	require.NoError(t, err)
	require.Equal(t, "ORDER-1", resp.TradeNo)
	require.Contains(t, resp.PayURL, "checkoutnow")
	require.Equal(t, "USD", resp.Currency)

	require.NotNil(t, state.createRequest)
	require.Equal(t, "CAPTURE", state.createRequest.Intent)
	require.Len(t, state.createRequest.PurchaseUnits, 1)
	require.Equal(t, "ORD42", state.createRequest.PurchaseUnits[0].CustomID)
	require.Equal(t, "9.99", state.createRequest.PurchaseUnits[0].Amount.Value)
	require.Equal(t, "USD", state.createRequest.PurchaseUnits[0].Amount.CurrencyCode)
	require.Equal(t, "Test Brand", state.createRequest.ApplicationContext.BrandName)
}

func TestPayPalTokenCaching(t *testing.T) {
	t.Parallel()

	srv, state := paypalStubServer(t)
	prov := makePayPalForTest(t, srv.URL)

	_, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "ORD42", Amount: "1.00",
	})
	require.NoError(t, err)
	_, err = prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "ORD43", Amount: "1.00",
	})
	require.NoError(t, err)

	require.Equal(t, 1, state.tokenCalls, "token should be fetched once and cached for subsequent calls")
}

func TestPayPalQueryOrder(t *testing.T) {
	t.Parallel()

	srv, _ := paypalStubServer(t)
	prov := makePayPalForTest(t, srv.URL)

	resp, err := prov.QueryOrder(context.Background(), "ORDER-1")
	require.NoError(t, err)
	require.Equal(t, "ORDER-1", resp.TradeNo)
	require.Equal(t, payment.ProviderStatusPaid, resp.Status)
	require.InDelta(t, 9.99, resp.Amount, 0.0001)
	require.Equal(t, "2026-05-19T01:00:00Z", resp.PaidAt)
}

func TestPayPalVerifyNotification(t *testing.T) {
	t.Parallel()

	srv, state := paypalStubServer(t)
	prov := makePayPalForTest(t, srv.URL)

	eventBody := `{
		"id":"WH-EVT-1","event_type":"PAYMENT.CAPTURE.COMPLETED","create_time":"2026-05-19T01:00:00Z",
		"resource":{"id":"CAP-1","status":"COMPLETED","custom_id":"ORD42",
			"amount":{"value":"9.99","currency_code":"USD"},
			"supplementary_data":{"related_ids":{"order_id":"ORDER-1"}}}
	}`
	headers := map[string]string{
		"paypal-auth-algo":         "SHA256withRSA",
		"paypal-cert-url":          "https://api.sandbox.paypal.com/v1/notifications/certs/CERT-x",
		"paypal-transmission-id":   "txn-1",
		"paypal-transmission-sig":  "sig-1",
		"paypal-transmission-time": "2026-05-19T01:00:00Z",
	}

	notification, err := prov.VerifyNotification(context.Background(), eventBody, headers)
	require.NoError(t, err)
	require.NotNil(t, notification)
	require.Equal(t, "ORD42", notification.OrderID)
	require.Equal(t, "ORDER-1", notification.TradeNo)
	require.Equal(t, payment.ProviderStatusSuccess, notification.Status)
	require.InDelta(t, 9.99, notification.Amount, 0.0001)
	require.Equal(t, "PAYMENT.CAPTURE.COMPLETED", notification.Metadata["event_type"])
	require.Equal(t, "CAP-1", notification.Metadata["capture_id"])

	require.NotNil(t, state.lastVerifyRequest)
	require.Equal(t, "WH-TEST", state.lastVerifyRequest.WebhookID)
	require.Equal(t, "txn-1", state.lastVerifyRequest.TransmissionID)
}

func TestPayPalVerifyNotificationFailureRejected(t *testing.T) {
	t.Parallel()

	srv, state := paypalStubServer(t)
	state.failVerification = true
	prov := makePayPalForTest(t, srv.URL)

	headers := map[string]string{
		"paypal-auth-algo":         "SHA256withRSA",
		"paypal-cert-url":          "https://api.sandbox.paypal.com/v1/notifications/certs/CERT-x",
		"paypal-transmission-id":   "txn-1",
		"paypal-transmission-sig":  "tampered",
		"paypal-transmission-time": "2026-05-19T01:00:00Z",
	}
	_, err := prov.VerifyNotification(context.Background(), `{"event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{}}`, headers)
	require.ErrorContains(t, err, "signature verification failed")
}

func TestPayPalVerifyNotificationMissingHeaders(t *testing.T) {
	t.Parallel()

	srv, _ := paypalStubServer(t)
	prov := makePayPalForTest(t, srv.URL)

	_, err := prov.VerifyNotification(context.Background(), `{}`, map[string]string{})
	require.ErrorContains(t, err, "missing required PayPal-* headers")
}

func TestPayPalVerifyNotificationIrrelevantEventReturnsNil(t *testing.T) {
	t.Parallel()

	srv, _ := paypalStubServer(t)
	prov := makePayPalForTest(t, srv.URL)

	eventBody := `{"event_type":"CHECKOUT.ORDER.APPROVED","resource":{"id":"ORDER-1"}}`
	headers := map[string]string{
		"paypal-auth-algo":         "SHA256withRSA",
		"paypal-cert-url":          "https://api.sandbox.paypal.com/v1/notifications/certs/CERT-x",
		"paypal-transmission-id":   "txn-1",
		"paypal-transmission-sig":  "sig-1",
		"paypal-transmission-time": "2026-05-19T01:00:00Z",
	}
	notification, err := prov.VerifyNotification(context.Background(), eventBody, headers)
	require.NoError(t, err)
	require.Nil(t, notification, "irrelevant events should return nil so the handler can ack 200")
}

func TestPayPalRefundReturnsNotSupported(t *testing.T) {
	t.Parallel()

	prov, err := NewPayPal("1", paypalBaseConfig(paypalSandboxAPIBase))
	require.NoError(t, err)
	_, err = prov.Refund(context.Background(), payment.RefundRequest{OrderID: "x", Amount: "1.00"})
	require.ErrorContains(t, err, "not implemented")
}

func TestPayPalAuthFailureSurfacesHelpfulMessage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/oauth2/token", r.URL.Path)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"Client Authentication failed"}`))
	}))
	t.Cleanup(srv.Close)

	prov := makePayPalForTest(t, srv.URL)
	_, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "x", Amount: "1.00",
	})
	require.ErrorContains(t, err, "401")
	require.ErrorContains(t, err, "invalid_client")
}

// Sanity check the URL-encoded grant_type body is what PayPal expects.
func TestPayPalTokenRequestBodyIsClientCredentials(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/oauth2/token" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		values, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, "client_credentials", values.Get("grant_type"))
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"access_token":"T","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)

	prov := makePayPalForTest(t, srv.URL)
	_, err := prov.accessToken(context.Background())
	require.NoError(t, err)
}

// extractOutTradeNo is exercised via the handler in integration tests; here we
// ensure the JSON path we documented is reachable with a typical PayPal payload.
func TestPayPalWebhookBodyHasCustomIDPath(t *testing.T) {
	t.Parallel()

	body := `{"id":"WH-1","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"id":"CAP-1","custom_id":"ORD42"}}`
	var payload struct {
		Resource struct {
			CustomID string `json:"custom_id"`
		} `json:"resource"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &payload))
	require.Equal(t, "ORD42", strings.TrimSpace(payload.Resource.CustomID))
}

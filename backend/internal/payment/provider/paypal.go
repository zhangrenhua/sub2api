package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// PayPal constants. The provider integrates against PayPal's Orders v2 REST API
// using the hosted-redirect flow ("intent": "CAPTURE"):
//  1. POST /v2/checkout/orders                — create an order, get the approve URL
//  2. Redirect the user to the approve URL    — user logs into PayPal and approves
//  3. Webhook PAYMENT.CAPTURE.COMPLETED       — authoritative confirmation
//
// Webhook signatures are verified by calling PayPal's
// /v1/notifications/verify-webhook-signature endpoint (rather than fetching and
// caching the upstream cert ourselves), which is the recommended low-friction
// approach for server-side integrations.
const (
	paypalSandboxAPIBase  = "https://api-m.sandbox.paypal.com"
	paypalProdAPIBase     = "https://api-m.paypal.com"
	paypalHTTPTimeout     = 15 * time.Second
	paypalMaxResponseSize = 1 << 20
	paypalMaxErrorSummary = 512
	paypalTokenSkew       = 5 * time.Minute

	paypalEventCaptureCompleted = "PAYMENT.CAPTURE.COMPLETED"
	paypalEventCaptureDenied    = "PAYMENT.CAPTURE.DENIED"
	paypalEventOrderApproved    = "CHECKOUT.ORDER.APPROVED"
	paypalEventCaptureRefunded  = "PAYMENT.CAPTURE.REFUNDED"

	paypalOrderStatusCompleted = "COMPLETED"
	paypalOrderStatusVoided    = "VOIDED"
)

// PayPal implements payment.Provider against PayPal Orders v2.
type PayPal struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
}

type paypalTokenState struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

var paypalAccessTokens sync.Map

// NewPayPal constructs a PayPal provider.
//
// config keys:
//   - clientId:      PayPal REST API client ID
//   - clientSecret:  PayPal REST API client secret
//   - webhookId:     ID of the webhook registered in PayPal, used for signature verification
//   - apiBase:       PayPal API host (sandbox: https://api-m.sandbox.paypal.com,
//                    production: https://api-m.paypal.com)
//   - currency:      default ISO 4217 currency code (e.g. USD); defaults to USD
//                    if not set, validated via payment.NormalizePaymentCurrency
//   - brandName:     optional brand name shown on PayPal's hosted page
func NewPayPal(instanceID string, config map[string]string) (*PayPal, error) {
	for _, k := range []string{"clientId", "clientSecret", "webhookId", "apiBase"} {
		if strings.TrimSpace(config[k]) == "" {
			return nil, fmt.Errorf("paypal config missing required key: %s", k)
		}
	}
	cfg := cloneStringMap(config)
	apiBase, err := normalizePaypalAPIBase(cfg["apiBase"])
	if err != nil {
		return nil, err
	}
	cfg["apiBase"] = apiBase
	currency := strings.TrimSpace(cfg["currency"])
	if currency == "" {
		currency = "USD"
	}
	normalized, err := payment.NormalizePaymentCurrency(currency)
	if err != nil {
		return nil, fmt.Errorf("paypal config currency: %w", err)
	}
	cfg["currency"] = normalized
	return &PayPal{
		instanceID: instanceID,
		config:     cfg,
		httpClient: &http.Client{Timeout: paypalHTTPTimeout},
	}, nil
}

func normalizePaypalAPIBase(raw string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if base == "" {
		return "", fmt.Errorf("paypal apiBase is required")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("paypal apiBase must be a valid URL")
	}
	host := strings.ToLower(parsed.Host)
	if host != "api-m.paypal.com" && host != "api-m.sandbox.paypal.com" {
		return "", fmt.Errorf("paypal apiBase host must be api-m.paypal.com or api-m.sandbox.paypal.com")
	}
	return base, nil
}

func (p *PayPal) Name() string        { return "PayPal" }
func (p *PayPal) ProviderKey() string { return payment.TypePayPal }
func (p *PayPal) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypePayPal}
}

func (p *PayPal) MerchantIdentityMetadata() map[string]string {
	if p == nil {
		return nil
	}
	clientID := strings.TrimSpace(p.config["clientId"])
	if clientID == "" {
		return nil
	}
	env := "production"
	if strings.Contains(p.config["apiBase"], "sandbox") {
		env = "sandbox"
	}
	return map[string]string{"client_id": clientID, "env": env}
}

// --- Auth ---

type paypalTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func (p *PayPal) tokenCacheKey() string {
	sum := sha256.Sum256([]byte(p.config["clientSecret"]))
	return p.config["apiBase"] + "|" + p.config["clientId"] + "|" + hex.EncodeToString(sum[:8])
}

func (p *PayPal) accessToken(ctx context.Context) (string, error) {
	cacheKey := p.tokenCacheKey()
	rawState, _ := paypalAccessTokens.LoadOrStore(cacheKey, &paypalTokenState{})
	state, ok := rawState.(*paypalTokenState)
	if !ok {
		return "", fmt.Errorf("paypal token cache state type mismatch")
	}
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.token != "" && time.Now().Add(paypalTokenSkew).Before(state.expiresAt) {
		return state.token, nil
	}

	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.config["apiBase"]+"/v1/oauth2/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(p.config["clientId"], p.config["clientSecret"])

	body, status, err := p.do(req)
	if err != nil {
		return "", fmt.Errorf("paypal auth: %w", err)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return "", formatPaypalAuthError(status, body)
	}
	var resp paypalTokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse paypal auth response: %w", err)
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return "", fmt.Errorf("paypal auth response missing access_token")
	}
	state.token = resp.AccessToken
	ttl := time.Duration(resp.ExpiresIn) * time.Second
	if ttl < time.Minute {
		ttl = 8 * time.Hour
	}
	state.expiresAt = time.Now().Add(ttl)
	return state.token, nil
}

func formatPaypalAuthError(status int, body []byte) error {
	summary := summarizePaypalResponse(body)
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("paypal auth HTTP %d: %s; check clientId/clientSecret and apiBase environment (sandbox: https://api-m.sandbox.paypal.com, production: https://api-m.paypal.com)", status, summary)
	}
	return fmt.Errorf("paypal auth HTTP %d: %s", status, summary)
}

// --- CreatePayment ---

type paypalAmount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

type paypalPurchaseUnit struct {
	ReferenceID string       `json:"reference_id,omitempty"`
	Description string       `json:"description,omitempty"`
	CustomID    string       `json:"custom_id,omitempty"`
	InvoiceID   string       `json:"invoice_id,omitempty"`
	Amount      paypalAmount `json:"amount"`
}

type paypalCreateOrderRequest struct {
	Intent             string                    `json:"intent"`
	PurchaseUnits      []paypalPurchaseUnit      `json:"purchase_units"`
	ApplicationContext *paypalApplicationContext `json:"application_context,omitempty"`
}

type paypalApplicationContext struct {
	BrandName          string `json:"brand_name,omitempty"`
	UserAction         string `json:"user_action,omitempty"`
	ShippingPreference string `json:"shipping_preference,omitempty"`
	ReturnURL          string `json:"return_url,omitempty"`
	CancelURL          string `json:"cancel_url,omitempty"`
}

type paypalLink struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}

type paypalOrderResponse struct {
	ID            string                       `json:"id"`
	Status        string                       `json:"status"`
	Links         []paypalLink                 `json:"links"`
	PurchaseUnits []paypalOrderResponseUnit    `json:"purchase_units"`
}

type paypalOrderResponseUnit struct {
	CustomID string                        `json:"custom_id,omitempty"`
	Amount   paypalAmount                  `json:"amount"`
	Payments *paypalOrderResponsePayments  `json:"payments,omitempty"`
}

type paypalOrderResponsePayments struct {
	Captures []paypalCapture `json:"captures"`
}

type paypalCapture struct {
	ID         string       `json:"id"`
	Status     string       `json:"status"`
	Amount     paypalAmount `json:"amount"`
	CustomID   string       `json:"custom_id,omitempty"`
	CreateTime string       `json:"create_time,omitempty"`
}

func (p *PayPal) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	if strings.TrimSpace(req.Amount) == "" {
		return nil, fmt.Errorf("paypal create: amount required")
	}
	if strings.TrimSpace(req.OrderID) == "" {
		return nil, fmt.Errorf("paypal create: orderID required")
	}
	currency := p.config["currency"]
	body := paypalCreateOrderRequest{
		Intent: "CAPTURE",
		PurchaseUnits: []paypalPurchaseUnit{{
			ReferenceID: req.OrderID,
			CustomID:    req.OrderID,
			Description: req.Subject,
			Amount:      paypalAmount{CurrencyCode: currency, Value: req.Amount},
		}},
		ApplicationContext: &paypalApplicationContext{
			BrandName:          strings.TrimSpace(p.config["brandName"]),
			UserAction:         "PAY_NOW",
			ShippingPreference: "NO_SHIPPING",
			ReturnURL:          req.ReturnURL,
			CancelURL:          req.ReturnURL,
		},
	}
	var resp paypalOrderResponse
	if err := p.doJSON(ctx, http.MethodPost, "/v2/checkout/orders", body, &resp); err != nil {
		return nil, fmt.Errorf("paypal create: %w", err)
	}
	approve := pickPaypalLink(resp.Links, "approve")
	if approve == "" {
		return nil, fmt.Errorf("paypal create: response missing approve link")
	}
	return &payment.CreatePaymentResponse{
		TradeNo:  resp.ID,
		PayURL:   approve,
		Currency: currency,
	}, nil
}

func pickPaypalLink(links []paypalLink, rel string) string {
	for _, l := range links {
		if strings.EqualFold(l.Rel, rel) {
			return l.Href
		}
	}
	return ""
}

// --- QueryOrder ---

func (p *PayPal) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, fmt.Errorf("paypal query: tradeNo required")
	}
	var resp paypalOrderResponse
	if err := p.doJSON(ctx, http.MethodGet, "/v2/checkout/orders/"+url.PathEscape(tradeNo), nil, &resp); err != nil {
		return nil, fmt.Errorf("paypal query: %w", err)
	}
	// intent=CAPTURE orders sit at APPROVED after the buyer approves until the
	// merchant captures. Capture here so the return-poll / reconciliation path
	// can complete the order without waiting on a webhook.
	if strings.EqualFold(strings.TrimSpace(resp.Status), "APPROVED") {
		if captured, capErr := p.captureOrder(ctx, tradeNo); capErr == nil && captured != nil {
			resp = *captured
		} else if isPaypalAlreadyCaptured(capErr) {
			// Race with the webhook-triggered capture: re-fetch the latest state.
			var refetched paypalOrderResponse
			if err := p.doJSON(ctx, http.MethodGet, "/v2/checkout/orders/"+url.PathEscape(tradeNo), nil, &refetched); err == nil {
				resp = refetched
			}
		}
		// Other capture errors: fall through with the APPROVED order (stays
		// Pending); the next poll or the APPROVED webhook retries the capture.
	}
	// Funds are settled only when a CAPTURE is COMPLETED. A COMPLETED *order*
	// whose capture is still PENDING (eCheck / risk review) must NOT be reported
	// as paid — that matches the webhook, which only credits on
	// PAYMENT.CAPTURE.COMPLETED. Only an explicitly voided order is a failure;
	// everything else (CREATED/APPROVED/COMPLETED-without-completed-capture)
	// stays pending until a completed capture appears.
	status := payment.ProviderStatusPending
	if strings.EqualFold(strings.TrimSpace(resp.Status), paypalOrderStatusVoided) {
		status = payment.ProviderStatusFailed
	}
	amount := 0.0
	paidAt := ""
	if len(resp.PurchaseUnits) > 0 {
		unit := resp.PurchaseUnits[0]
		amount, _ = strconv.ParseFloat(unit.Amount.Value, 64)
		if unit.Payments != nil {
			for _, capture := range unit.Payments.Captures {
				if strings.EqualFold(capture.Status, paypalOrderStatusCompleted) {
					status = payment.ProviderStatusPaid
					paidAt = capture.CreateTime
					// Capture responses may omit the purchase-unit amount; the
					// authoritative captured amount lives on the capture itself.
					if amount == 0 {
						if capAmt, err := strconv.ParseFloat(capture.Amount.Value, 64); err == nil && capAmt > 0 {
							amount = capAmt
						}
					}
					break
				}
			}
		}
	}
	return &payment.QueryOrderResponse{
		TradeNo:  tradeNo,
		Status:   status,
		Amount:   amount,
		PaidAt:   paidAt,
		Metadata: p.MerchantIdentityMetadata(),
	}, nil
}

// --- Capture ---

// captureOrder captures funds for an APPROVED order.
//
// PayPal Orders v2 with intent=CAPTURE does NOT move funds when the buyer
// approves on the hosted page — the merchant must explicitly call this endpoint.
// A successful capture transitions the order to COMPLETED and triggers the
// PAYMENT.CAPTURE.COMPLETED webhook. Capturing an already-captured order returns
// a 422 (ORDER_ALREADY_CAPTURED), which callers treat as a benign race.
func (p *PayPal) captureOrder(ctx context.Context, orderID string) (*paypalOrderResponse, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("paypal capture: orderID required")
	}
	var resp paypalOrderResponse
	// The capture endpoint requires a (possibly empty) JSON body.
	if err := p.doJSON(ctx, http.MethodPost, "/v2/checkout/orders/"+url.PathEscape(orderID)+"/capture", struct{}{}, &resp); err != nil {
		return nil, fmt.Errorf("paypal capture: %w", err)
	}
	return &resp, nil
}

// isPaypalAlreadyCaptured reports whether a capture error means the order was
// already captured/completed (safe to treat as success and re-query).
func isPaypalAlreadyCaptured(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "ORDER_ALREADY_CAPTURED") || strings.Contains(msg, "ORDER_ALREADY_COMPLETED")
}

// --- VerifyNotification ---

type paypalVerifyRequest struct {
	AuthAlgo         string          `json:"auth_algo"`
	CertURL          string          `json:"cert_url"`
	TransmissionID   string          `json:"transmission_id"`
	TransmissionSig  string          `json:"transmission_sig"`
	TransmissionTime string          `json:"transmission_time"`
	WebhookID        string          `json:"webhook_id"`
	WebhookEvent     json.RawMessage `json:"webhook_event"`
}

type paypalVerifyResponse struct {
	VerificationStatus string `json:"verification_status"`
}

type paypalWebhookEvent struct {
	ID         string                  `json:"id"`
	EventType  string                  `json:"event_type"`
	CreateTime string                  `json:"create_time"`
	Resource   paypalWebhookEventResource `json:"resource"`
}

type paypalWebhookEventResource struct {
	ID                 string                                  `json:"id"`
	Status             string                                  `json:"status"`
	Amount             *paypalAmount                           `json:"amount,omitempty"`
	CustomID           string                                  `json:"custom_id,omitempty"`
	InvoiceID          string                                  `json:"invoice_id,omitempty"`
	SupplementaryData  *paypalWebhookEventSupplementaryData    `json:"supplementary_data,omitempty"`
}

type paypalWebhookEventSupplementaryData struct {
	RelatedIDs map[string]string `json:"related_ids"`
}

func (p *PayPal) VerifyNotification(ctx context.Context, rawBody string, headers map[string]string) (*payment.PaymentNotification, error) {
	authAlgo := strings.TrimSpace(headers["paypal-auth-algo"])
	certURL := strings.TrimSpace(headers["paypal-cert-url"])
	transID := strings.TrimSpace(headers["paypal-transmission-id"])
	transSig := strings.TrimSpace(headers["paypal-transmission-sig"])
	transTime := strings.TrimSpace(headers["paypal-transmission-time"])
	if authAlgo == "" || certURL == "" || transID == "" || transSig == "" || transTime == "" {
		return nil, fmt.Errorf("paypal notification missing required PayPal-* headers")
	}

	verifyReq := paypalVerifyRequest{
		AuthAlgo:         authAlgo,
		CertURL:          certURL,
		TransmissionID:   transID,
		TransmissionSig:  transSig,
		TransmissionTime: transTime,
		WebhookID:        p.config["webhookId"],
		WebhookEvent:     json.RawMessage(rawBody),
	}
	var verifyResp paypalVerifyResponse
	if err := p.doJSON(ctx, http.MethodPost, "/v1/notifications/verify-webhook-signature", verifyReq, &verifyResp); err != nil {
		return nil, fmt.Errorf("paypal verify webhook: %w", err)
	}
	if !strings.EqualFold(verifyResp.VerificationStatus, "SUCCESS") {
		return nil, fmt.Errorf("paypal webhook signature verification failed (status=%s)", verifyResp.VerificationStatus)
	}

	var event paypalWebhookEvent
	if err := json.Unmarshal([]byte(rawBody), &event); err != nil {
		return nil, fmt.Errorf("paypal parse webhook event: %w", err)
	}

	// Only payment-completion events are relevant to the order lifecycle.
	// Other event types (CHECKOUT.ORDER.APPROVED, REFUNDED, etc.) return nil so
	// the caller acks with 200 without mutating state.
	var status string
	switch strings.ToUpper(event.EventType) {
	case paypalEventCaptureCompleted:
		status = payment.ProviderStatusSuccess
	case paypalEventCaptureDenied:
		status = payment.ProviderStatusFailed
	case paypalEventOrderApproved:
		// Buyer approved but funds are not yet captured (intent=CAPTURE requires an
		// explicit capture). Capture now; the resulting PAYMENT.CAPTURE.COMPLETED
		// webhook completes the order. resource.id here is the ORDER id.
		// Return nil so this event itself does not mutate order state.
		orderID := strings.TrimSpace(event.Resource.ID)
		if orderID != "" {
			if _, err := p.captureOrder(ctx, orderID); err != nil && !isPaypalAlreadyCaptured(err) {
				// Surface a transient capture failure as an error so PayPal retries
				// the APPROVED webhook (covers the buyer-closed-the-tab case).
				return nil, fmt.Errorf("paypal capture on approved order %s: %w", orderID, err)
			}
		}
		return nil, nil
	default:
		return nil, nil
	}

	orderID := strings.TrimSpace(event.Resource.CustomID)
	tradeNo := strings.TrimSpace(event.Resource.ID) // capture ID
	if event.Resource.SupplementaryData != nil {
		if related := event.Resource.SupplementaryData.RelatedIDs["order_id"]; strings.TrimSpace(related) != "" {
			tradeNo = strings.TrimSpace(related) // prefer order ID when available
		}
	}

	amount := 0.0
	if event.Resource.Amount != nil {
		amount, _ = strconv.ParseFloat(event.Resource.Amount.Value, 64)
	}

	metadata := p.MerchantIdentityMetadata()
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["event_type"] = event.EventType
	metadata["capture_id"] = event.Resource.ID

	return &payment.PaymentNotification{
		TradeNo:  tradeNo,
		OrderID:  orderID,
		Amount:   amount,
		Status:   status,
		RawData:  rawBody,
		Metadata: metadata,
	}, nil
}

// --- Refund ---

// Refund is intentionally not supported in this initial integration; callers
// must process refunds from the PayPal dashboard. Returning a typed error lets
// the admin UI surface the limitation cleanly.
func (p *PayPal) Refund(_ context.Context, _ payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("paypal refunds are not implemented in this build; process the refund from the PayPal merchant dashboard")
}

// --- HTTP plumbing ---

func (p *PayPal) doJSON(ctx context.Context, method, path string, payload, out any) error {
	var bodyReader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.config["apiBase"]+path, bodyReader)
	if err != nil {
		return err
	}
	token, err := p.accessToken(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	body, status, err := p.do(req)
	if err != nil {
		return err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", status, summarizePaypalResponse(body))
	}
	if out == nil || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

func (p *PayPal) do(req *http.Request) ([]byte, int, error) {
	client := p.httpClient
	if client == nil {
		client = &http.Client{Timeout: paypalHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, paypalMaxResponseSize))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func summarizePaypalResponse(body []byte) string {
	summary := strings.Join(strings.Fields(string(body)), " ")
	if summary == "" {
		return "<empty>"
	}
	if len(summary) > paypalMaxErrorSummary {
		return summary[:paypalMaxErrorSummary] + "..."
	}
	return summary
}

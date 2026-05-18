package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// Kyren constants. Kyren exposes an Epay-compatible API surface
// (submit.php / mapi.php / api.php) signed with MD5, plus the modern /v1/* API.
// This provider implements the Epay path because it supports per-method
// preselection via the `type` query parameter, matching the existing
// payment-method button UX.
const (
	kyrenHTTPTimeout    = 10 * time.Second
	maxKyrenResponseLen = 1 << 20 // 1MB
)

// kyrenEpayMethods maps internal payment types to Kyren's Epay `type` value.
// Kyren accepts: alipay, wxpay, creditcard, paynow, crypto.
var kyrenEpayMethods = map[string]string{
	payment.TypeAlipay: "alipay",
	payment.TypeWxpay:  "wxpay",
	payment.TypeCard:   "creditcard",
}

// Kyren implements payment.Provider for the Kyren payment gateway.
type Kyren struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
}

// NewKyren creates a new Kyren provider.
//
// config keys:
//   - pid:        Kyren merchant ID
//   - pkey:       Kyren API key (the secret used for MD5 signing — `kyren_live_*` / `kyren_test_*`)
//   - apiBase:    e.g. https://api.kyren.top (production) or https://test-api.kyren.top (sandbox)
//   - notifyUrl:  default async notify URL (overridable per request)
//   - returnUrl:  default browser return URL after payment (overridable per request)
func NewKyren(instanceID string, config map[string]string) (*Kyren, error) {
	for _, k := range []string{"pid", "pkey", "apiBase", "notifyUrl", "returnUrl"} {
		if strings.TrimSpace(config[k]) == "" {
			return nil, fmt.Errorf("kyren config missing required key: %s", k)
		}
	}
	cfg := make(map[string]string, len(config))
	for k, v := range config {
		cfg[k] = v
	}
	cfg["apiBase"] = normalizeEasyPayAPIBase(cfg["apiBase"])
	return &Kyren{
		instanceID: instanceID,
		config:     cfg,
		httpClient: &http.Client{Timeout: kyrenHTTPTimeout},
	}, nil
}

func (k *Kyren) Name() string        { return "Kyren" }
func (k *Kyren) ProviderKey() string { return payment.TypeKyren }
func (k *Kyren) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay, payment.TypeWxpay, payment.TypeCard}
}

func (k *Kyren) MerchantIdentityMetadata() map[string]string {
	if k == nil {
		return nil
	}
	pid := strings.TrimSpace(k.config["pid"])
	if pid == "" {
		return nil
	}
	return map[string]string{"pid": pid}
}

func (k *Kyren) apiBase() string {
	if k == nil {
		return ""
	}
	return normalizeEasyPayAPIBase(k.config["apiBase"])
}

// resolveURLs returns (notifyURL, returnURL) preferring per-request values,
// falling back to instance config defaults.
func (k *Kyren) resolveURLs(req payment.CreatePaymentRequest) (string, string) {
	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = k.config["notifyUrl"]
	}
	returnURL := req.ReturnURL
	if returnURL == "" {
		returnURL = k.config["returnUrl"]
	}
	return notifyURL, returnURL
}

// resolveEpayMethod maps an internal payment type to Kyren's `type` value.
func (k *Kyren) resolveEpayMethod(paymentType string) (string, error) {
	base := payment.GetBasePaymentType(paymentType)
	if v, ok := kyrenEpayMethods[base]; ok {
		return v, nil
	}
	if v, ok := kyrenEpayMethods[paymentType]; ok {
		return v, nil
	}
	return "", fmt.Errorf("kyren unsupported payment type: %s", paymentType)
}

// CreatePayment routes between mapi.php (default — returns payurl/qrcode in one call)
// and submit.php (popup mode — builds a redirect URL the browser opens).
func (k *Kyren) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	method, err := k.resolveEpayMethod(req.PaymentType)
	if err != nil {
		return nil, err
	}
	if k.config["paymentMode"] == paymentModePopup {
		return k.createRedirectPayment(req, method)
	}
	return k.createAPIPayment(ctx, req, method)
}

// createRedirectPayment builds a /epay/submit.php URL for browser redirect.
// TradeNo is empty; it arrives via the async notify callback after payment.
func (k *Kyren) createRedirectPayment(req payment.CreatePaymentRequest, method string) (*payment.CreatePaymentResponse, error) {
	notifyURL, returnURL := k.resolveURLs(req)
	params := map[string]string{
		"pid":          k.config["pid"],
		"type":         method,
		"out_trade_no": req.OrderID,
		"notify_url":   notifyURL,
		"return_url":   returnURL,
		"name":         req.Subject,
		"money":        req.Amount,
	}
	if req.IsMobile {
		params["device"] = deviceMobile
	}
	params["sign"] = easyPaySign(params, k.config["pkey"])
	params["sign_type"] = signTypeMD5

	q := url.Values{}
	for key, v := range params {
		q.Set(key, v)
	}
	payURL := k.apiBase() + "/epay/submit.php?" + q.Encode()
	return &payment.CreatePaymentResponse{PayURL: payURL}, nil
}

// createAPIPayment calls /epay/mapi.php and returns trade_no plus payurl/qrcode.
func (k *Kyren) createAPIPayment(ctx context.Context, req payment.CreatePaymentRequest, method string) (*payment.CreatePaymentResponse, error) {
	notifyURL, returnURL := k.resolveURLs(req)
	params := map[string]string{
		"pid":          k.config["pid"],
		"type":         method,
		"out_trade_no": req.OrderID,
		"notify_url":   notifyURL,
		"return_url":   returnURL,
		"name":         req.Subject,
		"money":        req.Amount,
		"clientip":     req.ClientIP,
	}
	if req.IsMobile {
		params["device"] = deviceMobile
	}
	params["sign"] = easyPaySign(params, k.config["pkey"])
	params["sign_type"] = signTypeMD5

	body, err := k.post(ctx, k.apiBase()+"/epay/mapi.php", params)
	if err != nil {
		return nil, fmt.Errorf("kyren create: %w", err)
	}
	var resp struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		TradeNo string `json:"trade_no"`
		PayURL  string `json:"payurl"`
		QRCode  string `json:"qrcode"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kyren parse: %w", err)
	}
	if resp.Code != easypayCodeSuccess {
		return nil, fmt.Errorf("kyren error: %s", resp.Msg)
	}
	return &payment.CreatePaymentResponse{
		TradeNo: resp.TradeNo,
		PayURL:  resp.PayURL,
		QRCode:  resp.QRCode,
	}, nil
}

func (k *Kyren) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	params := map[string]string{
		"act":          "order",
		"pid":          k.config["pid"],
		"key":          k.config["pkey"],
		"out_trade_no": tradeNo,
	}
	body, err := k.post(ctx, k.apiBase()+"/epay/api.php", params)
	if err != nil {
		return nil, fmt.Errorf("kyren query: %w", err)
	}
	var resp struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		Status int    `json:"status"`
		Money  string `json:"money"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kyren parse query: %w", err)
	}
	status := payment.ProviderStatusPending
	if resp.Status == easypayStatusPaid {
		status = payment.ProviderStatusPaid
	}
	amount, _ := strconv.ParseFloat(resp.Money, 64)
	return &payment.QueryOrderResponse{
		TradeNo:  tradeNo,
		Status:   status,
		Amount:   amount,
		Metadata: k.MerchantIdentityMetadata(),
	}, nil
}

// VerifyNotification parses Kyren's Epay-style notify callback.
// The callback arrives as application/x-www-form-urlencoded (or query string)
// with the same MD5 sign scheme as submit.php.
func (k *Kyren) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	values, err := url.ParseQuery(rawBody)
	if err != nil {
		return nil, fmt.Errorf("kyren parse notify: %w", err)
	}
	params := make(map[string]string, len(values))
	for key := range values {
		params[key] = values.Get(key)
	}
	sign := params["sign"]
	if sign == "" {
		return nil, fmt.Errorf("kyren missing sign")
	}
	if !easyPayVerifySign(params, k.config["pkey"], sign) {
		return nil, fmt.Errorf("kyren invalid signature")
	}
	status := payment.ProviderStatusFailed
	if params["trade_status"] == tradeStatusSuccess {
		status = payment.ProviderStatusSuccess
	}
	amount, _ := strconv.ParseFloat(params["money"], 64)

	metadata := k.MerchantIdentityMetadata()
	if pid := strings.TrimSpace(params["pid"]); pid != "" {
		if metadata == nil {
			metadata = map[string]string{}
		}
		metadata["pid"] = pid
	}
	return &payment.PaymentNotification{
		TradeNo:  params["trade_no"],
		OrderID:  params["out_trade_no"],
		Amount:   amount,
		Status:   status,
		RawData:  rawBody,
		Metadata: metadata,
	}, nil
}

// Refund returns an error because Kyren's public Epay endpoint does not expose
// programmatic refunds (per docs.kyren.top: "退款功能暂不支持，请联系客服").
// Refunds are dashboard-only at the time of writing; merchants must contact
// Kyren support.
func (k *Kyren) Refund(_ context.Context, _ payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("kyren does not support programmatic refunds; please initiate refund from the Kyren dashboard or contact Kyren support")
}

func (k *Kyren) post(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	form := url.Values{}
	for key, v := range params {
		form.Set(key, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := k.httpClient
	if client == nil {
		client = &http.Client{Timeout: kyrenHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxKyrenResponseLen))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("kyren HTTP %d: %s", resp.StatusCode, summarizeEasyPayResponse(body))
	}
	return body, nil
}

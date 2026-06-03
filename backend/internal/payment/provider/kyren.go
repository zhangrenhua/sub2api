package provider

import (
	"context"
	"net/url"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// Kyren implements payment.Provider for the Kyren gateway (https://kyren.top).
//
// Kyren is 易支付/epay-compatible, so alipay/wxpay reuse the exact EasyPay
// protocol (submit.php / mapi.php + MD5 sign). Kyren additionally exposes a
// credit-card channel via the submit.php redirect endpoint with type=creditcard.
//
// Implementation note: rather than duplicate the EasyPay protocol, Kyren embeds
// an *EasyPay (same package, so all unexported helpers/sign are shared) and
// delegates alipay/wxpay, query, verify and refund to it. Only the credit-card
// create path is Kyren-specific.
type Kyren struct {
	ep *EasyPay
}

// NewKyren creates a new Kyren provider.
// config keys: pid, pkey, apiBase, notifyUrl, returnUrl, cidAlipay, cidWxpay (optional).
// apiBase should point at Kyren's epay-compatible base, e.g. https://api.kyren.top/epay.
func NewKyren(instanceID string, config map[string]string) (*Kyren, error) {
	ep, err := NewEasyPay(instanceID, config)
	if err != nil {
		return nil, err
	}
	return &Kyren{ep: ep}, nil
}

func (k *Kyren) Name() string        { return "Kyren" }
func (k *Kyren) ProviderKey() string { return payment.TypeKyren }

func (k *Kyren) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay, payment.TypeWxpay, payment.TypeCreditCard}
}

func (k *Kyren) MerchantIdentityMetadata() map[string]string {
	return k.ep.MerchantIdentityMetadata()
}

// CreatePayment routes credit-card payments through the submit.php redirect
// endpoint (type=creditcard, per Kyren docs), and delegates alipay/wxpay to the
// shared EasyPay flow (which honours the instance's paymentMode).
func (k *Kyren) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	if req.PaymentType == payment.TypeCreditCard {
		return k.createCreditCardPayment(req)
	}
	return k.ep.CreatePayment(ctx, req)
}

// createCreditCardPayment builds a submit.php redirect URL with type=creditcard.
// Kyren's credit-card channel is a hosted-page (redirect) flow only — there is no
// cid for credit card, and TradeNo arrives later via the notify callback.
func (k *Kyren) createCreditCardPayment(req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	notifyURL, returnURL := k.ep.resolveURLs(req)
	params := map[string]string{
		"pid":          k.ep.config["pid"],
		"type":         payment.TypeCreditCard,
		"out_trade_no": req.OrderID,
		"notify_url":   notifyURL,
		"return_url":   returnURL,
		"name":         req.Subject,
		"money":        req.Amount,
	}
	if req.IsMobile {
		params["device"] = deviceMobile
	}
	params["sign"] = easyPaySign(params, k.ep.config["pkey"])
	params["sign_type"] = signTypeMD5

	q := url.Values{}
	for key, val := range params {
		q.Set(key, val)
	}
	payURL := k.ep.apiBase() + "/submit.php?" + q.Encode()
	return &payment.CreatePaymentResponse{PayURL: payURL}, nil
}

func (k *Kyren) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	resp, err := k.ep.QueryOrder(ctx, tradeNo)
	return resp, err
}

func (k *Kyren) VerifyNotification(ctx context.Context, rawBody string, headers map[string]string) (*payment.PaymentNotification, error) {
	return k.ep.VerifyNotification(ctx, rawBody, headers)
}

func (k *Kyren) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	return k.ep.Refund(ctx, req)
}

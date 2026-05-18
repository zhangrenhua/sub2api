//go:build unit

package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func kyrenBaseConfig(apiBase string) map[string]string {
	return map[string]string{
		"pid":       "1001",
		"pkey":      "kyren_test_secret",
		"apiBase":   apiBase,
		"notifyUrl": "https://example.com/webhook/kyren",
		"returnUrl": "https://example.com/return",
	}
}

func TestNewKyrenValidatesConfig(t *testing.T) {
	t.Parallel()

	_, err := NewKyren("1", map[string]string{
		"pid":  "1001",
		"pkey": "secret",
	})
	require.ErrorContains(t, err, "missing required key")

	prov, err := NewKyren("1", kyrenBaseConfig("https://test-api.kyren.top"))
	require.NoError(t, err)
	require.Equal(t, payment.TypeKyren, prov.ProviderKey())
	require.Equal(t, "Kyren", prov.Name())
	require.ElementsMatch(t, []payment.PaymentType{
		payment.TypeAlipay, payment.TypeWxpay, payment.TypeCard,
	}, prov.SupportedTypes())
	require.Equal(t, map[string]string{"pid": "1001"}, prov.MerchantIdentityMetadata())
}

func TestKyrenResolveEpayMethod(t *testing.T) {
	t.Parallel()

	prov, err := NewKyren("1", kyrenBaseConfig("https://test-api.kyren.top"))
	require.NoError(t, err)

	cases := map[string]string{
		payment.TypeAlipay: "alipay",
		payment.TypeWxpay:  "wxpay",
		payment.TypeCard:   "creditcard",
	}
	for in, want := range cases {
		got, err := prov.resolveEpayMethod(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got, in)
	}

	_, err = prov.resolveEpayMethod("paynow")
	require.ErrorContains(t, err, "unsupported payment type")
}

func TestKyrenCreatePaymentAPIMode(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/epay/mapi.php", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		values, err := url.ParseQuery(string(body))
		require.NoError(t, err)

		require.Equal(t, "1001", values.Get("pid"))
		require.Equal(t, "creditcard", values.Get("type"))
		require.Equal(t, "ORDER123", values.Get("out_trade_no"))
		require.Equal(t, "12.34", values.Get("money"))
		require.Equal(t, "203.0.113.1", values.Get("clientip"))
		require.NotEmpty(t, values.Get("sign"))
		require.Equal(t, "MD5", values.Get("sign_type"))

		// Verify the sign matches what easyPaySign would compute.
		params := map[string]string{}
		for k := range values {
			params[k] = values.Get(k)
		}
		expected := easyPaySign(params, "kyren_test_secret")
		require.Equal(t, expected, values.Get("sign"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"trade_no":"KYREN789","payurl":"https://test-api.kyren.top/pay/abc","qrcode":""}`))
	}))
	defer srv.Close()

	prov, err := NewKyren("1", kyrenBaseConfig(srv.URL))
	require.NoError(t, err)

	resp, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "ORDER123",
		Amount:      "12.34",
		PaymentType: payment.TypeCard,
		Subject:     "Recharge $12.34",
		ClientIP:    "203.0.113.1",
	})
	require.NoError(t, err)
	require.Equal(t, "KYREN789", resp.TradeNo)
	require.Equal(t, "https://test-api.kyren.top/pay/abc", resp.PayURL)
}

func TestKyrenCreatePaymentPopupMode(t *testing.T) {
	t.Parallel()

	cfg := kyrenBaseConfig("https://test-api.kyren.top")
	cfg["paymentMode"] = paymentModePopup
	prov, err := NewKyren("1", cfg)
	require.NoError(t, err)

	resp, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "ORD42",
		Amount:      "9.99",
		PaymentType: payment.TypeAlipay,
		Subject:     "Test",
	})
	require.NoError(t, err)
	require.Empty(t, resp.TradeNo, "popup mode does not return trade_no until notify")
	require.True(t, strings.HasPrefix(resp.PayURL, "https://test-api.kyren.top/epay/submit.php?"), resp.PayURL)

	u, err := url.Parse(resp.PayURL)
	require.NoError(t, err)
	q := u.Query()
	require.Equal(t, "alipay", q.Get("type"))
	require.Equal(t, "ORD42", q.Get("out_trade_no"))
	require.NotEmpty(t, q.Get("sign"))
}

func TestKyrenQueryOrder(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/epay/api.php", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		require.Equal(t, "order", values.Get("act"))
		require.Equal(t, "ORDER123", values.Get("out_trade_no"))

		_, _ = w.Write([]byte(`{"code":1,"status":1,"money":"12.34"}`))
	}))
	defer srv.Close()

	prov, err := NewKyren("1", kyrenBaseConfig(srv.URL))
	require.NoError(t, err)

	resp, err := prov.QueryOrder(context.Background(), "ORDER123")
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusPaid, resp.Status)
	require.InDelta(t, 12.34, resp.Amount, 0.0001)
}

func TestKyrenVerifyNotification(t *testing.T) {
	t.Parallel()

	prov, err := NewKyren("1", kyrenBaseConfig("https://test-api.kyren.top"))
	require.NoError(t, err)

	params := map[string]string{
		"pid":          "1001",
		"trade_no":     "KYREN789",
		"out_trade_no": "ORDER123",
		"type":         "alipay",
		"name":         "Test",
		"money":        "12.34",
		"trade_status": "TRADE_SUCCESS",
	}
	sign := easyPaySign(params, "kyren_test_secret")
	params["sign"] = sign
	params["sign_type"] = "MD5"

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	notification, err := prov.VerifyNotification(context.Background(), form.Encode(), nil)
	require.NoError(t, err)
	require.Equal(t, "KYREN789", notification.TradeNo)
	require.Equal(t, "ORDER123", notification.OrderID)
	require.Equal(t, payment.ProviderStatusSuccess, notification.Status)
	require.InDelta(t, 12.34, notification.Amount, 0.0001)
	require.Equal(t, "1001", notification.Metadata["pid"])

	// Tampered signature must fail.
	params["sign"] = strings.Repeat("0", 32)
	form = url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	_, err = prov.VerifyNotification(context.Background(), form.Encode(), nil)
	require.ErrorContains(t, err, "invalid signature")
}

func TestKyrenRefundReturnsNotSupported(t *testing.T) {
	t.Parallel()

	prov, err := NewKyren("1", kyrenBaseConfig("https://test-api.kyren.top"))
	require.NoError(t, err)

	_, err = prov.Refund(context.Background(), payment.RefundRequest{
		OrderID: "ORDER123",
		Amount:  "12.34",
	})
	require.ErrorContains(t, err, "does not support programmatic refunds")
}

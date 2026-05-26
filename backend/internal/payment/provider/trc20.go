package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// Mainnet USDT (TRC20) contract address. Overridable via instance config
// "usdtContract" for testnet (e.g. Shasta/Nile) verification.
const defaultUSDTContract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

// TRC20 implements payment.Provider for self-custodied USDT collection on the
// TRON network.
//
// Unlike gateway-backed providers, TRC20 has no upstream that pushes a webhook:
//   - CreatePayment returns the user's per-user deposit address (resolved and
//     persisted by the service layer and passed in via the request) plus the
//     exact USDT amount, encoded as a tron: URI for QR rendering.
//   - Payment confirmation is driven by the service layer's dedicated
//     reconciliation loop, which scans the deposit address for an inbound USDT
//     transfer matching the order. The generic QueryOrder path is therefore not
//     used for TRC20 matching (it lacks the order's expected amount/address).
//   - VerifyNotification is a no-op (no webhook).
//   - Refund is manual (on-chain transfers are irreversible).
type TRC20 struct {
	instanceID string
	config     map[string]string
}

// NewTRC20 builds a TRC20 provider from an instance's decrypted config.
// config keys: usdtContract (optional, defaults to mainnet), trongridApiBase
// (optional), confirmations (optional).
func NewTRC20(instanceID string, config map[string]string) (*TRC20, error) {
	cfg := make(map[string]string, len(config))
	for k, v := range config {
		cfg[k] = v
	}
	if strings.TrimSpace(cfg["usdtContract"]) == "" {
		cfg["usdtContract"] = defaultUSDTContract
	}
	return &TRC20{instanceID: instanceID, config: cfg}, nil
}

func (t *TRC20) Name() string        { return "USDT-TRC20" }
func (t *TRC20) ProviderKey() string { return payment.TypeTRC20 }
func (t *TRC20) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeTRC20}
}

// USDTContract returns the configured token contract address.
func (t *TRC20) USDTContract() string { return t.config["usdtContract"] }

// CreatePayment encodes the deposit address + amount for the frontend. The
// per-user deposit address is resolved by the service layer and passed via
// req.OpenID (reused here as a generic carrier to avoid widening the shared
// CreatePaymentRequest struct for a single provider).
func (t *TRC20) CreatePayment(_ context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	addr := strings.TrimSpace(req.OpenID)
	if addr == "" {
		return nil, fmt.Errorf("trc20: missing deposit address for order %s", req.OrderID)
	}
	// Encode the PLAIN TRON address in the QR (not a tron: URI). Exchange
	// withdrawal scanners (OKX, Binance, etc.) and wallet apps reliably parse a
	// bare address but mishandle URI schemes; the exact amount and the TRC20
	// network are surfaced as text by the frontend instead.
	return &payment.CreatePaymentResponse{
		QRCode:     addr,
		Currency:   "USD", // USDT is treated as USD-pegged within the currency framework
		ResultType: payment.CreatePaymentResultOrderCreated,
	}, nil
}

// QueryOrder is intentionally inert for TRC20: matching needs the order's
// expected amount and deposit address, which this signature does not carry.
// The service layer's ReconcilePendingTRC20Orders performs matching directly
// against the tron client. Returning pending keeps any accidental generic-path
// call safe (it will neither confirm nor cancel).
func (t *TRC20) QueryOrder(_ context.Context, _ string) (*payment.QueryOrderResponse, error) {
	return &payment.QueryOrderResponse{Status: payment.ProviderStatusPending}, nil
}

// VerifyNotification is a no-op: there is no upstream webhook for on-chain USDT.
func (t *TRC20) VerifyNotification(_ context.Context, _ string, _ map[string]string) (*payment.PaymentNotification, error) {
	return nil, nil
}

// Refund is unsupported: on-chain USDT transfers are irreversible and must be
// handled manually by an operator.
func (t *TRC20) Refund(_ context.Context, _ payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("trc20: refunds must be processed manually (on-chain transfers are irreversible)")
}

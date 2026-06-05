package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// Mainnet token contract addresses. Overridable via the matching instance
// config key (usdtContract / usdcContract) for testnet (e.g. Sepolia) use.
const (
	defaultERC20USDTContract = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
	defaultERC20USDCContract = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
)

// ERC20 implements payment.Provider for self-custodied stablecoin collection on
// the Ethereum network. It mirrors the TRC20 provider: no upstream webhook,
// confirmation is driven by the service's reconciliation loop, the QR encodes
// the plain deposit address, and refunds are manual.
//
// One type serves both USDT and USDC: they share the same chain, signer, and
// per-user deposit address (derivation is by network, not token) and differ only
// in the token contract, provider key, and the config key that overrides it.
type ERC20 struct {
	instanceID  string
	config      map[string]string
	name        string
	providerKey string
	contractKey string // config key holding the token contract (usdtContract / usdcContract)
}

// NewERC20 builds the USDT-ERC20 provider from an instance's decrypted config.
// config keys: usdtContract (optional, defaults to mainnet), etherscanApiBase
// (optional, defaults to the Etherscan V2 endpoint), chainId (optional, defaults
// to "1" mainnet; e.g. "11155111" for Sepolia), confirmSeconds (optional).
func NewERC20(instanceID string, config map[string]string) (*ERC20, error) {
	return newEthToken(instanceID, config, "USDT-ERC20", payment.TypeERC20, "usdtContract", defaultERC20USDTContract), nil
}

// NewUSDCERC20 builds the USDC-ERC20 provider, mirroring NewERC20 with the USDC
// contract and the "usdcContract" config override key.
func NewUSDCERC20(instanceID string, config map[string]string) (*ERC20, error) {
	return newEthToken(instanceID, config, "USDC-ERC20", payment.TypeUSDC, "usdcContract", defaultERC20USDCContract), nil
}

func newEthToken(instanceID string, config map[string]string, name, providerKey, contractKey, defaultContract string) *ERC20 {
	cfg := make(map[string]string, len(config))
	for k, v := range config {
		cfg[k] = v
	}
	if strings.TrimSpace(cfg[contractKey]) == "" {
		cfg[contractKey] = defaultContract
	}
	return &ERC20{instanceID: instanceID, config: cfg, name: name, providerKey: providerKey, contractKey: contractKey}
}

func (e *ERC20) Name() string        { return e.name }
func (e *ERC20) ProviderKey() string { return e.providerKey }
func (e *ERC20) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{e.providerKey}
}

// USDTContract returns the configured token contract address.
func (e *ERC20) USDTContract() string { return e.config[e.contractKey] }

// CreatePayment encodes the user's per-user Ethereum deposit address (resolved
// by the service layer and passed via req.OpenID) as the QR content.
func (e *ERC20) CreatePayment(_ context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	addr := strings.TrimSpace(req.OpenID)
	if addr == "" {
		return nil, fmt.Errorf("erc20: missing deposit address for order %s", req.OrderID)
	}
	// Plain address (not an ethereum: URI) for maximum wallet/exchange scanner
	// compatibility; amount and the ERC20 network are surfaced as text.
	return &payment.CreatePaymentResponse{
		QRCode:     addr,
		Currency:   "USD",
		ResultType: payment.CreatePaymentResultOrderCreated,
	}, nil
}

// QueryOrder is inert for ERC20: matching needs the order's amount and address,
// which this signature lacks. The service's ReconcilePendingERC20Orders matches
// directly against the Etherscan client.
func (e *ERC20) QueryOrder(_ context.Context, _ string) (*payment.QueryOrderResponse, error) {
	return &payment.QueryOrderResponse{Status: payment.ProviderStatusPending}, nil
}

// VerifyNotification is a no-op: on-chain USDT has no upstream webhook.
func (e *ERC20) VerifyNotification(_ context.Context, _ string, _ map[string]string) (*payment.PaymentNotification, error) {
	return nil, nil
}

// Refund is unsupported: on-chain transfers are irreversible; handle manually.
func (e *ERC20) Refund(_ context.Context, _ payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("erc20: refunds must be processed manually (on-chain transfers are irreversible)")
}

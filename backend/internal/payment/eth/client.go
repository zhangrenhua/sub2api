// Package eth provides Ethereum (ERC20) read access via the Etherscan V2 REST
// API: inbound USDT transfers (for payment reconciliation) and ETH / ERC20
// balances (for the admin wallet overview and sweep planning).
//
// Write paths (building, signing and broadcasting sweep transactions) live in
// signer.go and use go-ethereum's ethclient over JSON-RPC.
package eth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
)

const (
	// defaultEtherscanBase is the unified Etherscan V2 endpoint. The legacy V1
	// per-network hosts (api.etherscan.io/api, api-sepolia.etherscan.io/api, …)
	// are deprecated; V2 uses one base for every chain plus a chainid param.
	defaultEtherscanBase = "https://api.etherscan.io/v2/api"
	defaultChainID       = "1" // Ethereum mainnet
	httpTimeout          = 15 * time.Second
	maxRespBytes         = 8 << 20 // 8MB
	usdtDecimals         = 6       // mainnet USDT (ERC20) precision

	// Etherscan free tier allows 5 req/s with an API key; gate just under that.
	// Keyless access is far stricter (~1 req/5s), so back off hard when no key.
	keyedMinInterval   = 220 * time.Millisecond
	keylessMinInterval = 5 * time.Second
)

// Client is an Etherscan V2 REST client. Use NewClient.
type Client struct {
	apiBase string
	apiKey  string
	chainID string
	http    *http.Client

	// rate paces outbound calls to stay under Etherscan's per-second limit. A
	// single Client is reused for every call within one reconcile pass, so this
	// serializes that pass's requests; concurrent callers queue in arrival order.
	rateMu      sync.Mutex
	nextCallAt  time.Time
	minInterval time.Duration
}

// NewClient builds an Etherscan V2 client. apiBase may be empty (defaults to the
// unified V2 endpoint). chainID selects the network (empty → "1" mainnet; e.g.
// "11155111" for Sepolia). A single API key works across all V2 chains.
func NewClient(apiBase, apiKey, chainID string) *Client {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		base = defaultEtherscanBase
	}
	cid := strings.TrimSpace(chainID)
	if cid == "" {
		cid = defaultChainID
	}
	key := strings.TrimSpace(apiKey)
	minInterval := keyedMinInterval
	if key == "" {
		minInterval = keylessMinInterval
	}
	return &Client{
		apiBase:     base,
		apiKey:      key,
		chainID:     cid,
		http:        &http.Client{Timeout: httpTimeout},
		minInterval: minInterval,
	}
}

// ERC20Transfer is a single ERC20 token transfer event.
type ERC20Transfer struct {
	TxHash          string
	From            string
	To              string
	ContractAddress string
	Value           string // raw integer string in the token's smallest unit
	TokenDecimal    int
	BlockTime       int64 // unix seconds
	Confirmations   int64
}

// Amount converts the raw transfer value to a human USDT amount.
func (t ERC20Transfer) Amount() float64 {
	dec := t.TokenDecimal
	if dec <= 0 {
		dec = usdtDecimals
	}
	v, ok := new(big.Int).SetString(strings.TrimSpace(t.Value), 10)
	if !ok {
		return 0
	}
	return decimal.NewFromBigInt(v, int32(-dec)).InexactFloat64()
}

// InboundERC20Transfers returns recent transfers of the given token contract
// sent TO the address, newest first.
func (c *Client) InboundERC20Transfers(ctx context.Context, address, contract string, limit int) ([]ERC20Transfer, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := url.Values{}
	q.Set("chainid", c.chainID)
	q.Set("module", "account")
	q.Set("action", "tokentx")
	q.Set("contractaddress", contract)
	q.Set("address", address)
	q.Set("page", "1")
	q.Set("offset", strconv.Itoa(limit))
	q.Set("sort", "desc")
	if c.apiKey != "" {
		q.Set("apikey", c.apiKey)
	}

	body, err := c.get(ctx, c.apiBase+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Result  []struct {
			Hash            string `json:"hash"`
			From            string `json:"from"`
			To              string `json:"to"`
			Value           string `json:"value"`
			ContractAddress string `json:"contractAddress"`
			TokenDecimal    string `json:"tokenDecimal"`
			TimeStamp       string `json:"timeStamp"`
			Confirmations   string `json:"confirmations"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("eth: parse tokentx: %w", err)
	}
	// Etherscan returns status "0" with an empty result for "no transactions";
	// treat that as an empty (non-error) result.
	if resp.Status != "1" && len(resp.Result) == 0 {
		return nil, nil
	}
	out := make([]ERC20Transfer, 0, len(resp.Result))
	for _, r := range resp.Result {
		dec, _ := strconv.Atoi(strings.TrimSpace(r.TokenDecimal))
		ts, _ := strconv.ParseInt(strings.TrimSpace(r.TimeStamp), 10, 64)
		conf, _ := strconv.ParseInt(strings.TrimSpace(r.Confirmations), 10, 64)
		out = append(out, ERC20Transfer{
			TxHash:          r.Hash,
			From:            r.From,
			To:              r.To,
			ContractAddress: r.ContractAddress,
			Value:           r.Value,
			TokenDecimal:    dec,
			BlockTime:       ts,
			Confirmations:   conf,
		})
	}
	return out, nil
}

// ERC20Balance returns the address's balance of the given token as a human USDT amount.
func (c *Client) ERC20Balance(ctx context.Context, address, contract string) (float64, error) {
	q := url.Values{}
	q.Set("chainid", c.chainID)
	q.Set("module", "account")
	q.Set("action", "tokenbalance")
	q.Set("contractaddress", contract)
	q.Set("address", address)
	q.Set("tag", "latest")
	if c.apiKey != "" {
		q.Set("apikey", c.apiKey)
	}
	raw, err := c.resultString(ctx, c.apiBase+"?"+q.Encode())
	if err != nil {
		return 0, err
	}
	v, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok {
		return 0, nil
	}
	return decimal.NewFromBigInt(v, -usdtDecimals).InexactFloat64(), nil
}

// ETHBalance returns the address's native ETH balance (whole ETH). Used to
// gauge the fee wallet's gas runway.
func (c *Client) ETHBalance(ctx context.Context, address string) (float64, error) {
	q := url.Values{}
	q.Set("chainid", c.chainID)
	q.Set("module", "account")
	q.Set("action", "balance")
	q.Set("address", address)
	q.Set("tag", "latest")
	if c.apiKey != "" {
		q.Set("apikey", c.apiKey)
	}
	raw, err := c.resultString(ctx, c.apiBase+"?"+q.Encode())
	if err != nil {
		return 0, err
	}
	wei, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok {
		return 0, nil
	}
	return decimal.NewFromBigInt(wei, -18).InexactFloat64(), nil
}

// resultString fetches an Etherscan call whose "result" is a scalar string.
func (c *Client) resultString(ctx context.Context, endpoint string) (string, error) {
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return "", err
	}
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("eth: parse result: %w", err)
	}
	return resp.Result, nil
}

// throttle blocks until this Client's rate limiter admits another call, or ctx
// is cancelled. The slot is reserved up-front so concurrent callers don't all
// fire at once after a shared wait.
func (c *Client) throttle(ctx context.Context) error {
	c.rateMu.Lock()
	now := time.Now()
	if c.nextCallAt.Before(now) {
		c.nextCallAt = now
	}
	wait := c.nextCallAt.Sub(now)
	c.nextCallAt = c.nextCallAt.Add(c.minInterval)
	c.rateMu.Unlock()

	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) get(ctx context.Context, endpoint string) ([]byte, error) {
	if err := c.throttle(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eth: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return nil, fmt.Errorf("eth: read body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("eth: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// IsValidAddress reports whether s is a well-formed Ethereum hex address.
func IsValidAddress(s string) bool {
	s = strings.TrimSpace(s)
	if !common.IsHexAddress(s) {
		return false
	}
	return true
}

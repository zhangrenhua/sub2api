// Package tron provides a thin TronGrid REST client for the read paths needed
// by the TRC20 payment integration: looking up inbound USDT transfers (for
// payment reconciliation) and querying TRX / TRC20 balances (for the admin
// wallet overview and sweep planning).
//
// Write paths (building, signing and broadcasting sweep transactions) live with
// the sweep service and use gotron-sdk; they are intentionally not here.
package tron

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

	"github.com/shopspring/decimal"
)

const (
	defaultAPIBase = "https://api.trongrid.io"
	httpTimeout    = 15 * time.Second
	maxRespBytes   = 4 << 20 // 4MB
	// usdtDecimals is the TRC20 USDT contract's token precision (6).
	usdtDecimals = 6

	// TronGrid's free tier allows ~15 req/s with an API key; gate well under
	// that. Keyless access is heavily throttled, so back off hard when no key.
	keyedMinInterval   = 120 * time.Millisecond
	keylessMinInterval = 5 * time.Second
)

// Client is a TronGrid REST client. Zero value is not usable; use NewClient.
type Client struct {
	apiBase string
	apiKey  string
	http    *http.Client

	// rate paces outbound calls to stay under TronGrid's per-second limit. A
	// single Client is reused for every call within one reconcile pass, so this
	// serializes that pass's requests; concurrent callers queue in arrival order.
	rateMu      sync.Mutex
	nextCallAt  time.Time
	minInterval time.Duration
}

// NewClient builds a TronGrid client. apiBase may be empty (defaults to the
// public TronGrid endpoint). apiKey is the TronGrid API key (sent as the
// TRON-PRO-API-KEY header); empty is allowed but heavily rate-limited.
func NewClient(apiBase, apiKey string) *Client {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		base = defaultAPIBase
	}
	key := strings.TrimSpace(apiKey)
	minInterval := keyedMinInterval
	if key == "" {
		minInterval = keylessMinInterval
	}
	return &Client{
		apiBase:     base,
		apiKey:      key,
		http:        &http.Client{Timeout: httpTimeout},
		minInterval: minInterval,
	}
}

// TRC20Transfer is a single inbound/outbound TRC20 token transfer event.
type TRC20Transfer struct {
	TxID            string
	From            string
	To              string
	ContractAddress string
	// Value is the raw token amount as an integer string in the token's
	// smallest unit (for USDT, 1e6 = 1 USDT).
	Value         string
	BlockTimestmp int64 // unix milliseconds
}

// Amount converts the raw transfer value into a human USDT amount using the
// USDT contract's 6-decimal precision.
func (t TRC20Transfer) Amount() float64 {
	v, ok := new(big.Int).SetString(strings.TrimSpace(t.Value), 10)
	if !ok {
		return 0
	}
	return decimal.NewFromBigInt(v, -usdtDecimals).InexactFloat64()
}

// BlockTimeRFC3339 returns the block time formatted as RFC3339, or "" if unset.
func (t TRC20Transfer) BlockTimeRFC3339() string {
	if t.BlockTimestmp <= 0 {
		return ""
	}
	return time.UnixMilli(t.BlockTimestmp).UTC().Format(time.RFC3339)
}

// InboundTRC20Transfers returns recent TRC20 transfers sent TO the given
// address for the given token contract, newest first.
func (c *Client) InboundTRC20Transfers(ctx context.Context, address, contract string, limit int) ([]TRC20Transfer, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := url.Values{}
	q.Set("only_to", "true")
	q.Set("limit", strconv.Itoa(limit))
	q.Set("contract_address", contract)
	endpoint := fmt.Sprintf("%s/v1/accounts/%s/transactions/trc20?%s", c.apiBase, url.PathEscape(address), q.Encode())

	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			TransactionID string `json:"transaction_id"`
			From          string `json:"from"`
			To            string `json:"to"`
			Value         string `json:"value"`
			BlockTime     int64  `json:"block_timestamp"`
			TokenInfo     struct {
				Address string `json:"address"`
			} `json:"token_info"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("tron: parse trc20 transfers: %w", err)
	}
	out := make([]TRC20Transfer, 0, len(resp.Data))
	for _, d := range resp.Data {
		out = append(out, TRC20Transfer{
			TxID:            d.TransactionID,
			From:            d.From,
			To:              d.To,
			ContractAddress: d.TokenInfo.Address,
			Value:           d.Value,
			BlockTimestmp:   d.BlockTime,
		})
	}
	return out, nil
}

// TRC20Balance returns the address's balance of the given token contract as a
// human USDT amount.
func (c *Client) TRC20Balance(ctx context.Context, address, contract string) (float64, error) {
	acc, err := c.account(ctx, address)
	if err != nil {
		return 0, err
	}
	for _, t := range acc.TRC20 {
		for caddr, raw := range t {
			if strings.EqualFold(caddr, contract) {
				v, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
				if !ok {
					return 0, nil
				}
				return decimal.NewFromBigInt(v, -usdtDecimals).InexactFloat64(), nil
			}
		}
	}
	return 0, nil
}

// TRXBalance returns the address's native TRX balance (used to gauge gas runway
// of the fee wallet). Returned in whole TRX.
func (c *Client) TRXBalance(ctx context.Context, address string) (float64, error) {
	acc, err := c.account(ctx, address)
	if err != nil {
		return 0, err
	}
	// balance is in SUN (1 TRX = 1e6 SUN).
	return decimal.NewFromInt(acc.Balance).Div(decimal.New(1, 6)).InexactFloat64(), nil
}

type tronAccount struct {
	Balance int64               `json:"balance"`
	TRC20   []map[string]string `json:"trc20"`
}

func (c *Client) account(ctx context.Context, address string) (*tronAccount, error) {
	endpoint := fmt.Sprintf("%s/v1/accounts/%s", c.apiBase, url.PathEscape(address))
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Success bool          `json:"success"`
		Data    []tronAccount `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("tron: parse account: %w", err)
	}
	if len(resp.Data) == 0 {
		// Address not yet activated on-chain: treat as zero balances.
		return &tronAccount{}, nil
	}
	return &resp.Data[0], nil
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
	if c.apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tron: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return nil, fmt.Errorf("tron: read body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("tron: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

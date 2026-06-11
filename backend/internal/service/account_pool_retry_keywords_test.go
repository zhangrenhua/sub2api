//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPoolModeRetryKeywords(t *testing.T) {
	tests := []struct {
		name     string
		account  *Account
		expected []string
	}{
		{name: "nil_account_returns_nil", account: nil, expected: nil},
		{
			name:     "nil_credentials_returns_nil",
			account:  &Account{Type: AccountTypeAPIKey, Platform: PlatformAnthropic},
			expected: nil,
		},
		{
			name:     "missing_key_returns_nil",
			account:  &Account{Credentials: map[string]any{"pool_mode": true}},
			expected: nil,
		},
		{
			name: "trim_lowercase_dedup_and_drop_empty",
			account: &Account{
				Credentials: map[string]any{
					"pool_mode_retry_keywords": []any{"  Overloaded ", "overloaded", "Rate Limit", "", "  "},
				},
			},
			expected: []string{"overloaded", "rate limit"},
		},
		{
			name: "non_string_entries_skipped",
			account: &Account{
				Credentials: map[string]any{
					"pool_mode_retry_keywords": []any{"timeout", 123, true, "BUSY"},
				},
			},
			expected: []string{"timeout", "busy"},
		},
		{
			name: "wrong_type_returns_nil",
			account: &Account{
				Credentials: map[string]any{"pool_mode_retry_keywords": "overloaded"},
			},
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.account.GetPoolModeRetryKeywords())
		})
	}
}

func TestIsPoolModeRetryableBody(t *testing.T) {
	acc := &Account{
		Credentials: map[string]any{
			"pool_mode_retry_keywords": []any{"overloaded", "rate limit"},
		},
	}
	tests := []struct {
		name    string
		account *Account
		body    string
		want    bool
	}{
		{name: "case_insensitive_hit", account: acc, body: `{"error":"OVERLOADED, retry later"}`, want: true},
		{name: "phrase_hit", account: acc, body: `you hit the rate limit`, want: true},
		{name: "no_hit", account: acc, body: `{"error":"invalid request"}`, want: false},
		{name: "empty_body_no_hit", account: acc, body: ``, want: false},
		{
			name:    "no_keywords_configured_no_hit",
			account: &Account{Credentials: map[string]any{"pool_mode": true}},
			body:    `overloaded`,
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.IsPoolModeRetryableBody([]byte(tt.body)))
		})
	}
}

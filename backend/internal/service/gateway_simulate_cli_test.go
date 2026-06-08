package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// ginCtxWithUA builds a gin.Context whose request carries the given User-Agent.
func ginCtxWithUA(ua string) *gin.Context {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	if ua != "" {
		c.Request.Header.Set("User-Agent", ua)
	}
	return c
}

// legacy metadata.user_id that ParseMetadataUserID accepts (device=64hex, session=36).
const realCliMetadataUserID = "user_" +
	"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
	"_account__session_12345678-1234-1234-1234-123456789012"

func TestShouldSimulateCliHeaders(t *testing.T) {
	s := &GatewayService{}
	anthropicAPIKey := func(flag bool) *Account {
		return &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, SimulateClaudeCliClient: flag}
	}

	cases := []struct {
		name    string
		ctx     context.Context
		account *Account
		ua      string
		body    string
		want    bool
	}{
		{
			name:    "anthropic apikey + flag on + non-CLI client => simulate",
			ctx:     context.Background(),
			account: anthropicAPIKey(true),
			ua:      "curl/8.4.0",
			body:    `{"model":"claude-sonnet-4-5","messages":[]}`,
			want:    true,
		},
		{
			name:    "real claude-cli client (UA + metadata) => passthrough, no simulate",
			ctx:     context.Background(),
			account: anthropicAPIKey(true),
			ua:      "claude-cli/2.1.0 (external, cli)",
			body:    `{"model":"claude-sonnet-4-5","metadata":{"user_id":"` + realCliMetadataUserID + `"}}`,
			want:    false,
		},
		{
			name:    "ctx already marked Claude Code => no simulate",
			ctx:     SetClaudeCodeClient(context.Background(), true),
			account: anthropicAPIKey(true),
			ua:      "curl/8.4.0",
			body:    `{"model":"claude-sonnet-4-5"}`,
			want:    false,
		},
		{
			name:    "flag off => no simulate",
			ctx:     context.Background(),
			account: anthropicAPIKey(false),
			ua:      "curl/8.4.0",
			body:    `{"model":"claude-sonnet-4-5"}`,
			want:    false,
		},
		{
			name:    "wrong platform (openai) => no simulate",
			ctx:     context.Background(),
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, SimulateClaudeCliClient: true},
			ua:      "curl/8.4.0",
			body:    `{"model":"gpt-5"}`,
			want:    false,
		},
		{
			name:    "wrong type (oauth) => no simulate",
			ctx:     context.Background(),
			account: &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, SimulateClaudeCliClient: true},
			ua:      "curl/8.4.0",
			body:    `{"model":"claude-sonnet-4-5"}`,
			want:    false,
		},
		{
			name:    "nil account => no simulate",
			ctx:     context.Background(),
			account: nil,
			ua:      "curl/8.4.0",
			body:    `{}`,
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.shouldSimulateCliHeaders(tc.ctx, ginCtxWithUA(tc.ua), tc.account, []byte(tc.body))
			require.Equal(t, tc.want, got)
		})
	}
}

// TestApplyClaudeCodeMimicHeaders_RewritesIdentity proves the actual "effect":
// once shouldSimulateCliHeaders is true, the builder's applyClaudeCodeMimicHeaders
// rewrites the outbound headers to the Claude CLI fingerprint.
func TestApplyClaudeCodeMimicHeaders_RewritesIdentity(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	// Simulate a leftover client UA that must be overwritten.
	req.Header.Set("User-Agent", "curl/8.4.0")

	applyClaudeCodeMimicHeaders(req, false)

	// Headers are written with exact wire casing via setHeaderRaw; read them with
	// getHeaderRaw (casing-insensitive) rather than canonical http.Header.Get.
	ua := getHeaderRaw(req.Header, "User-Agent")
	require.Truef(t, strings.HasPrefix(strings.ToLower(ua), "claude-cli/"),
		"User-Agent should be rewritten to claude-cli/*, got %q", ua)
	require.Equal(t, "cli", getHeaderRaw(req.Header, "x-app"))
	require.NotEmpty(t, getHeaderRaw(req.Header, "x-stainless-lang"))
	require.Equal(t, "application/json", getHeaderRaw(req.Header, "Accept"))
	require.NotEmpty(t, getHeaderRaw(req.Header, "x-client-request-id"))

	// streaming variant sets the stream helper-method
	reqStream := httptest.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	applyClaudeCodeMimicHeaders(reqStream, true)
	require.Equal(t, "stream", getHeaderRaw(reqStream.Header, "x-stainless-helper-method"))
}

// TestApplySimulateClaudeCliBody is the core of the upstream-detection fix:
// for a simulating account, the outbound body must (1) drop `temperature` and
// (2) carry `metadata.user_id` in the JSON "new" format
// {"device_id","account_uuid","session_id"} — the two discriminators the
// upstream "official Claude Code client" gate actually checks. Legacy/absent
// metadata or a present temperature get the request silently rejected upstream.
func TestApplySimulateClaudeCliBody(t *testing.T) {
	s := &GatewayService{}
	const deviceID = "9c99211590831dcaee9aed07f57c8edadbe5010ce4b3c98ef0aa2a51dd7c82b5"
	// claude_user_id present => deviceID resolves without the fingerprint service,
	// so a nil gin.Context / identityService is fine here.
	acc := &Account{
		ID:       7,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"claude_user_id": deviceID},
	}

	t.Run("strips temperature and forces JSON metadata", func(t *testing.T) {
		body := []byte(`{"model":"claude-opus-4-8","max_tokens":256,"temperature":0.7,` +
			`"messages":[{"role":"user","content":"hi"}]}`)
		out := s.applySimulateClaudeCliBody(context.Background(), nil, body, nil, acc)

		require.False(t, gjson.GetBytes(out, "temperature").Exists(),
			"temperature must be removed (upstream rejects requests that carry it)")

		uid := gjson.GetBytes(out, "metadata.user_id").String()
		require.Truef(t, strings.HasPrefix(uid, "{"), "metadata.user_id must be JSON, got %q", uid)
		require.Equal(t, deviceID, gjson.Get(uid, "device_id").String())
		require.NotEmpty(t, gjson.Get(uid, "session_id").String())
	})

	t.Run("already-JSON metadata is left untouched", func(t *testing.T) {
		existing := `{"device_id":"abc","account_uuid":"u","session_id":"s"}`
		body := []byte(`{"model":"claude-opus-4-8","metadata":{"user_id":` +
			strconv.Quote(existing) + `}}`)
		out := s.applySimulateClaudeCliBody(context.Background(), nil, body, nil, acc)
		require.JSONEq(t, existing, gjson.GetBytes(out, "metadata.user_id").String())
	})

	t.Run("nil account is a no-op", func(t *testing.T) {
		body := []byte(`{"temperature":0.5}`)
		out := s.applySimulateClaudeCliBody(context.Background(), nil, body, nil, nil)
		require.JSONEq(t, `{"temperature":0.5}`, string(out))
	})
}

// TestAggregateAnthropicStreamToResponse covers the SSE→non-stream aggregation
// that backs the "upstream stream, client non-stream" path: text/thinking deltas
// concatenate by block index, stop_reason comes from message_delta, and usage is
// merged across message_start (input/cache) + message_delta (output).
func TestAggregateAnthropicStreamToResponse(t *testing.T) {
	s := &GatewayService{}
	sse := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-4-8","content":[],"usage":{"input_tokens":10,"cache_read_input_tokens":2}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hmm"}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Hello"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":" world"}}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	resp, err := s.aggregateAnthropicStreamToResponse(strings.NewReader(sse))
	require.NoError(t, err)
	require.Equal(t, "msg_1", resp.ID)
	require.Equal(t, "message", resp.Type)
	require.Equal(t, "end_turn", resp.StopReason)
	require.Len(t, resp.Content, 2)
	require.Equal(t, "hmm", resp.Content[0].Thinking)
	require.Equal(t, "Hello world", resp.Content[1].Text)
	require.Equal(t, 10, resp.Usage.InputTokens)
	require.Equal(t, 5, resp.Usage.OutputTokens)
	require.Equal(t, 2, resp.Usage.CacheReadInputTokens)

	// Marshals to a normal Anthropic Message the non-stream tail can re-parse.
	out, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Equal(t, "Hello world", gjson.GetBytes(out, "content.1.text").String())
	require.Equal(t, int64(5), gjson.GetBytes(out, "usage.output_tokens").Int())
}

func TestAggregateAnthropicStreamToResponse_NoMessage(t *testing.T) {
	s := &GatewayService{}
	_, err := s.aggregateAnthropicStreamToResponse(strings.NewReader("event: ping\ndata: {}\n\n"))
	require.Error(t, err)
}

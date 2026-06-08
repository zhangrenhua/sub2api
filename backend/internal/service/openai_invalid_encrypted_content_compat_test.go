package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestIsOpenAIInvalidEncryptedContentError 覆盖共享判定 helper：精确 code（含中转非标准
// code thinking_signature_invalid）+ 消息兜底，并验证「必须含 encrypted content」的强约束
// 不会被部分关键词误触发。
func TestIsOpenAIInvalidEncryptedContentError(t *testing.T) {
	cases := []struct {
		name string
		code string
		msg  string
		want bool
	}{
		{name: "标准 code", code: "invalid_encrypted_content", msg: "", want: true},
		{name: "中转非标准 code", code: "thinking_signature_invalid", msg: "", want: true},
		{name: "code 大小写+空白", code: "  Thinking_Signature_Invalid ", msg: "", want: true},
		{name: "消息含标准 code 串", code: "", msg: "upstream said invalid_encrypted_content", want: true},
		{name: "消息 verified 措辞", code: "", msg: "The encrypted content could not be verified.", want: true},
		{name: "消息 decrypted 措辞", code: "", msg: "Encrypted content could not be decrypted or parsed.", want: true},
		{name: "真实错误：非标准 code + 混合中文消息", code: "thinking_signature_invalid", msg: "The encrypted content - 运行验证 could not be verified. Reason: Encrypted content could not be decrypted or parsed.", want: true},
		{name: "无关 code + 无关消息", code: "rate_limit_exceeded", msg: "too many requests", want: false},
		{name: "强约束：verified 但不含 encrypted content", code: "", msg: "signature could not be verified", want: false},
		{name: "强约束：仅 encrypted content 无动词", code: "", msg: "the encrypted content is here", want: false},
		{name: "空 code + 空消息", code: "", msg: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isOpenAIInvalidEncryptedContentError(tc.code, tc.msg))
		})
	}
}

// TestClassifyOpenAIWSErrorEventFromRaw_InvalidEncryptedCompat 验证 WS 分类器经共享 helper
// 收敛后，也能识别非标准 code thinking_signature_invalid 与 decrypted 措辞（消除与 HTTP
// 路径的不对称），且不影响其他分类。
func TestClassifyOpenAIWSErrorEventFromRaw_InvalidEncryptedCompat(t *testing.T) {
	cases := []struct {
		name       string
		code       string
		errType    string
		msg        string
		wantReason string
		wantOK     bool
	}{
		{name: "非标准 code", code: "thinking_signature_invalid", wantReason: "invalid_encrypted_content", wantOK: true},
		{name: "标准 code", code: "invalid_encrypted_content", wantReason: "invalid_encrypted_content", wantOK: true},
		{name: "decrypted 消息兜底", msg: "Encrypted content could not be decrypted or parsed.", wantReason: "invalid_encrypted_content", wantOK: true},
		{name: "verified 消息兜底", msg: "The encrypted content could not be verified.", wantReason: "invalid_encrypted_content", wantOK: true},
		// 回归：其他分类不受影响
		{name: "升级要求不受影响", code: "upgrade_required", wantReason: "upgrade_required", wantOK: true},
		{name: "previous_response 不受影响", code: "previous_response_not_found", wantReason: "previous_response_not_found", wantOK: true},
		{name: "无关错误仍为 event_error", code: "boom", msg: "something else", wantReason: "event_error", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, ok := classifyOpenAIWSErrorEventFromRaw(tc.code, tc.errType, tc.msg)
			require.Equal(t, tc.wantReason, reason)
			require.Equal(t, tc.wantOK, ok)
		})
	}
}

// TestOpenAIGatewayService_Forward_HTTPIngressRetriesThinkingSignatureInvalidOnce 验证：
// 当中转返回非标准 code thinking_signature_invalid（且消息不含 encrypted-content 关键词，
// 即恢复完全由新增的 code 识别触发）时，非 WSv2 HTTP 路径也会删掉 reasoning 的
// encrypted_content 并重试一次，然后成功。
func TestOpenAIGatewayService_Forward_HTTPIngressRetriesThinkingSignatureInvalidOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	wsFallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer wsFallbackServer.Close()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "custom-client/1.0")
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	upstream := &httpUpstreamSequenceRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"error":{"code":"thinking_signature_invalid","type":"invalid_request_error","message":"thinking signature could not be parsed"}}`,
				)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"id":"resp_http_retry_tsig_ok","usage":{"input_tokens":1,"output_tokens":2,"input_tokens_details":{"cached_tokens":0}}}`,
				)),
			},
		},
	}

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
	}

	account := &Account{
		ID:          103,
		Name:        "openai-apikey-relay",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": wsFallbackServer.URL,
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"reasoning","encrypted_content":"gAAA","summary":[{"type":"summary_text","text":"keep me"}]},{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.OpenAIWSMode, "HTTP 入站应保持 HTTP 转发")
	require.Equal(t, 2, upstream.callCount, "命中 thinking_signature_invalid 后应只在 HTTP 路径重试一次")
	require.Len(t, upstream.bodies, 2)

	secondBody := upstream.bodies[1]
	require.Len(t, gjson.GetBytes(secondBody, "input").Array(), 1, "HTTP 重试应整项删除带 encrypted_content 的 reasoning 项")
	require.False(t, gjson.GetBytes(secondBody, `input.#(type=="reasoning")`).Exists(), "重试后不应再有 reasoning 项（连 id/summary 一并删除）")
	require.Equal(t, "input_text", gjson.GetBytes(secondBody, "input.0.type").String(), "非 reasoning input 应保持原样")
	require.Equal(t, "hello", gjson.GetBytes(secondBody, "input.0.text").String())
}

// TestSanitizeOpenAIEncryptedContentForRetry 覆盖 HTTP 专用的「整项删除」恢复函数：
// 丢弃任何 reasoning 项或子树含 encrypted_content 的顶层 input 项（绝不剥字段）+ 结构诊断。
func TestSanitizeOpenAIEncryptedContentForRetry(t *testing.T) {
	t.Run("丢弃所有 reasoning 项（含无顶层密文的）", func(t *testing.T) {
		body := map[string]any{
			"input": []any{
				map[string]any{"type": "reasoning", "id": "rs_1", "summary": []any{}, "encrypted_content": "gAAA"},
				map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "hi"}}},
				map[string]any{"type": "reasoning", "id": "rs_2"}, // 无顶层 encrypted_content 也要删
			},
		}
		diag, ok := sanitizeOpenAIEncryptedContentForRetry(body)
		require.True(t, ok)
		require.Equal(t, 2, diag.droppedReasoning, "两个 reasoning 项都应删除")
		require.Equal(t, 0, diag.residualEnc, "清理后不应残留 encrypted_content")
		items := body["input"].([]any)
		require.Len(t, items, 1)
		require.Equal(t, "message", items[0].(map[string]any)["type"])
	})

	t.Run("非 reasoning item 含嵌套 encrypted_content → 整项删除", func(t *testing.T) {
		// 模拟生产：坏密文挂在非 reasoning 承载点（嵌套 content 里）→ 整项删除该 item，
		// 绝不只剥字段（剥字段会被上游判 missing_required_parameter）。
		body := map[string]any{
			"input": []any{
				map[string]any{"type": "message", "role": "assistant", "content": []any{
					map[string]any{"type": "output_text", "text": "ok", "encrypted_content": "gAAA_bad"},
				}},
				map[string]any{"type": "input_text", "text": "go on"},
			},
		}
		diag, ok := sanitizeOpenAIEncryptedContentForRetry(body)
		require.True(t, ok, "含 encrypted_content 的 item 应触发删除")
		require.Equal(t, 0, diag.droppedReasoning)
		require.Equal(t, 1, diag.droppedEncrypted, "含嵌套 encrypted_content 的非 reasoning item 应整项删除")
		require.Equal(t, 0, diag.residualEnc, "整项删除后不应残留 encrypted_content")
		items := body["input"].([]any)
		require.Len(t, items, 1, "只剩不含密文的 input_text")
		require.Equal(t, "input_text", items[0].(map[string]any)["type"])
		require.Equal(t, "go on", items[0].(map[string]any)["text"])
	})

	t.Run("无 reasoning 且无密文 → 不改动", func(t *testing.T) {
		body := map[string]any{"input": []any{map[string]any{"type": "input_text", "text": "x"}}}
		_, ok := sanitizeOpenAIEncryptedContentForRetry(body)
		require.False(t, ok)
		require.Len(t, body["input"].([]any), 1)
	})

	t.Run("全为 reasoning → 删空后移除 input 键", func(t *testing.T) {
		body := map[string]any{"input": []any{map[string]any{"type": "reasoning", "encrypted_content": "gAAA"}}}
		diag, ok := sanitizeOpenAIEncryptedContentForRetry(body)
		require.True(t, ok)
		require.Equal(t, 1, diag.droppedReasoning)
		_, stillHas := body["input"]
		require.False(t, stillHas, "input 全删后应移除该键")
	})

	t.Run("[]map[string]any 形态", func(t *testing.T) {
		body := map[string]any{
			"input": []map[string]any{
				{"type": "reasoning", "id": "rs_1", "encrypted_content": "gAAA"},
				{"type": "input_text", "text": "hi"},
			},
		}
		diag, ok := sanitizeOpenAIEncryptedContentForRetry(body)
		require.True(t, ok)
		require.Equal(t, 1, diag.droppedReasoning)
		items := body["input"].([]any)
		require.Len(t, items, 1)
		require.Equal(t, "input_text", items[0].(map[string]any)["type"])
	})

	t.Run("无 input → 不改动", func(t *testing.T) {
		_, ok := sanitizeOpenAIEncryptedContentForRetry(map[string]any{})
		require.False(t, ok)
	})
}

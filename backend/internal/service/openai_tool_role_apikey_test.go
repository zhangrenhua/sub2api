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

// TestOpenAIGatewayService_Forward_APIKeyNormalizesToolRoleMessage 验证：
// API-key 账号走 Responses API 时，输入里的 role:"tool" 消息会被归一化为
// function_call_output（OAuth 的 codex transform 不覆盖 API-key，过去会直接把
// role:"tool" 透到上游，被判 invalid_value: input[i] 'tool'）。
func TestOpenAIGatewayService_Forward_APIKeyNormalizesToolRoleMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          7,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	// input[1] 是 Chat 风格的 role:"tool" 工具结果，混进了 Responses 请求
	body := []byte(`{"model":"gpt-5","stream":false,"input":[` +
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},` +
		`{"role":"tool","tool_call_id":"call_1","content":"42"}` +
		`]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)

	// 上游 body 里不应再有任何 role:"tool"
	require.False(t, gjson.GetBytes(upstream.lastBody, `input.#(role=="tool")`).Exists(), "role:tool 不应透到上游")
	// 应被转成 function_call_output，保留 call_id 与文本
	require.Equal(t, "function_call_output", gjson.GetBytes(upstream.lastBody, "input.1.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(upstream.lastBody, "input.1.call_id").String())
	require.Equal(t, "42", gjson.GetBytes(upstream.lastBody, "input.1.output").String())
	// 普通 user 消息原样保留
	require.Equal(t, "message", gjson.GetBytes(upstream.lastBody, "input.0.type").String())
	require.Equal(t, "user", gjson.GetBytes(upstream.lastBody, "input.0.role").String())
}

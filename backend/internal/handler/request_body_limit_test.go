package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyLimitTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limit := int64(16)
	router := gin.New()
	router.Use(middleware.RequestBodyLimit(limit))
	router.POST("/test", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			if maxErr, ok := extractMaxBytesError(err); ok {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": buildBodyTooLargeMessage(maxErr.Limit),
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "read_failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	payload := bytes.Repeat([]byte("a"), int(limit+1))
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(payload))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Contains(t, recorder.Body.String(), buildBodyTooLargeMessage(limit))
}

func TestEstimateRequestTokens_OpenAIMessages(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "Hello, how are you?"}
		]
	}`)
	tokens := estimateRequestTokens(body)
	// "You are a helpful assistant." = 29 runes, "Hello, how are you?" = 19 runes
	// Total 48 runes => ~16 tokens
	assert.Greater(t, tokens, int64(0))
	assert.Less(t, tokens, int64(100))
}

func TestEstimateRequestTokens_OpenAIMultipartContent(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": [
				{"type": "text", "text": "What is in this image?"},
				{"type": "image_url", "image_url": {"url": "https://example.com/img.png"}}
			]}
		]
	}`)
	tokens := estimateRequestTokens(body)
	// "What is in this image?" = 22 runes => ~7 tokens
	assert.Greater(t, tokens, int64(0))
	assert.Less(t, tokens, int64(50))
}

func TestEstimateRequestTokens_AnthropicMessages(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-opus",
		"system": "You are a helpful assistant.",
		"messages": [
			{"role": "user", "content": "Hello!"}
		]
	}`)
	tokens := estimateRequestTokens(body)
	// "You are a helpful assistant." = 29 runes, "Hello!" = 6 runes
	// Total 35 runes => ~12 tokens
	assert.Greater(t, tokens, int64(0))
	assert.Less(t, tokens, int64(50))
}

func TestEstimateRequestTokens_AnthropicSystemArray(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-opus",
		"system": [
			{"type": "text", "text": "You are a helpful assistant."},
			{"type": "text", "text": "Be concise."}
		],
		"messages": [
			{"role": "user", "content": "Hi"}
		]
	}`)
	tokens := estimateRequestTokens(body)
	assert.Greater(t, tokens, int64(0))
}

func TestEstimateRequestTokens_GeminiContents(t *testing.T) {
	body := []byte(`{
		"contents": [
			{"parts": [{"text": "Hello, world!"}]},
			{"parts": [{"text": "How are you today?"}]}
		],
		"systemInstruction": {
			"parts": [{"text": "You are helpful."}]
		}
	}`)
	tokens := estimateRequestTokens(body)
	assert.Greater(t, tokens, int64(0))
	assert.Less(t, tokens, int64(50))
}

func TestEstimateRequestTokens_OpenAIResponsesAPI(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4",
		"input": "Tell me a story about a cat.",
		"instructions": "You are a storyteller."
	}`)
	tokens := estimateRequestTokens(body)
	assert.Greater(t, tokens, int64(0))
	assert.Less(t, tokens, int64(50))
}

func TestEstimateRequestTokens_ResponsesAPIInputArray(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4",
		"input": [
			{"role": "user", "content": "Hello!"},
			{"role": "assistant", "content": "Hi there!"},
			{"role": "user", "content": "How are you?"}
		]
	}`)
	tokens := estimateRequestTokens(body)
	assert.Greater(t, tokens, int64(0))
}

func TestEstimateRequestTokens_FallbackEmptyMessages(t *testing.T) {
	body := []byte(`{"model": "gpt-4", "some_field": "value"}`)
	tokens := estimateRequestTokens(body)
	// Fallback to entire body rune count
	assert.Greater(t, tokens, int64(0))
}

func TestEstimateRequestTokens_LargeContent(t *testing.T) {
	// 600k runes of text => should estimate ~200k tokens
	largeText := strings.Repeat("a", 600000)
	body := []byte(`{"messages":[{"role":"user","content":"` + largeText + `"}]}`)
	tokens := estimateRequestTokens(body)
	assert.InDelta(t, 200000, tokens, 10)
}

func TestEstimateRequestTokens_LargeBase64Image(t *testing.T) {
	// 模拟3MB base64图片，文本内容很少
	base64Data := strings.Repeat("iVBORw0KGgo", 300000) // ~3MB base64
	body := []byte(`{"messages":[{"role":"user","content":[` +
		`{"type":"text","text":"What is in this image?"},` +
		`{"type":"image_url","image_url":{"url":"data:image/png;base64,` + base64Data + `"}}` +
		`]}]}`)

	// body ~3MB，但文本只有 "What is in this image?" (22 runes ≈ 8 tokens)
	tokens := estimateRequestTokens(body)
	assert.Less(t, tokens, int64(100), "base64 image data should not be counted as tokens")

	// 200k limit 下不应被拦截
	cfg := &config.Config{}
	cfg.Gateway.MaxRequestContentSize = 200000
	msg, exceeded := exceedsContentSizeLimit(cfg, body)
	assert.False(t, exceeded, "3MB image request should pass 200k token limit")
	assert.Empty(t, msg)
}

func TestExceedsContentSizeLimit_NilConfig(t *testing.T) {
	msg, exceeded := exceedsContentSizeLimit(nil, []byte(`{}`))
	assert.False(t, exceeded)
	assert.Empty(t, msg)
}

func TestExceedsContentSizeLimit_ZeroLimit(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.MaxRequestContentSize = 0
	msg, exceeded := exceedsContentSizeLimit(cfg, []byte(`{}`))
	assert.False(t, exceeded)
	assert.Empty(t, msg)
}

func TestExceedsContentSizeLimit_UnderLimit(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.MaxRequestContentSize = 200000
	body := []byte(`{"messages":[{"role":"user","content":"Hello"}]}`)
	msg, exceeded := exceedsContentSizeLimit(cfg, body)
	assert.False(t, exceeded)
	assert.Empty(t, msg)
}

func TestExceedsContentSizeLimit_OverLimit(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.MaxRequestContentSize = 100
	// 900 runes => ~300 tokens, well over 100
	largeText := strings.Repeat("a", 900)
	body := []byte(`{"messages":[{"role":"user","content":"` + largeText + `"}]}`)
	msg, exceeded := exceedsContentSizeLimit(cfg, body)
	assert.True(t, exceeded)
	assert.Contains(t, msg, "tokens")
}

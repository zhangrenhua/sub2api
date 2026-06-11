package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// upstreamStreamErrorEvent 表示上游在 HTTP 200 的 SSE 流中下发了 `event: error`
// (检测型中转/限流常见)。聚合路径在写客户端前就消费完整个流，因此遇到它时尚未写出
// 任何内容，可安全交由上层做 failover。
type upstreamStreamErrorEvent struct{ payload string }

func (e *upstreamStreamErrorEvent) Error() string {
	return "upstream stream error event: " + e.payload
}

// handleSimulatedNonStreamFromStream 读取上游 Anthropic SSE 流，聚合成一条完整的非流式
// Message，再复用 finishAnthropicNonStreamResponse 写回客户端。
//
// 用途:「模拟 Claude CLI」账号(anthropic API-key + 开关)在**客户端非流式**时,对上游改发
// stream:true(见 Forward 中的 simulateAggregate/upstreamStream),以绕过检测型上游中转的
// 形态白名单——它只接受「流式主循环」或「无 thinking/tools 的非流式 side-query」,
// 非流式带 thinking 会被判 unknown_messages_shape。这里把上游 SSE 聚合回非流式,对客户端透明。
//
// 健壮性(三种上游异常形态的兜底):
//  1. 上游忽略 stream:true 直接返回非流式 JSON Message → 直接当非流式响应透传;
//  2. 上游 SSE 冒号后无空格 / 仅 data 行 → 放宽解析(见 aggregateAnthropicStreamToResponse);
//  3. 既非有效 SSE 也非 JSON Message → 透传真实原因(含上游原文片段记日志),
//     不再一律兜底成无信息的「Upstream request failed」。
//
// 局限:聚合基于 apicompat 类型,cache_creation 的 5m/1h 明细与 thinking 块的 signature
// 不逐字段保留(前者退化为合计计费,后者对非 CLI 客户端无影响)。
func (s *GatewayService) handleSimulatedNonStreamFromStream(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, originalModel, mappedModel string) (*ClaudeUsage, error) {
	// 与 handleNonStreamingResponse 对齐:更新 5h 窗口状态。
	s.rateLimitService.UpdateSessionWindow(ctx, account, resp.Header)

	// 先读完整 body:既用于 SSE 聚合,也用于聚合失败时的 JSON 兜底与真因透传。
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil &&
		!errors.Is(readErr, context.Canceled) && !errors.Is(readErr, context.DeadlineExceeded) {
		return nil, s.writeSimulateAggregateFailure(c, account,
			fmt.Errorf("read upstream stream: %w", readErr), raw)
	}

	finalResp, err := s.aggregateAnthropicStreamToResponse(bytes.NewReader(raw))
	if err != nil {
		// (A) 上游 200 流内 error 事件:此刻还没向客户端写任何内容,做账号 failover。
		var streamErr *upstreamStreamErrorEvent
		if errors.As(err, &streamErr) {
			logger.L().Warn("simulate-cli aggregate: upstream emitted error event in 200 stream; failing over",
				zap.Int64("account_id", account.ID),
				zap.String("payload", truncateString(streamErr.payload, 500)),
			)
			return nil, &UpstreamFailoverError{
				StatusCode:   403,
				ResponseBody: []byte(streamErr.payload),
			}
		}

		// (B) 修复1:上游可能忽略 stream:true,直接返回非流式 JSON。按类型兜底处理。
		switch classifyAnthropicJSONBody(raw) {
		case "message":
			logger.L().Warn("simulate-cli aggregate: upstream returned non-stream JSON message; passing through as-is",
				zap.Int64("account_id", account.ID))
			resp.Header.Set("Content-Type", "application/json")
			return s.finishAnthropicNonStreamResponse(ctx, c, account, raw, resp.Header, http.StatusOK, originalModel, mappedModel)
		case "error":
			// 200 + JSON error body(检测型中转/限流的另一种形态)→ 同 stream error,做 failover。
			logger.L().Warn("simulate-cli aggregate: upstream returned 200 JSON error body; failing over",
				zap.Int64("account_id", account.ID),
				zap.String("payload", truncateString(strings.TrimSpace(string(raw)), 500)),
			)
			return nil, &UpstreamFailoverError{StatusCode: 403, ResponseBody: raw}
		}

		// (C) 修复3:既非有效 SSE 也非 JSON Message → 透传真实原因,不再兜底成 generic。
		return nil, s.writeSimulateAggregateFailure(c, account, err, raw)
	}

	body, err := json.Marshal(finalResp)
	if err != nil {
		return nil, fmt.Errorf("marshal aggregated message: %w", err)
	}

	// 聚合出的是 JSON,但 resp 是上游的 SSE 响应(Content-Type: text/event-stream)。
	// 覆盖为 application/json,避免 WriteFilteredHeaders 把 text/event-stream 透传到
	// 给客户端的非流式响应头里(gin 的 c.Data 不会覆盖已存在的 Content-Type)。
	resp.Header.Set("Content-Type", "application/json")

	// 聚合出的 JSON 与上游非流式响应同形,走同一条尾部逻辑(usage/cacheTTL/模型名/写回)。
	return s.finishAnthropicNonStreamResponse(ctx, c, account, body, resp.Header, http.StatusOK, originalModel, mappedModel)
}

// classifyAnthropicJSONBody 在 SSE 聚合失败时,判断上游 body 是否其实是一条完整的
// 非流式 JSON 响应:返回 "message"(完整 Message,可直接透传)、"error"(JSON 错误体,
// 应 failover)或 ""(不是可识别的 JSON,交由真因透传)。
func classifyAnthropicJSONBody(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return ""
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return ""
	}
	switch probe.Type {
	case "message":
		return "message"
	case "error":
		return "error"
	default:
		return ""
	}
}

// writeSimulateAggregateFailure 在聚合彻底失败(非 failover、非 JSON 兜底)时,向客户端写一条
// 携带真实原因的 502 错误,并把上游原文片段记入 Ops/日志供排障——取代以往无信息的
// 「Upstream request failed」兜底。返回 error 供上层日志记录;响应已写出,handler 不会重复写。
func (s *GatewayService) writeSimulateAggregateFailure(c *gin.Context, account *Account, cause error, raw []byte) error {
	reason := cause.Error()
	// 客户端/Ops 可见文案统一脱敏(read 错误可能裹 *net.OpError,默认 Error() 会泄露内部
	// IP/端口与上游地址);完整 reason 仅保留在下方低层 zap 日志中供运维诊断。
	safeReason := sanitizeUpstreamErrorMessage(reason)
	snippet := truncateString(strings.TrimSpace(string(raw)), 300)

	setOpsUpstreamError(c, http.StatusBadGateway, safeReason, "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: http.StatusBadGateway,
		Kind:               "request_error",
		Message:            safeReason,
		Detail:             snippet,
	})
	logger.L().With(zap.String("component", "service.gateway")).Warn(
		"simulate-cli aggregate failed",
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
		zap.String("reason", reason),
		zap.String("upstream_snippet", snippet),
	)

	if c != nil && c.Writer != nil && !c.Writer.Written() {
		c.JSON(http.StatusBadGateway, gin.H{
			"type": "error",
			"error": gin.H{
				"type": "upstream_error",
				// 真因透传:客户端能看到具体原因(如 upstream stream ended without a message),
				// 而非以往无信息的 "Upstream request failed"。
				"message": "Simulated Claude CLI: upstream did not return a valid Anthropic message stream (" + safeReason + ")",
			},
		})
	}
	return fmt.Errorf("simulate-cli aggregate failed: %s", safeReason)
}

// aggregateAnthropicStreamToResponse 解析 Anthropic SSE 事件流并累积成一条非流式响应。
// 与 gateway_forward_as_chat_completions.go 的缓冲聚合同构,但保留 Anthropic 原生结构
// (不再转 Responses/ChatCompletions),供原生 /v1/messages 非流式返回使用。
//
// 修复2:放宽 SSE 解析——逐行处理任意 `data:` 行(冒号后空格可选),不再要求 `event: ` 与
// `data: ` 严格成对且带空格。事件类型以 data 内 JSON 的 "type" 字段为准,因此兼容
// 「仅 data 行」「event:/data: 无空格」等中转常见变体。
//
// 若流中出现 type==error 的事件,返回 *upstreamStreamErrorEvent,由调用方决定 failover。
func (s *GatewayService) aggregateAnthropicStreamToResponse(r io.Reader) (*apicompat.AnthropicResponse, error) {
	scanner := bufio.NewScanner(r)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var finalResp *apicompat.AnthropicResponse
	var usage ClaudeUsage

	for scanner.Scan() {
		line := scanner.Text()
		payload, ok := sseDataPayload(line)
		if !ok {
			continue
		}
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var event apicompat.AnthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}

		switch event.Type {
		case "error":
			// 上游在 200 流里报错(检测型中转/限流)。带原始 payload 上抛,交由上层 failover。
			return nil, &upstreamStreamErrorEvent{payload: payload}
		case "message_start":
			// 初始响应骨架 + 命中缓存等初始 usage。
			if event.Message != nil {
				finalResp = event.Message
				mergeAnthropicUsage(&usage, event.Message.Usage)
			}
		case "content_block_start":
			if event.ContentBlock != nil && finalResp != nil {
				finalResp.Content = append(finalResp.Content, *event.ContentBlock)
			}
		case "content_block_delta":
			if event.Delta != nil && finalResp != nil && event.Index != nil {
				if idx := *event.Index; idx >= 0 && idx < len(finalResp.Content) {
					switch event.Delta.Type {
					case "text_delta":
						finalResp.Content[idx].Text += event.Delta.Text
					case "thinking_delta":
						finalResp.Content[idx].Thinking += event.Delta.Thinking
					case "input_json_delta":
						// tool_use 的完整 input 由 input_json_delta 增量拼出;而 content_block_start
						// 里携带的是占位的空对象 {}。首个 delta 到来时必须先丢弃该占位,否则会拼成
						// "{}{...}" 的非法 JSON,导致后续 marshal 报 invalid character '{' after top-level value。
						cur := finalResp.Content[idx].Input
						if isPlaceholderEmptyJSONObject(cur) {
							cur = nil
						}
						finalResp.Content[idx].Input = appendRawJSON(cur, event.Delta.PartialJSON)
					}
				}
			}
		case "message_delta":
			// 终态 usage(output_tokens)与 stop_reason。
			if event.Usage != nil {
				mergeAnthropicUsage(&usage, *event.Usage)
			}
			if event.Delta != nil && event.Delta.StopReason != "" && finalResp != nil {
				finalResp.StopReason = event.Delta.StopReason
			}
		}
	}

	if err := scanner.Err(); err != nil &&
		!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("read upstream stream: %w", err)
	}

	if finalResp == nil {
		return nil, fmt.Errorf("upstream stream ended without a message")
	}

	finalResp.Usage = apicompat.AnthropicUsage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
	}
	return finalResp, nil
}

// isPlaceholderEmptyJSONObject 判断 json.RawMessage 是否为 content_block_start 携带的占位空对象 `{}`。
// 仅用于在首个 input_json_delta 到来时丢弃占位;若 tool_use 无任何 delta,则该 `{}` 保留为合法空 input。
func isPlaceholderEmptyJSONObject(raw json.RawMessage) bool {
	return len(raw) > 0 && bytes.Equal(bytes.TrimSpace(raw), []byte("{}"))
}

// sseDataPayload 从一行中取出 SSE `data:` 字段值(冒号后空格可选);非 data 行返回 false。
func sseDataPayload(line string) (string, bool) {
	const prefix = "data:"
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	return strings.TrimSpace(line[len(prefix):]), true
}

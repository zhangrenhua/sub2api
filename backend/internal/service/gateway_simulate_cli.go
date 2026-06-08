package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
)

// handleSimulatedNonStreamFromStream 读取上游 Anthropic SSE 流，聚合成一条完整的非流式
// Message，再复用 finishAnthropicNonStreamResponse 写回客户端。
//
// 用途:「模拟 Claude CLI」账号(anthropic API-key + 开关)在**客户端非流式**时,对上游改发
// stream:true(见 Forward 中的 simulateAggregate/upstreamStream),以绕过检测型上游中转的
// 形态白名单——它只接受「流式主循环」或「无 thinking/tools 的非流式 side-query」,
// 非流式带 thinking 会被判 unknown_messages_shape。这里把上游 SSE 聚合回非流式,对客户端透明。
//
// 局限:聚合基于 apicompat 类型,cache_creation 的 5m/1h 明细与 thinking 块的 signature
// 不逐字段保留(前者退化为合计计费,后者对非 CLI 客户端无影响)。
func (s *GatewayService) handleSimulatedNonStreamFromStream(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, originalModel, mappedModel string) (*ClaudeUsage, error) {
	// 与 handleNonStreamingResponse 对齐:更新 5h 窗口状态。
	s.rateLimitService.UpdateSessionWindow(ctx, account, resp.Header)

	finalResp, err := s.aggregateAnthropicStreamToResponse(resp.Body)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(finalResp)
	if err != nil {
		return nil, fmt.Errorf("marshal aggregated message: %w", err)
	}

	// 聚合出的 JSON 与上游非流式响应同形,走同一条尾部逻辑(usage/cacheTTL/模型名/写回)。
	return s.finishAnthropicNonStreamResponse(ctx, c, account, body, resp.Header, http.StatusOK, originalModel, mappedModel)
}

// aggregateAnthropicStreamToResponse 解析 Anthropic SSE 事件流并累积成一条非流式响应。
// 与 gateway_forward_as_chat_completions.go 的缓冲聚合同构,但保留 Anthropic 原生结构
// (不再转 Responses/ChatCompletions),供原生 /v1/messages 非流式返回使用。
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
		if !strings.HasPrefix(line, "event: ") {
			continue
		}
		if !scanner.Scan() {
			break
		}
		dataLine := scanner.Text()
		if !strings.HasPrefix(dataLine, "data: ") {
			continue
		}

		var event apicompat.AnthropicStreamEvent
		if err := json.Unmarshal([]byte(dataLine[6:]), &event); err != nil {
			continue
		}

		switch event.Type {
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
						finalResp.Content[idx].Input = appendRawJSON(finalResp.Content[idx].Input, event.Delta.PartialJSON)
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

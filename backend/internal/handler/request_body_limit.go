package handler

import (
	"errors"
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
)

func extractMaxBytesError(err error) (*http.MaxBytesError, bool) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return maxErr, true
	}
	return nil, false
}

func formatBodyLimit(limit int64) string {
	const mb = 1024 * 1024
	if limit >= mb {
		return fmt.Sprintf("%dMB", limit/mb)
	}
	return fmt.Sprintf("%dB", limit)
}

func buildBodyTooLargeMessage(limit int64) string {
	return fmt.Sprintf("Request body too large, limit is %s", formatBodyLimit(limit))
}

func buildContentTooLargeMessage(estimatedTokens, limit int64) string {
	return fmt.Sprintf(
		"Request content too large (estimated %dk tokens, limit %dk tokens). Please compress the content or start a new conversation. 请求内容过大（预估%dk tokens，限制%dk tokens），请压缩内容或重建对话窗口。",
		estimatedTokens/1000, limit/1000, estimatedTokens/1000, limit/1000,
	)
}

// estimateRequestTokens 从请求体的文本内容中估算token数量。
// 支持 OpenAI、Anthropic 和 Gemini 请求格式。
// 使用启发式算法：约3个UTF-8字符对应1个token。
func estimateRequestTokens(body []byte) int64 {
	var totalRunes int64
	structuredFound := false // 是否识别到已知请求结构

	// OpenAI/Anthropic: messages[].content（字符串或内容数组）
	messages := gjson.GetBytes(body, "messages")
	if messages.Exists() && messages.IsArray() {
		structuredFound = true
		messages.ForEach(func(_, msg gjson.Result) bool {
			content := msg.Get("content")
			if content.Type == gjson.String {
				totalRunes += int64(utf8.RuneCountInString(content.Str))
			} else if content.IsArray() {
				content.ForEach(func(_, part gjson.Result) bool {
					text := part.Get("text")
					if text.Type == gjson.String {
						totalRunes += int64(utf8.RuneCountInString(text.Str))
					}
					return true
				})
			}
			return true
		})
	}

	// Anthropic: system（字符串或内容数组）
	system := gjson.GetBytes(body, "system")
	if system.Type == gjson.String {
		totalRunes += int64(utf8.RuneCountInString(system.Str))
	} else if system.IsArray() {
		system.ForEach(func(_, part gjson.Result) bool {
			text := part.Get("text")
			if text.Type == gjson.String {
				totalRunes += int64(utf8.RuneCountInString(text.Str))
			}
			return true
		})
	}

	// Gemini: contents[].parts[].text
	contents := gjson.GetBytes(body, "contents")
	if contents.Exists() && contents.IsArray() {
		structuredFound = true
		contents.ForEach(func(_, content gjson.Result) bool {
			parts := content.Get("parts")
			if parts.IsArray() {
				parts.ForEach(func(_, part gjson.Result) bool {
					text := part.Get("text")
					if text.Type == gjson.String {
						totalRunes += int64(utf8.RuneCountInString(text.Str))
					}
					return true
				})
			}
			return true
		})
	}

	// Gemini: systemInstruction.parts[].text
	sysInstr := gjson.GetBytes(body, "systemInstruction.parts")
	if sysInstr.IsArray() {
		sysInstr.ForEach(func(_, part gjson.Result) bool {
			text := part.Get("text")
			if text.Type == gjson.String {
				totalRunes += int64(utf8.RuneCountInString(text.Str))
			}
			return true
		})
	}

	// OpenAI Responses API: input（字符串或消息对象数组）
	input := gjson.GetBytes(body, "input")
	if input.Exists() {
		structuredFound = true
	}
	if input.Type == gjson.String {
		totalRunes += int64(utf8.RuneCountInString(input.Str))
	} else if input.IsArray() {
		input.ForEach(func(_, item gjson.Result) bool {
			content := item.Get("content")
			if content.Type == gjson.String {
				totalRunes += int64(utf8.RuneCountInString(content.Str))
			} else if content.IsArray() {
				content.ForEach(func(_, part gjson.Result) bool {
					text := part.Get("text")
					if text.Type == gjson.String {
						totalRunes += int64(utf8.RuneCountInString(text.Str))
					}
					return true
				})
			}
			return true
		})
	}

	// OpenAI Responses API: instructions
	instructions := gjson.GetBytes(body, "instructions")
	if instructions.Type == gjson.String {
		totalRunes += int64(utf8.RuneCountInString(instructions.Str))
	}

	// 兜底：仅当完全未识别到已知请求结构时，用整个body估算。
	// 如果识别到了结构（如messages/contents/input）但文本为空（如纯图片），不触发兜底。
	if totalRunes == 0 && !structuredFound {
		totalRunes = int64(utf8.RuneCount(body))
	}

	// 启发式估算：约3个rune对应1个token（适用于中英混合内容）
	return (totalRunes + 2) / 3
}

// exceedsContentSizeLimit 检查请求的预估token数是否超过配置的token限制。
// 超限时返回提示消息和 true，未超限或未配置时返回空字符串和 false。
func exceedsContentSizeLimit(cfg *config.Config, body []byte) (string, bool) {
	if cfg == nil {
		return "", false
	}
	limit := cfg.Gateway.MaxRequestContentSize
	if limit <= 0 {
		return "", false
	}
	estimated := estimateRequestTokens(body)
	if estimated > limit {
		return buildContentTooLargeMessage(estimated, limit), true
	}
	return "", false
}

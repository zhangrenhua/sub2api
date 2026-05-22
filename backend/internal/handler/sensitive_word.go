package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sensitiveword"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// sensitiveWordRejectionMessage 命中敏感词时返回给客户端的统一提示语前缀。
const sensitiveWordRejectionMessage = "请求内容不合规"

// sensitiveWordRejection 拼接命中词到统一提示语后返回。命中词为空时退化为前缀。
func sensitiveWordRejection(word string) string {
	if word == "" {
		return sensitiveWordRejectionMessage
	}
	return sensitiveWordRejectionMessage + "：" + word
}

// logSensitiveWordHit 把命中事件写到独立日志文件，作为拦截前的审计记录。
// logger 未配置（matcher 未启用或路径未配置）时直接返回，零开销。
func logSensitiveWordHit(c *gin.Context, cfg *config.Config, word string) {
	if cfg == nil || cfg.Gateway.SensitiveWordLogger == nil {
		return
	}
	var (
		userID   int64
		apiKeyID int64
		groupID  int64
	)
	if apiKey, ok := middleware.GetAPIKeyFromContext(c); ok && apiKey != nil {
		apiKeyID = apiKey.ID
		if apiKey.GroupID != nil {
			groupID = *apiKey.GroupID
		}
	}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		userID = subject.UserID
	}
	cfg.Gateway.SensitiveWordLogger.Info("sensitive_word_hit",
		zap.String("word", word),
		zap.Int64("user_id", userID),
		zap.Int64("api_key_id", apiKeyID),
		zap.Int64("group_id", groupID),
	)
}

// containsSensitiveWord 扫描请求体中的用户可见文本字段（兼容 Anthropic /
// OpenAI Chat/Responses / Gemini 四种协议），命中任一配置的敏感词即返回该词。
// 仅扫描文本字段，跳过 base64 图片、工具调用 arguments 的 JSON 片段等噪声区，
// 避免误伤。matcher 未配置时直接视为不命中，保持零开销。
func containsSensitiveWord(cfg *config.Config, body []byte) (string, bool) {
	if cfg == nil || cfg.Gateway.SensitiveWordMatcher == nil {
		return "", false
	}
	if len(body) == 0 {
		return "", false
	}
	matcher := cfg.Gateway.SensitiveWordMatcher
	var hit string
	walkRequestTexts(body, func(text string) bool {
		if text == "" {
			return true
		}
		if w, ok := matcher.FirstMatch(sensitiveword.Normalize(text)); ok {
			hit = w
			return false
		}
		return true
	})
	return hit, hit != ""
}

// walkRequestTexts 按协议结构遍历请求体中的用户可见文本字段，对每个文本调用 fn。
// fn 返回 false 时中断遍历，用于命中即返回的场景。
//
// 覆盖字段：
//   - OpenAI / Anthropic:       messages[].content (string | array[text])
//   - Anthropic:                system            (string | array[text])
//   - Gemini:                   contents[].parts[].text, systemInstruction.parts[].text
//   - OpenAI Responses API:     input (string | array[.content=string|array[text]]), instructions
func walkRequestTexts(body []byte, fn func(text string) bool) {
	cont := true
	emit := func(v gjson.Result) bool {
		if !cont {
			return false
		}
		if v.Type == gjson.String {
			if !fn(v.Str) {
				cont = false
				return false
			}
		}
		return true
	}

	if msgs := gjson.GetBytes(body, "messages"); msgs.IsArray() {
		msgs.ForEach(func(_, msg gjson.Result) bool {
			content := msg.Get("content")
			if content.Type == gjson.String {
				return emit(content)
			}
			if content.IsArray() {
				content.ForEach(func(_, part gjson.Result) bool {
					return emit(part.Get("text"))
				})
			}
			return cont
		})
	}

	if system := gjson.GetBytes(body, "system"); cont {
		if system.Type == gjson.String {
			emit(system)
		} else if system.IsArray() {
			system.ForEach(func(_, part gjson.Result) bool {
				return emit(part.Get("text"))
			})
		}
	}

	if contents := gjson.GetBytes(body, "contents"); cont && contents.IsArray() {
		contents.ForEach(func(_, c gjson.Result) bool {
			parts := c.Get("parts")
			if parts.IsArray() {
				parts.ForEach(func(_, part gjson.Result) bool {
					return emit(part.Get("text"))
				})
			}
			return cont
		})
	}

	if sysInstr := gjson.GetBytes(body, "systemInstruction.parts"); cont && sysInstr.IsArray() {
		sysInstr.ForEach(func(_, part gjson.Result) bool {
			return emit(part.Get("text"))
		})
	}

	if input := gjson.GetBytes(body, "input"); cont && input.Exists() {
		if input.Type == gjson.String {
			emit(input)
		} else if input.IsArray() {
			input.ForEach(func(_, item gjson.Result) bool {
				content := item.Get("content")
				if content.Type == gjson.String {
					return emit(content)
				}
				if content.IsArray() {
					content.ForEach(func(_, part gjson.Result) bool {
						return emit(part.Get("text"))
					})
				}
				return cont
			})
		}
	}

	if instructions := gjson.GetBytes(body, "instructions"); cont && instructions.Type == gjson.String {
		emit(instructions)
	}
}

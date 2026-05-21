package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sensitiveword"
)

func testCfgWithWords(words []string) *config.Config {
	cfg := &config.Config{}
	cfg.Gateway.SensitiveWordMatcher = sensitiveword.NewMatcher(words)
	return cfg
}

func TestContainsSensitiveWord_NilMatcher(t *testing.T) {
	cfg := &config.Config{}
	body := []byte(`{"messages":[{"role":"user","content":"奶子"}]}`)
	if w, hit := containsSensitiveWord(cfg, body); hit {
		t.Fatalf("expected no hit when matcher is nil, got %q", w)
	}
}

func TestContainsSensitiveWord_AnthropicMessages(t *testing.T) {
	cfg := testCfgWithWords([]string{"奶子", "CSAM"})
	body := []byte(`{"messages":[{"role":"user","content":"前面奶子后面"}]}`)
	w, hit := containsSensitiveWord(cfg, body)
	if !hit || w != "奶子" {
		t.Fatalf("want 奶子, got (%q, %v)", w, hit)
	}
}

func TestContainsSensitiveWord_AnthropicContentArray(t *testing.T) {
	cfg := testCfgWithWords([]string{"CSAM"})
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"discuss CSAM"}]}]}`)
	w, hit := containsSensitiveWord(cfg, body)
	if !hit || w != "csam" {
		t.Fatalf("want csam, got (%q, %v)", w, hit)
	}
}

func TestContainsSensitiveWord_AnthropicSystemArray(t *testing.T) {
	cfg := testCfgWithWords([]string{"SillyTavern"})
	body := []byte(`{"system":[{"type":"text","text":"You are SillyTavern"}],"messages":[]}`)
	w, hit := containsSensitiveWord(cfg, body)
	if !hit || w != "sillytavern" {
		t.Fatalf("want sillytavern, got (%q, %v)", w, hit)
	}
}

func TestContainsSensitiveWord_GeminiContents(t *testing.T) {
	cfg := testCfgWithWords([]string{"炸弹制作"})
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"请教炸弹制作流程"}]}]}`)
	w, hit := containsSensitiveWord(cfg, body)
	if !hit || w != "炸弹制作" {
		t.Fatalf("want 炸弹制作, got (%q, %v)", w, hit)
	}
}

func TestContainsSensitiveWord_GeminiSystemInstruction(t *testing.T) {
	cfg := testCfgWithWords([]string{"轮奸"})
	body := []byte(`{"systemInstruction":{"parts":[{"text":"含有轮奸内容"}]},"contents":[]}`)
	w, hit := containsSensitiveWord(cfg, body)
	if !hit || w != "轮奸" {
		t.Fatalf("want 轮奸, got (%q, %v)", w, hit)
	}
}

func TestContainsSensitiveWord_OpenAIResponsesInputString(t *testing.T) {
	cfg := testCfgWithWords([]string{"child porn"})
	body := []byte(`{"input":"text about child porn here"}`)
	w, hit := containsSensitiveWord(cfg, body)
	if !hit || w != "child porn" {
		t.Fatalf("want child porn, got (%q, %v)", w, hit)
	}
}

func TestContainsSensitiveWord_OpenAIResponsesInputArray(t *testing.T) {
	cfg := testCfgWithWords([]string{"萝莉控"})
	body := []byte(`{"input":[{"role":"user","content":[{"type":"input_text","text":"萝莉控"}]}]}`)
	w, hit := containsSensitiveWord(cfg, body)
	if !hit || w != "萝莉控" {
		t.Fatalf("want 萝莉控, got (%q, %v)", w, hit)
	}
}

func TestContainsSensitiveWord_OpenAIInstructions(t *testing.T) {
	cfg := testCfgWithWords([]string{"恋童"})
	body := []byte(`{"instructions":"规则：恋童等不可讨论","input":"ok"}`)
	w, hit := containsSensitiveWord(cfg, body)
	if !hit || w != "恋童" {
		t.Fatalf("want 恋童, got (%q, %v)", w, hit)
	}
}

func TestContainsSensitiveWord_Benign(t *testing.T) {
	cfg := testCfgWithWords([]string{"奶子", "CSAM", "SillyTavern"})
	body := []byte(`{"messages":[{"role":"user","content":"今天天气真好，请帮我写一段 Go 代码"}]}`)
	if w, hit := containsSensitiveWord(cfg, body); hit {
		t.Fatalf("unexpected hit on benign body: %q", w)
	}
}

func TestContainsSensitiveWord_IgnoresNonTextFields(t *testing.T) {
	// 敏感词仅作为图片 base64 的内容存在于 image_url 字段，不应触发（因为未在扫描字段表内）。
	cfg := testCfgWithWords([]string{"CSAM"})
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"image","source":{"data":"CSAMbase64"}}]}]}`)
	if w, hit := containsSensitiveWord(cfg, body); hit {
		t.Fatalf("should not scan non-text fields, got hit %q", w)
	}
}

func TestContainsSensitiveWord_ShortCircuitOnFirstHit(t *testing.T) {
	cfg := testCfgWithWords([]string{"奶子"})
	// 多条消息，命中第一条应立刻返回，第二条哪怕 payload 异常也不影响。
	body := []byte(`{"messages":[{"role":"user","content":"奶子"},{"role":"user","content":"normal"}]}`)
	w, hit := containsSensitiveWord(cfg, body)
	if !hit || w != "奶子" {
		t.Fatalf("want 奶子, got (%q, %v)", w, hit)
	}
}

func TestSensitiveWordRejectionMessage(t *testing.T) {
	if sensitiveWordRejectionMessage != "请求内容不合规" {
		t.Fatalf("rejection message changed unexpectedly: %q", sensitiveWordRejectionMessage)
	}
}

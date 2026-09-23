package handler

import (
	"errors"
	"testing"
)

func TestCountClaudeTokens_Basic(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet",
		"messages": [{"role": "user", "content": "count this prompt please"}],
		"tools": [{"name": "lookup", "description": "Look up a value", "input_schema": {"type": "object", "properties": {"query": {"type": "string"}}}}]
	}`)
	tokens, err := CountClaudeTokens(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens <= 0 {
		t.Fatalf("expected positive input_tokens, got %d", tokens)
	}
}

func TestCountClaudeTokens_MissingMessages(t *testing.T) {
	body := []byte(`{"model": "claude-3-5-sonnet"}`)
	_, err := CountClaudeTokens(body)
	if err == nil {
		t.Fatal("expected error for missing messages")
	}
	if !errors.Is(err, errCountTokensInvalid) {
		t.Fatalf("expected errCountTokensInvalid, got %v", err)
	}
	if err.Error() != "messages is required" {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestCountClaudeTokens_MissingModel(t *testing.T) {
	body := []byte(`{"messages": [{"role": "user", "content": "hi"}]}`)
	_, err := CountClaudeTokens(body)
	if err == nil || err.Error() != "model is required" {
		t.Fatalf("expected model required error, got %v", err)
	}
}

func TestCountClaudeTokens_InvalidBody(t *testing.T) {
	_, err := CountClaudeTokens([]byte(`not json`))
	if err == nil || !errors.Is(err, errCountTokensInvalid) {
		t.Fatalf("expected invalid request error, got %v", err)
	}
}

func TestCountClaudeTokens_ContentBlocks(t *testing.T) {
	// 内容块数组（含 text + image）应比同长纯文本计入更多 token（图片固定估值）
	body := []byte(`{
		"model": "claude-3-5-sonnet",
		"messages": [{"role": "user", "content": [
			{"type": "text", "text": "describe this"},
			{"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "aGVsbG8="}}
		]}]
	}`)
	tokens, err := CountClaudeTokens(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens < claudeImageTokens {
		t.Fatalf("expected image tokens to dominate, got %d", tokens)
	}
}

func TestCountClaudeTokens_SystemAndCJK(t *testing.T) {
	// system 文本应计入；CJK 文本 token 密度高于等长拉丁文本
	latin := []byte(`{"model":"m","system":"aaaaaaaaaa","messages":[{"role":"user","content":"x"}]}`)
	cjk := []byte(`{"model":"m","system":"你好世界你好世界你好","messages":[{"role":"user","content":"x"}]}`)
	lt, err := CountClaudeTokens(latin)
	if err != nil {
		t.Fatalf("latin err: %v", err)
	}
	ct, err := CountClaudeTokens(cjk)
	if err != nil {
		t.Fatalf("cjk err: %v", err)
	}
	if ct <= lt {
		t.Fatalf("expected CJK (%d) to exceed latin (%d) for equal char count", ct, lt)
	}
}

func TestEstimateTextTokens_Empty(t *testing.T) {
	if got := estimateTextTokens(""); got != 0 {
		t.Fatalf("expected 0 for empty string, got %d", got)
	}
}

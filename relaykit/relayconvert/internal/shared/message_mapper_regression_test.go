package shared

// 回归测试：content 归一化与 Claude 图片 source 字段（审查发现，修复后固化）。

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

// TestNormalizeContentParts_FromJSONAny 回归：JSON 直解产出的 []any（元素 map）此前
// 无法被消费方识别（对 map 断言 .(dto.ContentPart) 永假），多模态内容静默丢失。
func TestNormalizeContentParts_FromJSONAny(t *testing.T) {
	var content any
	if err := json.Unmarshal([]byte(`[
		{"type":"text","text":"hi"},
		{"type":"image_url","image_url":{"url":"data:image/png;base64,aGk="}}
	]`), &content); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	parts := NormalizeContentParts(content)
	if len(parts) != 2 {
		t.Fatalf("parts = %#v, want 2（[]any 形态未归一）", parts)
	}
	if parts[0].Type != "text" || parts[0].Text != "hi" {
		t.Errorf("parts[0] = %#v, want text part \"hi\"", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,aGk=" {
		t.Errorf("parts[1] = %#v, want image_url part", parts[1])
	}
}

// TestMapOpenAIImageToClaudeSource_URL 回归：远程 URL 图片的 source 此前写成
// {"type":"url","data":"https://..."}——Anthropic 要求 url 字段，填 data 上游直接 400。
func TestMapOpenAIImageToClaudeSource_URL(t *testing.T) {
	s := MapOpenAIImageToClaudeSource(dto.ImageURL{URL: "https://example.com/pic.png"})
	if s.Type != "url" {
		t.Errorf("type = %q, want url", s.Type)
	}
	if s.URL != "https://example.com/pic.png" {
		t.Errorf("url = %q, want https://example.com/pic.png", s.URL)
	}
	if s.Data != "" {
		t.Errorf("data = %q, want 空（URL 形态不得填 data 字段）", s.Data)
	}
}

// TestMapOpenAIImageToClaudeSource_DataURL 数据 URL 仍走 base64 解析路径。
func TestMapOpenAIImageToClaudeSource_DataURL(t *testing.T) {
	s := MapOpenAIImageToClaudeSource(dto.ImageURL{URL: "data:image/png;base64,aGk="})
	if s.Type != "base64" || s.MediaType != "image/png" || s.Data != "aGk=" {
		t.Errorf("source = %#v, want base64 image/png aGk=", s)
	}
}

package helper

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// provider 映射的权威单测集中在本包。
// 迁移前 handler/passthrough_test.go 与 relaykit_bridge/response_test.go 各有一份重复表测，
// 现逻辑已合并到 helper.ProviderNativeFormat，重复副本随之删除。

func TestProviderNativeFormat(t *testing.T) {
	tests := []struct {
		name         string
		providerType int
		want         constant.RelayFormat
	}{
		{"claude", int(constant.ProviderClaude), constant.RelayFormatClaude},
		{"gemini", int(constant.ProviderGemini), constant.RelayFormatGemini},
		{"ollama", int(constant.ProviderOllama), constant.RelayFormatOllama},
		{"openai", int(constant.ProviderOpenAI), constant.RelayFormatOpenAI},
		// OpenAI 兼容供应商无原生格式，默认走 OpenAI
		{"deepseek", int(constant.ProviderDeepSeek), constant.RelayFormatOpenAI},
		{"azure", int(constant.ProviderAzure), constant.RelayFormatOpenAI},
		{"aws", int(constant.ProviderAWS), constant.RelayFormatOpenAI},
		{"vertex", int(constant.ProviderVertex), constant.RelayFormatOpenAI},
		{"ali", int(constant.ProviderAli), constant.RelayFormatOpenAI},
		{"unknown_default", 999999, constant.RelayFormatOpenAI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProviderNativeFormat(tt.providerType); got != tt.want {
				t.Errorf("ProviderNativeFormat(%d) = %s, want %s", tt.providerType, got, tt.want)
			}
		})
	}
}

// TestProviderNativeFormat_VertexIsModelDriven 钉住「按渠道类型判 vertex 会得出 openai」
// 这一事实本身——它正是本次 bug 的根源。vertex 的正确判定必须走带模型名的入口。
func TestProviderNativeFormat_VertexIsModelDriven(t *testing.T) {
	if got := ProviderNativeFormat(int(constant.ProviderVertex)); got != constant.RelayFormatOpenAI {
		t.Fatalf("前提变化：ProviderNativeFormat(vertex) = %s；若已改为模型无关的固定格式，请同步修订 ProviderNativeFormatForModel", got)
	}
	// 同一渠道类型、不同模型 → 不同上游协议
	if got := ProviderNativeFormatForModel(int(constant.ProviderVertex), "gemini-2.5-pro"); got != constant.RelayFormatGemini {
		t.Errorf("vertex + gemini 模型 = %s, want %s", got, constant.RelayFormatGemini)
	}
	if got := ProviderNativeFormatForModel(int(constant.ProviderVertex), "claude-sonnet-4-5"); got != constant.RelayFormatClaude {
		t.Errorf("vertex + claude 模型 = %s, want %s", got, constant.RelayFormatClaude)
	}
}

func TestVertexModelFormat(t *testing.T) {
	tests := []struct {
		model string
		want  constant.RelayFormat
	}{
		{"claude-sonnet-4-5", constant.RelayFormatClaude},
		{"claude-3-7-sonnet@20250219", constant.RelayFormatClaude},
		{"CLAUDE-OPUS-4", constant.RelayFormatClaude}, // 大小写不敏感
		{"gemini-2.5-pro", constant.RelayFormatGemini},
		{"gemini-2.0-flash", constant.RelayFormatGemini},
		{"text-embedding-004", constant.RelayFormatGemini}, // 非 claude 一律按 Gemini 端点
		{"", constant.RelayFormatGemini},                   // 空模型名兜底 Gemini（与 adaptor 选端点口径一致）
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := VertexModelFormat(tt.model); got != tt.want {
				t.Errorf("VertexModelFormat(%q) = %s, want %s", tt.model, got, tt.want)
			}
		})
	}
}

// TestProviderNativeFormatForModel_NonVertexIgnoresModel 非 vertex 供应商的协议与模型无关：
// 模型名里含 "claude" 不得影响判定（否则 OpenAI 渠道上的 claude 兼容模型会被判成 Claude 协议）。
func TestProviderNativeFormatForModel_NonVertexIgnoresModel(t *testing.T) {
	for _, p := range []constant.ProviderType{
		constant.ProviderOpenAI, constant.ProviderDeepSeek, constant.ProviderAli,
	} {
		if got := ProviderNativeFormatForModel(int(p), "claude-sonnet-4-5"); got != constant.RelayFormatOpenAI {
			t.Errorf("provider %s + claude 模型 = %s, want %s（非 vertex 不应看模型名）", p, got, constant.RelayFormatOpenAI)
		}
	}
	if got := ProviderNativeFormatForModel(int(constant.ProviderGemini), "claude-sonnet-4-5"); got != constant.RelayFormatGemini {
		t.Errorf("gemini 渠道 = %s, want %s", got, constant.RelayFormatGemini)
	}
}

func TestProviderNativeFormatFor(t *testing.T) {
	mk := func(provider constant.ProviderType, upstreamModel, originModel string) *common.RelayInfo {
		return &common.RelayInfo{
			OriginModelName: originModel,
			ChannelMeta: &common.ChannelMeta{
				ChannelType:       int(provider),
				UpstreamModelName: upstreamModel,
			},
		}
	}

	// 取 UpstreamModelName（与 adaptor 构造 URL 所用字段一致）
	if got := ProviderNativeFormatFor(mk(constant.ProviderVertex, "claude-sonnet-4-5", "gemini-pro")); got != constant.RelayFormatClaude {
		t.Errorf("应按 UpstreamModelName 判定, got %s", got)
	}
	// UpstreamModelName 为空时回落客户端模型名
	if got := ProviderNativeFormatFor(mk(constant.ProviderVertex, "", "claude-sonnet-4-5")); got != constant.RelayFormatClaude {
		t.Errorf("UpstreamModelName 为空应回落 OriginModelName, got %s", got)
	}
	// nil 守卫
	if got := ProviderNativeFormatFor(nil); got != constant.RelayFormatOpenAI {
		t.Errorf("nil info = %s, want %s", got, constant.RelayFormatOpenAI)
	}
	if got := ProviderNativeFormatFor(&common.RelayInfo{}); got != constant.RelayFormatOpenAI {
		t.Errorf("nil ChannelMeta = %s, want %s", got, constant.RelayFormatOpenAI)
	}
}

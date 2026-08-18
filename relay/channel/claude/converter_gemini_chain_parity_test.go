package claude

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// blank import 触发内置转换器注册（与生产桥接路径一致）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// TestGeminiToClaudeChain_Parity 对拍 legacy 手工链（ConvertGeminiToClaude =
// ConvertGeminiToOpenAI → ConvertOpenAIToClaude）与 relaykit StepConverters 链。
//
// 第二跳两侧实现不同（legacy ConvertOpenAIToClaude vs oai_chat 转换器），已知差异与
// P0 responses→claude 链同款（converter_chain_parity_test.go 先例）：
//  1. thinking 注入：legacy injectClaudeThinking budget=80%×max_tokens；relaykit o2c
//     ThinkingAdapterBudgetTokensPercentage=0.5 且 temperature 处理不同
//  2. max_tokens 缺省：legacy 第二跳固定 4096；relaykit 走 DefaultMaxTokens hook
//  3. content 形态：legacy 纯文本输出字符串；relaykit 输出 [{"type":"text"}] 块数组
//  4. 消息结构：relaykit 把 tool 消息包装为 user 消息内 tool_result 块（Claude 协议正确形态）
func TestGeminiToClaudeChain_Parity(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "basic-with-max-tokens",
			body: `{"contents":[{"role":"user","parts":[{"text":"你好"}]}],
			"generationConfig":{"maxOutputTokens":1024}}`,
		},
		{
			name: "tools-history",
			body: `{"contents":[
				{"role":"user","parts":[{"text":"跑测试"}]},
				{"role":"model","parts":[{"functionCall":{"name":"run","args":{"file":"a.go"}}}]},
				{"role":"user","parts":[{"functionResponse":{"name":"run","response":{"ok":true}}}]},
				{"role":"user","parts":[{"text":"提交"}]}
			],"generationConfig":{"maxOutputTokens":512,"thinkingConfig":{"thoughtBudget":3000}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			legacyInfo := &common.RelayInfo{
				InboundFormat: constant.RelayFormatGemini,
				ChannelMeta: &common.ChannelMeta{
					ChannelType:       int(constant.ProviderClaude),
					IsModelMapped:     false,
					UpstreamModelName: "claude-sonnet-4-20250514",
				},
			}
			legacyReader, err := ConvertGeminiToClaude([]byte(tt.body), legacyInfo)
			require.NoError(t, err, "legacy chain")
			legacyBody, err := io.ReadAll(legacyReader)
			require.NoError(t, err)

			var geminiReq dto.GeminiChatRequest
			require.NoError(t, json.Unmarshal([]byte(tt.body), &geminiReq))
			kitInfo := &convmeta.Values{
				ChannelMetaAttached: true,
				OriginModelName:     "claude-sonnet-4",
				UpstreamModelName:   "claude-sonnet-4-20250514",
				Options: &convmeta.Options{
					Claude: convmeta.ClaudeOptions{DefaultMaxTokens: func(string) int { return 8192 }},
				},
			}
			spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterGeminiContentToClaudeMessages)
			require.True(t, ok, "expected chain spec registered")
			converted, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, kitInfo, &geminiReq)
			require.NoError(t, err, "relaykit chain")
			kitBody, err := json.Marshal(converted)
			require.NoError(t, err)

			var legacyReq, kitReq dto.ClaudeRequest
			require.NoError(t, json.Unmarshal(legacyBody, &legacyReq))
			require.NoError(t, json.Unmarshal(kitBody, &kitReq))

			// 顶层：模型一致；system 文本一致（形态归一化提取）
			assert.Equal(t, legacyReq.Model, kitReq.Model)
			assert.Equal(t, normalizeSystemText(legacyReq.System), normalizeSystemText(kitReq.System))
			if legacyReq.MaxTokens != nil && tt.name == "basic-with-max-tokens" {
				require.NotNil(t, kitReq.MaxTokens)
				assert.EqualValues(t, *legacyReq.MaxTokens, *kitReq.MaxTokens, "显式 max_output_tokens 应透传")
			}
			// tools / tool_choice 深度相等
			legacyToolsJSON, _ := json.Marshal(legacyReq.Tools)
			kitToolsJSON, _ := json.Marshal(kitReq.Tools)
			if string(legacyToolsJSON) != "null" {
				assert.JSONEq(t, string(legacyToolsJSON), string(kitToolsJSON), "tools 应深度相等")
			}

			// 消息序列语义对齐（角色序列 + 工具链路无损）：
			// legacy：user/assistant(tool_use)/tool/user；relaykit：user/assistant/user(tool_result)/user
			legacyRoles := roleSequence(legacyReq.Messages)
			legacyHasTool := strings.Contains(strings.Join(legacyRoles, ","), "tool")
			kitHasToolResult := containsToolResultBlock(kitReq.Messages)
			if legacyHasTool {
				assert.True(t, kitHasToolResult, "relaykit 链应保留工具结果（tool_result 块形态）")
			}
			// 文本内容无损：所有 user/assistant 文本在两侧均出现
			legacyTexts, kitTexts := collectTexts(legacyReq.Messages), collectTexts(kitReq.Messages)
			for _, text := range legacyTexts {
				if text != "" {
					assert.Contains(t, kitTexts, text, "文本 %q 应在 relaykit 侧保留", text)
				}
			}
		})
	}
}

func roleSequence(msgs []dto.ClaudeMessage) []string {
	roles := make([]string, 0, len(msgs))
	for _, m := range msgs {
		roles = append(roles, m.Role)
	}
	return roles
}

func containsToolResultBlock(msgs []dto.ClaudeMessage) bool {
	for _, m := range msgs {
		b, _ := json.Marshal(m.Content)
		if strings.Contains(string(b), "tool_result") {
			return true
		}
	}
	return false
}

func collectTexts(msgs []dto.ClaudeMessage) []string {
	var texts []string
	for _, m := range msgs {
		if s, ok := m.Content.(string); ok {
			texts = append(texts, s)
			continue
		}
		b, _ := json.Marshal(m.Content)
		var blocks []map[string]any
		if json.Unmarshal(b, &blocks) == nil {
			for _, blk := range blocks {
				if t, _ := blk["text"].(string); t != "" {
					texts = append(texts, t)
				}
			}
		}
	}
	return texts
}

func normalizeSystemText(system any) string {
	if system == nil {
		return ""
	}
	if s, ok := system.(string); ok {
		return s
	}
	b, _ := json.Marshal(system)
	var blocks []map[string]any
	if json.Unmarshal(b, &blocks) == nil {
		var parts []string
		for _, blk := range blocks {
			if t, _ := blk["text"].(string); t != "" {
				parts = append(parts, t)
			}
		}
		return strings.Join(parts, "\n")
	}
	return string(b)
}

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

// TestResponsesToClaudeChain_Parity 对拍 legacy 手工链（ConvertResponsesToClaude =
// ConvertResponsesToOpenAI → ConvertOpenAIToClaude）与 relaykit StepConverters 链。
//
// 第二跳两侧实现不同（legacy ConvertOpenAIToClaude vs oai_chat 转换器，后者为线上
// 常开的 OpenAI→Claude 主路径），存在三类已知形态差异（语义等价或 relaykit 更优）：
//  1. 纯文本消息 content：legacy 输出字符串，relaykit 输出 [{"type":"text"}] 块数组
//     （Claude API 两种形态均合法）
//  2. max_tokens 缺省：legacy 固定 4096，relaykit 取宿主 DefaultMaxTokens hook（模型相关，更正确）
//  3. content=nil 的 assistant 消息：legacy 产出字面 "<nil>" 文本块（既有 bug），
//     relaykit 产出空文本块
//
// 本测试做语义归一化比较（文本提取 + "<nil>" 剔除 + 缺省 max_tokens 剔除），
// 深层结构（tool_use/tool_result/tools/tool_choice）要求严格相等。
func TestResponsesToClaudeChain_Parity(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "basic-with-max-tokens",
			body: `{
				"model": "claude-sonnet-4",
				"instructions": "你是编码助手",
				"input": [{"type": "message", "role": "user", "content": "写一个加法函数"}],
				"max_output_tokens": 1024
			}`,
		},
		{
			name: "no-max-tokens-uses-default",
			body: `{
				"model": "claude-sonnet-4",
				"input": "你好"
			}`,
		},
		{
			name: "tools-and-history",
			body: `{
				"model": "claude-sonnet-4",
				"instructions": "仅中文",
				"input": [
					{"type": "message", "role": "user", "content": "跑测试"},
					{"type": "function_call", "call_id": "call_1", "name": "run", "arguments": "{\"file\":\"a.go\"}"},
					{"type": "function_call_output", "call_id": "call_1", "output": "PASS"}
				],
				"max_output_tokens": 512,
				"temperature": 0.3,
				"tools": [{"type": "function", "name": "run", "description": "执行", "parameters": {"type": "object", "properties": {"file": {"type": "string"}}}}],
				"tool_choice": {"type": "function", "function": {"name": "run"}}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			legacyInfo := &common.RelayInfo{
				ChannelMeta: &common.ChannelMeta{
					ChannelType:       int(constant.ProviderClaude),
					IsModelMapped:     false,
					UpstreamModelName: "claude-sonnet-4-20250514",
				},
			}
			legacyReader, err := ConvertResponsesToClaude([]byte(tt.body), legacyInfo)
			require.NoError(t, err, "legacy chain")
			legacyBody, err := io.ReadAll(legacyReader)
			require.NoError(t, err)

			var responsesReq dto.OpenAIResponsesRequest
			require.NoError(t, json.Unmarshal([]byte(tt.body), &responsesReq))

			kitInfo := &convmeta.Values{
				ChannelMetaAttached: true,
				OriginModelName:     "claude-sonnet-4",
				UpstreamModelName:   "claude-sonnet-4-20250514",
				Options: &convmeta.Options{
					Claude: convmeta.ClaudeOptions{DefaultMaxTokens: func(string) int { return 8192 }},
				},
			}
			spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterOpenAIResponsesToClaudeMessages)
			require.True(t, ok, "expected chain converter registered")
			converted, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, kitInfo, &responsesReq)
			require.NoError(t, err, "relaykit chain")
			kitBody, err := json.Marshal(converted)
			require.NoError(t, err)

			var legacyReq, kitReq dto.ClaudeRequest
			require.NoError(t, json.Unmarshal(legacyBody, &legacyReq))
			require.NoError(t, json.Unmarshal(kitBody, &kitReq))

			// 顶层字段：模型一致；system 文本一致（形态可能是 string 或块数组，归一化提取）
			assert.Equal(t, legacyReq.Model, kitReq.Model)
			assert.Equal(t, normalizeClaudeSystem(t, legacyReq.System), normalizeClaudeSystem(t, kitReq.System))
			// max_tokens：显式设置时必须相等；缺省时两侧来源不同（4096 vs DefaultMaxTokens），仅断言非零
			if legacyReq.MaxTokens != nil {
				if *legacyReq.MaxTokens == 4096 && tt.name == "no-max-tokens-uses-default" {
					assert.NotNil(t, kitReq.MaxTokens, "relaykit 缺省 max_tokens 不应为 nil")
				} else {
					require.NotNil(t, kitReq.MaxTokens)
					assert.EqualValues(t, *legacyReq.MaxTokens, *kitReq.MaxTokens)
				}
			}
			assert.Equal(t, legacyReq.Temperature, kitReq.Temperature)
			// tools / tool_choice 深度相等（含 input_schema 映射）
			assert.Equal(t, legacyReq.Tools, kitReq.Tools)
			assert.Equal(t, legacyReq.ToolChoice, kitReq.ToolChoice)

			// 消息序列：角色一致 + 每条消息的块序列语义等价
			require.Len(t, kitReq.Messages, len(legacyReq.Messages), "消息数量不一致")
			for i, legacyMsg := range legacyReq.Messages {
				kitMsg := kitReq.Messages[i]
				assert.Equal(t, legacyMsg.Role, kitMsg.Role, "消息 %d 角色", i)
				assert.Equal(t,
					normalizeClaudeBlocks(t, legacyMsg.Content),
					normalizeClaudeBlocks(t, kitMsg.Content),
					"消息 %d 内容块语义不等价", i)
			}
		})
	}
}

// normalizeClaudeSystem 提取 system 字段文本（string 或内容块数组两种形态）。
func normalizeClaudeSystem(t *testing.T, system any) string {
	t.Helper()
	if system == nil {
		return ""
	}
	if s, ok := system.(string); ok {
		return s
	}
	b, err := json.Marshal(system)
	require.NoError(t, err)
	var blocks []map[string]any
	if json.Unmarshal(b, &blocks) == nil {
		var parts []string
		for _, blk := range blocks {
			if txt, _ := blk["text"].(string); txt != "" {
				parts = append(parts, txt)
			}
		}
		return strings.Join(parts, "\n")
	}
	return string(b)
}

// normalizeClaudeBlocks 归一化消息内容：字符串视为单一文本块；剔除空文本与 legacy 的
// "<nil>" 垃圾文本块；其余块（tool_use/tool_result）原样保留参与严格比较。
func normalizeClaudeBlocks(t *testing.T, content any) []map[string]any {
	t.Helper()
	if content == nil {
		return nil
	}
	var blocks []map[string]any
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []map[string]any{{"type": "text", "text": v}}
	default:
		b, err := json.Marshal(v)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(b, &blocks))
	}
	normalized := make([]map[string]any, 0, len(blocks))
	for _, blk := range blocks {
		if blk["type"] == "text" {
			text, _ := blk["text"].(string)
			// 剔除空文本与 legacy 对 nil content 序列化出的 "<nil>" 垃圾块
			if text == "" || text == "<nil>" {
				continue
			}
		}
		normalized = append(normalized, blk)
	}
	return normalized
}

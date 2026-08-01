// Package coze_chat 实现 OpenAI Chat Completions ↔ Coze v3 双向转换器。
//
// Coze v3 的 chat 端点以 BotID 标识机器人、以 Query 携带用户输入，且只支持流式
// （非流式需轮询，复杂且慢）。因此请求侧强制 Stream=true，把最后一条 user 消息
// 文本作为 Query；响应侧把 Coze SSE 事件转回 OpenAI 格式（非流式客户端由宿主
// 把整段 SSE 缓冲后交给非流式转换器解析）。
package coze_chat

import (
	"context"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToCozeRequestConverter 将 OpenAI Chat Completions 请求转换为 Coze v3 请求。
type OpenAIToCozeRequestConverter struct{}

func (c *OpenAIToCozeRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIChatToCoze
}

func (c *OpenAIToCozeRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToCozeRequestConverter) To() types.RelayFormat {
	return types.RelayFormatCoze
}

func (c *OpenAIToCozeRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityFair
}

// ConvertRequest 将 OpenAI 请求转换为 Coze v3 创建对话请求。
// 强制 Stream=true：Coze 非流式需轮询，统一走流式，非流式场景由宿主缓冲后解析。
func (c *OpenAIToCozeRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeneralOpenAIRequest, got %T", request)
	}

	// Query 取最后一条 user 消息文本
	query := extractLastUserMessage(openaiReq.Messages)
	if query == "" {
		return nil, fmt.Errorf("no user message found in request")
	}

	// BotID：优先映射后的上游模型名（渠道模型映射目标），否则用客户端请求的模型名
	botID := openaiReq.Model
	if info != nil {
		if upstream := info.GetUpstreamModelName(); upstream != "" {
			botID = upstream
		}
	}

	// 用户标识：Coze 要求非空。relaykit 无法访问 tenant/user 上下文（Meta 未暴露），
	// 用客户端 User 或通用占位（与 Dify 一致）；per-用户归因属阶段 6 灰度对齐。
	user := openaiReq.User
	if user == "" {
		user = "relay-user"
	}

	cozeReq := &dto.CozeCreateRequest{
		BotID:  botID,
		User:   user,
		Query:  query,
		Stream: true, // 强制流式
	}

	return cozeReq, nil
}

// extractLastUserMessage 从消息列表中提取最后一条 user 角色的文本内容。
func extractLastUserMessage(messages []dto.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		switch content := messages[i].Content.(type) {
		case string:
			return content
		case []any:
			for _, part := range content {
				m, ok := part.(map[string]any)
				if !ok {
					continue
				}
				if t, _ := m["type"].(string); t == "text" {
					if text, ok := m["text"].(string); ok {
						return text
					}
				}
			}
		}
	}
	return ""
}

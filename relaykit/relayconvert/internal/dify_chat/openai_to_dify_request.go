// Package dify_chat 实现 OpenAI Chat Completions ↔ Dify chat-messages 双向转换器。
//
// Dify 是开源 LLM 应用开发平台，chat-messages 端点只接受单个 Query 字符串，
// 因此将 OpenAI 的多轮消息拼接为带角色前缀的 query。响应侧把 Dify 的
// blocking JSON / streaming SSE 转回 OpenAI Chat Completions 格式。
package dify_chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToDifyRequestConverter 将 OpenAI Chat Completions 请求转换为 Dify 请求。
type OpenAIToDifyRequestConverter struct{}

func (c *OpenAIToDifyRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIChatToDify
}

func (c *OpenAIToDifyRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToDifyRequestConverter) To() types.RelayFormat {
	return types.RelayFormatDify
}

func (c *OpenAIToDifyRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityFair
}

// ConvertRequest 将 OpenAI 请求转换为 Dify chat-messages 请求。
func (c *OpenAIToDifyRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeneralOpenAIRequest, got %T", request)
	}

	// 拼接所有消息为 query 字符串
	query := flattenMessages(openaiReq.Messages)
	if query == "" {
		return nil, fmt.Errorf("no messages found in request")
	}

	// response_mode 由是否流式决定
	responseMode := "blocking"
	if info != nil && info.GetIsStream() {
		responseMode = "streaming"
	}

	// 用户标识
	user := openaiReq.User
	if user == "" {
		user = "relay-user"
	}

	difyReq := &dto.DifyRequest{
		Inputs:       map[string]any{},
		Query:        query,
		ResponseMode: responseMode,
		User:         user,
	}

	return difyReq, nil
}

// flattenMessages 将 OpenAI 消息列表拼接为带角色前缀的字符串。
// 格式: "System: ...\nUser: ...\nAssistant: ..."
func flattenMessages(messages []dto.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var parts []string
	for _, msg := range messages {
		text := extractTextContent(msg.Content)
		if text == "" {
			continue
		}
		prefix := capitalizeFirst(msg.Role)
		parts = append(parts, prefix+": "+text)
	}

	return strings.Join(parts, "\n")
}

// capitalizeFirst 将字符串首字母大写（角色名均为单个 ASCII 单词，等价于 strings.Title 的行为）。
// 不使用已废弃的 strings.Title，避免 Unicode 标点边界问题。
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	if c := s[0]; c >= 'a' && c <= 'z' {
		return string(c-32) + s[1:]
	}
	return s
}

// extractTextContent 从消息内容中提取文本。
// Content 可以是 string 或 []any（多模态，JSON 解析后的原始数组）。
func extractTextContent(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		var texts []string
		for _, part := range c {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t == "text" {
				if text, ok := m["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, " ")
	default:
		return ""
	}
}

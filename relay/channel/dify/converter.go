package dify

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// DifyRequest Dify Chat Messages 请求体
type DifyRequest struct {
	Inputs       map[string]interface{} `json:"inputs"`
	Query        string                 `json:"query"`
	ResponseMode string                 `json:"response_mode"`
	User         string                 `json:"user"`
}

// DifyBlockingResponse Dify 非流式（blocking）响应
type DifyBlockingResponse struct {
	Answer   string `json:"answer"`
	Metadata struct {
		Usage struct {
			TotalTokens      int `json:"total_tokens"`
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	} `json:"metadata"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
}

// DifyStreamEvent Dify 流式 SSE 事件数据
type DifyStreamEvent struct {
	Event          string `json:"event"`
	Answer         string `json:"answer"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	Metadata       struct {
		Usage struct {
			TotalTokens      int `json:"total_tokens"`
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	} `json:"metadata"`
}

// convertOpenAIToDify 将 OpenAI Chat Completions 请求转换为 Dify 请求格式。
// 所有消息被拼接为一个 query 字符串，最后一条 user 消息作为主要查询内容。
func convertOpenAIToDify(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	var openaiReq dto.GeneralOpenAIRequest
	if err := json.Unmarshal(requestBody, &openaiReq); err != nil {
		return nil, fmt.Errorf("parse OpenAI request failed: %w", err)
	}

	// 拼接所有消息为 query 字符串
	query := flattenMessages(openaiReq.Messages)
	if query == "" {
		return nil, fmt.Errorf("no messages found in request")
	}

	// 确定 response_mode
	responseMode := "blocking"
	if info.IsStream {
		responseMode = "streaming"
	}

	// 用户标识
	user := openaiReq.User
	if user == "" {
		user = "relay-user"
	}

	difyReq := DifyRequest{
		Inputs:       map[string]interface{}{},
		Query:        query,
		ResponseMode: responseMode,
		User:         user,
	}

	return json.Marshal(difyReq)
}

// flattenMessages 将 OpenAI 消息列表拼接为带角色前缀的字符串。
// 格式: "System: ...\nUser: ...\nAssistant: ..."
// 最后一条 user 消息作为主要查询。
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

		prefix := strings.Title(msg.Role)
		parts = append(parts, prefix+": "+text)
	}

	return strings.Join(parts, "\n")
}

// extractTextContent 从消息内容中提取文本。
// Content 可以是 string 或 []ContentPart（多模态）。
func extractTextContent(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		// 多模态消息，提取文本部分
		var texts []string
		for _, part := range c {
			if m, ok := part.(map[string]interface{}); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
		return strings.Join(texts, " ")
	default:
		return ""
	}
}

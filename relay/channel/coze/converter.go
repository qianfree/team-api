package coze

import (
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// convertOpenAIToCoze 将 OpenAI Chat Completions 请求转换为 Coze v3 请求。
// BotID 来自上游模型名（UpstreamModelName），即渠道配置中的模型映射目标；
// query 取最后一条 user 角色消息的文本内容。
func convertOpenAIToCoze(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	var openaiReq dto.GeneralOpenAIRequest
	if err := json.Unmarshal(requestBody, &openaiReq); err != nil {
		return nil, fmt.Errorf("parse OpenAI request failed: %w", err)
	}

	// 提取最后一条 user 消息作为 query
	query := extractLastUserMessage(openaiReq.Messages)
	if query == "" {
		return nil, fmt.Errorf("no user message found in request")
	}

	// BotID：优先使用映射后的上游模型名，否则使用原始模型名
	botID := openaiReq.Model
	if info.ChannelMeta.IsModelMapped && info.ChannelMeta.UpstreamModelName != "" {
		botID = info.ChannelMeta.UpstreamModelName
	}

	// 用户标识
	user := openaiReq.User
	if user == "" {
		user = fmt.Sprintf("tenant_%d_user_%d", info.TenantID, info.UserID)
	}

	// 流式标识
	stream := false
	if openaiReq.Stream != nil {
		stream = *openaiReq.Stream
	}

	cozeReq := CozeCreateRequest{
		BotID:  botID,
		User:   user,
		Query:  query,
		Stream: stream,
	}

	return json.Marshal(cozeReq)
}

// extractLastUserMessage 从消息列表中提取最后一条 user 角色的文本内容
func extractLastUserMessage(messages []dto.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		switch content := messages[i].Content.(type) {
		case string:
			return content
		case []interface{}:
			// 多模态消息，提取文本部分
			for _, part := range content {
				if m, ok := part.(map[string]interface{}); ok {
					if m["type"] == "text" {
						if text, ok := m["text"].(string); ok {
							return text
						}
					}
				}
			}
		}
	}
	return ""
}

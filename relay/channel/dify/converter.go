package dify

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
	relaykitdto "github.com/qianfree/team-api/relaykit/dto"
)

// DifyRequest / DifyBlockingResponse / DifyStreamEvent 与 relaykit/dto 中定义字节相同
// （仅本地曾用匿名嵌套 struct 表达 Metadata.Usage，relaykit 用具名 DifyMeta/DifyUsage；
// JSON 序列化等价，字段访问路径一致）。别名到 relaykit 统一权威定义，消除本地重复。
type DifyRequest = relaykitdto.DifyRequest
type DifyBlockingResponse = relaykitdto.DifyBlockingResponse
type DifyStreamEvent = relaykitdto.DifyStreamEvent

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

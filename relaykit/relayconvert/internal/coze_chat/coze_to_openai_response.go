package coze_chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// CozeToOpenAIResponseConverter 将 Coze 响应转换为 OpenAI Chat Completions 响应。
//
// Coze 上游始终返回 SSE（即使非流式客户端，请求侧也已强制 Stream=true）。
// 非流式场景下宿主已把整段 SSE 缓冲为 []byte，本转换器扫描该缓冲、抽取
// conversation.message.completed / delta 事件中的 answer 内容。
type CozeToOpenAIResponseConverter struct{}

func (c *CozeToOpenAIResponseConverter) ID() string {
	return relayconvert.ResponseConverterCozeChatToOAIChat
}

func (c *CozeToOpenAIResponseConverter) From() types.RelayFormat {
	return types.RelayFormatCoze
}

func (c *CozeToOpenAIResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *CozeToOpenAIResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityFair
}

// ConvertResponse 解析 Coze SSE 缓冲体，构建 OpenAI 非流式响应。
// response 期望为 []byte（宿主缓冲的整段 SSE）。
func (c *CozeToOpenAIResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	raw, ok := response.([]byte)
	if !ok {
		return nil, fmt.Errorf("expected []byte (buffered Coze SSE), got %T", response)
	}

	var (
		fullContent   strings.Builder
		currentEvent  string
		gotCompleted  bool
	)

	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}

		switch currentEvent {
		case "conversation.message.delta":
			var msg dto.CozeMessage
			if err := json.Unmarshal([]byte(data), &msg); err != nil {
				continue
			}
			if msg.Type == "answer" && !gotCompleted {
				fullContent.WriteString(msg.Content)
			}

		case "conversation.message.completed":
			// completed 事件包含完整内容，优先使用
			var msg dto.CozeMessage
			if err := json.Unmarshal([]byte(data), &msg); err == nil && msg.Type == "answer" {
				fullContent.Reset()
				fullContent.WriteString(msg.Content)
				gotCompleted = true
			}

		case "error":
			return nil, fmt.Errorf("coze error: %s", data)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan coze response failed: %w", err)
	}

	content := fullContent.String()
	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

	openaiResp := &dto.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp()),
		Object:  "chat.completion",
		Created: 0,
		Model:   modelName,
		Choices: []dto.Choice{{
			Index: 0,
			Message: dto.Message{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: "stop",
		}},
		Usage: dto.UsageWithDetails{
			CompletionTokens: estimateTokens(content),
			TotalTokens:      estimateTokens(content),
		},
	}

	return openaiResp, nil
}

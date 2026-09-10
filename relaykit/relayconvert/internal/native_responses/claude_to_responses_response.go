package native_responses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ClaudeToResponsesResponseConverter 将 Claude Messages 非流式响应转换为 OpenAI Responses 响应。
// 文本块 → message 项，tool_use 块 → function_call 项；
// 思考内容无 Responses 非流式对应物，跳过（流式侧以 reasoning summary 事件透出）。
type ClaudeToResponsesResponseConverter struct{}

func (c *ClaudeToResponsesResponseConverter) ID() string {
	return relayconvert.ResponseConverterClaudeMessagesToOAIResponses
}

func (c *ClaudeToResponsesResponseConverter) From() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *ClaudeToResponsesResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *ClaudeToResponsesResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertResponse 将 *dto.ClaudeResponse 转换为 Responses 响应对象（map 构造，与旧桥一致）。
func (c *ClaudeToResponsesResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	claudeResp, ok := response.(*dto.ClaudeResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.ClaudeResponse, got %T", response)
	}

	// 构建 output：文本块 → message 项，tool_use 块 → function_call 项
	var textParts []string
	output := make([]map[string]any, 0)
	for _, block := range claudeResp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil && *block.Text != "" {
				textParts = append(textParts, *block.Text)
			}
		case "thinking", "redacted_thinking":
			// 思考内容无 Responses 非流式对应物，跳过（流式侧以 reasoning summary 事件透出）
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			output = append(output, map[string]any{
				"type":      "function_call",
				"id":        block.ID,
				"call_id":   block.ID,
				"name":      block.Name,
				"arguments": string(argsJSON),
				"status":    "completed",
			})
		}
	}
	if len(textParts) > 0 {
		msgItem := map[string]any{
			"type":   "message",
			"id":     fmt.Sprintf("msg_%s", claudeResp.ID),
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{{
				"type":        "output_text",
				"text":        strings.Join(textParts, "\n"),
				"annotations": []any{},
			}},
		}
		output = append([]map[string]any{msgItem}, output...)
	}

	modelName := claudeResp.Model
	if modelName == "" || isModelMapped(info) {
		modelName = originModelName(info)
	}
	respID := fmt.Sprintf("resp_%s", claudeResp.ID)
	if claudeResp.ID == "" {
		respID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
	}
	createdAt := int(time.Now().Unix())
	completedAt := createdAt

	// 客户端可见 usage 用 OpenAI 语义（input 含缓存，cached 为子集）
	visibleUsage := claudeVisibleUsage(claudeResp.Usage)

	return buildResponsesObjectMap(respID, createdAt, "completed", modelName, output,
		buildResponsesUsageMap(visibleUsage), &completedAt, info), nil
}

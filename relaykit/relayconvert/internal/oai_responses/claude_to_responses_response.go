package oai_responses

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

// NowFunc 时间源（默认 time.Now）。流式/非流式转换器生成响应时间戳与兜底 ID 时使用，
// golden 测试替换为固定时钟以保证输出确定性。
var NowFunc = time.Now

// responsesEchoProvider 宿主可选实现的 Meta 扩展接口：提供 Responses 入站请求快照，
// 供响应对象 echo 请求参数（instructions/max_output_tokens/temperature/top_p）。
// 宿主 relay/common.RelayInfo 实现本接口；未实现或快照缺失时回退默认值
// （temperature=1.0 / top_p=1.0，其余 nil）。
type responsesEchoProvider interface {
	ResponsesRequestSnapshot() *dto.OpenAIResponsesRequest
}

// responsesEcho 响应对象需 echo 的请求参数快照。
type responsesEcho struct {
	instructions    json.RawMessage
	maxOutputTokens *int
	temperature     *float64
	topP            *float64
}

// responsesEchoOf 从 Meta 提取 echo 参数（接口未实现/快照缺失时回退默认值）。
func responsesEchoOf(info convmeta.Meta) responsesEcho {
	echo := responsesEcho{temperature: ptrFloat64(1.0), topP: ptrFloat64(1.0)}
	provider, ok := info.(responsesEchoProvider)
	if !ok || provider == nil {
		return echo
	}
	rr := provider.ResponsesRequestSnapshot()
	if rr == nil {
		return echo
	}
	if rr.Temperature != nil {
		echo.temperature = rr.Temperature
	}
	if rr.TopP != nil {
		echo.topP = rr.TopP
	}
	if rr.MaxOutputTokens != nil {
		m := int(*rr.MaxOutputTokens)
		echo.maxOutputTokens = &m
	}
	if len(rr.Instructions) > 0 {
		echo.instructions = rr.Instructions
	}
	return echo
}

func ptrFloat64(v float64) *float64 { return &v }

// ClaudeToResponsesResponseConverter Claude 上游 → Responses 客户端（非流式响应侧）。
// 移植自宿主 relay/channel/claude/responses_bridge.go 的 handleNonStreamToResponses 纯映射部分。
type ClaudeToResponsesResponseConverter struct{}

func (c *ClaudeToResponsesResponseConverter) ID() string {
	return relayconvert.ConverterOpenAIResponsesToClaudeMessages
}

func (c *ClaudeToResponsesResponseConverter) From() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *ClaudeToResponsesResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

// ConvertResponse 入参断言 *dto.ClaudeResponse，输出 *dto.OpenAIResponsesResponse。
func (c *ClaudeToResponsesResponseConverter) ConvertResponse(
	ctx context.Context, info convmeta.Meta, response any,
) (any, *dto.UsageWithDetails, error) {
	claudeResp, ok := response.(*dto.ClaudeResponse)
	if !ok {
		return nil, nil, fmt.Errorf("expected *dto.ClaudeResponse, got %T", response)
	}
	result, usage := buildResponsesFromClaude(info, claudeResp)
	return result, usage, nil
}

// buildResponsesFromClaude 将 Claude 非流式响应转换为 Responses 响应对象。
// 文本块聚合为 message 项（置于输出数组最前），tool_use 块转为 function_call 项，
// thinking 块跳过（流式侧以 reasoning summary 事件透出）。
// 返回的 usage 为客户端可见口径（OpenAI 语义：input 含缓存，cached 为其子集）。
func buildResponsesFromClaude(info convmeta.Meta, claudeResp *dto.ClaudeResponse) (*dto.OpenAIResponsesResponse, *dto.UsageWithDetails) {
	// 构建 output：文本块 → message 项，tool_use 块 → function_call 项
	var textParts []string
	output := make([]dto.ResponsesOutput, 0)
	for _, block := range claudeResp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil && *block.Text != "" {
				textParts = append(textParts, *block.Text)
			}
		case "thinking", "redacted_thinking":
			// 思考内容无 Responses 非流式对应物，跳过
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			output = append(output, dto.ResponsesOutput{
				Type:      "function_call",
				ID:        block.ID,
				CallID:    block.ID,
				Name:      block.Name,
				Arguments: string(argsJSON),
				Status:    "completed",
			})
		}
	}
	if len(textParts) > 0 {
		msgItem := dto.ResponsesOutput{
			Type:   "message",
			ID:     fmt.Sprintf("msg_%s", claudeResp.ID),
			Status: "completed",
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{{
				Type:        "output_text",
				Text:        strings.Join(textParts, "\n"),
				Annotations: []dto.ResponsesAnnotation{},
			}},
		}
		output = append([]dto.ResponsesOutput{msgItem}, output...)
	}

	// 模型名：优先上游返回值，为空时回退客户端请求模型名。
	//（旧路径另有 IsModelMapped 时强制 OriginModelName 的分支，convmeta 无该信号，
	// 列为已知差异：映射渠道上游返回自身模型名时客户端看到上游名）
	modelName := claudeResp.Model
	if modelName == "" && info != nil {
		modelName = info.GetOriginModelName()
	}
	respID := fmt.Sprintf("resp_%s", claudeResp.ID)
	if claudeResp.ID == "" {
		respID = fmt.Sprintf("resp_%d", NowFunc().UnixNano())
	}
	createdAt := int(NowFunc().Unix())
	completedAt := createdAt

	// 客户端可见 usage 用 OpenAI 语义（input 含缓存，cached 为其子集）
	visibleUsage := &dto.UsageWithDetails{}
	if claudeResp.Usage != nil {
		promptTotal := claudeResp.Usage.InputTokens +
			claudeResp.Usage.CacheReadInputTokens +
			claudeResp.Usage.CacheCreationInputTokens
		visibleUsage = &dto.UsageWithDetails{
			PromptTokens:     promptTotal,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      promptTotal + claudeResp.Usage.OutputTokens,
		}
		if claudeResp.Usage.CacheReadInputTokens > 0 || claudeResp.Usage.CacheCreationInputTokens > 0 {
			visibleUsage.PromptTokensDetails = claudeUsageToTokenDetails(claudeResp.Usage)
		}
	}

	return buildResponsesObject(respID, createdAt, "completed", modelName, output,
		responsesUsageOf(visibleUsage), &completedAt, responsesEchoOf(info)), visibleUsage
}

// claudeUsageToTokenDetails 将 ClaudeUsage 转为 token 细分（含 5m/1h 缓存创建）。
func claudeUsageToTokenDetails(u *dto.ClaudeUsage) *dto.TokenDetails {
	if u == nil {
		return nil
	}
	td := &dto.TokenDetails{
		CachedTokens:         u.CacheReadInputTokens,
		CachedCreationTokens: u.CacheCreationInputTokens,
	}
	if u.CacheCreation != nil {
		td.CachedCreation5mTokens = u.CacheCreation.Ephemeral5mInputTokens
		td.CachedCreation1hTokens = u.CacheCreation.Ephemeral1hInputTokens
	}
	return td
}

// responsesUsageOf 将 OpenAI 语义 usage 转为 Responses usage 对象。
// 与宿主 BuildResponsesUsageMap 字段一致：无细分时输出 cached_tokens/reasoning_tokens 为 0 的默认对象。
func responsesUsageOf(usage *dto.UsageWithDetails) *dto.ResponsesUsage {
	if usage == nil {
		return nil
	}
	result := &dto.ResponsesUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  usage.TotalTokens,
	}
	if usage.PromptTokensDetails != nil {
		result.InputTokensDetails = &dto.InputTokenDetails{
			CachedTokens: usage.PromptTokensDetails.CachedTokens,
			AudioTokens:  usage.PromptTokensDetails.AudioTokens,
		}
	} else {
		result.InputTokensDetails = &dto.InputTokenDetails{}
	}
	if usage.CompletionTokenDetails != nil {
		result.OutputTokenDetails = &dto.OutputTokenDetails{
			ReasoningTokens:          usage.CompletionTokenDetails.ReasoningTokens,
			AudioTokens:              usage.CompletionTokenDetails.AudioTokens,
			AcceptedPredictionTokens: usage.CompletionTokenDetails.AcceptedPredictionTokens,
			RejectedPredictionTokens: usage.CompletionTokenDetails.RejectedPredictionTokens,
		}
	} else {
		result.OutputTokenDetails = &dto.OutputTokenDetails{}
	}
	return result
}

// buildResponsesObject 构建 Responses response 对象（非流式与流式 response.created/completed 共用）。
// 语义对齐宿主 openai.BuildResponsesObjectMap；store 恒 false——合成响应不落上游存储。
func buildResponsesObject(respID string, createdAt int, status string, model string,
	output []dto.ResponsesOutput, usage *dto.ResponsesUsage, completedAt *int, echo responsesEcho,
) *dto.OpenAIResponsesResponse {
	resp := &dto.OpenAIResponsesResponse{
		ID:                 respID,
		Object:             "response",
		CreatedAt:          createdAt,
		Status:             json.RawMessage(fmt.Sprintf("%q", status)),
		Error:              nil,
		Instructions:       echo.instructions,
		MaxOutputTokens:    echo.maxOutputTokens,
		Model:              model,
		Output:             output,
		ParallelToolCalls:  true,
		PreviousResponseID: nil,
		Reasoning:          &dto.ResponsesReasoning{Effort: nil, Summary: nil},
		Store:              false,
		Temperature:        echo.temperature,
		Text:               &dto.ResponsesText{Format: dto.ResponsesTextFormat{Type: "text"}},
		ToolChoice:         "auto",
		Tools:              []any{},
		TopP:               echo.topP,
		Truncation:         "disabled",
		User:               nil,
		Metadata:           map[string]any{},
		Usage:              usage,
	}
	if completedAt != nil {
		completed := *completedAt
		resp.CompletedAt = completed
	}
	return resp
}

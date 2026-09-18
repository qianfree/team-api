package oai_responses

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ResponsesToOpenAIStreamConverter 将 Responses API 流式响应实时转换为 Chat Completions 流式响应。
// 输出为裸 *dto.ChatCompletionStreamResponse chunk（OpenAI 风格数据帧，宿主负责 SSE 帧化与 [DONE]）。
type ResponsesToOpenAIStreamConverter struct{}

func (c *ResponsesToOpenAIStreamConverter) ID() string {
	return relayconvert.ResponseConverterOAIResponsesToOAIChatStream
}

func (c *ResponsesToOpenAIStreamConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *ResponsesToOpenAIStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *ResponsesToOpenAIStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将 Responses API SSE 事件流转换为 Chat Completions chunk 流。
// reader 提供上游 Responses SSE 事件，转换后的 chunk 通过 chunkWriter 回调写出。
func (c *ResponsesToOpenAIStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	responseID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	createAt := time.Now().Unix()
	// 模型名：未映射时取上游模型名（response.created / response.completed 事件可覆盖）；
	// 模型映射时一律用原始请求模型名，不向客户端泄漏上游真实模型（与非流式转换器口径一致）
	model := ""
	if info != nil {
		model = info.GetUpstreamModelName()
		if model == "" || isModelMapped(info) {
			model = info.GetOriginModelName()
		}
	}

	var (
		totalUsage            dto.UsageWithDetails
		usageText, outputText strings.Builder
		sentStart, sentStop   bool
		sawToolCall           bool
	)

	toolCallIndexByID := make(map[string]int)
	toolCallNameByID := make(map[string]string)
	toolCallArgsByID := make(map[string]string)
	toolCallNameSent := make(map[string]bool)

	sendStartIfNeeded := func() error {
		if sentStart {
			return nil
		}
		chunk := &dto.ChatCompletionStreamResponse{
			ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
			Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{Role: "assistant", Content: ""}}},
		}
		if err := chunkWriter(chunk); err != nil {
			return err
		}
		sentStart = true
		return nil
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}
		// Responses SSE 的 event: 行不参与解析（data 负载自带 type 字段）
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var streamResp dto.ResponsesStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			// 解析失败视为软错误跳过该行（与旧实现的 sr.Error 行为一致：流继续处理）
			continue
		}

		switch streamResp.Type {
		case "response.created":
			if streamResp.Response != nil {
				// 模型映射时不回填上游模型名，避免泄漏给客户端
				if streamResp.Response.Model != "" && !isModelMapped(info) {
					model = streamResp.Response.Model
				}
				if streamResp.Response.CreatedAt != 0 {
					createAt = int64(streamResp.Response.CreatedAt)
				}
			}

		case "response.reasoning_summary_text.delta":
			if streamResp.Delta == "" {
				continue
			}
			if err := sendStartIfNeeded(); err != nil {
				return fmt.Errorf("send start chunk failed: %w", err)
			}
			usageText.WriteString(streamResp.Delta)
			delta := streamResp.Delta
			chunk := &dto.ChatCompletionStreamResponse{
				ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
				Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{ReasoningContent: &delta}}},
			}
			if err := chunkWriter(chunk); err != nil {
				return fmt.Errorf("send reasoning chunk failed: %w", err)
			}

		case "response.output_text.delta":
			if err := sendStartIfNeeded(); err != nil {
				return fmt.Errorf("send start chunk failed: %w", err)
			}
			if streamResp.Delta != "" {
				outputText.WriteString(streamResp.Delta)
				usageText.WriteString(streamResp.Delta)
				delta := streamResp.Delta
				chunk := &dto.ChatCompletionStreamResponse{
					ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
					Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{Content: delta}}},
				}
				if err := chunkWriter(chunk); err != nil {
					return fmt.Errorf("send text chunk failed: %w", err)
				}
			}

		case "response.output_item.added", "response.output_item.done":
			if streamResp.Item == nil || streamResp.Item.Type != "function_call" {
				continue
			}
			callID := strings.TrimSpace(streamResp.Item.CallID)
			if callID == "" {
				callID = strings.TrimSpace(streamResp.Item.ID)
			}
			if callID == "" {
				continue
			}
			name := strings.TrimSpace(streamResp.Item.Name)
			if name != "" {
				toolCallNameByID[callID] = name
			}
			// added/done 事件的 arguments 是累计值：与已发送前缀做差得到增量，避免重复下发
			newArgs := streamResp.Item.Arguments
			prevArgs := toolCallArgsByID[callID]
			var argsDelta string
			if newArgs != "" {
				if strings.HasPrefix(newArgs, prevArgs) {
					argsDelta = newArgs[len(prevArgs):]
				} else {
					argsDelta = newArgs
				}
				toolCallArgsByID[callID] = newArgs
			}
			if err := sendToolCallChunk(chunkWriter, responseID, createAt, model, callID, name, argsDelta, toolCallIndexByID, toolCallNameByID, toolCallNameSent); err != nil {
				return fmt.Errorf("send tool call chunk failed: %w", err)
			}
			sawToolCall = true
			usageText.WriteString(name)
			usageText.WriteString(argsDelta)

		case "response.function_call_arguments.delta":
			callID := strings.TrimSpace(streamResp.ItemID)
			if callID == "" {
				continue
			}
			toolCallArgsByID[callID] += streamResp.Delta
			if err := sendToolCallChunk(chunkWriter, responseID, createAt, model, callID, "", streamResp.Delta, toolCallIndexByID, toolCallNameByID, toolCallNameSent); err != nil {
				return fmt.Errorf("send tool call args chunk failed: %w", err)
			}
			sawToolCall = true
			usageText.WriteString(streamResp.Delta)

		case "response.completed":
			if streamResp.Response != nil {
				// 模型映射时不回填上游模型名，避免泄漏给客户端
				if streamResp.Response.Model != "" && !isModelMapped(info) {
					model = streamResp.Response.Model
				}
				if streamResp.Response.CreatedAt != 0 {
					createAt = int64(streamResp.Response.CreatedAt)
				}
				if streamResp.Response.Usage != nil {
					totalUsage = responsesUsageToUsageWithDetails(streamResp.Response.Usage)
					if totalUsage.TotalTokens == 0 {
						totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
					}
				}
			}
			if err := sendStartIfNeeded(); err != nil {
				return fmt.Errorf("send start chunk failed: %w", err)
			}
			if !sentStop {
				finishReason := "stop"
				if sawToolCall && outputText.Len() == 0 {
					finishReason = "tool_calls"
				}
				fr := finishReason
				stopChunk := &dto.ChatCompletionStreamResponse{
					ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
					Choices: []dto.StreamChoice{{Index: 0, FinishReason: &fr}},
				}
				if err := chunkWriter(stopChunk); err != nil {
					return fmt.Errorf("send stop chunk failed: %w", err)
				}
				sentStop = true
			}

		case "response.error", "response.failed":
			errMsg := "responses stream error"
			if streamResp.Response != nil && streamResp.Response.Error != nil {
				if b, err := json.Marshal(streamResp.Response.Error); err == nil {
					errMsg = string(b)
				}
			}
			return fmt.Errorf("%s: %s", streamResp.Type, errMsg)
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 上游未返回 usage 时按累计文本估算（4 字符/token；流中断兜底由宿主处理）
	if totalUsage.TotalTokens == 0 && usageText.Len() > 0 {
		estimated := usageText.Len() / 4
		totalUsage.CompletionTokens = estimated
		totalUsage.TotalTokens = totalUsage.PromptTokens + estimated
	}
	if !sentStart {
		if err := sendStartIfNeeded(); err != nil {
			return fmt.Errorf("send start chunk failed: %w", err)
		}
	}
	if !sentStop {
		fr := "stop"
		stopChunk := &dto.ChatCompletionStreamResponse{
			ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
			Choices: []dto.StreamChoice{{Index: 0, FinishReason: &fr}},
		}
		if err := chunkWriter(stopChunk); err != nil {
			return fmt.Errorf("send stop chunk failed: %w", err)
		}
	}
	if totalUsage.TotalTokens > 0 {
		usageChunk := &dto.ChatCompletionStreamResponse{
			ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
			Choices: []dto.StreamChoice{},
			Usage:   &dto.UsageWithDetails{PromptTokens: totalUsage.PromptTokens, CompletionTokens: totalUsage.CompletionTokens, TotalTokens: totalUsage.TotalTokens, PromptTokensDetails: totalUsage.PromptTokensDetails, CompletionTokenDetails: totalUsage.CompletionTokenDetails},
		}
		if err := chunkWriter(usageChunk); err != nil {
			return fmt.Errorf("send usage chunk failed: %w", err)
		}
	}
	return nil
}

// sendToolCallChunk 下发一个工具调用增量 chunk（移植宿主 r2cSendToolCallChunk）。
// codex 客户端 serde 严格：FunctionCall.Arguments 无 omitempty，序列化恒带 arguments 键；
// name 仅在首个 chunk 携带（nameSent 去重）。
func sendToolCallChunk(chunkWriter func(chunk any) error, responseID string, createAt int64, model string, callID string, name string, argsDelta string, indexByID map[string]int, nameByID map[string]string, nameSent map[string]bool) error {
	idx, ok := indexByID[callID]
	if !ok {
		idx = len(indexByID)
		indexByID[callID] = idx
	}
	if name != "" {
		nameByID[callID] = name
	}
	if nameByID[callID] != "" {
		name = nameByID[callID]
	}
	tool := dto.ToolCall{ID: callID, Type: "function", Index: &idx, Function: dto.FunctionCall{Arguments: argsDelta}}
	if name != "" && !nameSent[callID] {
		tool.Function.Name = name
		nameSent[callID] = true
	}
	chunk := &dto.ChatCompletionStreamResponse{
		ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
		Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{ToolCalls: []dto.ToolCall{tool}}}},
	}
	return chunkWriter(chunk)
}

// responsesUsageToUsageWithDetails 将 Responses API usage 转换为带明细的 usage
// （移植宿主 responsesUsageToCommon 的字段映射；CacheIncludedInPrompt 语义由宿主处理）。
func responsesUsageToUsageWithDetails(u *dto.ResponsesUsage) dto.UsageWithDetails {
	usage := dto.UsageWithDetails{}
	if u == nil {
		return usage
	}
	usage.PromptTokens = u.InputTokens
	usage.CompletionTokens = u.OutputTokens
	usage.TotalTokens = u.TotalTokens
	if d := u.InputTokensDetails; d != nil {
		usage.PromptTokensDetails = &dto.TokenDetails{
			CachedTokens:     d.CachedTokens,
			CacheWriteTokens: d.CacheWriteTokens,
			TextTokens:       d.TextTokens,
			AudioTokens:      d.AudioTokens,
			ImageTokens:      d.ImageTokens,
		}
	}
	if d := u.OutputTokenDetails; d != nil {
		usage.CompletionTokenDetails = &dto.TokenDetails{
			TextTokens:               d.TextTokens,
			AudioTokens:              d.AudioTokens,
			ReasoningTokens:          d.ReasoningTokens,
			AcceptedPredictionTokens: d.AcceptedPredictionTokens,
			RejectedPredictionTokens: d.RejectedPredictionTokens,
		}
	}
	return usage
}

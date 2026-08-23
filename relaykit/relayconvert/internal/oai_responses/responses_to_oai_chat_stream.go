package oai_responses

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ResponsesToOpenAIChatStreamConverter Responses 上游 SSE → OpenAI Chat 客户端流
// （流式响应侧，P3——补齐当初因 StreamScannerHandler 依赖被排除的 B 方向流式）。
// 移植宿主 relay/channel/openai/converter.go 的 HandleResponsesStreamToChat 事件状态机，
// 去除 StreamScannerHandler 超时治理（PingTicker 兜底，与 P2 D1 同款取舍）。
// chunk 输出 *dto.ChatCompletionStreamResponse；既可被桥接直写 openai 客户端，
// 也可经 chainStreamConverters 组合转 claude/gemini 客户端。
//
// legacy 语义保持：
//   - 工具 callID 优先 Item.CallID 空 ID；output_item.added/done 都可能带累积 arguments
//     → 前缀差分（HasPrefix 取后缀，否则整段当 delta）
//   - name 只在该 callID 的首个 chunk 携带（nameSent 去重）；callID 首见分配递增 index
//   - finish 判据：sawToolCall && outputText 为空 → "tool_calls" 否则 "stop"
//     （有文本+有工具时报 stop——legacy 取舍，按 finish_reason 分派的客户端会忽略工具）
//   - usage 于 response.completed 直取（total==0 补 prompt+completion），独立 usage chunk
//     （Choices 空）携带——桥接从带 Usage 的 chunk 提取计费
//   - reasoning delta → reasoning_content（DeepSeek 风格键）；response.created 覆盖
//     model/createdAt；其它事件类型忽略
type ResponsesToOpenAIChatStreamConverter struct{}

func (c *ResponsesToOpenAIChatStreamConverter) ID() string {
	// 独立流式转换器：ID/From/To 表达自身真实流方向（responses→openai chat）
	return relayconvert.ConverterOpenAIResponsesToOpenAIChatStream
}

func (c *ResponsesToOpenAIChatStreamConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *ResponsesToOpenAIChatStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

// ConvertStreamResponse 读取 Responses SSE reader，经 chunkWriter 输出 chat 流 chunk。
// response.completed 后正常返回（[DONE] 由外层桥接写；组合链中 pipe 关闭即第二跳 EOF）。
func (c *ResponsesToOpenAIChatStreamConverter) ConvertStreamResponse(
	ctx context.Context, info convmeta.Meta, reader io.Reader, chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	requestID := convmeta.RequestIDOf(info)
	if requestID == "" {
		requestID = fmt.Sprintf("%d", NowFunc().UnixNano())
	}
	responseID := fmt.Sprintf("chatcmpl-%s", requestID)
	createdAt := NowFunc().Unix()
	modelName := ""
	if info != nil && info.HasChannelMeta() {
		modelName = info.GetUpstreamModelName()
	}

	var (
		totalUsage     dto.UsageWithDetails
		outputText     strings.Builder // finish_reason 判定用
		usageText      strings.Builder // 估算判据用（含 reasoning + 工具名 + 参数）
		sentStart      bool
		sentStop       bool
		sawToolCall    bool
		toolCallIdx    = 0                       // 已分配的工具调用序号（递增）
		toolSeen       = make(map[string]bool)   // callID → 是否已分配 index
		toolArgsByID   = make(map[string]string) // callID → 已发参数累积（前缀差分基准）
		toolNameSent   = make(map[string]bool)   // callID → name 是否已随 chunk 发出
		toolIndexByID  = make(map[string]int)    // callID → 分配的 delta index
		callIDByItemID = make(map[string]string) // item.id → call_id（delta 事件只带 item_id，需归一键）
	)

	// emit 基础 chunk 骨架（ID/Created/Model 各跳统一）
	newChunk := func(choices []dto.StreamChoice) *dto.ChatCompletionStreamResponse {
		return &dto.ChatCompletionStreamResponse{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: createdAt,
			Model:   modelName,
			Choices: choices,
		}
	}

	// sendStartIfNeeded 发首个 role chunk（部分客户端依赖首 chunk 的 role 字段）
	sendStartIfNeeded := func() error {
		if sentStart {
			return nil
		}
		sentStart = true
		return chunkWriter(newChunk([]dto.StreamChoice{{
			Index: 0,
			Delta: dto.Message{Role: "assistant", Content: ""},
		}}))
	}

	// sendToolCallChunk 发一个工具调用增量 chunk（name 首见携带、args 前缀差分）
	sendToolCallChunk := func(callID, name, argsDelta string) error {
		if !toolSeen[callID] {
			toolSeen[callID] = true
			toolIndexByID[callID] = toolCallIdx
			toolCallIdx++
		}
		tc := dto.ToolCall{
			Index:    toolIndexByID[callID],
			ID:       callID,
			Type:     "function",
			Function: dto.FunctionCall{Arguments: argsDelta},
		}
		if name != "" && !toolNameSent[callID] {
			tc.Function.Name = name
			toolNameSent[callID] = true
		}
		return chunkWriter(newChunk([]dto.StreamChoice{{
			Index: 0,
			Delta: dto.Message{ToolCalls: []dto.ToolCall{tc}},
		}}))
	}

	// sendFinish 发结束 chunk（finish_reason 判据为 legacy 口径）
	sendFinish := func() error {
		if sentStop {
			return nil
		}
		sentStop = true
		reason := "stop"
		if sawToolCall && outputText.Len() == 0 {
			reason = "tool_calls"
		}
		return chunkWriter(newChunk([]dto.StreamChoice{{
			Index:        0,
			Delta:        dto.Message{},
			FinishReason: &reason,
		}}))
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "event:") || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := extractSSEData(line)
		if data == "" || data == "[DONE]" {
			continue
		}

		var event dto.ResponsesStreamResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "response.created":
			if event.Response != nil {
				if event.Response.Model != "" {
					modelName = event.Response.Model
				}
				if event.Response.CreatedAt > 0 {
					createdAt = int64(event.Response.CreatedAt)
				}
			}

		case "response.reasoning_summary_text.delta":
			if event.Delta == "" {
				continue
			}
			if err := sendStartIfNeeded(); err != nil {
				return err
			}
			usageText.WriteString(event.Delta)
			rc := event.Delta
			if err := chunkWriter(newChunk([]dto.StreamChoice{{
				Index: 0,
				Delta: dto.Message{ReasoningContent: &rc},
			}})); err != nil {
				return err
			}

		case "response.output_text.delta":
			if event.Delta == "" {
				continue
			}
			if err := sendStartIfNeeded(); err != nil {
				return err
			}
			outputText.WriteString(event.Delta)
			usageText.WriteString(event.Delta)
			if err := chunkWriter(newChunk([]dto.StreamChoice{{
				Index: 0,
				Delta: dto.Message{Content: event.Delta},
			}})); err != nil {
				return err
			}

		case "response.output_item.added", "response.output_item.done":
			// 仅处理 function_call 项；done/added 都可能带累积 arguments → 前缀差分
			if event.Item == nil || event.Item.Type != "function_call" {
				continue
			}
			callID := event.Item.CallID
			if callID == "" {
				callID = event.Item.ID
			}
			if callID == "" {
				continue
			}
			// 登记 item.id → call_id：function_call_arguments.delta 事件只携带 item_id
			//（OpenAI 规范为 output item 的 id，如 "fc_xxx"，≠ call_id），必须归一到
			// 同一键，否则 done 事件的前缀差分基准为空、完整参数会被当作增量重发一遍，
			// 客户端组装出非法 JSON 工具入参（"Invalid tool parameters"）
			if event.Item.ID != "" && event.Item.ID != callID {
				callIDByItemID[event.Item.ID] = callID
			}
			if err := sendStartIfNeeded(); err != nil {
				return err
			}
			sawToolCall = true
			name := event.Item.Name
			argsDelta := ""
			if event.Item.Arguments != "" {
				prev := toolArgsByID[callID]
				full := event.Item.Arguments
				if strings.HasPrefix(full, prev) {
					argsDelta = full[len(prev):]
				} else {
					argsDelta = full
				}
				toolArgsByID[callID] = full
			}
			if name != "" {
				usageText.WriteString(name)
			}
			usageText.WriteString(argsDelta)
			// 无增量可发：args 无差分 且（无 name 或 name 已随首 chunk 发出）
			if argsDelta == "" && (name == "" || toolNameSent[callID]) {
				continue
			}
			if err := sendToolCallChunk(callID, name, argsDelta); err != nil {
				return err
			}

		case "response.function_call_arguments.delta":
			callID := event.ItemID
			// 归一到 output_item 事件的 call_id 键（见 output_item 分支注释）
			if mapped, ok := callIDByItemID[callID]; ok {
				callID = mapped
			}
			if callID == "" {
				continue
			}
			if err := sendStartIfNeeded(); err != nil {
				return err
			}
			sawToolCall = true
			toolArgsByID[callID] += event.Delta
			usageText.WriteString(event.Delta)
			if err := sendToolCallChunk(callID, "", event.Delta); err != nil {
				return err
			}

		case "response.completed":
			if event.Response != nil {
				if event.Response.Model != "" {
					modelName = event.Response.Model
				}
				if u := event.Response.Usage; u != nil {
					totalUsage.PromptTokens = u.InputTokens
					totalUsage.CompletionTokens = u.OutputTokens
					totalUsage.TotalTokens = u.TotalTokens
					if totalUsage.TotalTokens == 0 {
						totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
					}
					if d := u.InputTokensDetails; d != nil {
						totalUsage.PromptTokensDetails = &dto.TokenDetails{
							CachedTokens:     d.CachedTokens,
							CacheWriteTokens: d.CacheWriteTokens,
							AudioTokens:      d.AudioTokens,
						}
					}
					if d := u.OutputTokenDetails; d != nil {
						totalUsage.CompletionTokenDetails = &dto.TokenDetails{
							ReasoningTokens: d.ReasoningTokens,
							AudioTokens:     d.AudioTokens,
						}
					}
				}
			}
			if err := sendStartIfNeeded(); err != nil {
				return err
			}
			if err := sendFinish(); err != nil {
				return err
			}
			// 独立 usage chunk（Choices 空，OpenAI include_usage 语义）
			if totalUsage.TotalTokens > 0 {
				usageChunk := newChunk(nil)
				usageChunk.Usage = &totalUsage
				if err := chunkWriter(usageChunk); err != nil {
					return err
				}
			}
			return nil

		case "response.error", "response.failed":
			errPayload := map[string]any{"type": event.Type}
			if event.Response != nil && event.Response.Error != nil {
				errPayload["error"] = event.Response.Error
			}
			body, _ := json.Marshal(errPayload)
			return &types.EmbeddedUpstreamError{Body: body}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 上游未发 response.completed 即断流：补 start/finish 保证客户端正常收尾
	//（usage 缺失——桥接的估算兜底处理）
	if err := sendStartIfNeeded(); err != nil {
		return err
	}
	return sendFinish()
}

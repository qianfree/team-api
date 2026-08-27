package oai_responses

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIChatToResponsesStreamConverter OpenAI Chat 上游 SSE → Responses 客户端 SSE（流式响应侧）。
// 移植自宿主 relay/channel/openai/responses.go 的 handleResponsesInboundStream 状态机，
// 经流式注册表按 (openai→responses) 路由（codex 打 chat-only 渠道的流式主路径）。
//
// 与 legacy 的确定性差异（顺手修复项，golden/单测锁定）：
//   - 多工具的 done 事件与 completed output 数组按登记顺序（legacy 遍历 map 顺序随机）；
//   - 重复 finish_reason 不再重复发 done（legacy 无去重标志）；
//   - completedAt = max(NowFunc, createdAt)（legacy 为独立 Now()，可能出现 completed < created）。
//
// 思考内容（reasoning_content）产出为独立的 reasoning output item（output_item.added →
// reasoning_summary_text.delta/done → output_item.done，并进 completed 的 output 数组）——
// codex 等 Responses 客户端据此在后续轮次回传思考项，配合请求侧还原 reasoning_content，
// 满足 DeepSeek 等思考模式上游的回传要求。message 项为惰性开启（首个文本增量时才 added），
// 对齐真实 OpenAI 的 reasoning→message 项序。
//
// 错误契约（宿主桥接经 errors.Is/As 分类）：
//   - SSE 内嵌上游错误 → *types.EmbeddedUpstreamError（原文载荷）；
//   - 上游流非 chat 格式（假成功防护）→ 包装 types.ErrProtocolMismatch 的错误；
//   - 客户端断开 → ctx.Err()。
type OpenAIChatToResponsesStreamConverter struct{}

// chatReasoningSeg 已收尾的思考段（completed output 按 index 参与排序）。
//
// signature 为上游思考签名（Claude thinking 块的 signature / Gemini thoughtSignature），
// 经 Responses reasoning 项的 encrypted_content 字段透传给客户端。codex 等客户端会原样
// 回传该项，请求侧据此还原签名并重建上游需要的思考块——这是 Claude 扩展思考 + 工具调用
// 多轮（Anthropic 强制回传 thinking 块）与 Gemini 3 函数调用（强制回传 thoughtSignature）
// 能在 Responses 协议下闭环的唯一载体。
type chatReasoningSeg struct {
	id        string
	index     int
	text      string
	signature string
}

// chatToolCallState 流式聚合的工具调用；tools slice 按登记顺序是 done 事件与
// finalOutput 的唯一遍历源（替代 legacy 的 map 遍历，保证顺序确定）。
type chatToolCallState struct {
	id    string
	name  string
	args  strings.Builder
	index int  // 登记时分配的 output_index
	done  bool // 已发 function_call_arguments.done + output_item.done（去重）
}

// ConvertStreamResponse 读取 chat SSE reader，经 chunkWriter 输出 *dto.ResponsesStreamEvent。
func (c *OpenAIChatToResponsesStreamConverter) ConvertStreamResponse(
	ctx context.Context, info convmeta.Meta, reader io.Reader, chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}
	respID := fmt.Sprintf("resp_%d", NowFunc().UnixNano())
	msgID := fmt.Sprintf("msg_%d", NowFunc().UnixNano())
	createdAt := int(NowFunc().Unix())

	var usage dto.UsageWithDetails
	var contentBuilder strings.Builder
	sentCreated := false
	sentTextDone := false
	parsedChunks := 0
	sawChoices := false
	outputIndex := 0
	contentIndex := 0
	msgAdded := false // message 项惰性开启：首个文本增量时才发 output_item.added
	msgIndex := 0
	tools := make([]*chatToolCallState, 0)
	toolByID := make(map[string]*chatToolCallState)
	// OpenAI 流式中后续参数 chunk 的 ID 为空只有 index，靠 index 反查
	toolIDByUpstreamIndex := make(map[int]string)
	// 思考段状态：当前开启的 reasoning 项 + 已收尾的段（工具调用打断后再来思考增量会开新段）
	rsOpen := false
	rsSeq := 0
	rsID := ""
	rsIndex := 0
	var rsBuilder strings.Builder
	rsSignature := "" // 当前思考段的上游签名（Gemini thoughtSignature 经 chat 中间格式透传）
	reasoningSegs := make([]chatReasoningSeg, 0)
	echo := responsesEchoOf(info)

	// emit 输出一个 Responses SSE 事件（chunkWriter 出错即客户端写失败，终止转换）
	emit := func(eventType string, payload map[string]any) error {
		payload["type"] = eventType
		return chunkWriter(&dto.ResponsesStreamEvent{Type: eventType, Data: payload})
	}

	// ensureMessageItem 惰性开启 message 项（output_item.added + content_part.added）
	ensureMessageItem := func() error {
		if msgAdded {
			return nil
		}
		msgIndex = outputIndex
		outputIndex++
		msgAdded = true
		if err := emit("response.output_item.added", map[string]any{
			"output_index": msgIndex,
			"item": map[string]any{
				"type":    "message",
				"id":      msgID,
				"status":  "in_progress",
				"role":    "assistant",
				"content": []any{},
			},
		}); err != nil {
			return err
		}
		return emit("response.content_part.added", map[string]any{
			"item_id":       msgID,
			"output_index":  msgIndex,
			"content_index": contentIndex,
			"part": map[string]any{
				"type":        "output_text",
				"text":        "",
				"annotations": []any{},
			},
		})
	}

	// openReasoning 开启一个 reasoning 项（首个思考增量时调用；被打断后新增量开新段）
	openReasoning := func() error {
		if rsOpen {
			return nil
		}
		rsSeq++
		rsID = fmt.Sprintf("rs_%s", strings.TrimPrefix(msgID, "msg_"))
		if rsSeq > 1 {
			rsID = fmt.Sprintf("%s_%d", rsID, rsSeq)
		}
		rsIndex = outputIndex
		outputIndex++
		rsBuilder.Reset()
		rsOpen = true
		return emit("response.output_item.added", map[string]any{
			"output_index": rsIndex,
			"item": map[string]any{
				"type":    "reasoning",
				"id":      rsID,
				"summary": []any{}, // codex 等严格客户端的 Reasoning.summary 为必填键
			},
		})
	}

	// closeReasoning 收尾当前 reasoning 项（文本/工具开始或 finish 时调用）
	closeReasoning := func() error {
		if !rsOpen {
			return nil
		}
		finishedText := rsBuilder.String()
		if err := emit("response.reasoning_summary_text.done", map[string]any{
			"item_id":       rsID,
			"output_index":  rsIndex,
			"summary_index": 0,
			"text":          finishedText,
		}); err != nil {
			return err
		}
		doneItem := map[string]any{
			"type": "reasoning",
			"id":   rsID,
			"summary": []map[string]any{{
				"type": "summary_text",
				"text": finishedText,
			}},
		}
		// 上游思考签名经 encrypted_content 透传（Gemini 3 函数调用要求回传 thoughtSignature，
		// 缺失被上游 400）。codex 等客户端原样回传 reasoning 项，请求侧 r2o 据此还原
		if rsSignature != "" {
			doneItem["encrypted_content"] = rsSignature
		}
		if err := emit("response.output_item.done", map[string]any{
			"output_index": rsIndex,
			"item":         doneItem,
		}); err != nil {
			return err
		}
		reasoningSegs = append(reasoningSegs, chatReasoningSeg{
			id: rsID, index: rsIndex, text: finishedText, signature: rsSignature,
		})
		rsOpen = false
		rsSignature = ""
		return nil
	}

	// closeTextPart 关闭文本 content part（进入工具调用或 finish 时调用）
	closeTextPart := func() error {
		if sentTextDone {
			return nil
		}
		// message 项从未开启且已有其他输出（思考/工具）：无需合成空消息项
		if !msgAdded && (len(reasoningSegs) > 0 || rsOpen || len(tools) > 0) {
			sentTextDone = true
			return nil
		}
		if err := ensureMessageItem(); err != nil {
			return err
		}
		finishedText := contentBuilder.String()
		if err := emit("response.output_text.done", map[string]any{
			"item_id":       msgID,
			"output_index":  msgIndex,
			"content_index": contentIndex,
			"text":          finishedText,
		}); err != nil {
			return err
		}
		if err := emit("response.content_part.done", map[string]any{
			"item_id":       msgID,
			"output_index":  msgIndex,
			"content_index": contentIndex,
			"part": map[string]any{
				"type":        "output_text",
				"text":        finishedText,
				"annotations": []any{},
			},
		}); err != nil {
			return err
		}
		if err := emit("response.output_item.done", map[string]any{
			"output_index": msgIndex,
			"item": map[string]any{
				"type":   "message",
				"id":     msgID,
				"status": "completed",
				"role":   "assistant",
				"content": []map[string]any{{
					"type":        "output_text",
					"text":        finishedText,
					"annotations": []any{},
				}},
			},
		}); err != nil {
			return err
		}
		sentTextDone = true
		return nil
	}

	// finishTools 按登记顺序为每个未收尾的工具发 done 事件（done 标志兼作去重）。
	// 事件类型按请求侧 stash 的原始工具类型分派：function 走
	// function_call_arguments.done，custom 走 custom_tool_call_input.done
	// （解包 freeform 字符串），local_shell/apply_patch 无参数收尾事件（仅 output_item.done）。
	finishTools := func() error {
		for _, tool := range tools {
			if tool.done {
				continue
			}
			if evType, payloadKey, ok := toolCallArgsDoneEvent(info, tool.name); ok {
				payloadVal := tool.args.String()
				if payloadKey == "input" {
					payloadVal = unwrapCustomToolInput(payloadVal)
				}
				if err := emit(evType, map[string]any{
					"item_id":      tool.id,
					"output_index": tool.index,
					payloadKey:     payloadVal,
				}); err != nil {
					return err
				}
			}
			if err := emit("response.output_item.done", map[string]any{
				"output_index": tool.index,
				"item":         toolCallDoneItemPayload(info, tool.id, tool.name, tool.args.String()),
			}); err != nil {
				return err
			}
			tool.done = true
		}
		return nil
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
		if data == "[DONE]" {
			break
		}

		// SSE 流中内嵌的上游错误对象：部分聚合商出错时返回 HTTP 200 + SSE，
		// 错误信息夹在 data 行里。不识别会合成空的 response.completed（客户端"成功但无内容"）
		if errBody, ok := extractStreamEmbeddedError([]byte(data)); ok {
			return &types.EmbeddedUpstreamError{Body: errBody}
		}

		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		parsedChunks++
		if len(chunk.Choices) > 0 {
			sawChoices = true
		}

		// 第一个 chunk：发送 response.created（message 项惰性开启，见 ensureMessageItem——
		// 思考模型先到 reasoning 增量，reasoning 项先于 message 项，对齐真实 OpenAI 项序）
		if !sentCreated {
			if chunk.ID != "" {
				respID = fmt.Sprintf("resp_%s", chunk.ID)
				msgID = fmt.Sprintf("msg_%s", chunk.ID)
			}
			if chunk.Created > 0 {
				createdAt = int(chunk.Created)
			}
			if chunk.Model != "" && !modelMappedOf(info) {
				modelName = chunk.Model
			}

			if err := emit("response.created", map[string]any{
				"response": buildResponsesObject(respID, createdAt, "in_progress", modelName, []dto.ResponsesOutput{}, nil, nil, echo),
			}); err != nil {
				return err
			}
			sentCreated = true
		}

		// 提取 usage（legacy 口径：仅取三项数值，chunk 内的 usage details 丢弃）
		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
			usage.TotalTokens = chunk.Usage.TotalTokens
		}

		// 处理 choices delta
		for _, choice := range chunk.Choices {
			// n>1 的多 choice 流在 Responses 单输出流无对应物，交错输出会损坏事件流——只处理首个 choice
			if choice.Index > 0 {
				continue
			}
			// 推理内容：产出独立 reasoning 项（先于文本，对齐真实 OpenAI 项序），
			// 收尾后进 completed 的 output——codex 等客户端据此在后续轮次回传思考内容
			// 消息级 thoughtSignature（g2o 从 Gemini thought part 捕获）：附着到当前思考段
			if choice.Delta.ThoughtSignature != "" {
				if err := openReasoning(); err != nil {
					return err
				}
				rsSignature = choice.Delta.ThoughtSignature
			}
			if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
				if err := openReasoning(); err != nil {
					return err
				}
				rsBuilder.WriteString(*choice.Delta.ReasoningContent)
				if err := emit("response.reasoning_summary_text.delta", map[string]any{
					"item_id":       rsID,
					"output_index":  rsIndex,
					"summary_index": 0,
					"delta":         *choice.Delta.ReasoningContent,
				}); err != nil {
					return err
				}
			}

			// 文本内容 delta（仅 string 形态，非 string 静默丢弃——legacy 口径）：
			// 答案开始即思考段结束，先收尾 reasoning 再惰性开启 message 项
			if choice.Delta.Content != nil {
				if deltaText, ok := choice.Delta.Content.(string); ok && deltaText != "" {
					if err := closeReasoning(); err != nil {
						return err
					}
					if err := ensureMessageItem(); err != nil {
						return err
					}
					contentBuilder.WriteString(deltaText)
					if err := emit("response.output_text.delta", map[string]any{
						"item_id":       msgID,
						"output_index":  msgIndex,
						"content_index": contentIndex,
						"delta":         deltaText,
					}); err != nil {
						return err
					}
				}
			}

			// 工具调用增量
			for _, tc := range choice.Delta.ToolCalls {
				callID := tc.ID

				// 新 tool call：有 ID 和 name
				if callID != "" && tc.Function.Name != "" {
					toolIDByUpstreamIndex[tc.Index] = callID
					// 先收尾思考段与文本 content part
					if err := closeReasoning(); err != nil {
						return err
					}
					if err := closeTextPart(); err != nil {
						return err
					}
					tool := &chatToolCallState{id: callID, name: tc.Function.Name, index: outputIndex}
					tools = append(tools, tool)
					toolByID[callID] = tool

					if err := emit("response.output_item.added", map[string]any{
						"output_index": outputIndex,
						"item":         buildToolCallAddedItem(info, callID, tc.Function.Name),
					}); err != nil {
						return err
					}
					outputIndex++
				}

				// 参数 chunk：ID 可能为空，通过 index 查找对应的 callID
				if callID == "" {
					callID = toolIDByUpstreamIndex[tc.Index]
				}
				if callID == "" {
					continue
				}

				if tc.Function.Arguments != "" {
					// legacy 怪癖保持：未登记的 callID（仅 ID 无 name 的 chunk）参数不进入
					// done 事件与 completed output，仅透传 delta（output_index 取零值）
					toolIdx := 0
					toolName := ""
					if tool, ok := toolByID[callID]; ok {
						tool.args.WriteString(tc.Function.Arguments)
						toolIdx = tool.index
						toolName = tool.name
					}
					// 非 function 工具（custom/local_shell/apply_patch）的 arguments 为 JSON
					// 包装形态，抑制增量透出，缓冲至收尾事件一次性给出
					if deltaEvent, ok := toolCallArgsDeltaEvent(info, toolName); ok {
						if err := emit(deltaEvent, map[string]any{
							"item_id":      callID,
							"output_index": toolIdx,
							"delta":        tc.Function.Arguments,
						}); err != nil {
							return err
						}
					}
				}
			}

			// finish_reason：收尾思考段 + 关闭文本 part + 按登记顺序收尾全部工具
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				if err := closeReasoning(); err != nil {
					return err
				}
				if err := closeTextPart(); err != nil {
					return err
				}
				if err := finishTools(); err != nil {
					return err
				}
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 未到 finish_reason 即断流：思考段仍需收尾进 completed（文本 part 保持 legacy 口径不补 done）
	if err := closeReasoning(); err != nil {
		return err
	}

	// 假成功防护：上游流始终不是 chat 格式（无 choices 且无内容/工具/usage）时不再
	// 静默合成空响应——报错驱动重试换渠道/健康上报
	if !sawChoices && contentBuilder.Len() == 0 && len(tools) == 0 && usage.TotalTokens == 0 {
		return fmt.Errorf("%w: %d chunks parsed, none contained choices", types.ErrProtocolMismatch, parsedChunks)
	}

	// 客户端可见 usage 兜底（legacy 客户端可见口径：正常结束路径为 len/4 估算）
	if usage.CompletionTokens == 0 && contentBuilder.Len() > 0 {
		usage.CompletionTokens = contentBuilder.Len() / 4
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	// 构建 completed 的 output 数组（思考段 + 文本消息 + 工具，按 output_index 排序）
	type indexedOutput struct {
		index int
		out   dto.ResponsesOutput
	}
	items := make([]indexedOutput, 0, 1+len(reasoningSegs)+len(tools))
	// message 项保留口径与 legacy 一致：有文本、或未正常收尾（断流）；从未开启且已有
	// 其他输出时不合成空消息
	includeMsg := contentBuilder.Len() > 0 || !sentTextDone
	if includeMsg && !msgAdded && (len(reasoningSegs) > 0 || len(tools) > 0) {
		includeMsg = false
	}
	if includeMsg {
		items = append(items, indexedOutput{index: msgIndex, out: dto.ResponsesOutput{
			Type:   "message",
			ID:     msgID,
			Status: "completed",
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{{
				Type:        "output_text",
				Text:        contentBuilder.String(),
				Annotations: []dto.ResponsesAnnotation{},
			}},
		}})
	}
	for _, seg := range reasoningSegs {
		items = append(items, indexedOutput{index: seg.index, out: dto.ResponsesOutput{
			Type: "reasoning",
			ID:   seg.id,
			Summary: []dto.ResponsesSummaryPart{{
				Type: "summary_text",
				Text: seg.text,
			}},
			EncryptedContent: seg.signature,
		}})
	}
	for _, tool := range tools {
		items = append(items, indexedOutput{index: tool.index,
			out: buildToolCallDoneItem(info, tool.id, tool.name, tool.args.String())})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].index < items[j].index })
	finalOutput := make([]dto.ResponsesOutput, 0, len(items))
	for _, it := range items {
		finalOutput = append(finalOutput, it.out)
	}

	completedAt := int(NowFunc().Unix())
	if completedAt < createdAt {
		completedAt = createdAt
	}
	return emit("response.completed", map[string]any{
		"response": buildResponsesObject(respID, createdAt, "completed", modelName, finalOutput,
			responsesUsageOf(&usage), &completedAt, echo),
	})
}

// extractStreamEmbeddedError 检测 SSE data 行中内嵌的上游错误对象（存在 "error" 键且值非 null）。
// 部分供应商的正常 chunk 会携带空 error 字段，需排除 "error":null。
func extractStreamEmbeddedError(data []byte) (json.RawMessage, bool) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false
	}
	errBody, ok := raw["error"]
	if !ok {
		return nil, false
	}
	trimmed := string(bytes.TrimSpace(errBody))
	if trimmed == "" || trimmed == "null" {
		return nil, false
	}
	return errBody, true
}

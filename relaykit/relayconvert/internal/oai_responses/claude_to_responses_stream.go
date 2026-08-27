package oai_responses

import (
	"bufio"
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

// ClaudeToResponsesStreamConverter Claude 上游 SSE → Responses 客户端 SSE（流式响应侧）。
// 移植自宿主 relay/channel/claude/responses_bridge.go 的 handleStreamToResponses 状态机：
// 文本/思考增量、工具调用聚合（claudeToolCallState）、事件收尾顺序与
// chat→responses 桥接保持一致。宿主职责（SSE 头/保活/错误体写出/计费）不在本转换器内。
type ClaudeToResponsesStreamConverter struct{}

// ConvertStreamResponse 读取 Claude SSE reader，经 chunkWriter 输出 *dto.ResponsesStreamEvent。
// 宿主据此写出 `event: <type>\ndata: <json>` 格式的事件流。
func (c *ClaudeToResponsesStreamConverter) ConvertStreamResponse(
	ctx context.Context, info convmeta.Meta, reader io.Reader, chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	respID := fmt.Sprintf("resp_%d", NowFunc().UnixNano())
	msgID := fmt.Sprintf("msg_%d", NowFunc().UnixNano())
	createdAt := int(NowFunc().Unix())
	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

	var usage dto.ClaudeUsage
	var textBuf strings.Builder
	sentCreated := false
	sentTextDone := false
	sentCompleted := false
	outputIndex := 0
	contentIndex := 0
	msgAdded := false // message 项惰性开启：首个文本增量时才发 output_item.added
	msgIndex := 0
	toolCalls := make([]*claudeToolCallState, 0) // 有序聚合，completed 的 output 数组按此顺序
	toolIndexByID := make(map[string]int)        // callID → output_index
	var currentTool *claudeToolCallState         // 正在接收参数增量的工具调用
	// Claude 托管 web_search（server_tool_use/web_search_tool_result 块）→ web_search_call 项
	webSearches := make([]*claudeWebSearchState, 0)
	webSearchByID := make(map[string]*claudeWebSearchState)
	var currentWebSearch *claudeWebSearchState // 正在接收 query JSON 增量的搜索调用
	// 思考段状态：thinking 块产出独立 reasoning 项（工具调用打断后新 thinking 增量开新段）
	rsOpen := false
	rsSeq := 0
	rsID := ""
	rsIndex := 0
	var rsBuilder strings.Builder
	rsSignature := "" // 当前思考段的 Claude thinking 签名（signature_delta 累积）
	reasoningSegs := make([]chatReasoningSeg, 0)
	echo := responsesEchoOf(info)
	parsedChunks := 0       // 成功解析的 data 行数（假成功防护的诊断信息）
	sawClaudeEvent := false // 是否出现过已知 Claude 事件类型 —— Claude 流的协议特征

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

	// openReasoning 开启一个 reasoning 项（首个 thinking 增量时调用）
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
		rsSignature = ""
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

	// closeReasoning 收尾当前 reasoning 项（文本/工具开始或流结束时调用）
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
		// thinking 签名经 encrypted_content 透传给客户端：Anthropic 要求带 tool_use 的
		// assistant 轮回传原始 thinking 块（含 signature），否则下一轮 400
		// "Expected `thinking` or `redacted_thinking`, but found `tool_use`"。
		// codex 等客户端原样回传 reasoning 项，请求侧 r2o 据此还原
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
		return nil
	}

	// closeTextPart 关闭文本 content part（进入工具调用或流结束时调用）
	closeTextPart := func() error {
		if sentTextDone {
			return nil
		}
		// message 项从未开启且已有其他输出（思考/工具）：无需合成空消息项
		if !msgAdded && (len(reasoningSegs) > 0 || rsOpen || len(toolCalls) > 0) {
			sentTextDone = true
			return nil
		}
		if err := ensureMessageItem(); err != nil {
			return err
		}
		finishedText := textBuf.String()
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

	// finish 发送每个工具调用的收尾事件 + response.completed。
	// 事件类型按请求侧 stash 的原始工具类型分派（function→function_call_arguments.done，
	// custom→custom_tool_call_input.done，local_shell/apply_patch 仅 output_item.done）。
	finish := func() error {
		if sentCompleted {
			return nil
		}
		if err := closeReasoning(); err != nil {
			return err
		}
		if err := closeTextPart(); err != nil {
			return err
		}

		for _, tc := range toolCalls {
			if evType, payloadKey, ok := toolCallArgsDoneEvent(info, tc.name); ok {
				payloadVal := tc.args.String()
				if payloadKey == "input" {
					payloadVal = unwrapCustomToolInput(payloadVal)
				}
				if err := emit(evType, map[string]any{
					"item_id":      tc.id,
					"output_index": toolIndexByID[tc.id],
					payloadKey:     payloadVal,
				}); err != nil {
					return err
				}
			}
			if err := emit("response.output_item.done", map[string]any{
				"output_index": toolIndexByID[tc.id],
				"item":         toolCallDoneItemPayload(info, tc.id, tc.name, tc.args.String()),
			}); err != nil {
				return err
			}
		}

		// 托管搜索收尾：web_search_call 无参数增量事件，done 项携带完整 action
		// （query 由 input_json_delta 累积还原，sources 来自 web_search_tool_result 块）
		for _, ws := range webSearches {
			var action map[string]any
			_ = json.Unmarshal(webSearchAction(webSearchQueryOf(ws.query.String()), ws.sources), &action)
			if err := emit("response.output_item.done", map[string]any{
				"output_index": ws.index,
				"item": map[string]any{
					"type":   "web_search_call",
					"id":     ws.id,
					"status": "completed",
					"action": action,
				},
			}); err != nil {
				return err
			}
		}

		// 输出数组：思考段 + 文本消息（有文本才保留）+ 工具，按 output_index 排序
		type indexedOutput struct {
			index int
			out   dto.ResponsesOutput
		}
		items := make([]indexedOutput, 0, 1+len(reasoningSegs)+len(toolCalls))
		if textBuf.Len() > 0 {
			items = append(items, indexedOutput{index: msgIndex, out: dto.ResponsesOutput{
				Type:   "message",
				ID:     msgID,
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{{
					Type:        "output_text",
					Text:        textBuf.String(),
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
		for _, tc := range toolCalls {
			items = append(items, indexedOutput{index: toolIndexByID[tc.id],
				out: buildToolCallDoneItem(info, tc.id, tc.name, tc.args.String())})
		}
		for _, ws := range webSearches {
			items = append(items, indexedOutput{index: ws.index, out: dto.ResponsesOutput{
				Type:   "web_search_call",
				ID:     ws.id,
				Status: "completed",
				Action: webSearchAction(webSearchQueryOf(ws.query.String()), ws.sources),
			}})
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].index < items[j].index })
		finalOutput := make([]dto.ResponsesOutput, 0, len(items))
		for _, it := range items {
			finalOutput = append(finalOutput, it.out)
		}

		// 客户端可见 usage 用 OpenAI 语义（input 含缓存，cached 为其子集）
		promptTotal := usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
		visibleUsage := &dto.UsageWithDetails{
			PromptTokens:     promptTotal,
			CompletionTokens: usage.OutputTokens,
			TotalTokens:      promptTotal + usage.OutputTokens,
		}
		if usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 {
			visibleUsage.PromptTokensDetails = claudeUsageToTokenDetails(&usage)
		}
		completedAt := int(NowFunc().Unix())
		return emit("response.completed", map[string]any{
			"response": buildResponsesObject(respID, createdAt, "completed", modelName, finalOutput,
				responsesUsageOf(visibleUsage), &completedAt, echo),
		})
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := extractSSEData(line)
		if data == "" || data == "[DONE]" {
			continue
		}

		var event dto.ClaudeResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		parsedChunks++
		switch event.Type {
		case "message_start", "content_block_start", "content_block_delta",
			"content_block_stop", "message_delta", "message_stop", "error", "ping":
			sawClaudeEvent = true
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				//（旧路径仅在未做模型映射时采用上游模型名，convmeta 无该信号，
				// 列为已知差异：映射渠道时采用上游返回的模型名）
				if event.Message.Model != "" {
					modelName = event.Message.Model
				}
				if event.Message.Usage != nil {
					usage = *event.Message.Usage
				}
			}
			if sentCreated {
				continue
			}
			// message 项惰性开启（见 ensureMessageItem）：thinking 块先到时 reasoning 项
			// 先于 message 项，对齐真实 OpenAI 项序
			if err := emit("response.created", map[string]any{
				"response": buildResponsesObject(respID, createdAt, "in_progress", modelName, []dto.ResponsesOutput{}, nil, nil, echo),
			}); err != nil {
				return err
			}
			sentCreated = true

		case "content_block_start":
			if event.ContentBlock == nil {
				continue
			}
			switch event.ContentBlock.Type {
			case "tool_use":
				// 先收尾思考段与文本 content part，再开 function_call 项
				if err := closeReasoning(); err != nil {
					return err
				}
				if err := closeTextPart(); err != nil {
					return err
				}
				tc := &claudeToolCallState{id: event.ContentBlock.ID, name: event.ContentBlock.Name}
				toolCalls = append(toolCalls, tc)
				toolIndexByID[tc.id] = outputIndex
				currentTool = tc
				if err := emit("response.output_item.added", map[string]any{
					"output_index": outputIndex,
					"item":         buildToolCallAddedItem(info, tc.id, tc.name),
				}); err != nil {
					return err
				}
				outputIndex++
			case "server_tool_use":
				// Claude 托管搜索：开 web_search_call 项（query 由后续 input_json_delta 累积）
				if event.ContentBlock.Name != "web_search" {
					continue
				}
				if err := closeReasoning(); err != nil {
					return err
				}
				if err := closeTextPart(); err != nil {
					return err
				}
				ws := &claudeWebSearchState{id: event.ContentBlock.ID, index: outputIndex}
				webSearches = append(webSearches, ws)
				webSearchByID[ws.id] = ws
				currentWebSearch = ws
				if err := emit("response.output_item.added", map[string]any{
					"output_index": outputIndex,
					"item": map[string]any{
						"type":   "web_search_call",
						"id":     ws.id,
						"status": "in_progress",
						"action": map[string]any{"type": "search", "query": ""},
					},
				}); err != nil {
					return err
				}
				outputIndex++
			case "web_search_tool_result":
				// 搜索结果块（完整到达）：sources 暂存到对应搜索调用，done 时随 action 产出
				if ws, ok := webSearchByID[event.ContentBlock.ToolUseID]; ok {
					ws.sources = claudeWebSearchSources(event.ContentBlock.Content)
				}
			case "text", "thinking", "redacted_thinking":
				// 文本/思考块：均由增量驱动（text_delta 惰性开启 message 项，
				// thinking_delta 开启独立 reasoning 项）
			}

		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text != nil && *event.Delta.Text != "" {
					// 答案开始即思考段结束，先收尾 reasoning 再惰性开启 message 项
					if err := closeReasoning(); err != nil {
						return err
					}
					if err := ensureMessageItem(); err != nil {
						return err
					}
					textBuf.WriteString(*event.Delta.Text)
					if err := emit("response.output_text.delta", map[string]any{
						"item_id":       msgID,
						"output_index":  msgIndex,
						"content_index": contentIndex,
						"delta":         *event.Delta.Text,
					}); err != nil {
						return err
					}
				}
			case "thinking_delta":
				// 思考增量：产出独立 reasoning 项，收尾后进 completed 的 output——
				// codex 等客户端据此在后续轮次回传思考内容
				if event.Delta.Thinking != nil && *event.Delta.Thinking != "" {
					if err := openReasoning(); err != nil {
						return err
					}
					rsBuilder.WriteString(*event.Delta.Thinking)
					if err := emit("response.reasoning_summary_text.delta", map[string]any{
						"item_id":       rsID,
						"output_index":  rsIndex,
						"summary_index": 0,
						"delta":         *event.Delta.Thinking,
					}); err != nil {
						return err
					}
				}
			case "input_json_delta":
				if event.Delta.PartialJSON == nil || *event.Delta.PartialJSON == "" {
					continue
				}
				// web_search 的 query JSON 增量：缓冲（web_search_call 无参数增量事件，
				// done 时随 action 一次性产出）
				if currentWebSearch != nil {
					currentWebSearch.query.WriteString(*event.Delta.PartialJSON)
					continue
				}
				if currentTool != nil {
					currentTool.args.WriteString(*event.Delta.PartialJSON)
					// 非 function 工具（custom/local_shell/apply_patch）的 arguments 为 JSON
					// 包装形态，抑制增量透出，缓冲至收尾事件一次性给出
					if deltaEvent, ok := toolCallArgsDeltaEvent(info, currentTool.name); ok {
						if err := emit(deltaEvent, map[string]any{
							"item_id":      currentTool.id,
							"output_index": toolIndexByID[currentTool.id],
							"delta":        *event.Delta.PartialJSON,
						}); err != nil {
							return err
						}
					}
				}
			case "signature_delta":
				// 思考签名 → 当前 reasoning 段（收尾时写入 encrypted_content）。
				// 签名可能先于 thinking_delta 到达，必要时先开段承载
				if event.Delta.Signature != "" {
					if err := openReasoning(); err != nil {
						return err
					}
					rsSignature += event.Delta.Signature
				}
			}

		case "content_block_stop":
			// 块级收尾统一延迟到 message_stop / finish，这里仅结束当前工具/搜索块的增量定向
			if currentTool != nil {
				currentTool = nil
			}
			if currentWebSearch != nil {
				currentWebSearch = nil
			}

		case "message_delta":
			if event.Usage != nil {
				if event.Usage.InputTokens > 0 {
					usage.InputTokens = event.Usage.InputTokens
				}
				usage.OutputTokens = event.Usage.OutputTokens
				if event.Usage.CacheReadInputTokens > 0 {
					usage.CacheReadInputTokens = event.Usage.CacheReadInputTokens
				}
				if event.Usage.CacheCreationInputTokens > 0 {
					usage.CacheCreationInputTokens = event.Usage.CacheCreationInputTokens
				}
				if event.Usage.CacheCreation != nil {
					usage.CacheCreation = event.Usage.CacheCreation
				}
			}

		case "message_stop":
			if err := finish(); err != nil {
				return err
			}
			sentCompleted = true
			return nil

		case "error":
			errMsg := "claude stream error"
			if event.Error != nil {
				if b, err := json.Marshal(event.Error); err == nil {
					errMsg = fmt.Sprintf("claude stream error: %s", string(b))
				}
			}
			return fmt.Errorf("%s", errMsg)

		case "ping":
			// 保活事件，忽略
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 假成功防护：整段上游流不是 Claude SSE（零已知事件类型）——报 ErrProtocolMismatch，
	// 由宿主桥接层按上游错误处理。静默合成 completed 会让 codex 收到空响应且健康度记为成功。
	if !sawClaudeEvent {
		return fmt.Errorf("%w: %d chunks parsed, none was a Claude stream event", types.ErrProtocolMismatch, parsedChunks)
	}

	// 上游未发 message_stop 即断流：仍合成 completed，避免客户端挂起
	if err := finish(); err != nil {
		return err
	}
	return nil
}

// claudeToolCallState 流式中聚合的工具调用（Claude tool_use 块）
type claudeToolCallState struct {
	id   string
	name string
	args strings.Builder
}

// claudeWebSearchState 流式中聚合的托管搜索调用（server_tool_use 块的 query JSON 增量 +
// web_search_tool_result 块的 sources）；index 为 output_index（completed output 排序依据）
type claudeWebSearchState struct {
	id      string
	index   int
	query   strings.Builder
	sources []dto.ResponsesWebSearchSource
}

// extractSSEData 从 SSE data 行提取数据部分（与宿主 helper.ExtractSSEData 等价的纯实现）。
func extractSSEData(line string) string {
	data := strings.TrimPrefix(line, "data:")
	return strings.TrimSpace(data)
}

// Package oai_responses 实现 OpenAI Responses API ↔ Chat Completions 双向协议转换器。
//
// 从宿主 relay/channel/openai 的 converter.go / responses.go 移植纯转换逻辑：
//   - 请求：Responses → chat（ResponsesToOpenAIRequestConverter）、
//     chat → Responses（OpenAIToResponsesRequestConverter）
//   - 响应：Responses 上游 → chat 客户端（非流式 + 流式）、
//     chat 上游 → Responses 客户端（非流式 + 流式）
//
// 宿主侧职责（HTTP 写出、错误透传、SSE 帧化、[DONE]、流中断计费兜底、响应路由记录）
// 不在本包，由宿主桥接层处理。
package oai_responses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// ResponsesToOpenAIRequestConverter 将 Responses API 请求转换为 Chat Completions 请求。
type ResponsesToOpenAIRequestConverter struct{}

func (c *ResponsesToOpenAIRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIResponsesToOpenAIChat
}

func (c *ResponsesToOpenAIRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *ResponsesToOpenAIRequestConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *ResponsesToOpenAIRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityFair
}

// ConvertRequest 将 Responses API 请求转换为 Chat Completions 格式。
func (c *ResponsesToOpenAIRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	req, ok := request.(*dto.OpenAIResponsesRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.OpenAIResponsesRequest, got %T", request)
	}

	// 有状态请求快速失败：previous_response_id 的会话历史存储在上游 Responses 服务侧，
	// chat-only 渠道无法还原（网关不存储响应体），降级转换会静默丢失全部上下文。
	// 返回哨兵错误，由宿主按渠道级致命上报调度 FSM 换渠道。
	if req.PreviousResponseID != "" {
		return nil, fmt.Errorf("stateful responses (previous_response_id) not supported by chat-only channels: %w", relayconvert.ErrStatefulResponsesUnsupported)
	}

	// stash 请求快照，供上游 chat 响应合成回 Responses 格式时 echo 请求参数
	// （能力接口：宿主未实现时静默跳过）
	if stash, ok := info.(convmeta.ResponsesStash); ok {
		stash.StashResponsesRequest(req)
	}

	chatReq := &dto.GeneralOpenAIRequest{Model: req.Model}
	// 模型名：优先映射后的上游模型名，否则用客户端请求模型名
	if upstream := convmeta.UpstreamModelName(info); upstream != "" {
		chatReq.Model = upstream
	}

	messages := make([]dto.Message, 0)
	if len(req.Instructions) > 0 {
		var instructions string
		if err := json.Unmarshal(req.Instructions, &instructions); err == nil && instructions != "" {
			messages = append(messages, dto.Message{Role: "system", Content: instructions})
		}
	}
	inputMessages, err := r2cConvertInputToMessages(req.Input)
	if err != nil {
		return nil, fmt.Errorf("convert input to messages: %w", err)
	}
	messages = append(messages, inputMessages...)
	chatReq.Messages = messages

	if req.Stream != nil {
		chatReq.Stream = req.Stream
		if *req.Stream {
			chatReq.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
		}
	}
	chatReq.Temperature = req.Temperature
	chatReq.TopP = req.TopP
	if req.MaxOutputTokens != nil {
		maxTokens := int(*req.MaxOutputTokens)
		chatReq.MaxTokens = &maxTokens
	}
	if req.Logprobs != nil {
		logprobs := true
		chatReq.LogProbs = &logprobs
		chatReq.TopLogProbs = req.Logprobs
	} else if req.TopLogProbs != nil {
		logprobs := true
		chatReq.TopLogProbs = req.TopLogProbs
		chatReq.LogProbs = &logprobs
	}
	if len(req.Tools) > 0 {
		if chatTools := r2cConvertTools(req.Tools); len(chatTools) > 0 {
			chatReq.Tools = chatTools
		}

		// 服务端联网搜索：Responses 的 web_search 工具 → chat 的 web_search_options。
		// chat 是跨原生方向的转换中枢（Responses→OpenAI→Claude/Gemini 两跳链），
		// 这里不承载就等于整条链上的搜索能力全部丢失。
		//
		// 注意：web_search_options 同时是 Responses→Claude 链的**中间载体**（第二跳
		// OpenAI→Claude 据此还原 web_search 工具），此处不得按模型名做任何裁剪；
		// 「chat 终端 + claude 系模型」的剥离在 openai 适配器出口做（aggregator 场景，
		// 聚合器会把它映射成无法表达的原生搜索工具，导致空响应）。
		if spec := shared.DetectWebSearchFromResponsesTools(req.Tools); spec != nil {
			chatReq.WebSearchOptions = spec.ToOpenAIOptions()
		}
	}
	if len(req.ToolChoice) > 0 {
		chatReq.ToolChoice = r2cConvertToolChoice(req.ToolChoice)
	}
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	}
	if req.ServiceTier != "" {
		chatReq.ServiceTier = req.ServiceTier
	}
	if req.PromptCacheKey != "" {
		chatReq.PromptCacheKey = req.PromptCacheKey
	}
	if len(req.Text) > 0 {
		if rf := r2cParseTextFormat(req.Text); rf != nil {
			chatReq.ResponseFormat = rf
		}
	}
	if len(req.FrequencyPenalty) > 0 {
		var v float64
		if err := json.Unmarshal(req.FrequencyPenalty, &v); err == nil {
			chatReq.FrequencyPenalty = &v
		}
	}
	if len(req.PresencePenalty) > 0 {
		var v float64
		if err := json.Unmarshal(req.PresencePenalty, &v); err == nil {
			chatReq.PresencePenalty = &v
		}
	}
	if len(req.Metadata) > 0 {
		chatReq.Metadata = req.Metadata
	}

	return chatReq, nil
}

type r2cInputItem struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	CallID  string          `json:"call_id,omitempty"`
	Output  string          `json:"output,omitempty"`
	Text    string          `json:"text,omitempty"`
	// function_call 项字段（Responses 历史中的助手工具调用）
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	// reasoning 项的思考文本（OpenAI 官方形态在 summary[]，部分聚合器直连输出放 content[]）
	Summary []r2cTextPart `json:"summary,omitempty"`
}

// r2cTextPart Responses 输出项中文本部件的最小读取形态。
type r2cTextPart struct {
	Text string `json:"text"`
}

// r2cReasoningText 提取 reasoning 项的思考文本，summary 与 content 两种形态按序拼接。
func r2cReasoningText(item r2cInputItem) string {
	var parts []string
	for _, s := range item.Summary {
		if s.Text != "" {
			parts = append(parts, s.Text)
		}
	}
	if len(item.Content) > 0 {
		var contentParts []r2cTextPart
		if err := json.Unmarshal(item.Content, &contentParts); err == nil {
			for _, c := range contentParts {
				if c.Text != "" {
					parts = append(parts, c.Text)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

type r2cContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	URL      string `json:"url,omitempty"`
	Detail   string `json:"detail,omitempty"`
	// input_audio：Responses 为 {"type":"input_audio","input_audio":{"data","format"}}，
	// chat 同形，原样透传
	InputAudio *r2cInputAudio `json:"input_audio,omitempty"`
	// input_file：Responses 为扁平 {"type":"input_file","file_data","filename"}，
	// 转换为 chat 的 {"type":"file","file":{"file_data","filename"}}
	FileData string `json:"file_data,omitempty"`
	Filename string `json:"filename,omitempty"`
}

type r2cInputAudio struct {
	Data   string `json:"data,omitempty"`
	Format string `json:"format,omitempty"`
}

func r2cConvertInputToMessages(input json.RawMessage) ([]dto.Message, error) {
	if len(input) == 0 {
		return nil, nil
	}
	var simpleText string
	if err := json.Unmarshal(input, &simpleText); err == nil {
		return []dto.Message{{Role: "user", Content: simpleText}}, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, fmt.Errorf("input must be string or array: %w", err)
	}
	messages := make([]dto.Message, 0, len(items))
	// 连续的 function_call 项聚合为一条 assistant 消息（chat 协议的 tool_calls 数组语义），
	// 其后的 function_call_output 转为引用对应 tool_call_id 的 tool 消息
	var pendingToolCalls []dto.ToolCall
	// reasoning 项的思考文本：挂到**下一条** assistant 消息（Responses 的 reasoning 项
	// 总是排在对应的 message / function_call 之前）。DeepSeek 等推理上游的 thinking 模式
	// 要求多轮历史把 reasoning_content 传回，丢弃会导致 400：
	// "The `reasoning_content` in the thinking mode must be passed back to the API."
	// 仅当历史携带可读思考文本（明文 summary/content）时才会产出该字段——
	// 上游若从不返回 reasoning_content（如 OpenAI 官方 chat），历史里就没有这类项，天然不触发。
	var pendingReasoning string
	takeReasoning := func() *string {
		if pendingReasoning == "" {
			return nil
		}
		r := pendingReasoning
		pendingReasoning = ""
		return &r
	}
	flushToolCalls := func() {
		if len(pendingToolCalls) == 0 {
			return
		}
		messages = append(messages, dto.Message{
			Role:             "assistant",
			Content:          nil,
			ToolCalls:        pendingToolCalls,
			ReasoningContent: takeReasoning(),
		})
		pendingToolCalls = nil
	}
	for _, raw := range items {
		var item r2cInputItem
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		switch item.Type {
		case "message":
			flushToolCalls()
			if msg := r2cConvertMessage(item); msg != nil {
				if msg.Role == "assistant" {
					msg.ReasoningContent = takeReasoning()
				} else {
					pendingReasoning = "" // user 消息前的 reasoning 无人可挂，丢弃
				}
				messages = append(messages, *msg)
			}
		case "function_call":
			// 历史中的助手工具调用：转为 assistant.tool_calls 条目，id 用 call_id（与 tool 消息的 tool_call_id 对应）
			if item.CallID == "" && item.Name == "" {
				continue
			}
			pendingToolCalls = append(pendingToolCalls, dto.ToolCall{
				ID:   item.CallID,
				Type: "function",
				Function: dto.FunctionCall{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			})
		case "function_call_output":
			flushToolCalls()
			messages = append(messages, dto.Message{Role: "tool", ToolCallID: item.CallID, Content: item.Output})
		case "reasoning":
			pendingReasoning = r2cReasoningText(item)
		default:
			flushToolCalls()
			if item.Role != "" {
				if msg := r2cConvertMessage(item); msg != nil {
					if msg.Role == "assistant" {
						msg.ReasoningContent = takeReasoning()
					}
					messages = append(messages, *msg)
				}
			}
		}
	}
	flushToolCalls()
	return messages, nil
}

func r2cConvertMessage(item r2cInputItem) *dto.Message {
	role := item.Role
	if role == "" {
		role = "user"
	}
	// Responses 的 developer 角色（OpenAI 新式系统提示，codex 等客户端常用）
	// 多数第三方 chat 上游不识别（serde 严格校验直接拒绝），统一映射为 system
	if role == "developer" {
		role = "system"
	}
	if len(item.Content) == 0 {
		return nil
	}
	var textContent string
	if err := json.Unmarshal(item.Content, &textContent); err == nil {
		return &dto.Message{Role: role, Content: textContent}
	}
	var parts []r2cContentPart
	if err := json.Unmarshal(item.Content, &parts); err != nil {
		return nil
	}
	// 产出类型化 []dto.ContentPart（链内规范形态）：本方向常作为步骤链首跳
	// （Responses→OpenAI→Claude/Gemini），第二跳转换器按 NormalizeContentParts
	// 归一化处理类型化列表；map 切片等私有形态会导致下游多模态静默丢失。
	chatParts := make([]dto.ContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "input_text":
			chatParts = append(chatParts, dto.ContentPart{Type: "text", Text: part.Text})
		case "input_audio":
			// 音频输入：chat 的 input_audio 与 Responses 同形（input_audio:{data,format}）
			if part.InputAudio != nil && part.InputAudio.Data != "" {
				chatParts = append(chatParts, dto.ContentPart{Type: "input_audio", InputAudio: &dto.InputAudio{
					Data:   part.InputAudio.Data,
					Format: part.InputAudio.Format,
				}})
			}
		case "input_file":
			// 文件输入：Responses 扁平 {file_data,filename} → chat 的 file:{file_data,filename}
			if part.FileData != "" {
				chatParts = append(chatParts, dto.ContentPart{Type: "file", File: &dto.FileData{
					FileData: part.FileData,
					Filename: part.Filename,
				}})
			}
		case "input_image":
			imageURL := part.ImageURL
			if imageURL == "" {
				imageURL = part.URL
			}
			if imageURL != "" {
				chatParts = append(chatParts, dto.ContentPart{Type: "image_url", ImageURL: &dto.ImageURL{
					URL:    imageURL,
					Detail: part.Detail,
				}})
			}
		case "output_text":
			chatParts = append(chatParts, dto.ContentPart{Type: "text", Text: part.Text})
		}
	}
	if len(chatParts) == 0 {
		return nil
	}
	if len(chatParts) == 1 && chatParts[0].Type == "text" {
		return &dto.Message{Role: role, Content: chatParts[0].Text}
	}
	return &dto.Message{Role: role, Content: chatParts}
}

func r2cConvertTools(toolsRaw json.RawMessage) []dto.Tool {
	var tools []map[string]any
	if err := json.Unmarshal(toolsRaw, &tools); err != nil {
		return nil
	}
	chatTools := make([]dto.Tool, 0, len(tools))
	for _, tool := range tools {
		toolType, _ := tool["type"].(string)
		if toolType == "function" {
			name, _ := tool["name"].(string)
			description, _ := tool["description"].(string)
			chatTools = append(chatTools, dto.Tool{
				Type: "function",
				Function: dto.FunctionDef{
					Name:        name,
					Description: description,
					Parameters:  tool["parameters"],
				},
			})
		}
	}
	return chatTools
}

func r2cConvertToolChoice(tcRaw json.RawMessage) any {
	if len(tcRaw) == 0 {
		return "auto"
	}
	var strVal string
	if err := json.Unmarshal(tcRaw, &strVal); err == nil {
		return strVal
	}
	var tc map[string]any
	if err := json.Unmarshal(tcRaw, &tc); err != nil {
		return "auto"
	}
	if tc["type"] == "function" {
		// chat 嵌套形状 {"type":"function","function":{"name":...}}（防御性兼容）
		if fn, ok := tc["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				return map[string]any{"type": "function", "function": map[string]any{"name": name}}
			}
		}
		// Responses wire 扁平形状 {"type":"function","name":...}——必须重组为 chat 嵌套形状，
		// 否则强制工具选择在 chat 上游被静默忽略（能力守恒测试守护）
		if name, ok := tc["name"].(string); ok && name != "" {
			return map[string]any{"type": "function", "function": map[string]any{"name": name}}
		}
	}
	return tc
}

// r2cParseTextFormat 解析 Responses text.format（扁平 {type,name,schema,strict}）
// 为 chat 的 response_format（json_schema 时嵌套为 json_schema:{name,schema,strict}）。
// text 或未知类型返回 nil（chat 无对应字段，不映射）。
func r2cParseTextFormat(raw json.RawMessage) *dto.ResponseFormat {
	var textCfg struct {
		Format struct {
			Type   string          `json:"type"`
			Name   string          `json:"name"`
			Schema json.RawMessage `json:"schema"`
			Strict *bool           `json:"strict"`
		} `json:"format"`
	}
	if err := json.Unmarshal(raw, &textCfg); err != nil {
		return nil
	}
	switch textCfg.Format.Type {
	case "json_object":
		return &dto.ResponseFormat{Type: "json_object"}
	case "json_schema":
		jsonSchema := make(map[string]any, 3)
		if textCfg.Format.Name != "" {
			jsonSchema["name"] = textCfg.Format.Name
		}
		if len(textCfg.Format.Schema) > 0 {
			var schema any
			if err := json.Unmarshal(textCfg.Format.Schema, &schema); err == nil {
				jsonSchema["schema"] = schema
			}
		}
		if textCfg.Format.Strict != nil {
			jsonSchema["strict"] = *textCfg.Format.Strict
		}
		return &dto.ResponseFormat{Type: "json_schema", JSONSchema: jsonSchema}
	default:
		return nil
	}
}

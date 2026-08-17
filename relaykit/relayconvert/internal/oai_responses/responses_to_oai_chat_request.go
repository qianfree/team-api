// Package oai_responses 承载 OpenAI Responses 协议与 Chat Completions 协议的双向转换器。
// 语义蓝本为宿主 relay/channel/openai/converter.go 的 r2c*/c2r* 函数族（旧路径保留为回退）。
package oai_responses

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ResponsesToOpenAIChatRequestConverter Responses 客户端 → OpenAI Chat 上游（请求侧）。
// 仅做纯格式转换：
//   - 有状态检查（previous_response_id）与请求快照 stash 是宿主桥接层职责，本转换器不做；
//   - 吸收了旧路径 adaptor 后处理中的 reasoning_effort 注入与 stream_options 注入语义
//     （relaykit 接管后 adaptor.ConvertRequest 不再执行）。
type ResponsesToOpenAIChatRequestConverter struct{}

func (c *ResponsesToOpenAIChatRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIResponsesToOpenAIChat
}

func (c *ResponsesToOpenAIChatRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *ResponsesToOpenAIChatRequestConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *ResponsesToOpenAIChatRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityGood
}

// ConvertRequest 入参断言 *dto.OpenAIResponsesRequest，输出 *dto.GeneralOpenAIRequest
// （供宿主桥接层 marshal，也作为 responses→claude 链式转换第一跳的输出类型契约）。
func (c *ResponsesToOpenAIChatRequestConverter) ConvertRequest(
	ctx context.Context, info convmeta.Meta, request any,
) (any, error) {
	req, ok := request.(*dto.OpenAIResponsesRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.OpenAIResponsesRequest, got %T", request)
	}
	return buildChatRequest(info, req)
}

func buildChatRequest(info convmeta.Meta, req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	chatReq := &dto.GeneralOpenAIRequest{}

	// 模型：宿主 catalog 保证未映射时 UpstreamModelName 即客户端模型名
	//（internal/dispatchadapter/catalog.go: upstream 为空时回退 ModelName），
	// 因此与旧路径 IsModelMapped ? UpstreamModelName : req.Model 等价
	if info != nil && info.HasChannelMeta() {
		chatReq.Model = info.GetUpstreamModelName()
	} else {
		chatReq.Model = req.Model
	}

	messages := make([]dto.Message, 0)
	// instructions → system 消息
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
		stream := *req.Stream
		chatReq.Stream = &stream
	}
	// stream_options 注入：以网关侧 IsStream 为准（与旧路径 adaptor 的 InjectStreamOptions
	// 终态一致——legacy 转换器在 req.Stream=true 时写入，adaptor 在 info.IsStream 时补齐，
	// 两条路径的最终效果均为 IsStream 时存在 include_usage）
	if info != nil && info.GetIsStream() {
		chatReq.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}
	if req.Temperature != nil {
		chatReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		chatReq.TopP = req.TopP
	}
	if req.MaxOutputTokens != nil {
		maxTokens := int(*req.MaxOutputTokens)
		chatReq.MaxTokens = &maxTokens
	}
	if req.Logprobs != nil {
		logprobs := true
		chatReq.LogProbs = &logprobs
		chatReq.TopLogProbs = req.Logprobs
	} else if req.TopLogProbs != nil {
		chatReq.TopLogProbs = req.TopLogProbs
		logprobs := true
		chatReq.LogProbs = &logprobs
	}
	if len(req.Tools) > 0 {
		if chatTools := r2cConvertTools(req.Tools); len(chatTools) > 0 {
			chatReq.Tools = chatTools
		}
	}
	if len(req.ToolChoice) > 0 {
		chatReq.ToolChoice = r2cConvertToolChoice(req.ToolChoice)
	}
	// reasoning_effort：客户端显式设置优先；为空时回退宿主注入的 thinking 后缀映射
	//（吸收旧路径 adaptor 的 injectReasoningEffort「仅缺席时注入」语义）
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	} else if info != nil && info.GetReasoningEffort() != "" {
		chatReq.ReasoningEffort = info.GetReasoningEffort()
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

// r2cInputItem Responses input 数组中的输入项（通用解析结构，字段按需映射）
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
}

// r2cContentPart Responses 内容块（input_text/input_image/input_audio/input_file/output_text）
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

// r2cConvertInputToMessages 将 Responses input（字符串或项数组）转换为 chat 消息数组。
// 连续的 function_call 项聚合为一条 assistant 消息（chat 协议的 tool_calls 数组语义），
// 其后的 function_call_output 转为引用对应 tool_call_id 的 tool 消息；reasoning 项跳过。
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
	var pendingToolCalls []dto.ToolCall
	flushToolCalls := func() {
		if len(pendingToolCalls) == 0 {
			return
		}
		messages = append(messages, dto.Message{Role: "assistant", Content: nil, ToolCalls: pendingToolCalls})
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
				messages = append(messages, *msg)
			}
		case "function_call":
			// 历史中的助手工具调用：转为 assistant.tool_calls 条目，id 用 call_id
			//（与 tool 消息的 tool_call_id 对应）
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
			// reasoning 项（含加密思考内容）无 chat 协议对应物，跳过
			continue
		default:
			flushToolCalls()
			if item.Role != "" {
				if msg := r2cConvertMessage(item); msg != nil {
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
	// 单一文本块折叠为纯字符串 content（与旧路径一致）
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
			fn := dto.FunctionDef{
				Name:       toolName(tool),
				Parameters: tool["parameters"],
			}
			if desc, ok := tool["description"].(string); ok {
				fn.Description = desc
			}
			chatTools = append(chatTools, dto.Tool{Type: "function", Function: fn})
		}
	}
	return chatTools
}

// toolName 提取工具名（非法类型时退化为空串，与旧路径 nil→null 的差异仅为可忽略的序列化形态差）
func toolName(tool map[string]any) string {
	name, _ := tool["name"].(string)
	return name
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
		if fn, ok := tc["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				return map[string]any{"type": "function", "function": map[string]any{"name": name}}
			}
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

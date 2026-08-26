// Package oai_responses 承载 OpenAI Responses 协议与 Chat Completions 协议的双向转换器。
// 语义蓝本为宿主 relay/channel/openai/converter.go 的 r2c*/c2r* 函数族（旧路径保留为回退）。
package oai_responses

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// DegradationReporter 宿主可选实现的 Meta 扩展接口：转换丢弃无目标协议对应物的请求内容
// 时上报（宿主转发到日志与监控，使能力降级可见）。宿主 relay/common.RelayInfo 实现本接口；
// 未实现或 info 为 nil 时（单测/非宿主调用）静默跳过，零开销。
type DegradationReporter interface {
	ReportConversionDegradation(converterID, reason string, count int)
}

// degradationRecorder 聚合单次转换内的降级计数（按 reason），flush 时逐条上报。
// 所有方法 nil-safe：reporter 缺席时调用点无需判空。
type degradationRecorder struct {
	reporter    DegradationReporter
	converterID string
	counts      map[string]int
}

func newDegradationRecorder(info convmeta.Meta, converterID string) *degradationRecorder {
	rec := &degradationRecorder{converterID: converterID}
	if reporter, ok := info.(DegradationReporter); ok && reporter != nil {
		rec.reporter = reporter
		rec.counts = make(map[string]int)
	}
	return rec
}

// drop 记录一次降级（reason 即指标维度，含具体类型，长度截断防膨胀）。
func (d *degradationRecorder) drop(reason string) {
	if d == nil || d.reporter == nil {
		return
	}
	if len(reason) > 64 {
		reason = reason[:64]
	}
	d.counts[reason]++
}

// flush 上报聚合结果（结果稳定：按 reason 排序）。
func (d *degradationRecorder) flush() {
	if d == nil || d.reporter == nil || len(d.counts) == 0 {
		return
	}
	reasons := make([]string, 0, len(d.counts))
	for r := range d.counts {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		d.reporter.ReportConversionDegradation(d.converterID, r, d.counts[r])
	}
}

// ResponsesToOpenAIChatRequestConverter Responses 客户端 → OpenAI Chat 上游（请求侧）。
// 仅做纯格式转换：
//   - 有状态检查（previous_response_id）与请求快照 stash 是宿主桥接层职责，本转换器不做；
//   - reasoning 项还原为所属 assistant 消息的 reasoning_content（DeepSeek 等思考模式
//     上游要求多轮工具调用时回传思考内容）；
//   - codex 的非 function 工具（local_shell / custom / apply_patch / namespace 子工具、
//     additional_tools 输入项）映射为 function 工具并按映射名 stash 原始类型
//     （响应侧据此还原 local_shell_call/custom_tool_call/apply_patch_call 输出项）；
//     对应调用历史项（custom_tool_call 等）还原为 assistant tool_calls + tool 消息；
//   - 无 chat 对应物的输入（web_search 等内置工具、仅 encrypted_content 的
//     reasoning 项等）被丢弃，经 DegradationReporter 上报使降级可见；
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
	drops := newDegradationRecorder(info, relayconvert.ConverterOpenAIResponsesToOpenAIChat)
	seenTools := make(map[string]bool)
	inputMessages, extraTools, err := r2cConvertInputToMessages(info, req.Input, drops, seenTools)
	if err != nil {
		return nil, fmt.Errorf("convert input to messages: %w", err)
	}
	messages = append(messages, inputMessages...)
	chatReq.Messages = messages
	defer drops.flush()

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
		if chatTools := r2cConvertTools(info, req.Tools, drops, seenTools); len(chatTools) > 0 {
			chatReq.Tools = chatTools
		}
	}
	// additional_tools 输入项提取出的工具（codex 新版把 exec/wait 等嵌套工具定义
	// 放在 input 中）合并进 chat tools，排在顶层 tools 之后
	if len(extraTools) > 0 {
		chatReq.Tools = append(chatReq.Tools, extraTools...)
	}
	if len(req.ToolChoice) > 0 {
		chatReq.ToolChoice = r2cConvertToolChoice(req.ToolChoice)
	}
	// parallel_tool_calls 透传（codex 默认 false=单工具串行，claude 上游映射为
	// disable_parallel_tool_use；false 必须以非 nil 指针传递，不能用 omitempty 语义丢值）
	if len(req.ParallelToolCalls) > 0 {
		var ptc bool
		if err := json.Unmarshal(req.ParallelToolCalls, &ptc); err == nil {
			chatReq.ParallelToolCalls = &ptc
		}
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
	// reasoning 项字段（Responses 历史中回传的思考内容摘要）
	Summary []r2cReasoningPart `json:"summary,omitempty"`
	// additional_tools 项字段（codex 新版把 exec/wait 等嵌套工具定义放在该输入项）
	Tools json.RawMessage `json:"tools,omitempty"`
	// custom_tool_call 项字段（freeform 输入字符串）
	Input string `json:"input,omitempty"`
	// local_shell_call / apply_patch_call 项字段（结构化动作，原样透传为 arguments）
	Action json.RawMessage `json:"action,omitempty"`
}

// r2cReasoningPart reasoning 项的文本部分（summary 的 summary_text / content 的 reasoning_text）
type r2cReasoningPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// r2cExtractReasoningText 从 reasoning 输入项提取思考文本：content（完整思考）优先，
// 缺失时回退 summary。仅 encrypted_content 的思考项无文本可还原，返回空串。
func r2cExtractReasoningText(item r2cInputItem) string {
	var parts []string
	if len(item.Content) > 0 {
		var cps []r2cReasoningPart
		if err := json.Unmarshal(item.Content, &cps); err == nil {
			for _, p := range cps {
				if (p.Type == "reasoning_text" || p.Type == "text") && p.Text != "" {
					parts = append(parts, p.Text)
				}
			}
		}
	}
	if len(parts) == 0 {
		for _, s := range item.Summary {
			if s.Text != "" {
				parts = append(parts, s.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
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
// 返回值 extraTools 为从 additional_tools 输入项提取并映射的工具（附加到 chat tools）。
// 助手轮重建规则（DeepSeek 等思考模式上游要求单轮单条 assistant 消息，且多轮工具调用时
// 必须回传 reasoning_content）：
//   - 连续的 function_call 项聚合为 tool_calls 数组；紧邻的 assistant 文本消息属同一助手轮，
//     tool_calls 合并进该消息而非另起一条（中间隔了丢弃项/其他角色消息则不合并）；
//   - reasoning 项还原为所属 assistant 消息的 reasoning_content：思考项先于消息出现
//     （真实 OpenAI 项序）时暂存待附着，晚于消息出现（本网关 completed output 项序为
//     message→reasoning→function_call）时直接附着到紧邻的前一条 assistant 消息；
//   - function_call_output 转为引用对应 tool_call_id 的 tool 消息；
//   - custom_tool_call / local_shell_call / apply_patch_call 历史项按映射名还原为
//     assistant tool_calls（custom 的 input 字符串包装为 {"input":...}，与响应侧解包互逆），
//     对应 output 项转为 tool 消息——多轮 agent 循环在 chat 上游不断裂。
//
// 无 chat 对应物的输入项（仅 encrypted_content 的 reasoning 项、未知类型）被丢弃并经
// drops 上报——降级必须可见，不允许静默砍能力。
// seenTools 为跨顶层 tools 与 additional_tools 共享的工具名去重集合（调用方持有）。
func r2cConvertInputToMessages(info convmeta.Meta, input json.RawMessage, drops *degradationRecorder, seenTools map[string]bool) ([]dto.Message, []dto.Tool, error) {
	if len(input) == 0 {
		return nil, nil, nil
	}
	var simpleText string
	if err := json.Unmarshal(input, &simpleText); err == nil {
		return []dto.Message{{Role: "user", Content: simpleText}}, nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, nil, fmt.Errorf("input must be string or array: %w", err)
	}
	messages := make([]dto.Message, 0, len(items))
	extraTools := make([]dto.Tool, 0)
	var pendingToolCalls []dto.ToolCall
	var pendingReasoning []string
	// mergeableAssistant：最后一条消息是本轮尚未闭合的 assistant 文本消息，
	// 其后的 function_call / reasoning 项仍属同一助手轮，可直接合并/附着
	mergeableAssistant := false

	// setReasoning 将思考文本写入消息（已有内容时换行追加）
	setReasoning := func(msg *dto.Message, text string) {
		if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
			text = *msg.ReasoningContent + "\n\n" + text
		}
		msg.ReasoningContent = &text
	}
	// consumePendingReasoning 将暂存的思考文本附着到 assistant 消息
	consumePendingReasoning := func(msg *dto.Message) {
		if len(pendingReasoning) == 0 || msg == nil || msg.Role != "assistant" {
			return
		}
		setReasoning(msg, strings.Join(pendingReasoning, "\n\n"))
		pendingReasoning = nil
	}
	// orphanPendingReasoning 丢弃无处附着的思考文本（后续无所属 assistant 消息）并上报
	orphanPendingReasoning := func() {
		for range pendingReasoning {
			drops.drop("input_item:reasoning_orphan")
		}
		pendingReasoning = nil
	}
	flushToolCalls := func() {
		if len(pendingToolCalls) == 0 {
			return
		}
		if mergeableAssistant && len(messages) > 0 &&
			messages[len(messages)-1].Role == "assistant" && len(messages[len(messages)-1].ToolCalls) == 0 {
			// 同一助手轮：tool_calls 合并进紧邻的 assistant 文本消息
			messages[len(messages)-1].ToolCalls = pendingToolCalls
			consumePendingReasoning(&messages[len(messages)-1])
		} else {
			msg := dto.Message{Role: "assistant", Content: nil, ToolCalls: pendingToolCalls}
			consumePendingReasoning(&msg)
			messages = append(messages, msg)
		}
		pendingToolCalls = nil
		mergeableAssistant = false
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
					consumePendingReasoning(msg)
				} else {
					orphanPendingReasoning()
				}
				messages = append(messages, *msg)
				mergeableAssistant = msg.Role == "assistant"
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
			orphanPendingReasoning()
			messages = append(messages, dto.Message{Role: "tool", ToolCallID: item.CallID, Content: item.Output})
			mergeableAssistant = false
		case "reasoning":
			// 思考项：还原为所属 assistant 消息的 reasoning_content（思考模式上游要求回传）；
			// 仅 encrypted_content 无文本可还原时仍属降级
			if text := r2cExtractReasoningText(item); text != "" {
				if mergeableAssistant && len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
					setReasoning(&messages[len(messages)-1], text)
				} else {
					pendingReasoning = append(pendingReasoning, text)
				}
			} else {
				drops.drop("input_item:reasoning")
			}
			continue
		case "additional_tools":
			// codex 新版把 exec/wait 等嵌套工具定义放在该输入项：提取并映射为
			// function 工具（合并进 chat tools），不再整体丢弃；无工具可提取时仍上报
			flushToolCalls()
			before := len(extraTools)
			if len(item.Tools) > 0 {
				extraTools = append(extraTools, r2cConvertTools(info, item.Tools, drops, seenTools)...)
			}
			if len(extraTools) == before {
				drops.drop("input_item:additional_tools")
			}
			mergeableAssistant = false
		case "custom_tool_call":
			// custom（freeform/grammar）工具调用历史：input 字符串包装为 {"input":...}
			// 的 arguments（与响应侧 custom_tool_call 合成时的解包互逆）
			if item.CallID == "" && item.Name == "" {
				continue
			}
			pendingToolCalls = append(pendingToolCalls, dto.ToolCall{
				ID:   item.CallID,
				Type: "function",
				Function: dto.FunctionCall{
					Name:      item.Name,
					Arguments: wrapCustomToolInput(item.Input),
				},
			})
		case "custom_tool_call_output":
			flushToolCalls()
			orphanPendingReasoning()
			messages = append(messages, dto.Message{Role: "tool", ToolCallID: item.CallID, Content: item.Output})
			mergeableAssistant = false
		case "local_shell_call", "apply_patch_call":
			// shell/patch 调用历史：action 对象原样作为 arguments，工具名取固定映射名
			//（与请求侧 local_shell→shell / apply_patch→apply_patch 的映射一致）
			name := localShellMappedName
			if item.Type == "apply_patch_call" {
				name = applyPatchMappedName
			}
			args := "{}"
			if len(item.Action) > 0 {
				args = string(item.Action)
			}
			pendingToolCalls = append(pendingToolCalls, dto.ToolCall{
				ID:   item.CallID,
				Type: "function",
				Function: dto.FunctionCall{
					Name:      name,
					Arguments: args,
				},
			})
		case "local_shell_call_output", "apply_patch_call_output":
			flushToolCalls()
			orphanPendingReasoning()
			messages = append(messages, dto.Message{Role: "tool", ToolCallID: item.CallID, Content: item.Output})
			mergeableAssistant = false
		default:
			flushToolCalls()
			if item.Role != "" {
				msg := r2cConvertMessage(item)
				if msg != nil {
					if msg.Role == "assistant" {
						consumePendingReasoning(msg)
					} else {
						orphanPendingReasoning()
					}
					messages = append(messages, *msg)
					mergeableAssistant = msg.Role == "assistant"
				} else {
					// 有 role 但无可转换内容（如未知新类型）：记录类型便于跟进协议演进
					drops.drop("input_item:" + item.Type)
					mergeableAssistant = false
				}
			} else {
				// 无 role 的未知类型输入项（如 codex 的 goal/plan 等新项）
				drops.drop("input_item:" + item.Type)
				mergeableAssistant = false
			}
		}
	}
	flushToolCalls()
	orphanPendingReasoning()
	return messages, extraTools, nil
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

// r2cConvertTools 转换 Responses 顶层 tools（或 additional_tools 项内的 tools）为 chat
// 工具数组。function 工具原样保留；local_shell / custom / apply_patch 经
// mapNonFunctionToolToChat 映射为 function 工具（按映射名 stash 原始类型，响应侧还原）；
// namespace 递归展开其子工具；web_search 等内置工具无 chat 对应物，丢弃并经 drops 上报。
// seen 为调用方持有的工具名去重集合（顶层 tools 与 additional_tools 共享）。
func r2cConvertTools(info convmeta.Meta, toolsRaw json.RawMessage, drops *degradationRecorder, seen map[string]bool) []dto.Tool {
	var tools []map[string]any
	if err := json.Unmarshal(toolsRaw, &tools); err != nil {
		return nil
	}
	chatTools := make([]dto.Tool, 0, len(tools))
	for _, tool := range tools {
		toolType, _ := tool["type"].(string)
		switch toolType {
		case "function":
			name := toolName(tool)
			if name != "" && seen[name] {
				drops.drop("tool:function_duplicate")
				continue
			}
			if name != "" {
				seen[name] = true
			}
			fn := dto.FunctionDef{
				Name:       name,
				Parameters: tool["parameters"],
			}
			if desc, ok := tool["description"].(string); ok {
				fn.Description = desc
			}
			chatTools = append(chatTools, dto.Tool{Type: "function", Function: fn})
		case "namespace":
			// 命名空间（codex 的 functions/collaboration 等）：递归展开子工具；
			// 展开后零产出说明其内容均无 chat 对应物，按 namespace 上报降级
			subRaw, err := json.Marshal(tool["tools"])
			if err != nil {
				drops.drop("tool:namespace")
				continue
			}
			sub := r2cConvertTools(info, subRaw, drops, seen)
			if len(sub) == 0 {
				drops.drop("tool:namespace")
				continue
			}
			chatTools = append(chatTools, sub...)
		case ToolKindLocalShell, ToolKindApplyPatch, ToolKindCustom:
			if mapped, ok := mapNonFunctionToolToChat(info, tool, seen); ok {
				chatTools = append(chatTools, mapped)
			} else {
				drops.drop("tool:" + toolType)
			}
		default:
			// web_search / file_search / computer_use / mcp 等内置工具无 chat 对应物
			if toolType != "" {
				drops.drop("tool:" + toolType)
			}
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
		// chat 形态：{"type":"function","function":{"name":...}}
		if fn, ok := tc["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				return map[string]any{"type": "function", "function": map[string]any{"name": name}}
			}
		}
		// Responses 扁平形态：{"type":"function","name":...}——包装为嵌套，
		// 原样透传会被 chat 上游拒绝（要求嵌套 function.name）
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

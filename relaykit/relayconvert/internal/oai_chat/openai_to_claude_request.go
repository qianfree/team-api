package oai_chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToClaudeRequestConverter 将 OpenAI Chat Completions 请求转换为 Claude Messages API 请求。
type OpenAIToClaudeRequestConverter struct{}

func (c *OpenAIToClaudeRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIChatToClaudeMessages
}

func (c *OpenAIToClaudeRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToClaudeRequestConverter) To() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *OpenAIToClaudeRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityFair
}

func (c *OpenAIToClaudeRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeneralOpenAIRequest, got %T", request)
	}

	// 确定上游模型名
	upstreamModel := info.GetUpstreamModelName()
	if upstreamModel == "" {
		upstreamModel = info.GetOriginModelName()
	}

	// 解析 thinking 后缀，若无需保留则从模型名中剥离
	thinkingInfo := shared.ParseThinkingSuffix(upstreamModel)
	opts := convmeta.OptionsOf(info)
	if !opts.ShouldPreserveThinkingSuffix(upstreamModel) {
		upstreamModel = thinkingInfo.BaseModel
	}

	claudeReq := &dto.ClaudeRequest{
		Model:    upstreamModel,
		Messages: make([]dto.ClaudeMessage, 0),
		Stream:   openaiReq.Stream,
	}

	// MaxTokens（Claude API 必填）；max_completion_tokens 为新式客户端（o 系列/gpt-5 系 SDK）的字段，同等生效
	maxTokensRequested := openaiReq.MaxTokens
	if maxTokensRequested == nil {
		maxTokensRequested = openaiReq.MaxCompletionTokens
	}
	if maxTokensRequested != nil {
		maxTokens := uint(*maxTokensRequested)
		claudeReq.MaxTokens = &maxTokens
	} else {
		// 尝试从 options 中获取默认值
		if maxTokens, ok := opts.Claude.DefaultMaxTokensFor(upstreamModel); ok {
			mt := uint(maxTokens)
			claudeReq.MaxTokens = &mt
		} else {
			return nil, fmt.Errorf("max_tokens is required for Claude API but not provided and no default available")
		}
	}

	// Temperature / TopP
	claudeReq.Temperature = openaiReq.Temperature
	claudeReq.TopP = openaiReq.TopP

	// TopK（OpenAI 没有此字段，但 Claude 有）
	// 保持为 nil，由 Claude 使用其默认值

	// Stop sequences
	if openaiReq.Stop != nil {
		switch v := openaiReq.Stop.(type) {
		case string:
			claudeReq.StopSequences = []string{v}
		case []string:
			claudeReq.StopSequences = v
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					claudeReq.StopSequences = append(claudeReq.StopSequences, s)
				}
			}
		}
	}

	// 转换 messages
	systemPrompts := make([]string, 0)
	// 连续 tool 消息聚合缓冲：Claude 要求同一 assistant tool_use 轮的全部 tool_result 位于
	// 紧随的下一条 user 消息——逐条映射为多条 user/tool_result 消息违反该约束，并行工具调用
	//（codex 等客户端对一轮多个 function_call 各回一条 function_call_output）必被上游 400
	var pendingToolResults []dto.ClaudeContentBlock
	flushToolResults := func() {
		if len(pendingToolResults) == 0 {
			return
		}
		claudeReq.Messages = append(claudeReq.Messages, dto.ClaudeMessage{Role: "user", Content: pendingToolResults})
		pendingToolResults = nil
	}
	for _, msg := range openaiReq.Messages {
		if msg.Role == "system" {
			flushToolResults()
			// system 消息放入独立的 System 字段
			systemPrompts = append(systemPrompts, shared.MapTextContent(msg.Content))
			continue
		}
		if msg.Role == "tool" && msg.ToolCallID != "" {
			// tool 结果：聚合为下一条 user 消息的 tool_result 块（连续 tool 消息合并）
			pendingToolResults = append(pendingToolResults, dto.ClaudeContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   shared.MapTextContent(msg.Content),
			})
			continue
		}
		flushToolResults()

		claudeMsg := dto.ClaudeMessage{
			Role: msg.Role,
		}

		// 转换 content。Anthropic 协议拒绝空 text 块（"text":"" 上游 400 参数错误），
		// 因此文本为空时不落块——带 tool_calls 的 assistant 消息 content 常为 nil/""，
		// 仅 tool_use 块即为合法内容。
		// 多模态经 NormalizeContentParts 归一：真实 JSON 流量为 []any（元素 map），
		// 链式转换第一跳产出 typed []dto.ContentPart，两种形态都须识别
		switch content := msg.Content.(type) {
		case string:
			if content != "" {
				claudeMsg.Content = []dto.ClaudeContentBlock{{Type: "text", Text: &content}}
			}
		default:
			if parts := shared.NormalizeContentParts(content); len(parts) > 0 {
				claudeMsg.Content = shared.MapOpenAIContentPartsToClaude(parts)
			} else if text := shared.MapTextContent(content); text != "" {
				claudeMsg.Content = []dto.ClaudeContentBlock{{Type: "text", Text: &text}}
			}
		}

		// 转换 tool calls（带 tool_calls 的 assistant 消息）
		if len(msg.ToolCalls) > 0 {
			toolBlocks := shared.MapOpenAIToolCallsToClaude(msg.ToolCalls)
			if blocks, ok := claudeMsg.Content.([]dto.ClaudeContentBlock); ok {
				claudeMsg.Content = append(blocks, toolBlocks...)
			} else {
				claudeMsg.Content = toolBlocks
			}
		}

		// 空内容兜底：以上转换后仍无任何内容块（content 为 nil 或空数组——如纯文本为空
		// 且无 tool_calls）时，Claude 拒绝 null/空 content——补单个空格 text 块
		//（对齐 legacy o2cConvertAssistantMessage）
		if blocks, ok := claudeMsg.Content.([]dto.ClaudeContentBlock); claudeMsg.Content == nil || (ok && len(blocks) == 0) {
			space := " "
			claudeMsg.Content = []dto.ClaudeContentBlock{{Type: "text", Text: &space}}
		}

		claudeReq.Messages = append(claudeReq.Messages, claudeMsg)
	}
	flushToolResults()

	// 设置 system prompt
	if len(systemPrompts) > 0 {
		claudeReq.System = strings.Join(systemPrompts, "\n\n")
	}

	// 转换 tools
	if len(openaiReq.Tools) > 0 {
		claudeReq.Tools = shared.MapOpenAIToolsToClaudeTools(openaiReq.Tools)
		// 转换 tool_choice。parallel_tool_calls=false 透传为 disable_parallel_tool_use
		//（codex 等客户端默认单工具串行，对齐 new-api 的映射；Claude 无 none 对应物，不设置）
		if openaiReq.ToolChoice != nil || openaiReq.ParallelToolCalls != nil {
			claudeChoice := &dto.ClaudeToolChoice{}
			switch tc := openaiReq.ToolChoice.(type) {
			case string:
				switch tc {
				case "auto":
					claudeChoice.Type = "auto"
				case "required":
					claudeChoice.Type = "any"
				case "none":
					// 不设置 tool_choice，或置为 null
				}
			case map[string]any:
				// {"type": "function", "function": {"name": "get_weather"}}
				if tc["type"] == "function" {
					if fn, ok := tc["function"].(map[string]any); ok {
						if name, ok := fn["name"].(string); ok {
							claudeChoice.Type = "tool"
							claudeChoice.Name = name
						}
					}
				}
			}
			if openaiReq.ParallelToolCalls != nil {
				claudeChoice.DisableParallelToolUse = !*openaiReq.ParallelToolCalls
				if claudeChoice.Type == "" {
					claudeChoice.Type = "auto"
				}
			}
			if claudeChoice.Type != "" {
				claudeReq.ToolChoice = claudeChoice
			}
		}
	}

	// web_search_options（responses 入站的 web_search 托管工具经 r2c 提取）→
	// Claude 托管 web_search server tool；与 function 工具并存，独立于上方 tools 块
	// （无 function 工具时搜索仍应生效）
	if len(openaiReq.WebSearchOptions) > 0 {
		claudeReq.Tools = append(claudeReq.Tools, dto.ClaudeTool{
			Type: "web_search_20250305",
			Name: "web_search",
		})
	}

	// 应用 thinking 适配器
	shared.ApplyThinkingToClaude(claudeReq, thinkingInfo, opts.Claude)

	// 请求体 reasoning_effort → thinking（legacy o2cConvertReasoningEffort 的迁移）。
	// 显式请求参数优先于模型名后缀，故在适配器之后应用；thinking 与 temperature 修改不兼容，须置 1.0
	if openaiReq.ReasoningEffort == "none" {
		// 显式关闭推理：覆盖模型名后缀注入的 thinking（显式参数优先）
		claudeReq.Thinking = nil
	} else if openaiReq.ReasoningEffort != "" {
		budget := 8192
		switch openaiReq.ReasoningEffort {
		case "minimal", "low":
			budget = 1024
		case "medium":
			budget = 8192
		case "high", "xhigh", "max", "ultra":
			budget = 32768
		}
		// Anthropic 约束：thinking.budget_tokens 须 ≥1024 且严格小于 max_tokens。codex 等客户端
		// 默认 reasoning.effort=medium 且不带 max_output_tokens（网关默认 max_tokens=4096），
		// budget 8192 装不进 4096 → 上游 400 "budget_tokens must be less than max_tokens"。
		// 对齐 ApplyThinkingToClaude 的处理：客户端 max_tokens 装不下时抬高 max_tokens
		if claudeReq.MaxTokens != nil && int(*claudeReq.MaxTokens) <= budget {
			raised := uint(budget + 1024)
			claudeReq.MaxTokens = &raised
		}
		claudeReq.Thinking = &dto.ClaudeThinking{
			Type:         "enabled",
			BudgetTokens: &budget,
		}
		one := 1.0
		claudeReq.Temperature = &one
	}

	return claudeReq, nil
}

// 转换器在宿主应用的包初始化阶段注册，而非在此 internal 实现包中完成。

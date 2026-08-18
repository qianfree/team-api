// Package relayconvert 承载格式转换器注册表与调度引擎。本文件定义请求侧转换器 spec
// 类型与注册 / 查找机制。
//
// 本文件只移植「spec 类型 + 函数类型 + 注册 / 查找」结构层。
// 调度引擎（ConvertRequest / ConvertRequestVia / ConvertRequestByID / execute* /
// inferRequestRelayFormat / prepareRequestForStep）与具体 adapter 函数
// （convertOpenAIRequestToClaude 等）依赖 DTO 与 internal/* 转换器实现。
package relayconvert

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

type RequestConverterFunc func(c context.Context, info convmeta.Meta, request any) (any, error)

type RequestConverterQuality string

const (
	RequestConverterQualityGood        RequestConverterQuality = "good"
	RequestConverterQualityFair        RequestConverterQuality = "fair"
	RequestConverterQualityDiscouraged RequestConverterQuality = "discouraged"
)

type RequestStep struct {
	Converter string
	From      types.RelayFormat
	To        types.RelayFormat
}

type RequestResult struct {
	Value     any
	From      types.RelayFormat
	To        types.RelayFormat
	Converter string
	Quality   RequestConverterQuality
	Steps     []RequestStep
}

type RequestConverterSpec struct {
	ID             string
	From           types.RelayFormat
	To             types.RelayFormat
	Quality        RequestConverterQuality
	Convert        RequestConverterFunc
	StepConverters []string
}

type requestConverterRoute struct {
	from types.RelayFormat
	to   types.RelayFormat
}

var (
	requestConverterMu           sync.RWMutex
	requestConverters            = make(map[string]RequestConverterSpec)
	requestConverterRoutes       = make(map[requestConverterRoute]string)
	requestConverterDirectRoutes = make(map[requestConverterRoute]string)
)

const (
	ConverterNone                             = "none"
	ConverterClaudeMessagesToOpenAIChat       = "anthropic_messages_to_openai_chat_completions"
	ConverterClaudeMessagesToOpenAIChatStream = "anthropic_messages_to_openai_chat_completions_stream"
	// ConverterClaudeMessagesToOpenAIResponsesStream Claude 上游 SSE → Responses 客户端 SSE（流式响应侧）
	ConverterClaudeMessagesToOpenAIResponsesStream = "anthropic_messages_to_openai_responses_stream"
	// ConverterOpenAIChatToOpenAIResponsesStream OpenAI Chat 上游 SSE → Responses 客户端 SSE（流式响应侧）
	ConverterOpenAIChatToOpenAIResponsesStream = "openai_chat_to_openai_responses_stream"
	// ConverterOpenAIChatToClaudeMessagesStream OpenAI Chat 上游 SSE → Claude 客户端 SSE（流式响应侧，P2）
	ConverterOpenAIChatToClaudeMessagesStream = "openai_chat_to_anthropic_messages_stream"
	// ConverterOpenAIChatToGeminiContentStream OpenAI Chat 上游 SSE → Gemini 客户端流（流式响应侧，P2）
	ConverterOpenAIChatToGeminiContentStream = "openai_chat_to_gemini_generate_content_stream"
	ConverterOpenAIChatToClaudeMessages            = "openai_chat_completions_to_anthropic_messages"
	ConverterOpenAIChatToOpenAIResponses           = "openai_chat_completions_to_openai_responses"
	ConverterOpenAIResponsesToOpenAIChat           = "openai_responses_to_openai_chat_completions"
	ConverterOpenAIResponsesToClaudeMessages       = "openai_responses_to_claude_messages"
	ConverterOpenAIResponsesToGemini               = "openai_responses_to_gemini_generate_content"
	ConverterGeminiContentToOpenAIChat             = "gemini_generate_content_to_openai_chat_completions"
	// ConverterGeminiContentToClaudeMessages Gemini 客户端 → Claude 上游（StepConverters 链：
	// gemini→openai→claude 两跳，替换宿主 claude/converter.go 的手工拼接链）
	ConverterGeminiContentToClaudeMessages = "gemini_generate_content_to_claude_messages"
	// ConverterClaudeMessagesToGeminiContent Claude 客户端 → Gemini 上游（StepConverters 链：
	// claude→openai→gemini 两跳，替换宿主 gemini/converter.go 的手工拼接链）
	ConverterClaudeMessagesToGeminiContent = "anthropic_messages_to_gemini_generate_content"
	// ConverterClaudeMessagesToOpenAIResponses Claude 客户端 → Responses 上游（StepConverters 链：
	// claude→openai→responses，P3——ChatViaResponses 渠道上的 claude 客户端）
	ConverterClaudeMessagesToOpenAIResponses = "anthropic_messages_to_openai_responses"
	// ConverterGeminiContentToOpenAIResponses Gemini 客户端 → Responses 上游（链：gemini→openai→responses）
	ConverterGeminiContentToOpenAIResponses = "gemini_generate_content_to_openai_responses"
	ConverterOpenAIChatToGeminiContent     = "openai_chat_completions_to_gemini_generate_content"

	// OpenAI → 原生格式供应商（请求侧）
	ConverterOpenAIChatToCoze   = "openai_chat_completions_to_coze_chat"
	ConverterOpenAIChatToDify   = "openai_chat_completions_to_dify_chat_messages"
	ConverterOpenAIChatToOllama = "openai_chat_completions_to_ollama_chat"
)

// registerBuiltinRequestConverter 注册一个请求转换器 spec。
// 直接转换器（Convert != nil）会同时进入 directRoutes；步骤转换器（StepConverters 非空）
// 在注册时即校验 From/To 连续性。builtin 列表通过 init() 调用本函数。
//
// 并发安全：持有 requestConverterMu 写锁保护注册过程，防止 data race。
// 虽然通常在包 init() 中调用（单线程），但加锁确保未来动态注册或测试并发场景安全。
func registerBuiltinRequestConverter(spec RequestConverterSpec) {
	spec.ID = strings.TrimSpace(spec.ID)
	if spec.ID == "" {
		panic("request converter ID is required")
	}
	if spec.From == "" || spec.To == "" {
		panic(fmt.Sprintf("request converter %q must declare from and to formats", spec.ID))
	}
	if spec.Quality == "" {
		panic(fmt.Sprintf("request converter %q must declare quality", spec.ID))
	}
	if spec.Convert == nil && len(spec.StepConverters) == 0 {
		panic(fmt.Sprintf("request converter %q must declare convert or step converters", spec.ID))
	}
	if spec.Convert != nil && len(spec.StepConverters) > 0 {
		panic(fmt.Sprintf("request converter %q cannot declare convert and step converters together", spec.ID))
	}

	requestConverterMu.Lock()
	defer requestConverterMu.Unlock()

	if _, exists := requestConverters[spec.ID]; exists {
		panic(fmt.Sprintf("request converter %q is already registered", spec.ID))
	}
	route := requestConverterRoute{from: spec.From, to: spec.To}
	if existingID, exists := requestConverterRoutes[route]; exists {
		panic(fmt.Sprintf("request converter route from %s to %s is already registered by %q", spec.From, spec.To, existingID))
	}

	if len(spec.StepConverters) > 0 {
		stepConverters := make([]string, 0, len(spec.StepConverters))
		current := spec.From
		for _, converterID := range spec.StepConverters {
			step, ok := requestConverters[converterID]
			if !ok {
				panic(fmt.Sprintf("request converter %q references unknown step converter %q", spec.ID, converterID))
			}
			if step.Convert == nil || len(step.StepConverters) > 0 {
				panic(fmt.Sprintf("request converter %q step %q must be a direct converter", spec.ID, converterID))
			}
			if step.From != current {
				panic(fmt.Sprintf("request converter %q step %q expects %s after %s", spec.ID, converterID, step.From, current))
			}
			stepConverters = append(stepConverters, converterID)
			current = step.To
		}
		if current != spec.To {
			panic(fmt.Sprintf("request converter %q ends at %s, expected %s", spec.ID, current, spec.To))
		}
		spec.StepConverters = stepConverters
	}

	requestConverters[spec.ID] = spec
	requestConverterRoutes[route] = spec.ID
	if len(spec.StepConverters) == 0 {
		requestConverterDirectRoutes[route] = spec.ID
	}
}

func LookupRequestConverter(converter string) (RequestConverterSpec, bool) {
	requestConverterMu.RLock()
	defer requestConverterMu.RUnlock()

	spec, ok := requestConverters[strings.TrimSpace(converter)]
	if !ok {
		return RequestConverterSpec{}, false
	}
	return cloneRequestConverterSpec(spec), true
}

func lookupRequestRoute(from types.RelayFormat, to types.RelayFormat) (RequestConverterSpec, bool) {
	requestConverterMu.RLock()
	defer requestConverterMu.RUnlock()

	converterID, ok := requestConverterRoutes[requestConverterRoute{from: from, to: to}]
	if !ok {
		return RequestConverterSpec{}, false
	}
	spec, ok := requestConverters[converterID]
	return cloneRequestConverterSpec(spec), ok
}

func lookupRequestDirectRoute(from types.RelayFormat, to types.RelayFormat) (RequestConverterSpec, bool) {
	requestConverterMu.RLock()
	defer requestConverterMu.RUnlock()

	converterID, ok := requestConverterDirectRoutes[requestConverterRoute{from: from, to: to}]
	if !ok {
		return RequestConverterSpec{}, false
	}
	spec, ok := requestConverters[converterID]
	return cloneRequestConverterSpec(spec), ok
}

func cloneRequestConverterSpec(spec RequestConverterSpec) RequestConverterSpec {
	if len(spec.StepConverters) > 0 {
		spec.StepConverters = append([]string{}, spec.StepConverters...)
	}
	return spec
}

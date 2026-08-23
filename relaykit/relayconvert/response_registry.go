// 响应侧转换器 spec 类型与注册 / 查找机制。包级文档见 doc.go。
package relayconvert

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

type ResponseConverterFunc func(c context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error)

type ResponseConverterQuality string

const (
	ResponseConverterQualityGood        ResponseConverterQuality = "good"
	ResponseConverterQualityFair        ResponseConverterQuality = "fair"
	ResponseConverterQualityDiscouraged ResponseConverterQuality = "discouraged"
)

type ResponseConverterSpec struct {
	ID             string
	From           types.RelayFormat
	To             types.RelayFormat
	Quality        ResponseConverterQuality
	Convert        ResponseConverterFunc
	StepConverters []string
}

type responseConverterRoute struct {
	from types.RelayFormat
	to   types.RelayFormat
}

const (
	ResponseConverterOAIChatToOAIResponses         = "oai_chat_to_oai_responses_resp"
	ResponseConverterOAIResponsesToOAIChat         = "oai_responses_to_oai_chat_resp"
	ResponseConverterOAIChatToClaudeMessages       = "oai_chat_to_claude_messages_resp"
	ResponseConverterOAIChatToGeminiChat           = "oai_chat_to_gemini_chat_resp"
	ResponseConverterClaudeMessagesToOAIChat       = "claude_messages_to_oai_chat_resp"
	ResponseConverterClaudeMessagesToOAIChatStream = "claude_messages_to_oai_chat_stream_resp"
	ResponseConverterGeminiChatToOAIChat           = "gemini_chat_to_oai_chat_resp"
	ResponseConverterGeminiChatToOAIChatStream     = "gemini_chat_to_oai_chat_stream_resp"

	// 原生格式供应商 → OpenAI（响应侧，含流式）
	ResponseConverterCozeChatToOAIChat         = "coze_chat_to_oai_chat_resp"
	ResponseConverterCozeChatToOAIChatStream   = "coze_chat_to_oai_chat_stream_resp"
	ResponseConverterDifyChatToOAIChat         = "dify_chat_to_oai_chat_resp"
	ResponseConverterDifyChatToOAIChatStream   = "dify_chat_to_oai_chat_stream_resp"
	ResponseConverterOllamaChatToOAIChat       = "ollama_chat_to_oai_chat_resp"
	ResponseConverterOllamaChatToOAIChatStream = "ollama_chat_to_oai_chat_stream_resp"
)

var (
	responseConverterMu      sync.RWMutex
	responseConverters       = make(map[string]ResponseConverterSpec)
	responseConverterAliases = make(map[string]string)
	responseConverterRoutes  = make(map[responseConverterRoute]string)
)

// registerBuiltinResponseConverter 注册响应侧转换器到注册表。
// init() 通过 registerBuiltinTextConverter() 调用本函数。
//
// 并发安全：持有 responseConverterMu 写锁保护注册过程，防止 data race。
// 虽然通常在包 init() 中调用（单线程），但加锁确保未来动态注册或测试并发场景安全。
func registerBuiltinResponseConverter(spec ResponseConverterSpec) {
	spec.ID = strings.TrimSpace(spec.ID)
	if spec.ID == "" {
		panic("response converter ID is required")
	}
	if spec.From == "" || spec.To == "" {
		panic(fmt.Sprintf("response converter %q must declare from and to formats", spec.ID))
	}
	if spec.Quality == "" {
		panic(fmt.Sprintf("response converter %q must declare quality", spec.ID))
	}
	if spec.Convert == nil && len(spec.StepConverters) == 0 {
		panic(fmt.Sprintf("response converter %q must declare convert or step converters", spec.ID))
	}
	if len(spec.StepConverters) > 0 && spec.Convert != nil {
		panic(fmt.Sprintf("response converter %q cannot declare direct implementations and step converters together", spec.ID))
	}

	responseConverterMu.Lock()
	defer responseConverterMu.Unlock()

	if _, exists := responseConverters[spec.ID]; exists {
		panic(fmt.Sprintf("response converter %q is already registered", spec.ID))
	}
	route := responseConverterRoute{from: spec.From, to: spec.To}
	if existingID, exists := responseConverterRoutes[route]; exists {
		panic(fmt.Sprintf("response converter route from %s to %s is already registered by %q", spec.From, spec.To, existingID))
	}

	if len(spec.StepConverters) > 0 {
		stepConverters := make([]string, 0, len(spec.StepConverters))
		current := spec.From
		for _, converterID := range spec.StepConverters {
			step, ok := responseConverters[converterID]
			if !ok {
				panic(fmt.Sprintf("response converter %q references unknown step converter %q", spec.ID, converterID))
			}
			if len(step.StepConverters) > 0 {
				panic(fmt.Sprintf("response converter %q step %q must be a direct converter", spec.ID, converterID))
			}
			if step.From != current {
				panic(fmt.Sprintf("response converter %q step %q expects %s after %s", spec.ID, converterID, step.From, current))
			}
			stepConverters = append(stepConverters, converterID)
			current = step.To
		}
		if current != spec.To {
			panic(fmt.Sprintf("response converter %q ends at %s, expected %s", spec.ID, current, spec.To))
		}
		spec.StepConverters = stepConverters
	}

	responseConverters[spec.ID] = spec
	responseConverterRoutes[route] = spec.ID
}

// registerResponseConverterAlias 注册响应转换器别名。
//
// 并发安全：持有 responseConverterMu 写锁保护注册过程，防止 data race。
// 虽然通常在包 init() 中调用（单线程），但加锁确保未来动态注册或测试并发场景安全。
func registerResponseConverterAlias(alias string, converter string) {
	alias = strings.TrimSpace(alias)
	converter = strings.TrimSpace(converter)
	if alias == "" {
		panic("response converter alias is required")
	}
	if converter == "" {
		panic(fmt.Sprintf("response converter alias %q target is required", alias))
	}
	if alias == converter {
		return
	}

	responseConverterMu.Lock()
	defer responseConverterMu.Unlock()

	if _, exists := responseConverters[alias]; exists {
		panic(fmt.Sprintf("response converter alias %q conflicts with registered converter", alias))
	}
	if _, exists := responseConverters[converter]; !exists {
		panic(fmt.Sprintf("response converter alias %q references unknown converter %q", alias, converter))
	}
	if existing, exists := responseConverterAliases[alias]; exists && existing != converter {
		panic(fmt.Sprintf("response converter alias %q is already registered for %q", alias, existing))
	}
	responseConverterAliases[alias] = converter
}

func LookupResponseConverter(converter string) (ResponseConverterSpec, bool) {
	responseConverterMu.RLock()
	defer responseConverterMu.RUnlock()

	converterID := resolveResponseConverterID(converter)
	spec, ok := responseConverters[converterID]
	if !ok {
		return ResponseConverterSpec{}, false
	}
	return cloneResponseConverterSpec(spec), true
}

func resolveResponseConverterID(converter string) string {
	converter = strings.TrimSpace(converter)
	if canonical, ok := responseConverterAliases[converter]; ok {
		return canonical
	}
	return converter
}

func cloneResponseConverterSpec(spec ResponseConverterSpec) ResponseConverterSpec {
	if len(spec.StepConverters) > 0 {
		spec.StepConverters = append([]string{}, spec.StepConverters...)
	}
	return spec
}

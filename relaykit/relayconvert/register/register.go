// Package register 将内置转换器注册进运行时注册表。
//
// 独立子包解决 import cycle：内置转换器（internal/oai_chat、internal/oai_gemini）
// 需要 import relayconvert 获取常量和类型，因此 relayconvert 本身不能反向 import 它们。
// 本包位于 relayconvert 之上，可同时 import relayconvert 与其 internal 转换器包，
// 由主项目在启动时通过 blank import 触发注册。
//
// 使用方式（主项目 relay 层）：
//
//	import _ "github.com/qianfree/team-api/relaykit/relayconvert/register"
package register

import (
	"context"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/claude_gemini"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/native_responses"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_chat"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_gemini"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_responses"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/ollama_chat"
	"github.com/qianfree/team-api/relaykit/types"
)

func init() {
	registerOpenAIToClaude()
	registerOpenAIToGemini()
	// 剩余原生格式供应商
	registerOpenAIToOllama()
	// 反向方向（非 OpenAI 入站 → OpenAI 上游）与 Responses 双向
	registerReverseAndResponsesDirections()
	// 跨原生方向（请求经 OpenAI 中枢链式，响应直连）
	registerCrossNativeDirections()
}

// registerOpenAIToClaude 注册 OpenAI → Claude 方向转换器。
// 客户端说 OpenAI，上游说 Claude：
//   - 请求侧 OpenAI → Claude
//   - 响应侧 Claude → OpenAI
func registerOpenAIToClaude() {
	reqConv := &oai_chat.OpenAIToClaudeRequestConverter{}
	respConv := &oai_chat.ClaudeToOpenAIResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIChatToClaudeMessages,
		From:    types.RelayFormatOpenAI,
		To:      types.RelayFormatClaude,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: reqConv.ConvertRequest,
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
			// 流式转换器签名 (io.Reader + callback) 与 ResponseStreamConverterFunc 不兼容，
			// 改由独立的流式注册表登记，宿主桥接层经 LookupStreamConverter 查找调用。
		},
	})

	// 流式响应侧：Claude SSE → OpenAI SSE（方向与请求相反）。
	relayconvert.RegisterStreamConverter(
		types.RelayFormatClaude, types.RelayFormatOpenAI,
		relayconvert.ConverterClaudeMessagesToOpenAIChatStream,
		(&oai_chat.ClaudeToOpenAIStreamConverter{}).ConvertStreamResponse,
	)
}

// registerOpenAIToGemini 注册 OpenAI → Gemini 方向转换器。
// 客户端说 OpenAI，上游说 Gemini：
//   - 请求侧 OpenAI → Gemini
//   - 响应侧 Gemini → OpenAI
func registerOpenAIToGemini() {
	reqConv := &oai_gemini.OpenAIToGeminiRequestConverter{}
	respConv := &oai_gemini.GeminiToOpenAIResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIChatToGeminiContent,
		From:    types.RelayFormatOpenAI,
		To:      types.RelayFormatGemini,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: reqConv.ConvertRequest,
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
			// 流式转换器登记在独立的流式注册表，宿主经 LookupStreamConverter 查找调用。
		},
	})

	// 流式响应侧：Gemini SSE → OpenAI SSE（方向与请求相反）。
	relayconvert.RegisterStreamConverter(
		types.RelayFormatGemini, types.RelayFormatOpenAI,
		relayconvert.ResponseConverterGeminiChatToOAIChatStream,
		(&oai_gemini.GeminiToOpenAIStreamConverter{}).ConvertStreamResponse,
	)
}

// registerOpenAIToOllama 注册 OpenAI → Ollama 方向转换器（仅 chat 路径）。
// 客户端说 OpenAI，上游说 Ollama /api/chat：
//   - 请求侧 OpenAI → Ollama
//   - 响应侧 Ollama → OpenAI（非流式 JSON；流式 NDJSON→SSE）
//
// 仅覆盖 RelayModeChatCompletions；generate/embedding 不注册 converter，桥接自动回退旧路径。
func registerOpenAIToOllama() {
	reqConv := &ollama_chat.OpenAIToOllamaRequestConverter{}
	respConv := &ollama_chat.OllamaToOpenAIResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIChatToOllama,
		From:    types.RelayFormatOpenAI,
		To:      types.RelayFormatOllama,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: reqConv.ConvertRequest,
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	// 流式响应侧：Ollama NDJSON → OpenAI SSE（方向与请求相反）。
	relayconvert.RegisterStreamConverter(
		types.RelayFormatOllama, types.RelayFormatOpenAI,
		relayconvert.ResponseConverterOllamaChatToOAIChatStream,
		(&ollama_chat.OllamaToOpenAIStreamConverter{}).ConvertStreamResponse,
	)
}

// registerReverseAndResponsesDirections 注册反向方向（非 OpenAI 入站 → OpenAI 上游）
// 与 Responses 双向转换器。
//
// 单侧注册：反向方向的请求侧与响应侧分属不同的配对 spec（如 Claude 入站 → OpenAI 上游，
// 请求走 c2o、响应走 o2c），故按方向分别注册，注册表允许某一侧为空。
func registerReverseAndResponsesDirections() {
	registerClaudeToOpenAI()
	registerGeminiToOpenAI()
	registerResponsesToOpenAI()
	registerOpenAIToResponses()
}

// registerClaudeToOpenAI 注册 Claude 入站 → OpenAI 上游方向。
// 请求侧 Claude → OpenAI；响应侧 OpenAI → Claude（含流式）。
func registerClaudeToOpenAI() {
	reqConv := &oai_chat.ClaudeToOpenAIRequestConverter{}
	respConv := &oai_chat.OpenAIToClaudeResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterClaudeMessagesToOpenAIChat,
		From:    types.RelayFormatClaude,
		To:      types.RelayFormatOpenAI,
		Quality: relayconvert.TextConverterQualityFair,
		Req: relayconvert.TextRequestSide{
			Convert: reqConv.ConvertRequest,
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	// 流式响应侧：OpenAI SSE → Claude 事件流（方向与请求相反）。
	relayconvert.RegisterStreamConverter(
		types.RelayFormatOpenAI, types.RelayFormatClaude,
		relayconvert.ResponseConverterOAIChatToClaudeMessagesStream,
		(&oai_chat.OpenAIToClaudeStreamConverter{}).ConvertStreamResponse,
	)
}

// registerGeminiToOpenAI 注册 Gemini 入站 → OpenAI 上游方向。
// 请求侧 Gemini → OpenAI；响应侧 OpenAI → Gemini（含流式）。
func registerGeminiToOpenAI() {
	reqConv := &oai_gemini.GeminiToOpenAIRequestConverter{}
	respConv := &oai_gemini.OpenAIToGeminiResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterGeminiContentToOpenAIChat,
		From:    types.RelayFormatGemini,
		To:      types.RelayFormatOpenAI,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: reqConv.ConvertRequest,
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	// 流式响应侧：OpenAI SSE → Gemini data 帧（方向与请求相反）。
	relayconvert.RegisterStreamConverter(
		types.RelayFormatOpenAI, types.RelayFormatGemini,
		relayconvert.ResponseConverterOAIChatToGeminiChatStream,
		(&oai_gemini.OpenAIToGeminiStreamConverter{}).ConvertStreamResponse,
	)
}

// registerResponsesToOpenAI 注册 Responses 入站 → OpenAI chat 上游方向。
// 请求侧 Responses → chat；响应侧 chat → Responses（含流式）。
func registerResponsesToOpenAI() {
	reqConv := &oai_responses.ResponsesToOpenAIRequestConverter{}
	respConv := &oai_responses.OpenAIToResponsesResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIResponsesToOpenAIChat,
		From:    types.RelayFormatOpenAIResponses,
		To:      types.RelayFormatOpenAI,
		Quality: relayconvert.TextConverterQualityFair,
		Req: relayconvert.TextRequestSide{
			Convert: reqConv.ConvertRequest,
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	// 流式响应侧：OpenAI chat SSE → Responses 事件流（方向与请求相反）。
	relayconvert.RegisterStreamConverter(
		types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses,
		relayconvert.ResponseConverterOAIChatToOAIResponsesStream,
		(&oai_responses.OpenAIToResponsesStreamConverter{}).ConvertStreamResponse,
	)
}

// registerOpenAIToResponses 注册 OpenAI chat 入站 → Responses 上游方向
// （渠道 ChatViaResponses 桥接：chat 请求经 /v1/responses 发送）。
// 请求侧 chat → Responses；响应侧 Responses → chat（含流式）。
func registerOpenAIToResponses() {
	reqConv := &oai_responses.OpenAIToResponsesRequestConverter{}
	respConv := &oai_responses.ResponsesToOpenAIResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIChatToOpenAIResponses,
		From:    types.RelayFormatOpenAI,
		To:      types.RelayFormatOpenAIResponses,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: reqConv.ConvertRequest,
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	// 流式响应侧：Responses 事件流 → OpenAI chat SSE（方向与请求相反）。
	relayconvert.RegisterStreamConverter(
		types.RelayFormatOpenAIResponses, types.RelayFormatOpenAI,
		relayconvert.ResponseConverterOAIResponsesToOAIChatStream,
		(&oai_responses.ResponsesToOpenAIStreamConverter{}).ConvertStreamResponse,
	)
}

// registerCrossNativeDirections 注册跨原生方向（Claude↔Gemini、Responses→Claude/Gemini）。
//
// 请求侧走「经 OpenAI 中枢」的步骤链（StepConverters）：如 Claude→Gemini 实际执行
// Claude→OpenAI→Gemini 两跳，各跳复用已注册的直连转换器，注册时校验 From/To 连续性；
// 响应侧为直连转换器（保真度高于两跳，thinking 签名等不经中间格式丢失）。
func registerCrossNativeDirections() {
	registerClaudeToGemini()
	registerGeminiToClaude()
	registerResponsesToClaude()
	registerResponsesToGemini()
}

// registerClaudeToGemini 注册 Claude 入站 → Gemini 上游。
// 请求链 Claude→OpenAI→Gemini；响应直连 Gemini→Claude（含流式）。
func registerClaudeToGemini() {
	respConv := &claude_gemini.GeminiToClaudeResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterClaudeMessagesToGeminiContent,
		From:    types.RelayFormatClaude,
		To:      types.RelayFormatGemini,
		Quality: relayconvert.TextConverterQualityFair,
		Req: relayconvert.TextRequestSide{
			StepConverters: []string{
				relayconvert.ConverterClaudeMessagesToOpenAIChat,
				relayconvert.ConverterOpenAIChatToGeminiContent,
			},
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	relayconvert.RegisterStreamConverter(
		types.RelayFormatGemini, types.RelayFormatClaude,
		relayconvert.ResponseConverterGeminiChatToClaudeMessagesStream,
		(&claude_gemini.GeminiToClaudeStreamConverter{}).ConvertStreamResponse,
	)
}

// registerGeminiToClaude 注册 Gemini 入站 → Claude 上游。
// 请求链 Gemini→OpenAI→Claude；响应直连 Claude→Gemini（含流式）。
func registerGeminiToClaude() {
	respConv := &claude_gemini.ClaudeToGeminiResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterGeminiContentToClaudeMessages,
		From:    types.RelayFormatGemini,
		To:      types.RelayFormatClaude,
		Quality: relayconvert.TextConverterQualityFair,
		Req: relayconvert.TextRequestSide{
			StepConverters: []string{
				relayconvert.ConverterGeminiContentToOpenAIChat,
				relayconvert.ConverterOpenAIChatToClaudeMessages,
			},
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	relayconvert.RegisterStreamConverter(
		types.RelayFormatClaude, types.RelayFormatGemini,
		relayconvert.ResponseConverterClaudeMessagesToGeminiChatStream,
		(&claude_gemini.ClaudeToGeminiStreamConverter{}).ConvertStreamResponse,
	)
}

// registerResponsesToClaude 注册 Responses 入站 → Claude 上游。
// 请求链 Responses→OpenAI→Claude；响应直连 Claude→Responses（含流式）。
func registerResponsesToClaude() {
	respConv := &native_responses.ClaudeToResponsesResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterResponsesToClaudeMessages,
		From:    types.RelayFormatOpenAIResponses,
		To:      types.RelayFormatClaude,
		Quality: relayconvert.TextConverterQualityFair,
		Req: relayconvert.TextRequestSide{
			StepConverters: []string{
				relayconvert.ConverterOpenAIResponsesToOpenAIChat,
				relayconvert.ConverterOpenAIChatToClaudeMessages,
			},
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	relayconvert.RegisterStreamConverter(
		types.RelayFormatClaude, types.RelayFormatOpenAIResponses,
		relayconvert.ResponseConverterClaudeMessagesToOAIResponsesStream,
		(&native_responses.ClaudeToResponsesStreamConverter{}).ConvertStreamResponse,
	)
}

// registerResponsesToGemini 注册 Responses 入站 → Gemini 上游。
// 请求链 Responses→OpenAI→Gemini；响应直连 Gemini→Responses（含流式）。
func registerResponsesToGemini() {
	respConv := &native_responses.GeminiToResponsesResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIResponsesToGemini,
		From:    types.RelayFormatOpenAIResponses,
		To:      types.RelayFormatGemini,
		Quality: relayconvert.TextConverterQualityFair,
		Req: relayconvert.TextRequestSide{
			StepConverters: []string{
				relayconvert.ConverterOpenAIResponsesToOpenAIChat,
				relayconvert.ConverterOpenAIChatToGeminiContent,
			},
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	relayconvert.RegisterStreamConverter(
		types.RelayFormatGemini, types.RelayFormatOpenAIResponses,
		relayconvert.ResponseConverterGeminiChatToOAIResponsesStream,
		(&native_responses.GeminiToResponsesStreamConverter{}).ConvertStreamResponse,
	)
}

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
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/coze_chat"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/dify_chat"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_chat"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_gemini"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_responses"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/ollama_chat"
	"github.com/qianfree/team-api/relaykit/types"
)

func init() {
	// Responses 方向须最先注册：后续的 responses→claude 链式 spec 引用其转换器 ID
	registerResponsesToOpenAIChat()
	registerOpenAIChatToResponses()
	registerOpenAIToClaude()
	// 链式 spec 依赖上述步骤转换器，须在其后注册
	registerResponsesToClaudeChain()
	registerOpenAIToGemini()
	// 剩余原生格式供应商
	registerOpenAIToCoze()
	registerOpenAIToDify()
	registerOpenAIToOllama()
}

// registerResponsesToOpenAIChat 注册 Responses → OpenAI Chat 方向转换器。
// 客户端说 Responses，上游说 OpenAI Chat（chat-only 渠道）：
//   - 请求侧 Responses → OpenAI Chat
//   - 响应侧（chat 上游 → Responses 客户端）：OpenAIChatToResponsesResponseConverter
//     （codex 打 chat-only 渠道的非流式响应合成）
func registerResponsesToOpenAIChat() {
	respConv := &oai_responses.OpenAIChatToResponsesResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIResponsesToOpenAIChat,
		From:    types.RelayFormatOpenAIResponses,
		To:      types.RelayFormatOpenAI,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: (&oai_responses.ResponsesToOpenAIChatRequestConverter{}).ConvertRequest,
		},
		Resp: relayconvert.TextResponseSide{
			// 与其余方向一致：usage 由宿主从转换后的响应体提取，注册表层丢弃
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, _, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})
}

// registerOpenAIChatToResponses 注册 OpenAI Chat → Responses 方向转换器（仅请求侧）。
// ChatViaResponses 渠道：chat 客户端桥接到 Responses 上游，请求侧 chat → Responses；
// 响应侧（Responses 上游 → chat 客户端）由宿主 chat_via_responses.go 承担。
func registerOpenAIChatToResponses() {
	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIChatToOpenAIResponses,
		From:    types.RelayFormatOpenAI,
		To:      types.RelayFormatOpenAIResponses,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: (&oai_responses.OpenAIChatToResponsesRequestConverter{}).ConvertRequest,
		},
	})
}

// registerResponsesToClaudeChain 注册 Responses → Claude 方向转换器。
//   - 请求侧为 StepConverters 两跳链（Responses→OpenAI Chat→Claude Messages），
//     替换旧路径 claude/converter.go 的手工拼接链 ConvertResponsesToClaude（后者保留为回退）；
//   - 响应侧（非流式）为 ClaudeToResponsesResponseConverter（spec 的 From/To 是请求方向语义，
//     Resp 侧做反向，与 ConverterOpenAIChatToClaudeMessages 先例一致）。
func registerResponsesToClaudeChain() {
	respConv := &oai_responses.ClaudeToResponsesResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:   relayconvert.ConverterOpenAIResponsesToClaudeMessages,
		From: types.RelayFormatOpenAIResponses,
		To:   types.RelayFormatClaude,
		// 多跳链路有中间格式信息损耗，质量标记为 fair
		Quality: relayconvert.TextConverterQualityFair,
		Req: relayconvert.TextRequestSide{
			StepConverters: []string{
				relayconvert.ConverterOpenAIResponsesToOpenAIChat,
				relayconvert.ConverterOpenAIChatToClaudeMessages,
			},
		},
		Resp: relayconvert.TextResponseSide{
			// 与其余方向一致：usage 由宿主从转换后的响应体提取，注册表层丢弃
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, _, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})

	// 流式响应侧：Claude SSE → Responses SSE（经独立流式注册表，宿主桥接层查找调用）
	relayconvert.RegisterStreamConverter(
		types.RelayFormatClaude, types.RelayFormatOpenAIResponses,
		relayconvert.ConverterClaudeMessagesToOpenAIResponsesStream,
		(&oai_responses.ClaudeToResponsesStreamConverter{}).ConvertStreamResponse,
	)
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

// registerOpenAIToCoze 注册 OpenAI → Coze 方向转换器。
// 客户端说 OpenAI，上游说 Coze v3：
//   - 请求侧 OpenAI → Coze
//   - 响应侧 Coze → OpenAI（非流式：解析缓冲 SSE；流式：SSE→SSE）
func registerOpenAIToCoze() {
	reqConv := &coze_chat.OpenAIToCozeRequestConverter{}
	respConv := &coze_chat.CozeToOpenAIResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIChatToCoze,
		From:    types.RelayFormatOpenAI,
		To:      types.RelayFormatCoze,
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

	// 流式响应侧：Coze SSE → OpenAI SSE（方向与请求相反）。
	relayconvert.RegisterStreamConverter(
		types.RelayFormatCoze, types.RelayFormatOpenAI,
		relayconvert.ResponseConverterCozeChatToOAIChatStream,
		(&coze_chat.CozeToOpenAIStreamConverter{}).ConvertStreamResponse,
	)
}

// registerOpenAIToDify 注册 OpenAI → Dify 方向转换器。
// 客户端说 OpenAI，上游说 Dify chat-messages：
//   - 请求侧 OpenAI → Dify
//   - 响应侧 Dify → OpenAI（非流式 blocking JSON；流式 SSE→SSE）
func registerOpenAIToDify() {
	reqConv := &dify_chat.OpenAIToDifyRequestConverter{}
	respConv := &dify_chat.DifyToOpenAIResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIChatToDify,
		From:    types.RelayFormatOpenAI,
		To:      types.RelayFormatDify,
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

	// 流式响应侧：Dify SSE → OpenAI SSE（方向与请求相反）。
	relayconvert.RegisterStreamConverter(
		types.RelayFormatDify, types.RelayFormatOpenAI,
		relayconvert.ResponseConverterDifyChatToOAIChatStream,
		(&dify_chat.DifyToOpenAIStreamConverter{}).ConvertStreamResponse,
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

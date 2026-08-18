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
	"encoding/json"
	"io"

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
	// Claude/Gemini 入站方向（P1-A）：spec B（gemini→openai）必须先于链 spec C 注册
	registerClaudeToOpenAIChat()
	registerGeminiToOpenAIChat()
	registerGeminiToClaudeChain()
	// P2：openai 上游 → claude/gemini 客户端的流式响应方向
	registerOpenAIToClaudeGeminiStreams()
	registerOpenAIToGemini()
	// 跨原生链（claude↔gemini / responses→gemini）依赖 openai→gemini 步骤，须在其后
	registerCrossNativeChains()
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

	// 流式响应侧：chat SSE → Responses SSE（codex 打 chat-only 渠道的流式主路径）
	relayconvert.RegisterStreamConverter(
		types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses,
		relayconvert.ConverterOpenAIChatToOpenAIResponsesStream,
		(&oai_responses.OpenAIChatToResponsesStreamConverter{}).ConvertStreamResponse,
	)
}

// registerOpenAIToClaudeGeminiStreams 注册 P2 的两个流式响应方向（openai 上游 →
// claude/gemini 客户端——Claude Code / Gemini 客户端打 openai 兼容渠道的流式）。
func registerOpenAIToClaudeGeminiStreams() {
	relayconvert.RegisterStreamConverter(
		types.RelayFormatOpenAI, types.RelayFormatClaude,
		relayconvert.ConverterOpenAIChatToClaudeMessagesStream,
		(&oai_chat.OpenAIToClaudeStreamConverter{}).ConvertStreamResponse,
	)
	relayconvert.RegisterStreamConverter(
		types.RelayFormatOpenAI, types.RelayFormatGemini,
		relayconvert.ConverterOpenAIChatToGeminiContentStream,
		(&oai_gemini.OpenAIToGeminiStreamConverter{}).ConvertStreamResponse,
	)
}

// registerOpenAIChatToResponses 注册 OpenAI Chat → Responses 方向转换器。
// ChatViaResponses 渠道：chat 客户端桥接到 Responses 上游：
//   - 请求侧 chat → Responses
//   - 响应侧（Responses 上游 → chat 客户端，非流式）：ResponsesToOpenAIChatResponseConverter
//     （流式侧由宿主 HandleResponsesStreamToChat 承担——依赖 StreamScannerHandler 超时治理，未收编）
func registerOpenAIChatToResponses() {
	respConv := &oai_responses.ResponsesToOpenAIChatResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterOpenAIChatToOpenAIResponses,
		From:    types.RelayFormatOpenAI,
		To:      types.RelayFormatOpenAIResponses,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: (&oai_responses.OpenAIChatToResponsesRequestConverter{}).ConvertRequest,
		},
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, _, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
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

// registerClaudeToOpenAIChat 注册 Claude → OpenAI Chat 方向转换器（spec A，P1-A）。
// 客户端说 Claude，上游说 OpenAI Chat：
//   - 请求侧 Claude → OpenAI Chat（宿主接管点在共享函数 ConvertToOpenAI 内部，
//     各 adaptor 的定制后处理照常执行）
//   - 响应侧（openai 上游 → claude 客户端，非流式）由 P1-B 的 Resp 侧承担
func registerClaudeToOpenAIChat() {
	respConv := &oai_chat.OpenAIToClaudeResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterClaudeMessagesToOpenAIChat,
		From:    types.RelayFormatClaude,
		To:      types.RelayFormatOpenAI,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: (&oai_chat.ClaudeToOpenAIRequestConverter{}).ConvertRequest,
		},
		// Resp 侧方向反转约定：spec 的 From/To 是请求方向语义，Resp 实际转换
		// openai 上游 → claude 客户端（与 ConverterOpenAIChatToClaudeMessages 先例一致）
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, _, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})
}

// registerGeminiToOpenAIChat 注册 Gemini → OpenAI Chat 方向转换器（spec B，P1-A）。
// 客户端说 Gemini，上游说 OpenAI Chat（宿主接管点同 spec A）。
// 注意：本 spec 必须先于 gemini→claude 链 spec 注册（链引用其转换器 ID）。
func registerGeminiToOpenAIChat() {
	respConv := &oai_gemini.OpenAIToGeminiResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:      relayconvert.ConverterGeminiContentToOpenAIChat,
		From:    types.RelayFormatGemini,
		To:      types.RelayFormatOpenAI,
		Quality: relayconvert.TextConverterQualityGood,
		Req: relayconvert.TextRequestSide{
			Convert: (&oai_gemini.GeminiToOpenAIRequestConverter{}).ConvertRequest,
		},
		// Resp 侧方向反转约定：实际转换 openai 上游 → gemini 客户端
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				result, _, err := respConv.ConvertResponse(ctx, info, response)
				return result, nil, err
			},
		},
	})
}

// registerGeminiToClaudeChain 注册 Gemini → Claude 请求链（spec C，P1-A）。
// P2 补 Resp 侧（claude 上游 → gemini 客户端，#18 修复）：两跳组合
// ClaudeToOpenAIResponseConverter → OpenAIToGeminiResponseConverter。
func registerGeminiToClaudeChain() {
	c2oResp := &oai_chat.ClaudeToOpenAIResponseConverter{}
	o2gResp := &oai_gemini.OpenAIToGeminiResponseConverter{}

	relayconvert.RegisterTextConverter(relayconvert.TextConverterSpec{
		ID:   relayconvert.ConverterGeminiContentToClaudeMessages,
		From: types.RelayFormatGemini,
		To:   types.RelayFormatClaude,
		// 多跳链路有中间格式信息损耗，质量标记为 fair
		Quality: relayconvert.TextConverterQualityFair,
		Req: relayconvert.TextRequestSide{
			StepConverters: []string{
				relayconvert.ConverterGeminiContentToOpenAIChat,
				relayconvert.ConverterOpenAIChatToClaudeMessages,
			},
		},
		// Resp 方向反转：实际转换 claude 上游 → gemini 客户端（两跳组合）
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				chatResp, err := c2oResp.ConvertResponse(ctx, info, response)
				if err != nil {
					return nil, nil, err
				}
				geminiResp, _, err := o2gResp.ConvertResponse(ctx, info, chatResp)
				if err != nil {
					return nil, nil, err
				}
				return geminiResp, nil, nil
			},
		},
	})
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

// registerCrossNativeChains 注册 P2 的跨原生链（claude↔gemini 交叉客户端 + Responses→Gemini）。
// 请求链均为 StepConverters 两跳；响应/流式侧由组合函数承担（Resp 直挂组合、流式 io.Pipe 串联）。
func registerCrossNativeChains() {
	registerClaudeToGeminiChain()
	registerResponsesToGeminiChain()
	registerCrossStreamChains()
}

// registerClaudeToGeminiChain Claude 客户端 → Gemini 上游（链：claude→openai→gemini）。
// 替换宿主 gemini/converter.go 的手工拼接链 ConvertClaudeToGemini（保留为回退）。
// Resp 侧（gemini→claude 响应）：ClaudeToOpenAIResponseConverter → OpenAIToGeminiResponseConverter 组合。
func registerClaudeToGeminiChain() {
	g2oResp := &oai_gemini.GeminiToOpenAIResponseConverter{}
	o2cResp := &oai_chat.OpenAIToClaudeResponseConverter{}

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
		// Resp 方向反转：实际转换 gemini 上游 → claude 客户端（两跳组合，无单实现）
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				chatResp, err := g2oResp.ConvertResponse(ctx, info, response)
				if err != nil {
					return nil, nil, err
				}
				claudeResp, _, err := o2cResp.ConvertResponse(ctx, info, chatResp)
				if err != nil {
					return nil, nil, err
				}
				return claudeResp, nil, nil
			},
		},
	})
}

// registerResponsesToGeminiChain Responses 客户端 → Gemini 上游（链：responses→openai→gemini）。
// 替换宿主 gemini/converter.go 的手工拼接链 ConvertResponsesToGemini（保留为回退）。
// Resp 侧（gemini→responses 响应）：GeminiToOpenAI → OpenAIChatToResponses 组合。
func registerResponsesToGeminiChain() {
	g2oResp := &oai_gemini.GeminiToOpenAIResponseConverter{}
	o2rResp := &oai_responses.OpenAIChatToResponsesResponseConverter{}

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
		// Resp 方向反转：实际转换 gemini 上游 → responses 客户端（两跳组合）
		Resp: relayconvert.TextResponseSide{
			Convert: func(ctx context.Context, info convmeta.Meta, response any) (any, *dto.Usage, error) {
				chatResp, err := g2oResp.ConvertResponse(ctx, info, response)
				if err != nil {
					return nil, nil, err
				}
				responsesResp, _, err := o2rResp.ConvertResponse(ctx, info, chatResp)
				if err != nil {
					return nil, nil, err
				}
				return responsesResp, nil, nil
			},
		},
	})
}

// chainStreamConverters 流式两跳组合：第一跳的 chat chunk 输出序列化为 SSE data: 行
// 写入 io.Pipe，第二跳从 pipe 读取解析（两个转换器均消费 chat SSE 帧格式）。
// 第一跳错误经 CloseWithError 传递给第二跳的 scanner（表现为 scanner 错误）。
func chainStreamConverters(first, second relayconvert.StreamConverterFunc) relayconvert.StreamConverterFunc {
	return func(ctx context.Context, info convmeta.Meta, reader io.Reader, chunkWriter func(chunk any) error) error {
		pr, pw := io.Pipe()
		go func() {
			err := first(ctx, info, reader, func(chunk any) error {
				streamChunk, ok := chunk.(*dto.ChatCompletionStreamResponse)
				if !ok {
					return nil
				}
				data, err := json.Marshal(streamChunk)
				if err != nil {
					return err
				}
				_, err = pw.Write([]byte("data: " + string(data) + "\n\n"))
				return err
			})
			_ = pw.CloseWithError(err) // nil 错误 → 正常 EOF
		}()
		return second(ctx, info, pr, chunkWriter)
	}
}

// registerCrossStreamChains 注册三条跨原生流式组合链。
func registerCrossStreamChains() {
	// claude 上游 → gemini 客户端：claude→openai + openai→gemini
	relayconvert.RegisterStreamConverter(
		types.RelayFormatClaude, types.RelayFormatGemini,
		"anthropic_messages_to_gemini_generate_content_stream",
		chainStreamConverters(
			(&oai_chat.ClaudeToOpenAIStreamConverter{}).ConvertStreamResponse,
			(&oai_gemini.OpenAIToGeminiStreamConverter{}).ConvertStreamResponse,
		),
	)
	// gemini 上游 → claude 客户端：gemini→openai + openai→claude
	relayconvert.RegisterStreamConverter(
		types.RelayFormatGemini, types.RelayFormatClaude,
		"gemini_generate_content_to_anthropic_messages_stream",
		chainStreamConverters(
			(&oai_gemini.GeminiToOpenAIStreamConverter{}).ConvertStreamResponse,
			(&oai_chat.OpenAIToClaudeStreamConverter{}).ConvertStreamResponse,
		),
	)
	// gemini 上游 → responses 客户端：gemini→openai + openai→responses
	relayconvert.RegisterStreamConverter(
		types.RelayFormatGemini, types.RelayFormatOpenAIResponses,
		"gemini_generate_content_to_oai_responses_stream",
		chainStreamConverters(
			(&oai_gemini.GeminiToOpenAIStreamConverter{}).ConvertStreamResponse,
			(&oai_responses.OpenAIChatToResponsesStreamConverter{}).ConvertStreamResponse,
		),
	)
}

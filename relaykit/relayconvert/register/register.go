// Package register 阶段 4：将阶段 3 实现的内置转换器注册进运行时注册表。
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
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_chat"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_gemini"
	"github.com/qianfree/team-api/relaykit/types"
)

func init() {
	registerOpenAIToClaude()
	registerOpenAIToGemini()
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

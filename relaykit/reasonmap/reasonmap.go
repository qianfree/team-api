package reasonmap

import (
	"strings"

	"github.com/qianfree/team-api/relaykit/types"
)

func ClaudeStopReasonToOpenAIFinishReason(stopReason string) string {
	switch strings.ToLower(stopReason) {
	case "stop_sequence":
		return "stop"
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "refusal":
		return types.FinishReasonContentFilter
	default:
		return stopReason
	}
}

func OpenAIFinishReasonToClaudeStopReason(finishReason string) string {
	switch strings.ToLower(finishReason) {
	case "stop":
		return "end_turn"
	case "stop_sequence":
		return "stop_sequence"
	case "length", "max_tokens":
		return "max_tokens"
	case types.FinishReasonContentFilter:
		return "refusal"
	case "tool_calls":
		return "tool_use"
	default:
		return finishReason
	}
}

// OpenAIFinishReasonToClaudeLegacySemantics 复刻宿主 relay/common.OpenAIFinishReasonToClaude
// 的精确语义（与上方函数的差异：不 ToLower——未知值原样透传保留大小写；空串→end_turn 而非空串）。
// P1-B 的 openai→claude 响应转换器使用本函数保持与 legacy 字节级一致。
func OpenAIFinishReasonToClaudeLegacySemantics(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case types.FinishReasonContentFilter:
		return "refusal"
	default:
		if reason != "" {
			return reason
		}
		return "end_turn"
	}
}

// OpenAIFinishReasonToGeminiFinishReason 复刻宿主 relay/common.OpenAIFinishReasonToGemini
// 的精确语义（注意 legacy 怪癖：tool_calls→STOP 而非语义对应值；未知值原样透传；空串透传空串）。
func OpenAIFinishReasonToGeminiFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "STOP"
	case "length":
		return "MAX_TOKENS"
	case types.FinishReasonContentFilter:
		return "SAFETY"
	case "tool_calls":
		return "STOP"
	default:
		return reason
	}
}

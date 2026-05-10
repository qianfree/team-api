package common

// OpenAI stop reason / finish_reason values
const (
	OpenAIStop          = "stop"
	OpenAILength        = "length"
	OpenAIToolCalls     = "tool_calls"
	OpenAIContentFilter = "content_filter"
)

// Claude stop_reason values
const (
	ClaudeEndTurn      = "end_turn"
	ClaudeMaxTokens    = "max_tokens"
	ClaudeToolUse      = "tool_use"
	ClaudeStopSequence = "stop_sequence"
	ClaudePauseTurn    = "pause_turn" // 扩展思考暂停
	ClaudeRefusal      = "refusal"    // 内容拒绝
)

// Gemini finishReason values
const (
	GeminiSTOP                      = "STOP"
	GeminiMAX_TOKENS                = "MAX_TOKENS"
	GeminiSAFETY                    = "SAFETY"
	GeminiRECITATION                = "RECITATION"
	GeminiOTHER                     = "OTHER"
	GeminiBLOCKLIST                 = "BLOCKLIST"
	GeminiPROHIBITED                = "PROHIBITED"
	GeminiSPII                      = "SPII"
	GeminiMALFORMED_FUNCTION_CALL   = "MALFORMED_FUNCTION_CALL"
	GeminiLANGUAGE                  = "LANGUAGE"
	GeminiIMAGE_SAFETY              = "IMAGE_SAFETY"
	GeminiIMAGE_PROHIBITED_CONTENT  = "IMAGE_PROHIBITED_CONTENT"
	GeminiIMAGE_OTHER               = "IMAGE_OTHER"
	GeminiNO_IMAGE                  = "NO_IMAGE"
	GeminiIMAGE_RECITATION          = "IMAGE_RECITATION"
	GeminiUNEXPECTED_TOOL_CALL      = "UNEXPECTED_TOOL_CALL"
	GeminiTOO_MANY_TOOL_CALLS       = "TOO_MANY_TOOL_CALLS"
	GeminiMISSING_THOUGHT_SIGNATURE = "MISSING_THOUGHT_SIGNATURE"
	GeminiMALFORMED_RESPONSE        = "MALFORMED_RESPONSE"
	GeminiFINISH_REASON_UNSPECIFIED = "FINISH_REASON_UNSPECIFIED"
)

// OpenAIFinishReasonToClaude 将 OpenAI finish_reason 转换为 Claude stop_reason
func OpenAIFinishReasonToClaude(reason string) string {
	switch reason {
	case OpenAIStop:
		return ClaudeEndTurn
	case OpenAILength:
		return ClaudeMaxTokens
	case OpenAIToolCalls:
		return ClaudeToolUse
	case OpenAIContentFilter:
		return ClaudeRefusal
	default:
		if reason != "" {
			return reason
		}
		return ClaudeEndTurn
	}
}

// ClaudeStopReasonToOpenAI 将 Claude stop_reason 转换为 OpenAI finish_reason
func ClaudeStopReasonToOpenAI(reason string) string {
	switch reason {
	case ClaudeEndTurn, ClaudeStopSequence, ClaudePauseTurn:
		return OpenAIStop
	case ClaudeMaxTokens:
		return OpenAILength
	case ClaudeToolUse:
		return OpenAIToolCalls
	case ClaudeRefusal:
		return OpenAIContentFilter
	default:
		return reason
	}
}

// GeminiFinishReasonToOpenAI 将 Gemini finishReason 转换为 OpenAI finish_reason
func GeminiFinishReasonToOpenAI(reason string) string {
	switch reason {
	case GeminiSTOP, GeminiFINISH_REASON_UNSPECIFIED:
		return OpenAIStop
	case GeminiMAX_TOKENS:
		return OpenAILength
	case GeminiSAFETY, GeminiRECITATION, GeminiOTHER, GeminiBLOCKLIST, GeminiPROHIBITED, GeminiSPII,
		GeminiLANGUAGE, GeminiIMAGE_SAFETY, GeminiIMAGE_PROHIBITED_CONTENT, GeminiIMAGE_OTHER,
		GeminiNO_IMAGE, GeminiIMAGE_RECITATION:
		return OpenAIContentFilter
	case GeminiMALFORMED_FUNCTION_CALL, GeminiUNEXPECTED_TOOL_CALL, GeminiTOO_MANY_TOOL_CALLS:
		return OpenAIToolCalls
	case "TOOL_CALLS":
		return OpenAIToolCalls
	case GeminiMISSING_THOUGHT_SIGNATURE, GeminiMALFORMED_RESPONSE:
		return OpenAIStop
	default:
		return reason
	}
}

// OpenAIFinishReasonToGemini 将 OpenAI finish_reason 转换为 Gemini finishReason
func OpenAIFinishReasonToGemini(reason string) string {
	switch reason {
	case OpenAIStop:
		return GeminiSTOP
	case OpenAILength:
		return GeminiMAX_TOKENS
	case OpenAIContentFilter:
		return GeminiSAFETY
	case OpenAIToolCalls:
		return GeminiSTOP
	default:
		return reason
	}
}

// ClaudeStopReasonToGemini 将 Claude stop_reason 转换为 Gemini finishReason
func ClaudeStopReasonToGemini(reason string) string {
	switch reason {
	case ClaudeEndTurn, ClaudeStopSequence, ClaudePauseTurn:
		return GeminiSTOP
	case ClaudeMaxTokens:
		return GeminiMAX_TOKENS
	case ClaudeToolUse:
		return GeminiSTOP
	case ClaudeRefusal:
		return GeminiSAFETY
	default:
		return reason
	}
}

// GeminiFinishReasonToClaude 将 Gemini finishReason 转换为 Claude stop_reason
func GeminiFinishReasonToClaude(reason string) string {
	switch reason {
	case GeminiSTOP, GeminiFINISH_REASON_UNSPECIFIED, GeminiMISSING_THOUGHT_SIGNATURE, GeminiMALFORMED_RESPONSE:
		return ClaudeEndTurn
	case GeminiMAX_TOKENS:
		return ClaudeMaxTokens
	case GeminiSAFETY, GeminiRECITATION, GeminiOTHER, GeminiBLOCKLIST, GeminiPROHIBITED, GeminiSPII,
		GeminiLANGUAGE, GeminiIMAGE_SAFETY, GeminiIMAGE_PROHIBITED_CONTENT, GeminiIMAGE_OTHER,
		GeminiNO_IMAGE, GeminiIMAGE_RECITATION:
		return ClaudeRefusal
	case GeminiMALFORMED_FUNCTION_CALL, GeminiUNEXPECTED_TOOL_CALL, GeminiTOO_MANY_TOOL_CALLS:
		return ClaudeToolUse
	default:
		return reason
	}
}

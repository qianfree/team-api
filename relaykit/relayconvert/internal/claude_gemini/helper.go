// Package claude_gemini 承载 Claude Messages ↔ Gemini GenerateContent 的跨原生响应桥。
//
// 请求侧由两段式请求转换完成（Gemini↔OpenAI↔Claude），本包只做响应侧：
// 把一方上游的响应/SSE 转回另一方客户端格式。代码从宿主
// relay/channel/claude/gemini_bridge.go 与 relay/channel/gemini/claude_bridge.go 逐行平移，
// 宿主侧的 SSE 写出、SetFirstResponseTime、StreamStatus、ping 保活不在本层，
// 流式输出统一以 relayconvert.StreamEvent 交由宿主桥接层帧化。
package claude_gemini

import (
	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// ========== stop_reason / finishReason 映射（与宿主 relay/common/reasonmap.go 同口径） ==========

// Claude stop_reason 取值
const (
	claudeEndTurn      = "end_turn"
	claudeMaxTokens    = "max_tokens"
	claudeToolUse      = "tool_use"
	claudeStopSequence = "stop_sequence"
	claudePauseTurn    = "pause_turn" // 扩展思考暂停
	claudeRefusal      = "refusal"    // 内容拒绝
)

// Gemini finishReason 取值
const (
	geminiSTOP                      = "STOP"
	geminiMAX_TOKENS                = "MAX_TOKENS"
	geminiSAFETY                    = "SAFETY"
	geminiRECITATION                = "RECITATION"
	geminiOTHER                     = "OTHER"
	geminiBLOCKLIST                 = "BLOCKLIST"
	geminiPROHIBITED                = "PROHIBITED"
	geminiSPII                      = "SPII"
	geminiMALFORMED_FUNCTION_CALL   = "MALFORMED_FUNCTION_CALL"
	geminiLANGUAGE                  = "LANGUAGE"
	geminiIMAGE_SAFETY              = "IMAGE_SAFETY"
	geminiIMAGE_PROHIBITED_CONTENT  = "IMAGE_PROHIBITED_CONTENT"
	geminiIMAGE_OTHER               = "IMAGE_OTHER"
	geminiNO_IMAGE                  = "NO_IMAGE"
	geminiIMAGE_RECITATION          = "IMAGE_RECITATION"
	geminiUNEXPECTED_TOOL_CALL      = "UNEXPECTED_TOOL_CALL"
	geminiTOO_MANY_TOOL_CALLS       = "TOO_MANY_TOOL_CALLS"
	geminiMISSING_THOUGHT_SIGNATURE = "MISSING_THOUGHT_SIGNATURE"
	geminiMALFORMED_RESPONSE        = "MALFORMED_RESPONSE"
	geminiFINISH_REASON_UNSPECIFIED = "FINISH_REASON_UNSPECIFIED"
)

// claudeStopReasonToGemini 将 Claude stop_reason 转换为 Gemini finishReason
func claudeStopReasonToGemini(reason string) string {
	switch reason {
	case claudeEndTurn, claudeStopSequence, claudePauseTurn:
		return geminiSTOP
	case claudeMaxTokens:
		return geminiMAX_TOKENS
	case claudeToolUse:
		return geminiSTOP
	case claudeRefusal:
		return geminiSAFETY
	default:
		return reason
	}
}

// geminiFinishReasonToClaude 将 Gemini finishReason 转换为 Claude stop_reason
func geminiFinishReasonToClaude(reason string) string {
	switch reason {
	case geminiSTOP, geminiFINISH_REASON_UNSPECIFIED, geminiMISSING_THOUGHT_SIGNATURE, geminiMALFORMED_RESPONSE:
		return claudeEndTurn
	case geminiMAX_TOKENS:
		return claudeMaxTokens
	case geminiSAFETY, geminiRECITATION, geminiOTHER, geminiBLOCKLIST, geminiPROHIBITED, geminiSPII,
		geminiLANGUAGE, geminiIMAGE_SAFETY, geminiIMAGE_PROHIBITED_CONTENT, geminiIMAGE_OTHER,
		geminiNO_IMAGE, geminiIMAGE_RECITATION:
		return claudeRefusal
	case geminiMALFORMED_FUNCTION_CALL, geminiUNEXPECTED_TOOL_CALL, geminiTOO_MANY_TOOL_CALLS:
		return claudeToolUse
	default:
		return reason
	}
}

// ========== Meta 相关辅助 ==========

// originModelName 取入站模型名（nil Meta 安全）
func originModelName(info convmeta.Meta) string {
	if info == nil {
		return ""
	}
	return info.GetOriginModelName()
}

// isModelMapped 判断是否经过模型名映射（宿主 info.ChannelMeta.IsModelMapped 的等价判定：
// 附加了渠道信息且上游模型名与入站模型名不同）
func isModelMapped(info convmeta.Meta) bool {
	if info == nil || !info.HasChannelMeta() {
		return false
	}
	upstream := info.GetUpstreamModelName()
	return upstream != "" && upstream != info.GetOriginModelName()
}

// ========== 计费用量换算（StreamEvent.Usage 载荷，带缓存明细） ==========

// claudeBillingUsage 按 Claude 计费口径换算（与宿主 buildUsageFromClaude 等价：
// input 不含缓存，cache 读/写明细随 PromptTokensDetails 透传）
func claudeBillingUsage(u *dto.ClaudeUsage) *dto.UsageWithDetails {
	if u == nil {
		return &dto.UsageWithDetails{}
	}
	return &dto.UsageWithDetails{
		PromptTokens:        u.InputTokens,
		CompletionTokens:    u.OutputTokens,
		TotalTokens:         u.InputTokens + u.OutputTokens,
		PromptTokensDetails: claudeUsageToTokenDetails(u),
	}
}

// claudeUsageToTokenDetails 将 ClaudeUsage 转换为 TokenDetails（含 cache token 细分），
// 与宿主同名函数等价
func claudeUsageToTokenDetails(u *dto.ClaudeUsage) *dto.TokenDetails {
	if u == nil {
		return nil
	}
	td := &dto.TokenDetails{
		CachedTokens:         u.CacheReadInputTokens,
		CachedCreationTokens: u.CacheCreationInputTokens,
	}
	if u.CacheCreation != nil {
		td.CachedCreation5mTokens = u.CacheCreation.Ephemeral5mInputTokens
		td.CachedCreation1hTokens = u.CacheCreation.Ephemeral1hInputTokens
	}
	return td
}

// geminiBillingUsage 按 Gemini 计费口径换算（与宿主 geminiUsageToCommon 等价：
// prompt 含 cached、completion 含 thoughts、模态明细透传）。
// 宿主 common.Usage 的 CacheIncludedInPrompt 标志在 UsageWithDetails 上无对应字段，
// 由宿主桥接层按「Gemini 上游」方向补挂。
func geminiBillingUsage(um *dto.GeminiUsageMetadata) *dto.UsageWithDetails {
	if um == nil {
		return &dto.UsageWithDetails{}
	}
	usage := &dto.UsageWithDetails{
		PromptTokens: um.PromptTokenCount,
		// Gemini 的 candidatesTokenCount 不含思考 token，thoughtsTokenCount 是输出侧
		// 独立字段（按输出价计费）。OpenAI 口径的 completion 含 reasoning（子集语义），
		// 计费用量必须 candidates+thoughts 合计，否则思考 token 漏计费
		CompletionTokens: um.CandidatesTokenCount + um.ThoughtsTokenCount,
		TotalTokens:      um.TotalTokenCount,
		// Gemini 的 promptTokenCount 已含 cachedContentTokenCount（cached 为其子集）
		PromptTokensDetails: &dto.TokenDetails{
			CachedTokens: um.CachedContentTokenCount,
		},
		CompletionTokenDetails: &dto.TokenDetails{
			ReasoningTokens: um.ThoughtsTokenCount,
		},
	}

	// 转换模态 Token 明细
	for _, mtc := range um.PromptTokensDetails {
		geminiModalityToTokenDetails(mtc, usage.PromptTokensDetails)
	}
	for _, mtc := range um.CandidatesTokensDetails {
		geminiModalityToTokenDetails(mtc, usage.CompletionTokenDetails)
	}

	return usage
}

// geminiModalityToTokenDetails 将 Gemini 模态 Token 计数转换为 TokenDetails 字段
func geminiModalityToTokenDetails(mtc dto.GeminiModalityTokenCount, td *dto.TokenDetails) {
	if td == nil {
		return
	}
	switch mtc.Modality {
	case "TEXT":
		td.TextTokens += mtc.TokenCount
	case "IMAGE":
		td.ImageTokens += mtc.TokenCount
	case "AUDIO":
		td.AudioTokens += mtc.TokenCount
	}
}

// ========== 指针辅助 ==========

// boolPtr 返回 bool 的指针
func boolPtr(v bool) *bool { return &v }

// intPtr 返回 int 的指针
func intPtr(v int) *int { return &v }

// strPtr 返回 string 的指针
func strPtr(v string) *string { return &v }

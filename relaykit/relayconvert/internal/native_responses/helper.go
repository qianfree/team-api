// Package native_responses 承载「Claude / Gemini 原生上游 → OpenAI Responses 客户端」的响应桥。
//
// 请求侧由两段式请求转换完成（Responses→OpenAI→Claude/Gemini），本包只做响应侧：
// 把原生上游的响应/SSE 转成 Responses 格式。代码从宿主
// relay/channel/claude/responses_bridge.go 与 relay/channel/gemini/responses_bridge.go 逐行平移；
// 宿主 openai 包的 EmitResponsesSSE / BuildResponsesObjectMap / BuildResponsesUsageMap
// 的纯逻辑部分在本包内做私有等价实现，事件发射改为 relayconvert.StreamEvent。
package native_responses

import (
	"encoding/json"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// ========== Responses 对象装配（宿主 openai.BuildResponsesObjectMap / BuildResponsesUsageMap 的等价实现） ==========

// responsesRequestEcho 合成 Responses 响应时需回显（echo）的请求参数
type responsesRequestEcho struct {
	temperature     *float64
	topP            *float64
	maxOutputTokens *int
	instructions    any
}

// extractResponsesRequestEcho 从 ResponsesStash 能力接口（宿主 stash 的请求快照）提取
// 合成响应应 echo 的请求参数。info 未实现该接口或快照缺失（直连路径/异常）时
// 回退 OpenAI 默认值（temperature=1.0 / top_p=1.0，其余 nil）。
func extractResponsesRequestEcho(info convmeta.Meta) responsesRequestEcho {
	echo := responsesRequestEcho{temperature: float64Ptr(1.0), topP: float64Ptr(1.0)}
	stash, ok := info.(convmeta.ResponsesStash)
	if !ok {
		return echo
	}
	rr := stash.StashedResponsesRequest()
	if rr == nil {
		return echo
	}
	if rr.Temperature != nil {
		echo.temperature = rr.Temperature
	}
	if rr.TopP != nil {
		echo.topP = rr.TopP
	}
	if rr.MaxOutputTokens != nil {
		m := int(*rr.MaxOutputTokens)
		echo.maxOutputTokens = &m
	}
	if len(rr.Instructions) > 0 {
		echo.instructions = json.RawMessage(rr.Instructions)
	}
	return echo
}

// buildResponsesObjectMap 构建 Responses API response 对象的完整字段 map。
// 请求参数从 ResponsesStash 快照 echo（快照缺失时回退默认值）；
// store 恒为 false——合成响应不落上游存储，客户端不可经生命周期端点 retrieve。
func buildResponsesObjectMap(respID string, createdAt int, status string, model string, output any, usageObj map[string]any, completedAt *int, info convmeta.Meta) map[string]any {
	echo := extractResponsesRequestEcho(info)
	m := map[string]any{
		"id":                   respID,
		"object":               "response",
		"created_at":           createdAt,
		"status":               status,
		"error":                nil,
		"incomplete_details":   nil,
		"instructions":         echo.instructions,
		"max_output_tokens":    echo.maxOutputTokens,
		"model":                model,
		"output":               output,
		"parallel_tool_calls":  true,
		"previous_response_id": nil,
		"reasoning":            map[string]any{"effort": nil, "summary": nil},
		"store":                false,
		"temperature":          *echo.temperature,
		"text":                 map[string]any{"format": map[string]any{"type": "text"}},
		"tool_choice":          "auto",
		"tools":                []any{},
		"top_p":                *echo.topP,
		"truncation":           "disabled",
		"user":                 nil,
		"metadata":             map[string]any{},
	}
	if completedAt != nil {
		m["completed_at"] = *completedAt
	}
	if usageObj != nil {
		m["usage"] = usageObj
	}
	return m
}

// buildResponsesUsageMap 构建 Responses API usage 对象
func buildResponsesUsageMap(usage *dto.UsageWithDetails) map[string]any {
	inputDetails := map[string]any{"cached_tokens": 0}
	outputDetails := map[string]any{"reasoning_tokens": 0}
	if usage.PromptTokensDetails != nil {
		inputDetails = map[string]any{
			"cached_tokens":      usage.PromptTokensDetails.CachedTokens,
			"cache_write_tokens": usage.PromptTokensDetails.CacheWriteTokens,
			"audio_tokens":       usage.PromptTokensDetails.AudioTokens,
		}
	}
	if usage.CompletionTokenDetails != nil {
		outputDetails = map[string]any{
			"reasoning_tokens":           usage.CompletionTokenDetails.ReasoningTokens,
			"audio_tokens":               usage.CompletionTokenDetails.AudioTokens,
			"accepted_prediction_tokens": usage.CompletionTokenDetails.AcceptedPredictionTokens,
			"rejected_prediction_tokens": usage.CompletionTokenDetails.RejectedPredictionTokens,
		}
	}
	return map[string]any{
		"input_tokens":          usage.PromptTokens,
		"output_tokens":         usage.CompletionTokens,
		"total_tokens":          usage.TotalTokens,
		"input_tokens_details":  inputDetails,
		"output_tokens_details": outputDetails,
	}
}

// ========== 用量换算 ==========

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

// claudeVisibleUsage 按 OpenAI 语义换算 Claude 用量（Responses 客户端可见值：
// input 含缓存，cached 为子集）
func claudeVisibleUsage(u *dto.ClaudeUsage) *dto.UsageWithDetails {
	if u == nil {
		return &dto.UsageWithDetails{}
	}
	promptTotal := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
	return &dto.UsageWithDetails{
		PromptTokens:        promptTotal,
		CompletionTokens:    u.OutputTokens,
		TotalTokens:         promptTotal + u.OutputTokens,
		PromptTokensDetails: claudeUsageToTokenDetails(u),
	}
}

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

// geminiUsageToDetails 按 Gemini 计费口径换算（与宿主 geminiUsageToCommon 等价：
// prompt 含 cached、completion 含 thoughts、模态明细透传）。
// 本方向 Responses 的客户端可见 usage 与计费口径**恰好一致**（都是 OpenAI 语义），
// 故可见值与 StreamEvent.Usage 共用本函数。宿主 common.Usage 的 CacheIncludedInPrompt
// 标志在 UsageWithDetails 上无对应字段，由宿主桥接层按「Gemini 上游」方向补挂。
func geminiUsageToDetails(um *dto.GeminiUsageMetadata) *dto.UsageWithDetails {
	if um == nil {
		return &dto.UsageWithDetails{}
	}
	usage := &dto.UsageWithDetails{
		PromptTokens: um.PromptTokenCount,
		// Gemini 的 candidatesTokenCount 不含思考 token，OpenAI 口径的 completion 含
		// reasoning（子集语义），必须 candidates+thoughts 合计，否则思考 token 漏计
		CompletionTokens: um.CandidatesTokenCount + um.ThoughtsTokenCount,
		TotalTokens:      um.TotalTokenCount,
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

// ========== Gemini part 辅助（与 claude_gemini 包各自私有，避免跨包依赖） ==========

// geminiFunctionArgs 提取 functionCall 参数，nil 时回退为空对象
func geminiFunctionArgs(fc *dto.GeminiFunctionCall) any {
	if fc == nil || fc.Arguments == nil {
		return map[string]any{}
	}
	return fc.Arguments
}

// marshalGeminiArgs 将 functionCall 参数序列化为 Responses 期望的 JSON 字符串
func marshalGeminiArgs(fc *dto.GeminiFunctionCall) string {
	b, err := json.Marshal(geminiFunctionArgs(fc))
	if err != nil {
		return "{}"
	}
	return string(b)
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

// float64Ptr 返回 float64 的指针
func float64Ptr(v float64) *float64 { return &v }

// claudeWebSearchCallItem 把 claude 的 web_search server_tool_use 构造为 Responses 的
// web_search_call 输出项。argsJSON 为服务端工具的 input 序列化（{"query":"..."}），
// 解析失败时 query 留空（codex 等客户端按项类型展示搜索动作，query 缺失可容忍）。
func claudeWebSearchCallItem(id, argsJSON string) map[string]any {
	query := ""
	var input map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &input); err == nil {
		if q, ok := input["query"].(string); ok {
			query = q
		}
	}
	return map[string]any{
		"type":   "web_search_call",
		"id":     id,
		"status": "completed",
		"action": map[string]any{"type": "search", "query": query},
	}
}

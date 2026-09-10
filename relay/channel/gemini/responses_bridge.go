package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// ========== Responses 入站桥接：Gemini → OpenAI Responses ==========
//
// 请求侧由 relaykit 的步骤链完成（Responses → OpenAI 中枢 → Gemini），
// 响应侧同样由 relaykit 转换器完成（Gemini → Responses，含 SSE 事件序列产出）。
// 本文件只保留宿主侧接线：HTTP 状态码/安全过滤错误处理、响应写出与计费用量提取，
// 以及 Code Assist 强制流式聚合路径复用的非流式装配函数（buildResponsesBodyFromGemini
// 及其助手，由 adaptor.go 的 handleCodeAssistAggregatedStream 调用）。
//
// 用量口径：Responses 的客户端可见 usage 与本渠道的计费口径**恰好一致**
// （都是 OpenAI 语义：input 含缓存、output 含思考），故计费值直接取 geminiUsageToCommon，
// 不像 claude→responses 那样需要两套换算。

// handleNonStreamToResponses 将 Gemini 非流式响应转换为 Responses 格式
func (a *Adaptor) handleNonStreamToResponses(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}
	if resp.StatusCode != http.StatusOK {
		// 不写响应：交上层 WriteRelayError 统一写入，避免双重写入与重试时的响应污染
		return nil, buildGeminiUpstreamError(body, resp.StatusCode)
	}

	// 安全过滤（promptFeedback.blockReason）属上游错误语义，先于协议转换判定
	var geminiResp dto.GeminiChatResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err)
	}
	if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
		return nil, constant.NewRequestError(
			fmt.Sprintf("request blocked by Gemini safety filter: %s", geminiResp.PromptFeedback.BlockReason), nil,
		)
	}

	// relaykit 响应转换（唯一路径，hard-fail：解析/转换失败即向上返回错误，不再回退旧实现）
	convertedBody, _, handled, convErr := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !handled {
		return nil, constant.NewChannelError("gemini adaptor: no relaykit converter for responses client response", nil)
	}
	if convErr != nil {
		return nil, relaykit_bridge.ResponseConvertError(convErr, resp.StatusCode, resp.Header)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

	// 计费用量取 Gemini 原生口径（与客户端可见口径一致）
	return geminiUsageToCommon(geminiResp.UsageMetadata), nil
}

// buildResponsesBodyFromGemini 构建 Responses 非流式响应体与用量。
// 直连非流式与 Code Assist 强制流式的聚合结果共用同一套装配逻辑。
func buildResponsesBodyFromGemini(geminiResp *dto.GeminiChatResponse, info *common.RelayInfo) ([]byte, *common.Usage, error) {
	output := buildResponsesOutputFromGemini(geminiResp, info)

	modelName := geminiResp.ModelName
	if modelName == "" || info.ChannelMeta.IsModelMapped {
		modelName = info.OriginModelName
	}
	createdAt := int(time.Now().Unix())
	completedAt := createdAt

	usage := geminiUsageToCommon(geminiResp.UsageMetadata)

	body, err := json.Marshal(openai.BuildResponsesObjectMap(
		responsesIDFromGemini(geminiResp, info), createdAt, "completed", modelName,
		output, openai.BuildResponsesUsageMap(usage), &completedAt, info,
	))
	if err != nil {
		return nil, nil, fmt.Errorf("marshal responses body failed: %w", err)
	}
	return body, usage, nil
}

// handleStreamToResponses 将 Gemini 流式响应转换为 Responses 格式的 SSE
func (a *Adaptor) handleStreamToResponses(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 不写响应：交上层 WriteRelayError 统一写入
		return nil, buildGeminiUpstreamError(body, resp.StatusCode)
	}

	// relaykit 流式转换（唯一路径，hard-fail）：写入前失败由桥接层返回未接管，此处显式报错
	if usage, ok, err := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer); ok {
		// 桥接层已写出 SSE 并完成收尾裁决：err 非空即流以错误/客户端中断结束
		//（已带 ResponseWritten，上层只记账与上报调度，不重写响应体、不换渠道重试），
		// usage 中断兜底亦已在桥接层完成，此处原样上抛即可。
		return usage, err
	}
	return nil, constant.NewChannelError("gemini adaptor: relaykit stream converter unavailable for responses client", nil)
}

// buildResponsesOutputFromGemini 构建 Responses 非流式响应的 output 数组。
// 文本合并为单个 message 项（居首），functionCall 各成一个 function_call 项。
func buildResponsesOutputFromGemini(geminiResp *dto.GeminiChatResponse, info *common.RelayInfo) []map[string]any {
	var textParts []string
	output := make([]map[string]any, 0)
	toolIdx := 0

	for _, candidate := range geminiResp.Candidates {
		if candidate.Content == nil {
			continue
		}
		for i := range candidate.Content.Parts {
			part := &candidate.Content.Parts[i]
			isThought := part.Thought != nil && *part.Thought

			if part.Text != "" && !isThought {
				textParts = append(textParts, part.Text)
			}
			// 思考内容无 Responses 非流式对应物，跳过（与 claude→responses 口径一致）

			if part.FunctionCall != nil {
				id := geminiResponsesCallID(part.FunctionCall, info, toolIdx)
				toolIdx++
				output = append(output, map[string]any{
					"type":      "function_call",
					"id":        id,
					"call_id":   id,
					"name":      part.FunctionCall.FunctionName,
					"arguments": marshalGeminiArgs(part.FunctionCall),
					"status":    "completed",
				})
			}

			if fallback := geminiPartFallbackText(part); fallback != "" {
				textParts = append(textParts, fallback)
			}
		}
	}

	if len(textParts) > 0 {
		msgItem := map[string]any{
			"type":   "message",
			"id":     fmt.Sprintf("msg_%s", info.RequestID),
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{{
				"type":        "output_text",
				"text":        strings.Join(textParts, ""),
				"annotations": []any{},
			}},
		}
		output = append([]map[string]any{msgItem}, output...)
	}

	return output
}

// responsesIDFromGemini 生成 Responses 响应 ID，优先用上游 responseId
func responsesIDFromGemini(geminiResp *dto.GeminiChatResponse, info *common.RelayInfo) string {
	if geminiResp.ResponseID != "" {
		return fmt.Sprintf("resp_%s", geminiResp.ResponseID)
	}
	return fmt.Sprintf("resp_%s", info.RequestID)
}

// geminiResponsesCallID 取工具调用 ID：Gemini 的 functionCall.id 可选，缺失时合成。
// Responses 的 function_call 必须带 call_id，客户端下一轮按此 ID 回传结果。
func geminiResponsesCallID(fc *dto.GeminiFunctionCall, info *common.RelayInfo, idx int) string {
	if fc != nil && fc.ID != "" {
		return fc.ID
	}
	return fmt.Sprintf("call_%s_%d", info.RequestID, idx)
}

// marshalGeminiArgs 将 functionCall 参数序列化为 Responses 期望的 JSON 字符串
func marshalGeminiArgs(fc *dto.GeminiFunctionCall) string {
	b, err := json.Marshal(geminiFunctionArgs(fc))
	if err != nil {
		return "{}"
	}
	return string(b)
}

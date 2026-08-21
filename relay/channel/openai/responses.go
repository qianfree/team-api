package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// ========== 响应转换：Chat Completions → Responses ==========

// handleResponsesInboundNonStream 将 Chat Completions 非流式响应转换为 Responses 格式
func (a *Adaptor) handleResponsesInboundNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	// 非 200：透传上游错误响应并返回上游错误（驱动重试/渠道健康上报）
	if resp.StatusCode != http.StatusOK {
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamErrorFromResponse(resp, body)
	}

	// relaykit 唯一路径（legacy 回退已收割）：未接管按转换失败报错
	responsesBody, usage, ok := relaykit_bridge.TryConvertResponsesResponseViaRelaykit(ctx, info, body)
	if !ok {
		return nil, fmt.Errorf("[relaykit] chat→responses 响应转换失败")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(responsesBody)
	return usage, nil
}

// responsesRequestEcho 合成 Responses 响应时需回显（echo）的请求参数
type responsesRequestEcho struct {
	temperature     *float64
	topP            *float64
	maxOutputTokens *int
	instructions    any
}

// extractResponsesRequestEcho 从 info.ResponsesRequest 提取合成响应应 echo 的请求参数。
// 快照缺失（直连路径/异常）时回退 OpenAI 默认值（temperature=1.0 / top_p=1.0，其余 nil）。
func extractResponsesRequestEcho(info *common.RelayInfo) responsesRequestEcho {
	echo := responsesRequestEcho{temperature: float64Ptr(1.0), topP: float64Ptr(1.0)}
	if info == nil || info.ResponsesRequest == nil {
		return echo
	}
	rr := info.ResponsesRequest
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

// EmitResponsesSSE 发送一个 Responses API 格式的 SSE 事件（导出供 claude 等适配器的 Responses 桥接复用）
func EmitResponsesSSE(w http.ResponseWriter, eventType string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(jsonData))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// buildResponsesObjectMap 构建 Responses API response 对象的完整字段 map。
// 请求参数从 info.ResponsesRequest echo（快照缺失时回退默认值）；
// store 恒为 false——合成响应不落上游存储，客户端不可经生命周期端点 retrieve。
func BuildResponsesObjectMap(respID string, createdAt int, status string, model string, output any, usageObj map[string]any, completedAt *int, info *common.RelayInfo) map[string]any {
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
func BuildResponsesUsageMap(usage *common.Usage) map[string]any {
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

// float64Ptr 返回 float64 的指针
func float64Ptr(v float64) *float64 {
	return &v
}

// ========== 上游为 Responses 协议：Responses 响应原样透传 ==========

// responsesUsageToCommon 将 Responses API usage 转换为 common.Usage。
// OpenAI 原生 API 的 input_tokens 已含缓存 token（cache 是其子集），故 CacheIncludedInPrompt=true。
func responsesUsageToCommon(u *dto.ResponsesUsage) *common.Usage {
	usage := &common.Usage{CacheIncludedInPrompt: true}
	if u == nil {
		return usage
	}
	usage.PromptTokens = u.InputTokens
	usage.CompletionTokens = u.OutputTokens
	usage.TotalTokens = u.TotalTokens
	if d := u.InputTokensDetails; d != nil {
		usage.PromptTokensDetails = &common.TokenDetails{
			CachedTokens:     d.CachedTokens,
			CacheWriteTokens: d.CacheWriteTokens,
			TextTokens:       d.TextTokens,
			AudioTokens:      d.AudioTokens,
			ImageTokens:      d.ImageTokens,
		}
	}
	if d := u.OutputTokenDetails; d != nil {
		usage.CompletionTokenDetails = &common.TokenDetails{
			TextTokens:               d.TextTokens,
			AudioTokens:              d.AudioTokens,
			ReasoningTokens:          d.ReasoningTokens,
			AcceptedPredictionTokens: d.AcceptedPredictionTokens,
			RejectedPredictionTokens: d.RejectedPredictionTokens,
		}
	}
	return usage
}

// recordResponseRoute 记录 response_id → 渠道路由（Redis），供 GET/DELETE/cancel
// 生命周期端点还原原始请求落到的渠道。ModelName 存 lookupModel 口径（BaseModelName），
// 与 MaterializeSelection 入参一致。
func recordResponseRoute(ctx context.Context, info *common.RelayInfo, responseID string) {
	if responseID == "" || info == nil || info.ChannelMeta == nil {
		return
	}
	common.DefaultResponseRouteStore.Record(ctx, info.TenantID, responseID, common.ResponseRoute{
		ChannelID: info.ChannelMeta.ChannelID,
		ModelName: info.BaseModelName,
	})
}

// handleResponsesUpstreamNonStream 上游为 Responses 协议时的非流式响应：
// 解析 usage 后原样透传上游响应体（模型映射时回写客户端请求的模型名）。
func (a *Adaptor) handleResponsesUpstreamNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	// 非 200：透传上游错误响应并返回上游错误（驱动重试/渠道健康上报，同 handleChatNonStreamResponse）
	if resp.StatusCode != http.StatusOK {
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamErrorFromResponse(resp, body)
	}

	if info.ChannelMeta.IsModelMapped {
		body = helper.ReplaceModelName(body, info.OriginModelName)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)

	// 解析 usage（计费用），响应体仍透传上游原始内容
	var responsesResp dto.OpenAIResponsesResponse
	if err := json.Unmarshal(body, &responsesResp); err == nil {
		recordResponseRoute(ctx, info, responsesResp.ID)
		return responsesUsageToCommon(responsesResp.Usage), nil
	}
	return &common.Usage{}, nil
}

// handleResponsesUpstreamStream 上游为 Responses 协议时的流式响应：
// 逐行原样透传 SSE（含 event: 行），从 response.completed / response.done 事件解析 usage。
func (a *Adaptor) handleResponsesUpstreamStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	// 非 200：上游在 SSE 开始前返回错误（非 SSE 体），透传并返回上游错误驱动重试
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		}
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamErrorFromResponse(resp, body)
	}

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var usage common.Usage
	var contentBuilder strings.Builder

	flush := func() {
		if f, ok := writer.(http.Flusher); ok {
			f.Flush()
		}
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			// 流中断计费兜底：输出缺失按已转发文本 2 字符/token 估算，输入用请求侧估算值补齐
			helper.ApplyInterruptedUsageFallback(info, &usage, contentBuilder.Len())
			return &usage, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" {
			fmt.Fprintf(writer, "\n")
			flush()
			continue
		}
		// 原样透传 event: 行
		if strings.HasPrefix(line, "event:") {
			fmt.Fprintf(writer, "%s\n", line)
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data, _ := helper.ExtractSSEData(line)

		if data != "" && data != "[DONE]" {
			info.SetFirstResponseTime()
		}

		// 解析 data 提取 usage（不影响原样透传）
		if data != "[DONE]" {
			var streamResp dto.ResponsesStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err == nil {
				switch streamResp.Type {
				case "response.created", "response.completed", "response.done":
					if r := streamResp.Response; r != nil {
						// 路由记录：created 即记录使 cancel 尽早可用，completed/done 刷新 TTL（SET 幂等）
						recordResponseRoute(ctx, info, r.ID)
						if streamResp.Type != "response.created" && r.Usage != nil {
							if u := responsesUsageToCommon(r.Usage); u != nil {
								usage = *u
							}
						}
					}
				case "response.output_text.delta":
					contentBuilder.WriteString(streamResp.Delta)
				}
			}
		}

		// 模型映射时回写客户端请求的模型名：response.created / response.completed 等事件携带上游模型名，
		// 直连透传前替换（同 chat StreamHandler 的逐行替换）
		outLine := line
		if info.ChannelMeta.IsModelMapped && data != "" && data != "[DONE]" {
			outLine = "data: " + string(helper.ReplaceModelName([]byte(data), info.OriginModelName))
		}

		fmt.Fprintf(writer, "%s\n", outLine)
		flush()

		if data == "[DONE]" {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		if err != io.EOF && ctx.Err() == nil {
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
			return &usage, fmt.Errorf("stream scanner error: %w", err)
		}
	}

	// 估算 usage（正常结束 4 字符/token；异常部分传输 2 字符/token）
	if usage.CompletionTokens == 0 {
		text := contentBuilder.String()
		if len(text) > 0 {
			usage.CompletionTokens = helper.EstimateStreamOutputTokens(info, len(text))
		}
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	usage.CacheIncludedInPrompt = true

	if info.StreamStatus.GetEndReason() == "" {
		info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	}

	return &usage, nil
}

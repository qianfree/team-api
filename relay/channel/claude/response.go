package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// handleNonStreamToOpenAI 将 Claude 非流式响应转换为 OpenAI 格式
func (a *Adaptor) handleNonStreamToOpenAI(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	// relaykit 响应转换（唯一路径，hard-fail：解析/转换失败即向上返回错误，不再回退旧实现）
	convertedBody, _, handled, convErr := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !handled {
		return nil, constant.NewChannelError("claude adaptor: no relaykit converter for openai client response", nil)
	}
	if convErr != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", convErr).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

	// relaykit 转换器返回的 Usage 为 nil（ResponseConverterFunc 签名约束），从原始 Claude 响应提取
	var claudeResp dto.ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		// Usage 解析失败，返回空 Usage（已写响应，不能重试）
		// 静默处理：非致命错误，响应已正确写入
		return &common.Usage{}, nil
	}
	if claudeResp.Usage != nil {
		usage := &common.Usage{
			PromptTokens:        claudeResp.Usage.InputTokens,
			CompletionTokens:    claudeResp.Usage.OutputTokens,
			TotalTokens:         claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
			CacheCreationTokens: claudeResp.Usage.CacheCreationInputTokens,
			PromptTokensDetails: claudeUsageToTokenDetails(claudeResp.Usage),
		}
		return usage, nil
	}
	// Usage 为 nil，返回空 Usage
	return &common.Usage{}, nil
}

// handleStreamToOpenAI 将 Claude 流式响应转换为 OpenAI SSE 格式
func (a *Adaptor) handleStreamToOpenAI(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	// relaykit 流式转换（唯一路径，hard-fail）：写入前失败由桥接层返回未接管，此处显式报错
	if usage, ok, err := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer); ok {
		// 桥接层已写出 SSE 并完成收尾裁决：err 非空即流以错误/客户端中断结束
		//（已带 ResponseWritten，上层只记账与上报调度，不重写响应体、不换渠道重试），
		// usage 中断兜底亦已在桥接层完成，此处原样上抛即可。
		return usage, err
	}
	return nil, constant.NewChannelError("claude adaptor: relaykit stream converter unavailable for openai client", nil)
}

// handleClaudeNativeResponse 直通 Claude 原生格式响应
func (a *Adaptor) handleClaudeNativeResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if info.IsStream {
		return a.handleClaudeNativeStream(ctx, resp, info, writer)
	}
	return a.handleClaudeNativeNonStream(ctx, resp, info, writer)
}

// handleClaudeNativeNonStream 直通 Claude 非流式响应
func (a *Adaptor) handleClaudeNativeNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	if info.ChannelMeta.IsModelMapped {
		body = helper.ReplaceModelName(body, info.OriginModelName)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(resp.StatusCode)
	_, _ = writer.Write(body)

	var claudeResp dto.ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		// Usage 解析失败，返回空 Usage（静默处理，非致命错误）
		return &common.Usage{}, nil
	}
	if claudeResp.Usage != nil {
		return &common.Usage{
			PromptTokens:        claudeResp.Usage.InputTokens,
			CompletionTokens:    claudeResp.Usage.OutputTokens,
			TotalTokens:         claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
			CacheCreationTokens: claudeResp.Usage.CacheCreationInputTokens,
			PromptTokensDetails: claudeUsageToTokenDetails(claudeResp.Usage),
		}, nil
	}

	return &common.Usage{}, nil
}

// handleClaudeNativeStream 直通 Claude 流式响应
func (a *Adaptor) handleClaudeNativeStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	// 创建可取消的上下文，用于在客户端断开时立即中止上游读取
	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()

	// cleanup 函数：关闭上游连接，停止 token 生成
	cleanup := func() {
		cancelStream()
		if resp.Body != nil {
			resp.Body.Close()
		}
	}

	// 在提交任何响应头之前检查客户端是否已断开（常见于上游 TTFB 较慢、客户端在
	// DoRequest 阶段超时并主动关闭连接的场景）。此时 context 已被取消，若继续写
	// SSE 头再检测 Done，客户端会收到残缺的 text/event-stream 响应，Anthropic SDK
	// 尝试解析时报 "Failed to parse JSON"。提前检测并以正常 relay 错误路径返回，
	// 让上层写出标准 Claude JSON 错误体。
	if ctx.Err() != nil {
		resp.Body.Close()
		g.Log().Warningf(context.Background(),
			"[ClaudeNativeStream] DoResponse 入口 ctx 已取消，放弃写响应头 request_id=%s ctx.Err=%v",
			info.RequestID, ctx.Err())
		info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
		return nil, common.ErrStreamInterrupted
	}

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	stopPing := helper.PingTicker(writer, 15*time.Second)
	defer stopPing()

	reader := bufio.NewReaderSize(resp.Body, 64*1024)
	// 按帧攒齐再写：逐行写会让并发的保活 ping 把自带空行插进帧中间，客户端据此派发出
	// 一个 data 为空的事件（JSON.parse("") 报错并中止请求），网关侧只看到 ctx 取消被记成
	// client_gone，真实成因被掩盖。SafeWriter 只保证单次 Write 原子，保证不了整帧原子。
	frame := helper.NewSSEFrameWriter(writer)
	var usage dto.ClaudeUsage
	var transferredTextLen int // 已转发的文本/思考内容长度，供流中断输出估算

	// onWriteFail 写客户端失败：立即关闭上游连接，停止 token 生成，按流中断结算
	onWriteFail := func(writeErr error) (*common.Usage, error) {
		g.Log().Warningf(context.Background(),
			"[ClaudeNativeStream] 写入客户端失败 request_id=%s writeErr=%v ctx.Err=%v elapsed=%v",
			info.RequestID, writeErr, ctx.Err(), time.Since(info.StartTime))
		cleanup()
		info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, writeErr)
		interruptedUsage := buildUsageFromClaude(&usage)
		helper.ApplyInterruptedUsageFallback(info, interruptedUsage, transferredTextLen)
		return interruptedUsage, common.ErrStreamInterrupted
	}

	for {
		select {
		case <-streamCtx.Done():
			// SSE 头已提交（SetEventStreamHeaders 在循环前已调用），直接关闭连接会
			// 让 SDK 收到"200 + SSE头 + 无事件 + EOF"，Anthropic SDK 进入等待状态，
			// 后续请求的响应会被误判为当前流的 SSE 数据 → "Failed to parse JSON"。
			// 发送一个 Claude 格式的 error event，让 SDK 以正常 API Error 退出，而非挂起。
			// 未完成的半帧尚未出网，先丢弃，避免与 error 帧拼成非法 SSE 输出。
			frame.Discard()
			_, _ = fmt.Fprintf(writer,
				"event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"api_error\",\"message\":\"upstream disconnected\"}}\n\n")
			if f, ok := writer.(http.Flusher); ok {
				f.Flush()
			}
			cleanup()
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, streamCtx.Err())
			interruptedUsage := buildUsageFromClaude(&usage)
			helper.ApplyInterruptedUsageFallback(info, interruptedUsage, transferredTextLen)
			return interruptedUsage, common.ErrStreamInterrupted
		default:
		}

		// ReadString 在上游 EOF 时可能同时返回「最后一段无换行的数据 + io.EOF」，
		// 必须先处理 line 再判 err，否则会丢掉上游未以换行结尾的最后一行。
		line, err := reader.ReadString('\n')

		if err != nil && err != io.EOF {
			// 上游读取出错：缓冲里的残留是不完整的帧，写给客户端只会造成非法 SSE，丢弃
			frame.Discard()
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
			// 已有部分输出时按部分成功处理（避免标记为完全失败）
			interruptedUsage := buildUsageFromClaude(&usage)
			if transferredTextLen > 0 {
				helper.ApplyInterruptedUsageFallback(info, interruptedUsage, transferredTextLen)
			}
			return interruptedUsage, fmt.Errorf("upstream stream interrupted: %w", err)
		}

		if line != "" {
			if strings.HasPrefix(line, "data:") {
				data, _ := helper.ExtractSSEData(line)

				if data != "" && data != "[DONE]" {
					info.SetFirstResponseTime()
				}

				var event dto.ClaudeResponse
				if json.Unmarshal([]byte(data), &event) != nil {
					// JSON 解析失败：静默跳过
				} else {
					switch event.Type {
					case "message_start":
						if event.Message != nil && event.Message.Usage != nil {
							usage = *event.Message.Usage
						}
					case "content_block_delta":
						// 累计已转发文本长度，供流中断（message_delta 未到达时）输出估算
						if event.Delta != nil {
							if event.Delta.Text != nil {
								transferredTextLen += len(*event.Delta.Text)
							}
							if event.Delta.Thinking != nil {
								transferredTextLen += len(*event.Delta.Thinking)
							}
						}
					case "message_delta":
						if event.Usage != nil {
							if event.Usage.InputTokens > 0 {
								usage.InputTokens = event.Usage.InputTokens
							}
							usage.OutputTokens = event.Usage.OutputTokens
							if event.Usage.CacheReadInputTokens > 0 {
								usage.CacheReadInputTokens = event.Usage.CacheReadInputTokens
							}
							if event.Usage.CacheCreationInputTokens > 0 {
								usage.CacheCreationInputTokens = event.Usage.CacheCreationInputTokens
							}
							if event.Usage.CacheCreation != nil {
								usage.CacheCreation = event.Usage.CacheCreation
							}
						}
					case "error":
						info.StreamStatus.SetEndReason(common.StreamEndReasonError, fmt.Errorf("claude upstream stream error"))
					}
				}

				if info.ChannelMeta.IsModelMapped {
					replaced := string(helper.ReplaceModelName([]byte(data), info.OriginModelName))
					line = fmt.Sprintf("data: %s\n", replaced)
				}
			}

			if werr := frame.WriteLine(line); werr != nil {
				return onWriteFail(werr)
			}
		}

		if err == io.EOF {
			// 上游未以空行结尾时冲刷残留，避免丢掉最后一帧
			if ferr := frame.FlushPartial(); ferr != nil {
				return onWriteFail(ferr)
			}
			break
		}
	}

	info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)

	return buildUsageFromClaude(&usage), nil
}

// buildUsageFromClaude 从已累积的 ClaudeUsage 构建 common.Usage，保留 cache 字段
func buildUsageFromClaude(u *dto.ClaudeUsage) *common.Usage {
	return &common.Usage{
		PromptTokens:        u.InputTokens,
		CompletionTokens:    u.OutputTokens,
		TotalTokens:         u.InputTokens + u.OutputTokens,
		CacheCreationTokens: u.CacheCreationInputTokens,
		PromptTokensDetails: claudeUsageToTokenDetails(u),
	}
}

// claudeUsageToTokenDetails 将 ClaudeUsage 转换为 TokenDetails（含 cache token 细分）
func claudeUsageToTokenDetails(u *dto.ClaudeUsage) *common.TokenDetails {
	if u == nil {
		return nil
	}
	td := &common.TokenDetails{
		CachedTokens:         u.CacheReadInputTokens,
		CachedCreationTokens: u.CacheCreationInputTokens,
	}
	if u.CacheCreation != nil {
		td.CachedCreation5mTokens = u.CacheCreation.Ephemeral5mInputTokens
		td.CachedCreation1hTokens = u.CacheCreation.Ephemeral1hInputTokens
	}
	return td
}

// claudeToOpenAIResponse 将 Claude 非流式响应转换为 OpenAI ChatCompletion 格式
func claudeToOpenAIResponse(claudeResp *dto.ClaudeResponse, info *common.RelayInfo) dto.ChatCompletionResponse {
	resp := dto.ChatCompletionResponse{
		ID:      claudeResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   claudeResp.Model,
		Choices: []dto.Choice{{
			Index:        0,
			FinishReason: common.ClaudeStopReasonToOpenAI(claudeResp.StopReason),
		}},
	}

	if info.ChannelMeta.IsModelMapped {
		resp.Model = info.OriginModelName
	}

	var textParts []string
	var thinkingParts []string
	var toolCalls []dto.ToolCall

	for _, block := range claudeResp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil {
				textParts = append(textParts, *block.Text)
			}
		case "thinking":
			if block.Thinking != nil {
				thinkingParts = append(thinkingParts, *block.Thinking)
			}
		case "redacted_thinking":
			// 脱敏思考，OpenAI 格式无等价物，忽略
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, dto.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: dto.FunctionCall{
					Name:      block.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	message := dto.Message{
		Role:    "assistant",
		Content: joinTextPartsResponse(textParts),
	}
	if len(thinkingParts) > 0 {
		thinking := strings.Join(thinkingParts, "")
		message.ReasoningContent = &thinking
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}
	resp.Choices[0].Message = message

	if claudeResp.Usage != nil {
		// Claude 的 input_tokens 不含缓存（三项并列），OpenAI 的 prompt_tokens 含缓存
		//（cached 是其子集），转换做加法并透出缓存明细，客户端按 OpenAI 语义解析才正确
		promptTotal := claudeResp.Usage.InputTokens +
			claudeResp.Usage.CacheReadInputTokens +
			claudeResp.Usage.CacheCreationInputTokens
		resp.Usage = dto.UsageWithDetails{
			PromptTokens:     promptTotal,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      promptTotal + claudeResp.Usage.OutputTokens,
		}
		if claudeResp.Usage.CacheReadInputTokens > 0 || claudeResp.Usage.CacheCreationInputTokens > 0 {
			resp.Usage.PromptTokensDetails = &dto.TokenDetails{
				CachedTokens:         claudeResp.Usage.CacheReadInputTokens,
				CachedCreationTokens: claudeResp.Usage.CacheCreationInputTokens,
			}
		}
	}

	return resp
}

// writeStreamChunk 写入流式 chunk
func writeStreamChunk(w http.ResponseWriter, chunk *dto.ChatCompletionStreamResponse) {
	data, _ := json.Marshal(chunk)
	_ = helper.WriteSSEData(w, string(data))
}

func joinTextPartsResponse(parts []string) any {
	if len(parts) == 0 {
		return nil
	}
	result := make([]byte, 0, 64)
	for i, p := range parts {
		if i > 0 {
			result = append(result, '\n')
		}
		result = append(result, p...)
	}
	return string(result)
}

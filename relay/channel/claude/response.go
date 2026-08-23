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

// handleNonStreamToOpenAI 将 Claude 非流式响应转换为 OpenAI 格式。
// relaykit 唯一路径（legacy 回退已收割）：未接管按转换失败报错。
func (a *Adaptor) handleNonStreamToOpenAI(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	convertedBody, _, ok := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !ok {
		return nil, fmt.Errorf("[relaykit] claude→openai 响应转换失败")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

	// relaykit 转换器返回的 Usage 为 nil（ResponseConverterFunc 签名约束），从原始 Claude 响应提取
	var claudeResp dto.ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		// Usage 解析失败，返回空 Usage（已写响应，不能重试）
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

// handleStreamToOpenAI 将 Claude 流式响应转换为 OpenAI SSE 格式。
// relaykit 唯一路径（legacy 回退已收割）：未接管按转换失败报错。
// 转换中途失败由桥接层优雅降级（按客户端格式补终止事件 + end reason），不走本函数。
func (a *Adaptor) handleStreamToOpenAI(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	usage, ok := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer)
	if !ok {
		return nil, fmt.Errorf("[relaykit] claude→openai 流式转换失败（无匹配转换器或转换失败）")
	}
	return usage, nil
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
	var usage dto.ClaudeUsage
	var transferredTextLen int // 已转发的文本/思考内容长度，供流中断输出估算

	for {
		select {
		case <-streamCtx.Done():
			// SSE 头已提交（SetEventStreamHeaders 在循环前已调用），直接关闭连接会
			// 让 SDK 收到"200 + SSE头 + 无事件 + EOF"，Anthropic SDK 进入等待状态，
			// 后续请求的响应会被误判为当前流的 SSE 数据 → "Failed to parse JSON"。
			// 发送一个 Claude 格式的 error event，让 SDK 以正常 API Error 退出，而非挂起。
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

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
			// 已有部分输出时按部分成功处理（避免标记为完全失败）
			interruptedUsage := buildUsageFromClaude(&usage)
			if transferredTextLen > 0 {
				helper.ApplyInterruptedUsageFallback(info, interruptedUsage, transferredTextLen)
			}
			return interruptedUsage, fmt.Errorf("upstream stream interrupted: %w", err)
		}

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

		if _, err := writer.Write([]byte(line)); err != nil {
			// 写入客户端失败：立即关闭上游连接，停止生成
			g.Log().Warningf(context.Background(),
				"[ClaudeNativeStream] 写入客户端失败 request_id=%s writeErr=%v ctx.Err=%v elapsed=%v",
				info.RequestID, err, ctx.Err(), time.Since(info.StartTime))
			cleanup()
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, err)
			interruptedUsage := buildUsageFromClaude(&usage)
			helper.ApplyInterruptedUsageFallback(info, interruptedUsage, transferredTextLen)
			return interruptedUsage, common.ErrStreamInterrupted
		}

		if len(line) == 1 && line[0] == '\n' {
			if f, ok := writer.(http.Flusher); ok {
				f.Flush()
			}
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

// claudeToOpenAIResponse 等 legacy 响应转换已随 relaykit 收割删除：
// claude→openai 方向统一走 relaykit_bridge.TryConvertResponse/TryConvertStreamViaRelaykit。

package dify

import (
	"bufio"
	"bytes"
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
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/override"
)

// Adaptor Dify 供应商适配器。
// Dify 是开源 LLM 应用开发平台，通过 App API 提供对话能力。
// 本适配器将 OpenAI Chat Completions 请求转换为 Dify chat-messages 格式，
// 并将 Dify 响应（blocking/streaming）转换回 OpenAI 格式。
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL。Dify chat-messages 端点: {baseURL}/v1/chat-messages
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions:
		return baseURL + "/v1/chat-messages", nil
	default:
		return "", fmt.Errorf("unsupported relay mode for Dify: %d", info.RelayMode)
	}
}

// SetupRequestHeader 设置上游请求头。ApiKey 为 Dify App API Key。
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 将 OpenAI Chat 请求转换为 Dify 请求格式
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

	difyBody, err := convertOpenAIToDify(requestBody, info)
	if err != nil {
		return nil, fmt.Errorf("convert to Dify request failed: %w", err)
	}
	return bytes.NewReader(difyBody), nil
}

// DoRequest 发送请求到 Dify 上游
func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	if err := a.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}

	if hdrOverrides, hdrErr := override.ApplyHeaderOverride(info); hdrErr == nil && len(hdrOverrides) > 0 {
		override.MergeHeaderOverrides(httpReq.Header, hdrOverrides)
	}

	timeout := info.ChannelMeta.Settings.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120 // Dify 应用可能包含复杂工作流，给更长超时
	}
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations && timeout < 600 {
		timeout = 600
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理 Dify 上游响应，转换为 OpenAI 格式。
//
// Dify blocking 模式返回 JSON:
//
//	{"answer": "...", "metadata": {"usage": {"total_tokens": N}}}
//
// Dify streaming 模式返回 SSE:
//
//	data: {"event": "message", "answer": "chunk text"}
//	data: {"event": "message_end", "metadata": {"usage": {...}}}
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	if info.IsStream {
		return a.handleStreamResponse(ctx, resp, info, writer)
	}
	return a.handleNonStreamResponse(ctx, resp, info, writer)
}

// handleNonStreamResponse 处理 Dify blocking 模式响应
func (a *Adaptor) handleNonStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Dify response body failed: %w", err)
	}

	info.SetFirstResponseTime()

	var difyResp DifyBlockingResponse
	if err := json.Unmarshal(body, &difyResp); err != nil {
		return nil, fmt.Errorf("parse Dify response failed: %w", err)
	}

	// 构建 OpenAI 非流式响应
	chatResp := dto.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-dify-%s", info.RequestID),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   info.OriginModelName,
		Choices: []dto.Choice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: difyResp.Answer,
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.UsageWithDetails{
			PromptTokens:     difyResp.Metadata.Usage.PromptTokens,
			CompletionTokens: difyResp.Metadata.Usage.CompletionTokens,
			TotalTokens:      difyResp.Metadata.Usage.TotalTokens,
		},
	}

	// 如果 Dify 未返回用量信息，使用粗略估算
	if chatResp.Usage.TotalTokens == 0 {
		chatResp.Usage.CompletionTokens = helper.EstimateTokens(difyResp.Answer)
		chatResp.Usage.TotalTokens = chatResp.Usage.CompletionTokens
	}

	respJSON, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAI response failed: %w", err)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respJSON)

	return &common.Usage{
		PromptTokens:     chatResp.Usage.PromptTokens,
		CompletionTokens: chatResp.Usage.CompletionTokens,
		TotalTokens:      chatResp.Usage.TotalTokens,
	}, nil
}

// handleStreamResponse 处理 Dify streaming 模式 SSE 响应
func (a *Adaptor) handleStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	streamStatus := info.StreamStatus
	if streamStatus == nil {
		streamStatus = common.NewStreamStatus()
		info.StreamStatus = streamStatus
	}

	chatID := fmt.Sprintf("chatcmpl-dify-%s", info.RequestID)
	createdAt := time.Now().Unix()
	modelName := info.OriginModelName

	scanner := bufio.NewScanner(resp.Body)
	var usage common.Usage

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			streamStatus.SetEndReason(common.StreamEndReasonClientGone, common.ErrStreamInterrupted)
			return &usage, nil
		default:
		}

		line := scanner.Text()

		// Dify SSE 格式: "data: {...}"
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data, _ := helper.ExtractSSEData(line)
		if data == "" {
			continue
		}

		info.SetFirstResponseTime()

		var event DifyStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Event {
		case "message":
			// 内容增量
			if event.Answer == "" {
				continue
			}

			chunk := helper.BuildOpenAIStreamChunk(chatID, createdAt, modelName, event.Answer, nil)
			chunkJSON, err := json.Marshal(chunk)
			if err != nil {
				continue
			}
			if err := helper.WriteSSEData(writer, string(chunkJSON)); err != nil {
				streamStatus.SetEndReason(common.StreamEndReasonClientGone, common.ErrStreamInterrupted)
				return &usage, nil
			}

		case "message_end":
			// 流结束，包含用量信息
			usage.PromptTokens = event.Metadata.Usage.PromptTokens
			usage.CompletionTokens = event.Metadata.Usage.CompletionTokens
			usage.TotalTokens = event.Metadata.Usage.TotalTokens

			// 发送带 finish_reason 的最终 chunk
			finishReason := "stop"
			chunk := helper.BuildOpenAIStreamChunk(chatID, createdAt, modelName, "", &finishReason)
			chunkJSON, _ := json.Marshal(chunk)
			_ = helper.WriteSSEData(writer, string(chunkJSON))

			// 发送 [DONE]
			_ = helper.WriteSSEData(writer, "[DONE]")
			streamStatus.SetEndReason(common.StreamEndReasonDone, nil)
			return &usage, nil

		case "error":
			streamStatus.SetEndReason(common.StreamEndReasonError, fmt.Errorf("dify error: %s", data))
			return &usage, fmt.Errorf("dify stream error: %s", data)
		}
	}

	if err := scanner.Err(); err != nil {
		streamStatus.SetEndReason(common.StreamEndReasonError, err)
		return &usage, fmt.Errorf("read dify stream failed: %w", err)
	}

	// 流异常结束（未收到 message_end），补充兜底的结束 chunk 和 [DONE]
	if streamStatus.GetEndReason() == "" {
		finishReason := "stop"
		chunk := helper.BuildOpenAIStreamChunk(chatID, createdAt, modelName, "", &finishReason)
		chunkJSON, _ := json.Marshal(chunk)
		_ = helper.WriteSSEData(writer, string(chunkJSON))
		_ = helper.WriteSSEData(writer, "[DONE]")
		streamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	}

	return &usage, nil
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

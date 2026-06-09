package coze

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

// Adaptor Coze 供应商适配器。
// Coze v3 API 支持流式 SSE 响应，本适配器将 Coze SSE 事件转换为 OpenAI 格式的 SSE 流。
// 非流式请求也通过流式端点收集完整响应后以 OpenAI JSON 格式返回。
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL。Coze v3 chat 端点: {baseURL}/v3/chat
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")
	return baseURL + "/v3/chat", nil
}

// SetupRequestHeader 设置上游请求头。ApiKey 为 Coze Personal Access Token。
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 将 OpenAI Chat 请求转换为 Coze v3 请求格式
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

	// 非流式请求也强制使用流式模式，以便在 DoResponse 中统一处理
	cozeBody, err := convertOpenAIToCoze(requestBody, info)
	if err != nil {
		return nil, fmt.Errorf("convert to Coze request failed: %w", err)
	}

	// 强制开启流式模式：Coze 非流式需要轮询，实现复杂，
	// 这里统一走流式，非流式场景在 DoResponse 中收集完整响应后一次性返回
	var cozeReq CozeCreateRequest
	if err := json.Unmarshal(cozeBody, &cozeReq); err == nil {
		cozeReq.Stream = true
		cozeBody, _ = json.Marshal(cozeReq)
	}

	return bytes.NewReader(cozeBody), nil
}

// DoRequest 发送请求到 Coze 上游
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
		timeout = 120 // Coze 可能响应较慢，给更长超时
	}
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations && timeout < 600 {
		timeout = 600
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理 Coze 上游 SSE 响应，转换为 OpenAI 格式。
// Coze SSE 事件格式:
//
//	event: conversation.message.delta
//	data: {"role":"assistant","type":"answer","content":"Hello"}
//
//	event: conversation.message.completed
//	data: {"role":"assistant","type":"answer","content":"Hello world"}
//
//	event: done
//	data: {}
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

// handleStreamResponse 流式模式：逐事件读取 Coze SSE，转换为 OpenAI SSE 格式输出
func (a *Adaptor) handleStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	streamStatus := info.StreamStatus
	if streamStatus == nil {
		streamStatus = common.NewStreamStatus()
		info.StreamStatus = streamStatus
	}

	chatID := fmt.Sprintf("chatcmpl-coze-%s", info.RequestID)
	createdAt := time.Now().Unix()
	modelName := info.OriginModelName

	scanner := bufio.NewScanner(resp.Body)
	var currentEvent string
	var completionTokens int

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			streamStatus.SetEndReason(common.StreamEndReasonClientGone, common.ErrStreamInterrupted)
			return &common.Usage{CompletionTokens: completionTokens}, nil
		default:
		}

		line := scanner.Text()

		// 解析 SSE event 行
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		// 解析 SSE data 行
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data, _ := helper.ExtractSSEData(line)

		info.SetFirstResponseTime()

		switch currentEvent {
		case "conversation.message.delta":
			var msg CozeMessage
			if err := json.Unmarshal([]byte(data), &msg); err != nil {
				continue
			}
			// 只转发 answer 类型的消息
			if msg.Type != "answer" {
				continue
			}
			completionTokens += helper.EstimateTokens(msg.Content)

			chunk := helper.BuildOpenAIStreamChunk(chatID, createdAt, modelName, msg.Content, nil)
			chunkJSON, err := json.Marshal(chunk)
			if err != nil {
				continue
			}
			if err := helper.WriteSSEData(writer, string(chunkJSON)); err != nil {
				streamStatus.SetEndReason(common.StreamEndReasonClientGone, common.ErrStreamInterrupted)
				return &common.Usage{CompletionTokens: completionTokens}, nil
			}

		case "conversation.message.completed":
			// 完成事件，发送带 finish_reason 的最终 chunk
			finishReason := "stop"
			chunk := helper.BuildOpenAIStreamChunk(chatID, createdAt, modelName, "", &finishReason)
			chunkJSON, _ := json.Marshal(chunk)
			_ = helper.WriteSSEData(writer, string(chunkJSON))

		case "done":
			// 流结束
			_ = helper.WriteSSEData(writer, "[DONE]")
			streamStatus.SetEndReason(common.StreamEndReasonDone, nil)
			return &common.Usage{
				CompletionTokens: completionTokens,
				TotalTokens:      completionTokens,
			}, nil

		case "error":
			// Coze 错误事件
			streamStatus.SetEndReason(common.StreamEndReasonError, fmt.Errorf("coze error: %s", data))
			return &common.Usage{CompletionTokens: completionTokens}, fmt.Errorf("coze stream error: %s", data)
		}

		currentEvent = ""
	}

	if err := scanner.Err(); err != nil {
		streamStatus.SetEndReason(common.StreamEndReasonError, err)
		return &common.Usage{CompletionTokens: completionTokens}, fmt.Errorf("read coze stream failed: %w", err)
	}

	streamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	return &common.Usage{
		CompletionTokens: completionTokens,
		TotalTokens:      completionTokens,
	}, nil
}

// handleNonStreamResponse 非流式模式：读取 Coze SSE 收集完整内容，转换为 OpenAI JSON 响应
func (a *Adaptor) handleNonStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	scanner := bufio.NewScanner(resp.Body)
	var currentEvent string
	var fullContent strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while reading coze response")
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data, _ := helper.ExtractSSEData(line)

		info.SetFirstResponseTime()

		switch currentEvent {
		case "conversation.message.delta":
			var msg CozeMessage
			if err := json.Unmarshal([]byte(data), &msg); err != nil {
				continue
			}
			if msg.Type == "answer" {
				fullContent.WriteString(msg.Content)
			}

		case "conversation.message.completed":
			// completed 事件包含完整内容，优先使用
			var msg CozeMessage
			if err := json.Unmarshal([]byte(data), &msg); err == nil && msg.Type == "answer" {
				fullContent.Reset()
				fullContent.WriteString(msg.Content)
			}

		case "error":
			return nil, fmt.Errorf("coze error: %s", data)
		}

		currentEvent = ""
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read coze response failed: %w", err)
	}

	content := fullContent.String()
	completionTokens := helper.EstimateTokens(content)

	// 构建 OpenAI 非流式响应
	chatResp := dto.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-coze-%s", info.RequestID),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   info.OriginModelName,
		Choices: []dto.Choice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.UsageWithDetails{
			CompletionTokens: completionTokens,
			TotalTokens:      completionTokens,
		},
	}

	respJSON, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAI response failed: %w", err)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respJSON)

	return &common.Usage{
		CompletionTokens: completionTokens,
		TotalTokens:      completionTokens,
	}, nil
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

package ollama

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

// Adaptor Ollama 供应商适配器
type Adaptor struct {
	info *common.RelayInfo
}

// 确保接口实现
var _ common.Adaptor = (*Adaptor)(nil)

// Init 初始化适配器
func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeEmbeddings:
		return baseURL + "/api/embed", nil
	case constant.RelayModeCompletions:
		return baseURL + "/api/generate", nil
	case constant.RelayModeChatCompletions:
		return baseURL + "/api/chat", nil
	default:
		return baseURL + "/api/chat", nil
	}
}

// SetupRequestHeader 设置上游请求头
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Content-Type", "application/json")
	// Ollama 认证是可选的
	if info.ChannelMeta.ApiKey != "" {
		header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	}
	return nil
}

// ConvertRequest 将入站请求体转换为 Ollama 原生格式
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

	var converted []byte
	var err error

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions:
		converted, err = convertChatRequest(requestBody, info)
	case constant.RelayModeCompletions:
		converted, err = convertCompletionsRequest(requestBody, info)
	case constant.RelayModeEmbeddings:
		converted, err = convertEmbeddingRequest(requestBody, info)
	default:
		converted, err = convertChatRequest(requestBody, info)
	}

	if err != nil {
		return nil, fmt.Errorf("convert request failed: %w", err)
	}
	return bytes.NewReader(converted), nil
}

// DoRequest 发送请求到 Ollama 上游
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

	// Ollama 默认 300s 超时（大模型推理可能很慢）
	timeout := info.ChannelMeta.Settings.TimeoutSeconds
	if timeout <= 0 {
		timeout = 300
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	return resp, nil
}

// DoResponse 处理 Ollama 上游响应并写回客户端
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeEmbeddings:
		return a.handleEmbeddingResponse(ctx, resp, info, writer)
	case constant.RelayModeCompletions:
		if info.IsStream {
			return a.handleGenerateStreamResponse(ctx, resp, info, writer)
		}
		return a.handleGenerateNonStreamResponse(ctx, resp, info, writer)
	default:
		// Chat Completions
		if info.IsStream {
			return a.handleChatStreamResponse(ctx, resp, info, writer)
		}
		return a.handleChatNonStreamResponse(ctx, resp, info, writer)
	}
}

// GetChannelName 返回渠道名称
func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

// handleChatNonStreamResponse 处理 Ollama Chat 非流式响应，转换为 OpenAI 格式
func (a *Adaptor) handleChatNonStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	var ollamaResp OllamaChatResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err)
	}

	// 转换为 OpenAI ChatCompletion 格式
	finishReason := "stop"
	openaiResp := dto.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", info.RequestID),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   info.OriginModelName,
		Choices: []dto.Choice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    ollamaResp.Message.Role,
					Content: ollamaResp.Message.Content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: dto.UsageWithDetails{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}

	respBody, _ := json.Marshal(openaiResp)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	return &common.Usage{
		PromptTokens:     ollamaResp.PromptEvalCount,
		CompletionTokens: ollamaResp.EvalCount,
		TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
	}, nil
}

// handleChatStreamResponse 处理 Ollama Chat 流式响应（NDJSON → SSE 转换）
func (a *Adaptor) handleChatStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var usage common.Usage

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			return &usage, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var ollamaResp OllamaChatResponse
		if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
			continue
		}

		info.SetFirstResponseTime()

		if ollamaResp.Done {
			// 最后一条消息包含用量信息
			usage.PromptTokens = ollamaResp.PromptEvalCount
			usage.CompletionTokens = ollamaResp.EvalCount
			usage.TotalTokens = ollamaResp.PromptEvalCount + ollamaResp.EvalCount

			// 发送带 finish_reason 的结束 chunk
			finishReason := "stop"
			endChunk := dto.ChatCompletionStreamResponse{
				ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
				Object: "chat.completion.chunk",
				Model:  info.OriginModelName,
				Choices: []dto.StreamChoice{
					{
						Index:        0,
						Delta:        dto.Message{},
						FinishReason: &finishReason,
					},
				},
				Usage: &dto.UsageWithDetails{
					PromptTokens:     usage.PromptTokens,
					CompletionTokens: usage.CompletionTokens,
					TotalTokens:      usage.TotalTokens,
				},
			}
			writeStreamChunk(writer, &endChunk)
			break
		}

		// 构建 OpenAI 流式 chunk
		chunk := dto.ChatCompletionStreamResponse{
			ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
			Object: "chat.completion.chunk",
			Model:  info.OriginModelName,
			Choices: []dto.StreamChoice{
				{
					Index: 0,
					Delta: dto.Message{
						Role:    ollamaResp.Message.Role,
						Content: ollamaResp.Message.Content,
					},
				},
			},
		}
		writeStreamChunk(writer, &chunk)
	}

	// 发送 [DONE]
	_ = helper.WriteSSEData(writer, "[DONE]")
	info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		return &usage, fmt.Errorf("stream scanner error: %w", err)
	}

	return &usage, nil
}

// handleGenerateNonStreamResponse 处理 Ollama Generate 非流式响应，转换为 OpenAI Completions 格式
func (a *Adaptor) handleGenerateNonStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err)
	}

	// 转换为 OpenAI Completions 格式
	openaiResp := dto.CompletionsResponse{
		ID:      fmt.Sprintf("cmpl-%s", info.RequestID),
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   info.OriginModelName,
		Choices: []dto.CompletionsChoice{
			{
				Index:        0,
				Text:         ollamaResp.Response,
				FinishReason: "stop",
			},
		},
		Usage: dto.UsageWithDetails{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}

	respBody, _ := json.Marshal(openaiResp)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	return &common.Usage{
		PromptTokens:     ollamaResp.PromptEvalCount,
		CompletionTokens: ollamaResp.EvalCount,
		TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
	}, nil
}

// handleGenerateStreamResponse 处理 Ollama Generate 流式响应（NDJSON → SSE 转换）
func (a *Adaptor) handleGenerateStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var usage common.Usage

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			return &usage, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var ollamaResp OllamaGenerateResponse
		if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
			continue
		}

		info.SetFirstResponseTime()

		if ollamaResp.Done {
			usage.PromptTokens = ollamaResp.PromptEvalCount
			usage.CompletionTokens = ollamaResp.EvalCount
			usage.TotalTokens = ollamaResp.PromptEvalCount + ollamaResp.EvalCount

			finishReason := "stop"
			endChunk := dto.CompletionsStreamResponse{
				ID:     fmt.Sprintf("cmpl-%s", info.RequestID),
				Object: "text_completion",
				Model:  info.OriginModelName,
				Choices: []dto.CompletionsStreamChoice{
					{
						Index:        0,
						Text:         "",
						FinishReason: &finishReason,
					},
				},
				Usage: &dto.UsageWithDetails{
					PromptTokens:     usage.PromptTokens,
					CompletionTokens: usage.CompletionTokens,
					TotalTokens:      usage.TotalTokens,
				},
			}
			writeStreamChunk(writer, &endChunk)
			break
		}

		chunk := dto.CompletionsStreamResponse{
			ID:     fmt.Sprintf("cmpl-%s", info.RequestID),
			Object: "text_completion",
			Model:  info.OriginModelName,
			Choices: []dto.CompletionsStreamChoice{
				{
					Index: 0,
					Text:  ollamaResp.Response,
				},
			},
		}
		writeStreamChunk(writer, &chunk)
	}

	_ = helper.WriteSSEData(writer, "[DONE]")
	info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		return &usage, fmt.Errorf("stream scanner error: %w", err)
	}

	return &usage, nil
}

// handleEmbeddingResponse 处理 Ollama Embedding 响应，转换为 OpenAI 格式
func (a *Adaptor) handleEmbeddingResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	var ollamaResp OllamaEmbeddingResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err)
	}

	// 转换为 OpenAI Embedding 格式
	embeddings := make([]dto.Embedding, 0, len(ollamaResp.Embeddings))
	for i, emb := range ollamaResp.Embeddings {
		embeddings = append(embeddings, dto.Embedding{
			Object:    "embedding",
			Index:     i,
			Embedding: emb,
		})
	}

	openaiResp := dto.EmbeddingResponse{
		Object: "list",
		Data:   embeddings,
		Model:  info.OriginModelName,
		Usage: dto.UsageWithDetails{
			PromptTokens: 0, // Ollama 不返回 embedding 的 token 用量
			TotalTokens:  0,
		},
	}

	respBody, _ := json.Marshal(openaiResp)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	return &common.Usage{}, nil
}

// writeStreamChunk 将任意结构体序列化为 JSON 并写入 SSE data 行
func writeStreamChunk(w http.ResponseWriter, chunk any) {
	data, _ := json.Marshal(chunk)
	_ = helper.WriteSSEData(w, string(data))
}

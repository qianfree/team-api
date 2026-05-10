package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"

	"github.com/gogf/gf/v2/frame/g"
)

// handleAudioSpeechResponse 处理 TTS 响应（二进制音频流）
func handleAudioSpeechResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			return &common.Usage{}, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
		}
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	// 透传响应头（Content-Type 可能是 audio/mpeg 等）
	for k, vs := range resp.Header {
		for _, v := range vs {
			writer.Header().Add(k, v)
		}
	}
	writer.WriteHeader(http.StatusOK)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read audio response failed", err)
	}

	if _, err := writer.Write(body); err != nil {
		g.Log().Warningf(ctx, "[OpenAI.handleAudioSpeechResponse] write audio body failed: %v", err)
	}

	// TTS 没有标准 usage 返回，按音频大小粗略估算
	// 1 分钟音频 ≈ 1MB mp3 ≈ 1000 tokens
	estimatedTokens := len(body) / 1000
	if estimatedTokens == 0 {
		estimatedTokens = 1
	}
	return &common.Usage{
		CompletionTokens: estimatedTokens,
		TotalTokens:      estimatedTokens,
	}, nil
}

// handleAudioTranscriptionResponse 处理 STT/翻译响应（JSON）
func handleAudioTranscriptionResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read transcription response failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			return &common.Usage{}, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
		}
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)

	// 尝试从响应中提取 usage
	usage := extractAudioUsage(body)
	return usage, nil
}

// extractAudioUsage 从 STT/翻译响应中提取 token 使用量
func extractAudioUsage(body []byte) *common.Usage {
	// 尝试解析为 verbose_json 格式
	var verboseResp dto.WhisperVerboseJSONResponse
	if err := json.Unmarshal(body, &verboseResp); err == nil && verboseResp.Duration > 0 {
		// 按音频时长估算：1 分钟 ≈ 150 tokens
		estimatedTokens := int(verboseResp.Duration/60.0) * 150
		if estimatedTokens == 0 {
			estimatedTokens = 1
		}
		return &common.Usage{
			PromptTokens: estimatedTokens,
			TotalTokens:  estimatedTokens,
		}
	}

	// 普通 json 格式按文本长度估算
	var simpleResp dto.AudioResponse
	if err := json.Unmarshal(body, &simpleResp); err == nil {
		estimatedTokens := len(simpleResp.Text) / 4
		if estimatedTokens == 0 {
			estimatedTokens = 1
		}
		return &common.Usage{
			PromptTokens: estimatedTokens,
			TotalTokens:  estimatedTokens,
		}
	}

	return &common.Usage{}
}

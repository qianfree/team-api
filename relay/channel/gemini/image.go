package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// convertImageRequestToChat 将 OpenAI ImageRequest 转为 Gemini ChatRequest（Banana 内生图模式）
func convertImageRequestToChat(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var imgReq dto.ImageRequest
	if err := json.Unmarshal(requestBody, &imgReq); err != nil {
		return nil, fmt.Errorf("parse image request: %w", err)
	}

	// Banana 内生图模式不支持 N>1，每次请求只生成一张图
	if imgReq.N != nil && *imgReq.N > 1 {
		return nil, constant.NewRequestError("Gemini native image generation (Banana mode) does not support n > 1; use Imagen models (e.g. imagen-3.0-generate-002) or call the endpoint multiple times", nil)
	}

	generationConfig := &dto.GeminiGenerationConfig{
		ResponseModalities: []string{"TEXT", "IMAGE"},
	}

	// 将 OpenAI 的 size 参数映射为 Gemini 的 imageConfig（宽高比 + 分辨率）
	if imgReq.Size != "" || imgReq.Quality != "" {
		generationConfig.ImageConfig = &dto.GeminiImageConfig{
			AspectRatio: imageSizeToAspectRatio(imgReq.Size),
			ImageSize:   qualityToImageSize(imgReq.Quality),
		}
	}

	chatReq := dto.GeminiChatRequest{
		Contents: []dto.GeminiContent{
			{
				Role: "user",
				Parts: []dto.GeminiPart{
					{Text: imgReq.Prompt},
				},
			},
		},
		GenerationConfig: generationConfig,
	}

	result, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini chat request: %w", err)
	}
	return strings.NewReader(string(result)), nil
}

// handleBananaImageResponse 将 Gemini generateContent 响应中的图片转为 OpenAI ImageResponse
func handleBananaImageResponse(_ context.Context, resp *http.Response, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read banana response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &common.Usage{}, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	var geminiResp dto.GeminiChatResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("parse banana response: %w", err)
	}

	openaiResp := dto.ImageResponse{
		Created: 0,
	}

	for _, candidate := range geminiResp.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil {
				openaiResp.Data = append(openaiResp.Data, dto.ImageData{
					B64JSON:     part.InlineData.Data,
					ContentType: part.InlineData.MimeType,
				})
			}
		}
	}

	if len(openaiResp.Data) == 0 {
		return &common.Usage{}, constant.NewRequestError("model did not return any image data", nil)
	}

	usage := &common.Usage{}
	if geminiResp.UsageMetadata != nil {
		usage.PromptTokens = geminiResp.UsageMetadata.PromptTokenCount
		usage.CompletionTokens = geminiResp.UsageMetadata.CandidatesTokenCount
		usage.TotalTokens = geminiResp.UsageMetadata.TotalTokenCount
	}

	respBody, err := json.Marshal(openaiResp)
	if err != nil {
		return usage, fmt.Errorf("marshal openai image response: %w", err)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	return usage, nil
}

// convertImageRequest 将 OpenAI 图片生成请求转换为 Gemini Imagen 格式
func convertImageRequest(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var imgReq dto.ImageRequest
	if err := json.Unmarshal(requestBody, &imgReq); err != nil {
		return nil, fmt.Errorf("parse image request: %w", err)
	}

	sampleCount := 1
	if imgReq.N != nil && *imgReq.N > 0 {
		sampleCount = *imgReq.N
	}

	geminiReq := dto.GeminiImageRequest{
		Instances: []dto.GeminiImageInstance{{Prompt: imgReq.Prompt}},
		Parameters: dto.GeminiImageParameters{
			SampleCount:      sampleCount,
			AspectRatio:      imageSizeToAspectRatio(imgReq.Size),
			PersonGeneration: "allow_adult",
			ImageSize:        qualityToImageSize(imgReq.Quality),
		},
	}

	result, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini image request: %w", err)
	}
	return strings.NewReader(string(result)), nil
}

// handleImagenResponse 将 Gemini Imagen 响应转换为 OpenAI 图片格式
func handleImagenResponse(_ context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read imagen response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &common.Usage{}, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	var geminiResp dto.GeminiImageResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("parse imagen response: %w", err)
	}

	openaiResp := dto.ImageResponse{
		Created: info.StartTime.Unix(),
	}

	for _, pred := range geminiResp.Predictions {
		if pred.RaiFilteredReason != "" {
			continue
		}
		openaiResp.Data = append(openaiResp.Data, dto.ImageData{
			B64JSON:     pred.BytesBase64Encoded,
			ContentType: pred.MimeType,
		})
	}

	if len(openaiResp.Data) == 0 {
		return &common.Usage{}, constant.NewRequestError("all images were filtered by safety filters", nil)
	}

	respBody, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, fmt.Errorf("marshal openai image response: %w", err)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	return &common.Usage{}, nil
}

// imageSizeToAspectRatio 将图片尺寸映射为 Gemini 宽高比
// 支持 OpenAI 格式（1024x1024）和 Gemini 原生比例格式（1:1/16:9 等）直接透传
func imageSizeToAspectRatio(size string) string {
	if size == "" {
		return ""
	}
	// OpenAI 格式映射
	switch size {
	case "1024x1024":
		return "1:1"
	case "1792x1024", "1280x720":
		return "16:9"
	case "1024x1792", "720x1280":
		return "9:16"
	case "1536x1024":
		return "3:2"
	case "1024x1536":
		return "2:3"
	case "1152x864":
		return "4:3"
	case "864x1152":
		return "3:4"
	case "1344x576":
		return "21:9"
	}

	// Gemini 原生比例格式（如 "1:1"、"16:9"、"3:4"）直接透传
	if strings.Contains(size, ":") {
		return size
	}

	return "1:1"
}

// qualityToImageSize 将 OpenAI quality 映射为 Gemini imageSize
// 支持 OpenAI 格式（hd/standard）和 Gemini 原生格式（256/512/1K/2K/4K）直接透传
func qualityToImageSize(quality string) string {
	if quality == "" {
		return ""
	}
	// Gemini 原生格式直接透传（256/512/1K/2K/4K）
	lower := strings.ToLower(quality)
	switch lower {
	case "256", "512", "1k", "2k", "4k":
		return quality // 保持用户原始大小写
	}
	// OpenAI 兼容格式映射
	switch lower {
	case "hd", "high":
		return "2K"
	default:
		return "1K"
	}
}

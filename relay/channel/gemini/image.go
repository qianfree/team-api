package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// convertImageRequestToChat 将 OpenAI ImageRequest 转为 Gemini ChatRequest（Banana 内生图模式）
func convertImageRequestToChat(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var imgReq dto.ImageRequest
	if err := json.Unmarshal(requestBody, &imgReq); err != nil {
		return nil, fmt.Errorf("parse image request: %w", err)
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
		GenerationConfig: &dto.GeminiGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
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
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		return &common.Usage{}, fmt.Errorf("banana API error: %d, body: %s", resp.StatusCode, string(body))
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
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		errBody, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"type":    "banana_no_image",
				"message": "model did not return any image data",
			},
		})
		_, _ = writer.Write(errBody)
		return &common.Usage{}, fmt.Errorf("banana response contained no image data")
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
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		return &common.Usage{}, fmt.Errorf("imagen API error: %d", resp.StatusCode)
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
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		errBody, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"type":    "imagen_filtered",
				"message": "all images were filtered by safety filters",
			},
		})
		_, _ = writer.Write(errBody)
		return &common.Usage{}, fmt.Errorf("all images filtered by safety")
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

// imageSizeToAspectRatio 将 OpenAI 图片尺寸映射为 Imagen 宽高比
func imageSizeToAspectRatio(size string) string {
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

	// 支持原生比例格式（如 "1:1"、"16:9"）
	if strings.Contains(size, ":") {
		return size
	}

	return "1:1"
}

// qualityToImageSize 将 OpenAI quality 映射为 Imagen imageSize
func qualityToImageSize(quality string) string {
	switch strings.ToLower(quality) {
	case "hd", "high":
		return "2K"
	default:
		return "1K"
	}
}

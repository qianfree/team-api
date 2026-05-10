package minimax

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// MiniMax 图片生成请求/响应结构

type miniMaxImageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	N              int    `json:"n,omitempty"`
}

type miniMaxImageResponse struct {
	ID   string `json:"id"`
	Data struct {
		ImageURLs   []string `json:"image_urls"`
		ImageBase64 []string `json:"image_base64"`
	} `json:"data"`
	Metadata map[string]any `json:"metadata"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// convertImageRequest 将 OpenAI 图片生成请求转换为 MiniMax 格式
func convertImageRequest(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var imgReq dto.ImageRequest
	if err := json.Unmarshal(requestBody, &imgReq); err != nil {
		return nil, fmt.Errorf("parse image request: %w", err)
	}

	minimaxReq := miniMaxImageRequest{
		Model:          info.ChannelMeta.UpstreamModelName,
		Prompt:         imgReq.Prompt,
		ResponseFormat: normalizeResponseFormat(imgReq.ResponseFormat),
		N:              1,
	}

	if minimaxReq.Model == "" {
		minimaxReq.Model = "image-01"
	}
	if imgReq.N != nil && *imgReq.N > 0 {
		minimaxReq.N = *imgReq.N
	}
	if ar := aspectRatioFromSize(imgReq.Size); ar != "" {
		minimaxReq.AspectRatio = ar
	}

	result, err := json.Marshal(minimaxReq)
	if err != nil {
		return nil, fmt.Errorf("marshal minimax image request: %w", err)
	}
	return strings.NewReader(string(result)), nil
}

// handleImageResponse 将 MiniMax 图片生成响应转换为 OpenAI 格式
func handleImageResponse(resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read image response body: %w", err)
	}

	var mmResp miniMaxImageResponse
	if err := json.Unmarshal(body, &mmResp); err != nil {
		return nil, fmt.Errorf("parse minimax image response: %w", err)
	}

	if mmResp.BaseResp.StatusCode != 0 {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		errBody, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"type":    "minimax_image_error",
				"message": mmResp.BaseResp.StatusMsg,
				"code":    fmt.Sprintf("%d", mmResp.BaseResp.StatusCode),
			},
		})
		_, _ = writer.Write(errBody)
		return &common.Usage{}, fmt.Errorf("minimax image error: %s (code %d)", mmResp.BaseResp.StatusMsg, mmResp.BaseResp.StatusCode)
	}

	openaiResp := dto.ImageResponse{
		Created: info.StartTime.Unix(),
	}

	for _, url := range mmResp.Data.ImageURLs {
		openaiResp.Data = append(openaiResp.Data, dto.ImageData{URL: url})
	}
	for _, b64 := range mmResp.Data.ImageBase64 {
		openaiResp.Data = append(openaiResp.Data, dto.ImageData{B64JSON: b64})
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

// normalizeResponseFormat 将 OpenAI 响应格式映射为 MiniMax 格式
func normalizeResponseFormat(format string) string {
	switch strings.ToLower(format) {
	case "", "url":
		return "url"
	case "b64_json", "base64":
		return "base64"
	default:
		return format
	}
}

// aspectRatioFromSize 将 OpenAI 图片尺寸映射为 MiniMax 宽高比
func aspectRatioFromSize(size string) string {
	switch size {
	case "1024x1024":
		return "1:1"
	case "1792x1024":
		return "16:9"
	case "1024x1792":
		return "9:16"
	case "1536x1024", "1248x832":
		return "3:2"
	case "1024x1536", "832x1248":
		return "2:3"
	case "1152x864":
		return "4:3"
	case "864x1152":
		return "3:4"
	case "1344x576":
		return "21:9"
	}

	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return ""
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return ""
	}

	d := gcd(w, h)
	ratio := fmt.Sprintf("%d:%d", w/d, h/d)
	for _, valid := range []string{"1:1", "16:9", "4:3", "3:2", "2:3", "3:4", "9:16", "21:9"} {
		if ratio == valid {
			return ratio
		}
	}
	return ""
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

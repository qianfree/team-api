package ali

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// 阿里云 DashScope「同步 multimodal」图片生成（qwen-image-2.x 系列）。
//
// 与异步 wanx / qwen-image image-synthesis 端点不同，qwen-image-2.0 仅支持同步调用，走
// /api/v1/services/aigc/multimodal-generation/generation，请求体为 messages 格式、响应从
// output.choices[].message.content[].image 取图片 URL。这里做 OpenAI Images ↔ DashScope
// multimodal 的双向转换，使其能经同步 /v1/images/generations 一次性返回。

// isMultimodalImageMode 判断当前请求是否为阿里云同步 multimodal 图片生成。
func isMultimodalImageMode(info *common.RelayInfo) bool {
	return constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations &&
		constant.IsAliSyncMultimodalImageModel(info.ChannelMeta.UpstreamModelName)
}

// ==================== 请求/响应结构体 ====================

type multimodalImageContent struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

type multimodalImageMessage struct {
	Role    string                   `json:"role"`
	Content []multimodalImageContent `json:"content"`
}

type multimodalImageInput struct {
	Messages []multimodalImageMessage `json:"messages"`
}

type multimodalImageRequest struct {
	Model      string               `json:"model"`
	Input      multimodalImageInput `json:"input"`
	Parameters map[string]any       `json:"parameters,omitempty"`
}

type multimodalImageResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content []multimodalImageContent `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// ==================== 转换实现 ====================

// referenceImages 从 OpenAI Images 请求体提取参考图（图生图/编辑素材）。
// 字段形态与网关 dto.ImageRequest 约定一致：image 接受单张字符串或多张数组，images 为数组；
// 取值可以是 URL 或 base64 data URL（DashScope multimodal 的 image 内容项两种都收）。
func referenceImages(req map[string]any) []string {
	var out []string
	collect := func(v any) {
		switch t := v.(type) {
		case string:
			if t != "" {
				out = append(out, t)
			}
		case []any:
			for _, item := range t {
				if s, ok := item.(string); ok && s != "" {
					out = append(out, s)
				}
			}
		}
	}
	collect(req["image"])
	collect(req["images"])
	return out
}

// convertMultimodalImageRequest 将 OpenAI Images 请求转换为 DashScope multimodal 请求体。
func convertMultimodalImageRequest(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var req map[string]any
	if err := json.Unmarshal(requestBody, &req); err != nil {
		return nil, fmt.Errorf("parse image request: %w", err)
	}

	prompt, _ := req["prompt"].(string)
	content := make([]multimodalImageContent, 0, 2)
	// 参考图（如有）排在文本前：DashScope multimodal 里图片素材与文本同为 content 项，
	// 垫图/编辑模型按内容顺序理解「先给图、再给指令」
	for _, img := range referenceImages(req) {
		content = append(content, multimodalImageContent{Image: img})
	}
	content = append(content, multimodalImageContent{Text: prompt})
	mm := multimodalImageRequest{
		Model: info.ChannelMeta.UpstreamModelName,
		Input: multimodalImageInput{
			Messages: []multimodalImageMessage{
				{Role: "user", Content: content},
			},
		},
		Parameters: map[string]any{},
	}

	// size: 1024x1024 → 1024*1024
	if v, ok := req["size"].(string); ok && v != "" {
		mm.Parameters["size"] = strings.ReplaceAll(v, "x", "*")
	}
	if v, ok := req["n"]; ok {
		mm.Parameters["n"] = v
	}
	if v, ok := req["negative_prompt"].(string); ok && v != "" {
		mm.Parameters["negative_prompt"] = v
	}
	if v, ok := req["seed"]; ok {
		mm.Parameters["seed"] = v
	}
	if v, ok := req["prompt_extend"]; ok {
		mm.Parameters["prompt_extend"] = v
	}
	if v, ok := req["watermark"]; ok {
		mm.Parameters["watermark"] = v
	}
	if len(mm.Parameters) == 0 {
		mm.Parameters = nil
	}

	data, err := json.Marshal(mm)
	if err != nil {
		return nil, fmt.Errorf("marshal multimodal image request: %w", err)
	}
	return bytes.NewReader(data), nil
}

// handleMultimodalImageResponse 将 DashScope multimodal 响应转换为 OpenAI Images 响应写回客户端。
func handleMultimodalImageResponse(resp *http.Response, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp multimodalImageResponse
		msg := string(body)
		if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
			msg = errResp.Message
		}
		return nil, constant.NewUpstreamError(resp.StatusCode, msg, nil)
	}

	var mmResp multimodalImageResponse
	if err := json.Unmarshal(body, &mmResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "parse response failed", err)
	}
	// 200 但业务错误（顶层 code/message）
	if mmResp.Code != "" {
		return nil, constant.NewUpstreamError(resp.StatusCode, mmResp.Message, nil)
	}

	// 收集所有 choices 里的图片 URL
	data := make([]map[string]any, 0)
	for _, ch := range mmResp.Output.Choices {
		for _, c := range ch.Message.Content {
			if c.Image != "" {
				data = append(data, map[string]any{"url": c.Image})
			}
		}
	}
	if len(data) == 0 {
		return nil, constant.NewUpstreamError(resp.StatusCode, "upstream returned no image", nil)
	}

	out := map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
	}
	outBody, err := json.Marshal(out)
	if err != nil {
		return nil, constant.NewUpstreamError(http.StatusInternalServerError, "marshal image response failed", err)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(outBody)

	// 图片按次/张计费，token 用量为空（与 OpenAI dall-e 处理一致）
	return &common.Usage{}, nil
}

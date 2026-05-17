package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
)

// imageStreamEvent GPT Image SSE 事件
type imageStreamEvent struct {
	Type  string          `json:"type"`
	Data  []dto.ImageData `json:"data"`
	Usage *dto.ImageUsage `json:"usage"`
}

// handleImageResponse 处理图像生成非流式响应
func (a *Adaptor) handleImageResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if info.IsStream {
		return a.handleImageStreamResponse(ctx, resp, info, writer)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	// 解析 GPT Image 响应中的 usage（dall-e-2/3 无此字段，imgResp.Usage 为 nil）
	var imgResp dto.ImageResponse
	usage := &common.Usage{}
	if err := json.Unmarshal(body, &imgResp); err == nil && imgResp.Usage != nil {
		usage = imageUsageToCommon(imgResp.Usage)
	}

	// 透传原始响应
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(resp.StatusCode)
	_, _ = writer.Write(body)

	return usage, nil
}

// handleImageStreamResponse 处理 GPT Image 流式响应（SSE）
func (a *Adaptor) handleImageStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	var totalUsage common.Usage

	helper.StreamScannerHandler(ctx, resp, info, writer, func(data string, sr *helper.StreamResult) {
		// 透传 SSE 数据到客户端
		if err := helper.WriteSSEData(writer, data); err != nil {
			sr.Stop(err)
			return
		}

		// 解析 usage（generation.completed 事件包含完整 usage）
		var event imageStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err == nil && event.Usage != nil {
			u := imageUsageToCommon(event.Usage)
			totalUsage.PromptTokens = u.PromptTokens
			totalUsage.CompletionTokens = u.CompletionTokens
			totalUsage.TotalTokens = u.TotalTokens
			totalUsage.PromptTokensDetails = u.PromptTokensDetails
		}
	})

	if info.StreamStatus == nil || !info.StreamStatus.IsPartialStreamEnd() {
		_ = helper.WriteSSEData(writer, "[DONE]")
	}

	if info.StreamStatus != nil && info.StreamStatus.IsPartialStreamEnd() {
		return &totalUsage, common.ErrStreamInterrupted
	}
	return &totalUsage, nil
}

// imageUsageToCommon 将 GPT Image usage 映射到通用 Usage 结构
func imageUsageToCommon(u *dto.ImageUsage) *common.Usage {
	usage := &common.Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.TotalTokens,
	}
	if u.InputTokensDetails != nil {
		usage.PromptTokensDetails = &common.TokenDetails{
			TextTokens:  u.InputTokensDetails.TextTokens,
			ImageTokens: u.InputTokensDetails.ImageTokens,
		}
	}
	return usage
}

package volcengine

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
)

// DispatchAdaptor 火山引擎派发适配器：按入站协议分流。
//   - Claude 入站 → claudeAdaptor（Anthropic 兼容端点 /api/coding/v1/messages）
//   - 其余（OpenAI/Gemini/Responses/Embeddings/Images 等）→ openaiAdaptor（OpenAI 兼容端点 /api/v3/*）
//
// 火山两链路 URL 前缀不兼容（/api/v3 vs /api/coding）且共享同一 baseURL，
// 无法复用现成 openai/claude 适配器的 GetRequestURL；鉴权头统一 Bearer、
// HTTP 发送逻辑共享，故用 setupHeader/doSend 辅助函数复用，子适配器只定制 URL/Convert/DoResponse。
type DispatchAdaptor struct {
	common.Adaptor
}

// Init 按入站协议选定子适配器并完成初始化。
func (a *DispatchAdaptor) Init(info *common.RelayInfo) {
	if info.GetOriginalClientFormat() == constant.RelayFormatClaude {
		a.Adaptor = &claudeAdaptor{}
	} else {
		a.Adaptor = &openaiAdaptor{}
	}
	a.Adaptor.Init(info)
}

// GetChannelName 覆盖子适配器名称，日志统一记为火山渠道。
func (a *DispatchAdaptor) GetChannelName() string {
	return ChannelName
}

// setupHeader 火山统一鉴权头（Bearer）。两链路共用。
func setupHeader(header http.Header, info *common.RelayInfo) {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
}

// doSend 火山统一 HTTP 发送。url 由子适配器的 GetRequestURL 计算。
func doSend(ctx context.Context, info *common.RelayInfo, url string, requestBody io.Reader) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	setupHeader(httpReq.Header, info)
	if hdrOverrides, hdrErr := override.ApplyHeaderOverride(info); hdrErr == nil && len(hdrOverrides) > 0 {
		override.MergeHeaderOverrides(httpReq.Header, hdrOverrides)
	}
	timeout := info.ChannelMeta.Settings.GetTimeoutSeconds(info.RelayMode)
	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)
	return client.Do(httpReq)
}

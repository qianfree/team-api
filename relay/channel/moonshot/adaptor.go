package moonshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/qianfree/team-api/relay/channel/claude"
	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
)

// Adaptor Moonshot 供应商适配器（OpenAI 兼容）
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL。
// Claude 协议走 Anthropic 兼容端点，其他走标准 OpenAI 路径。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeClaudeMessages:
		return baseURL + "/anthropic/v1/messages", nil
	case constant.RelayModeChatCompletions:
		return baseURL + "/v1/chat/completions", nil
	case constant.RelayModeCompletions:
		return baseURL + "/v1/completions", nil
	case constant.RelayModeEmbeddings:
		return baseURL + "/v1/embeddings", nil
	default:
		return baseURL + "/v1/chat/completions", nil
	}
}

func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 转换请求体。
// Claude 入站直接透传到 Anthropic 兼容端点，其他格式先转为 OpenAI 再做模型映射。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// Claude 入站：仅做模型映射
	if info.InboundFormat == constant.RelayFormatClaude {
		return convertClaudeRequest(requestBody, info)
	}

	if info.ChannelMeta.IsModelMapped {
		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(requestBody, &rawMap); err != nil {
			return bytes.NewReader(requestBody), nil
		}
		rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
		converted, err := json.Marshal(rawMap)
		if err != nil {
			return nil, fmt.Errorf("marshal converted request failed: %w", err)
		}
		return bytes.NewReader(converted), nil
	}
	return bytes.NewReader(requestBody), nil
}

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

	timeout := info.ChannelMeta.Settings.GetTimeoutSeconds(info.RelayMode)

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理上游响应。
// Claude 入站委托 claude.Adaptor 原生直通；其他格式委托 openai.Adaptor。
// OpenAI 系路径补读 Kimi 缓存写入 TTL 响应头（Msh-Usage-Cache-Write-Tokens-5m/1h）：
// Kimi Chat/Responses 的 usage JSON 只报写入总量（cache_write_tokens），TTL 细分仅在
// 响应头，不读头则 1h 写入全部按短桶基础价计费（官方 1h 价为短桶 2×，少收一半）
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if info.GetOriginalClientFormat() == constant.RelayFormatClaude {
		delegate := &claude.Adaptor{}
		delegate.Init(info)
		return delegate.DoResponse(ctx, resp, info, writer)
	}

	ttl5m, ttl1h := cacheWriteTTLFromHeader(resp.Header)
	delegate := &openai.Adaptor{}
	delegate.Init(info)
	usage, err := delegate.DoResponse(ctx, resp, info, writer)
	if err != nil {
		return usage, err
	}
	applyCacheWriteTTL(ttl5m, ttl1h, usage)
	return usage, nil
}

// Kimi 缓存写入 TTL 响应头：各档本次写入的 token 数（全命中无新增写入时为 0）
const (
	mshHeaderCacheWrite5m = "Msh-Usage-Cache-Write-Tokens-5m"
	mshHeaderCacheWrite1h = "Msh-Usage-Cache-Write-Tokens-1h"
)

// cacheWriteTTLFromHeader 读取 TTL 响应头（缺失/非法/负值按 0 处理）
func cacheWriteTTLFromHeader(h http.Header) (fiveMin, oneHour int) {
	parse := func(key string) int {
		v, err := strconv.Atoi(h.Get(key))
		if err != nil || v < 0 {
			return 0
		}
		return v
	}
	return parse(mshHeaderCacheWrite5m), parse(mshHeaderCacheWrite1h)
}

// applyCacheWriteTTL 将响应头 TTL 写入量填入计费双桶。Kimi 的 TTL 就是 5m/1h，
// 与双桶字面同构，无需归一换算。防御：
//   - usage 已带 TTL 细分（上游开始在 JSON 里报）不覆盖；
//   - 头值与 JSON 写入总量矛盾（和大于 cache_write_tokens 且总量 > 0）时丢弃，
//     回退 resolveTokenCounts 合并桶口径，不重复计费；
//   - JSON 未报写入总量时以头值补齐，保证 usage_logs/合并桶口径完整
func applyCacheWriteTTL(ttl5m, ttl1h int, usage *common.Usage) {
	if usage == nil || (ttl5m <= 0 && ttl1h <= 0) {
		return
	}
	d := usage.PromptTokensDetails
	if d == nil {
		d = &common.TokenDetails{}
		usage.PromptTokensDetails = d
	}
	if d.CachedCreation5mTokens > 0 || d.CachedCreation1hTokens > 0 {
		return
	}
	if d.CacheWriteTokens > 0 && ttl5m+ttl1h > d.CacheWriteTokens {
		return
	}
	d.CachedCreation5mTokens = ttl5m
	d.CachedCreation1hTokens = ttl1h
	if d.CacheWriteTokens < ttl5m+ttl1h {
		d.CacheWriteTokens = ttl5m + ttl1h
	}
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

// convertClaudeRequest 处理 Claude 入站请求的模型映射。
func convertClaudeRequest(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	if !info.ChannelMeta.IsModelMapped {
		return bytes.NewReader(requestBody), nil
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(requestBody), nil
	}
	rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(requestBody), nil
	}
	return bytes.NewReader(result), nil
}

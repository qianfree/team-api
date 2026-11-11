package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/relay/constant"
)

// FetchChannelUpstreamModels 拉取渠道上游可用模型列表。
//
// 只读探测：用渠道配置的 base_url 与一个可用 Key 请求上游的模型列表接口，
// 结果仅供「添加模型能力」弹窗挑选，不落库、不影响调度目录。
//
// 各家列表协议不一致，按供应商分支：
//   - Gemini：GET /v1beta/models（x-goog-api-key）
//   - Claude：GET /v1/models（x-api-key + anthropic-version）
//   - 其它：GET /v1/models（Bearer，OpenAI 兼容；覆盖 DeepSeek / Moonshot / 智谱 / 各类中转站）
//
// 拉不到时返回可读错误，前端降级为手动输入——不阻断添加流程。
func (s *sAdmin) FetchChannelUpstreamModels(ctx context.Context, req *v1.ChannelUpstreamModelsReq) (*v1.ChannelUpstreamModelsRes, error) {
	type channelRow struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Type     int    `json:"type"`
		BaseURL  string `json:"base_url"`
		Settings string `json:"settings"`
	}

	var ch *channelRow
	if err := dao.ChnChannels.Ctx(ctx).
		Where("id", req.ChannelID).
		Fields("id, name, type, base_url, settings").
		Scan(&ch); err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, common.NewNotFoundError("渠道")
	}

	providerType := constant.ProviderType(ch.Type)
	if !supportsUpstreamModelList(providerType) {
		return nil, common.NewBadRequestError(fmt.Sprintf(
			"「%s」类型渠道没有标准模型列表接口，请手动填写模型名", providerType.String()))
	}

	type keyRow struct {
		EncryptedKey string `json:"encrypted_key"`
	}
	var keyInfo *keyRow
	if err := dao.ChnChannelKeys.Ctx(ctx).
		Where("channel_id", req.ChannelID).
		Where("status", "active").
		Fields("encrypted_key").
		OrderAsc("last_used_at").
		Limit(1).
		Scan(&keyInfo); err != nil {
		return nil, err
	}
	if keyInfo == nil || keyInfo.EncryptedKey == "" {
		return nil, common.NewBadRequestError("渠道没有可用的 API Key，请先在「凭证」页添加后再获取上游模型")
	}

	apiKey, err := relay.DecryptChannelKey(relay.GetEncryptionKey(), keyInfo.EncryptedKey)
	if err != nil {
		return nil, common.NewBadRequestError("解密 API Key 失败")
	}

	// 与测试渠道保持一致的代理开关口径（settings.use_proxy）
	useProxy := false
	if ch.Settings != "" {
		var settings struct {
			UseProxy bool `json:"use_proxy"`
		}
		if json.Unmarshal([]byte(ch.Settings), &settings) == nil {
			useProxy = settings.UseProxy
		}
	}

	start := time.Now()
	list, protocol, err := fetchUpstreamModelList(ctx, providerType, ch.BaseURL, apiKey, useProxy)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		g.Log().Warningf(ctx, "[UpstreamModels] 渠道 %s (%d) 拉取上游模型失败 | 协议: %s | 耗时: %dms | 错误: %v",
			ch.Name, ch.ID, protocol, elapsed, err)
		return nil, err
	}

	g.Log().Infof(ctx, "[UpstreamModels] 渠道 %s (%d) 拉取上游模型成功 | 协议: %s | 数量: %d | 耗时: %dms",
		ch.Name, ch.ID, protocol, len(list), elapsed)

	return &v1.ChannelUpstreamModelsRes{
		List:      list,
		Total:     len(list),
		Protocol:  protocol,
		ElapsedMs: elapsed,
	}, nil
}

// supportsUpstreamModelList 判断该类型渠道是否具备标准模型列表接口。
// 任务/生成类渠道（视频、图像、音乐）只有提交任务与轮询结果接口，没有 /models。
func supportsUpstreamModelList(pt constant.ProviderType) bool {
	switch pt {
	case constant.ProviderJimeng, constant.ProviderSora, constant.ProviderKling,
		constant.ProviderSuno, constant.ProviderMidjourney, constant.ProviderJSTask:
		return false
	default:
		return true
	}
}

// fetchUpstreamModelList 按供应商协议请求模型列表，返回（模型列表、协议标识、错误）。
func fetchUpstreamModelList(ctx context.Context, pt constant.ProviderType, baseURL, apiKey string, useProxy bool) ([]v1.UpstreamModelItem, string, error) {
	base := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, "", common.NewBadRequestError("渠道未配置 API 地址（base_url），无法获取上游模型列表")
	}

	protocol := "openai"
	reqURL := joinModelsURL(base, "/v1/models")
	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": "Bearer " + apiKey,
	}

	switch pt {
	case constant.ProviderGemini:
		protocol = "gemini"
		reqURL = joinModelsURL(base, "/v1beta/models") + "?pageSize=1000"
		headers = map[string]string{
			"Accept":         "application/json",
			"x-goog-api-key": apiKey,
		}
	case constant.ProviderClaude:
		protocol = "claude"
		reqURL = joinModelsURL(base, "/v1/models")
		headers = map[string]string{
			"Accept":            "application/json",
			"x-api-key":         apiKey,
			"anthropic-version": "2023-06-01",
		}
	}

	resp, err := doTestHTTPRequest(ctx, "GET", reqURL, headers, nil, useProxy)
	if err != nil {
		return nil, protocol, common.NewBadRequestError(fmt.Sprintf("请求上游失败：%v", err))
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, protocol, common.NewBadRequestError(fmt.Sprintf(
			"上游返回 HTTP %d：%s（可手动填写模型名）", resp.StatusCode, truncateStr(strings.TrimSpace(string(raw)), 200)))
	}

	list := parseUpstreamModelList(raw)
	if len(list) == 0 {
		return nil, protocol, common.NewBadRequestError(
			"上游未返回可识别的模型列表（可能不支持该接口），请手动填写模型名")
	}
	return list, protocol, nil
}

// joinModelsURL 拼接模型列表地址。
// 中转站与云厂商的 base_url 常已带版本段（https://host/v1、https://open.bigmodel.cn/api/paas/v4、
// https://dashscope.aliyuncs.com/compatible-mode/v1），再拼一次 /v1/models 会得到
// /v1/v1/models 这种 404 地址，所以这里检测 base 是否已有版本段，有则只补 /models。
func joinModelsURL(base, suffix string) string {
	base = strings.TrimSuffix(strings.TrimSpace(base), "/")
	_, tail := splitVersionSuffix(suffix)
	if hasVersionSegment(base) {
		return base + tail
	}
	return base + suffix
}

// splitVersionSuffix 把 "/v1/models" 拆成 ("/v1", "/models")；没有版本段时返回 ("", suffix)。
func splitVersionSuffix(suffix string) (string, string) {
	i := strings.LastIndex(suffix, "/")
	if i <= 0 {
		return "", suffix
	}
	head, tail := suffix[:i], suffix[i:]
	if !isVersionSegment(head[strings.LastIndex(head, "/")+1:]) {
		return "", suffix
	}
	return head, tail
}

func hasVersionSegment(base string) bool {
	for _, seg := range strings.Split(base, "/") {
		if isVersionSegment(seg) {
			return true
		}
	}
	return false
}

// isVersionSegment 识别 v1 / v4 / v1beta / v2alpha 这类版本段。
func isVersionSegment(seg string) bool {
	s := strings.ToLower(seg)
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	rest := s[1:]
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i == 0 {
		return false
	}
	rest = rest[i:]
	return rest == "" || rest == "alpha" || rest == "beta"
}

// upstreamModelEntry 兼容各家列表条目的字段差异（OpenAI data[].id / Gemini models[].name）。
type upstreamModelEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Model       string `json:"model"`
	OwnedBy     string `json:"owned_by"`
	OwnedByAlt  string `json:"ownedBy"`
	DisplayName string `json:"display_name"`
}

// parseUpstreamModelList 解析上游返回的 JSON，兼容多种信封结构与数组根。
func parseUpstreamModelList(raw []byte) []v1.UpstreamModelItem {
	var entries []upstreamModelEntry

	var env struct {
		Data   []upstreamModelEntry `json:"data"`
		Models []upstreamModelEntry `json:"models"`
		Result []upstreamModelEntry `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err == nil {
		switch {
		case len(env.Data) > 0:
			entries = env.Data
		case len(env.Models) > 0:
			entries = env.Models
		case len(env.Result) > 0:
			entries = env.Result
		}
	}

	if len(entries) == 0 {
		var arr []upstreamModelEntry
		if json.Unmarshal(raw, &arr) == nil {
			entries = arr
		}
	}

	if len(entries) == 0 {
		var strs []string
		if json.Unmarshal(raw, &strs) == nil {
			out := make([]v1.UpstreamModelItem, 0, len(strs))
			for _, s := range strs {
				if id := normalizeUpstreamModelID(s); id != "" {
					out = append(out, v1.UpstreamModelItem{ID: id})
				}
			}
			return dedupeAndSortModels(out)
		}
	}

	out := make([]v1.UpstreamModelItem, 0, len(entries))
	for _, e := range entries {
		id := e.ID
		if strings.TrimSpace(id) == "" {
			id = e.Model
		}
		if strings.TrimSpace(id) == "" {
			id = e.Name
		}
		id = normalizeUpstreamModelID(id)
		if id == "" {
			continue
		}
		ownedBy := e.OwnedBy
		if ownedBy == "" {
			ownedBy = e.OwnedByAlt
		}
		out = append(out, v1.UpstreamModelItem{ID: id, OwnedBy: ownedBy})
	}
	return dedupeAndSortModels(out)
}

// normalizeUpstreamModelID 清理模型名：Gemini 返回 models/gemini-2.0-flash，需要去掉前缀。
func normalizeUpstreamModelID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "models/")
	return id
}

func dedupeAndSortModels(list []v1.UpstreamModelItem) []v1.UpstreamModelItem {
	seen := make(map[string]bool, len(list))
	out := make([]v1.UpstreamModelItem, 0, len(list))
	for _, m := range list {
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

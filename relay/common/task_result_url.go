package common

import (
	"encoding/json"
	"strings"
)

// ResultURLFromArtifacts 从 url 型产物声明回填任务 ResultURL（tsk_model_tasks.result_url）。
// 插件契约 v1 无 resultUrl 成员，产物声明是成品地址的唯一来源；不回填则 /v1/videos/{id}/content
// 等依赖 result_url 的标准端点恒报 content_not_available。
// 仅取绝对 http(s) 直链：相对 URL / channel-bearer 产物需渠道鉴权才能下载，回源拉流无凭据，
// 这类产物只能等转存完成后由对象存储供给。多产物时优先 video/* 类型。
func ResultURLFromArtifacts(data []byte, artifacts []TaskArtifact) string {
	if len(artifacts) == 0 || len(data) == 0 {
		return ""
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return ""
	}
	fallback := ""
	for _, art := range artifacts {
		if art.SourceType != "url" || art.SourceAuth != "" {
			continue
		}
		raw, _ := GetJSONPath(snapshot, art.SourcePath).(string)
		raw = strings.TrimSpace(raw)
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			continue
		}
		if strings.HasPrefix(art.MimeType, "video/") {
			return raw
		}
		if fallback == "" {
			fallback = raw
		}
	}
	return fallback
}

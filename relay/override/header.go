package override

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/qianfree/team-api/relay/common"
)

// ApplyHeaderOverride 处理渠道级 Header 覆盖规则。
// 返回最终要设置的 header map（覆盖 adaptor 的默认 header）。
func ApplyHeaderOverride(info *common.RelayInfo) (map[string]string, error) {
	headerOverride := info.ChannelMeta.Settings.HeaderOverride
	if len(headerOverride) == 0 && info.RuntimeHeadersOverride == nil {
		return nil, nil
	}

	result := make(map[string]string)

	// 1. 先处理渠道级静态 HeaderOverride
	for key, value := range headerOverride {
		strVal := toString(value)

		// 通配符透传：* 表示透传所有客户端 header（排除不安全的）
		if key == "*" {
			if info.RequestHeaders != nil {
				for h, vals := range info.RequestHeaders {
					if isUnsafeHeader(h) {
						continue
					}
					if len(vals) > 0 && vals[0] != "" {
						result[h] = vals[0]
					}
				}
			}
			continue
		}

		// 正则透传：re:pattern 或 regex:pattern
		if strings.HasPrefix(key, "re:") || strings.HasPrefix(key, "regex:") {
			pattern := key
			if strings.HasPrefix(key, "regex:") {
				pattern = key[6:]
			} else {
				pattern = key[3:]
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if info.RequestHeaders != nil {
				for h, vals := range info.RequestHeaders {
					if isUnsafeHeader(h) {
						continue
					}
					if re.MatchString(h) && len(vals) > 0 && vals[0] != "" {
						result[h] = vals[0]
					}
				}
			}
			continue
		}

		// 替换占位符
		resolved := resolveHeaderPlaceholder(strVal, info)
		result[key] = resolved
	}

	// 2. 运行时 header 覆盖（来自 ParamOverride 中的 set_header/delete_header）优先级最高
	if info.RuntimeHeadersOverride != nil {
		for k, v := range info.RuntimeHeadersOverride {
			result[k] = v
		}
	}

	return result, nil
}

// resolveHeaderPlaceholder 替换 header 值中的占位符
func resolveHeaderPlaceholder(template string, info *common.RelayInfo) string {
	result := template

	// {api_key} → 渠道 API Key
	result = strings.ReplaceAll(result, "{api_key}", info.ChannelMeta.ApiKey)

	// {client_header:xxx} → 客户端原始请求 header
	for {
		start := strings.Index(result, "{client_header:")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start
		headerName := result[start+len("{client_header:") : end]
		clientValue := ""
		if info.RequestHeaders != nil {
			if vals := info.RequestHeaders.Values(headerName); len(vals) > 0 {
				clientValue = vals[0]
			}
		}
		result = result[:start] + clientValue + result[end+1:]
	}

	return result
}

// MergeHeaderOverrides 将 header override 合并到已有的 HTTP header 上
func MergeHeaderOverrides(header http.Header, overrides map[string]string) {
	for k, v := range overrides {
		header.Set(k, v)
	}
}

package billing

import (
	"net"
	"strings"
)

// Scope constants
const (
	ScopeFull           = "full"
	ScopeChatOnly       = "chat_only"
	ScopeEmbeddingsOnly = "embeddings_only"
	ScopeImagesOnly     = "images_only"
	ScopeAudioOnly      = "audio_only"
	ScopeReadOnly       = "read_only"
)

// CheckScope 检查 API Key scope 是否允许访问指定 relay mode
func CheckScope(scope string, relayMode string) bool {
	if scope == "" || scope == ScopeFull {
		return true
	}

	switch scope {
	case ScopeReadOnly:
		// read_only 仅允许 GET /models
		return relayMode == ""
	case ScopeChatOnly, "chat":
		return relayMode == "chat_completions" ||
			relayMode == "completions" ||
			relayMode == "responses" ||
			relayMode == "claude_messages" ||
			relayMode == "gemini_generate_content" ||
			relayMode == "realtime"
	case ScopeEmbeddingsOnly, "embedding":
		return relayMode == "embeddings"
	case ScopeImagesOnly, "image":
		return relayMode == "images_generations" || relayMode == "images_edits"
	case ScopeAudioOnly, "audio":
		return relayMode == "audio" ||
			relayMode == "tts" ||
			relayMode == "stt" ||
			relayMode == "audio_speech" ||
			relayMode == "audio_transcriptions" ||
			relayMode == "audio_translations"
	default:
		// custom scope: 以逗号分隔的模式列表
		allowed := strings.Split(scope, ",")
		for _, a := range allowed {
			if strings.TrimSpace(a) == relayMode {
				return true
			}
		}
		return false
	}
}

// IsReadOnlyScope 检查 scope 是否为只读
func IsReadOnlyScope(scope string) bool {
	return scope == ScopeReadOnly
}

// CheckIPWhitelist 检查 IP 白名单
// whitelist 为逗号分隔的 IP/CIDR 列表，空列表表示不限制
func CheckIPWhitelist(whitelist string, clientIP string) bool {
	if whitelist == "" {
		return true
	}

	// 提取 IP（去除端口）
	host, _, err := net.SplitHostPort(clientIP)
	if err != nil {
		// 没有端口，直接使用原始值
		host = clientIP
	}
	// 去除 IPv6 方括号
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")

	parsedIP := net.ParseIP(host)
	if parsedIP == nil {
		return false
	}

	allowed := strings.Split(whitelist, ",")
	for _, a := range allowed {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}

		// 精确 IP 匹配
		if a == host {
			return true
		}

		// CIDR 匹配
		if strings.Contains(a, "/") {
			_, cidr, err := net.ParseCIDR(a)
			if err != nil {
				continue
			}
			if cidr.Contains(parsedIP) {
				return true
			}
		}
	}

	return false
}

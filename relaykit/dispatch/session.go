package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
)

// SessionSource 会话键来源标记，用于观测各来源占比（dispatch_session_source_total）。
type SessionSource string

const (
	SourceHeader    SessionSource = "hdr"       // 显式 X-Session-Id 头
	SourceAnthropic SessionSource = "anthropic" // Anthropic metadata.user_id 中的 session 段
	SourceOpenAI    SessionSource = "openai"    // OpenAI Responses previous_response_id / conversation_id
	SourceIdentity  SessionSource = "ident"     // 身份四元组回退
)

// SessionSignals 从请求中提取的会话信号原始值（提取由 handler 完成，本包只做解析决策）。
type SessionSignals struct {
	HeaderSessionID    string // X-Session-Id 头
	AnthropicUserID    string // Anthropic 格式的 metadata.user_id
	PreviousResponseID string // OpenAI Responses previous_response_id
	ConversationID     string // OpenAI conversation_id / thread
}

// SessionKey 解析后的会话键。Key 为哈希后的稳定标识，可直接作为 Redis 绑定 key 的一部分。
type SessionKey struct {
	Source SessionSource
	Key    string // 形如 sk:<source>:<sha256 hex>
}

const (
	maxHeaderTokenLen   = 256 // 显式头最大长度，超长视为无效（gpt §7.1）
	maxProtocolTokenLen = 512 // 协议内信号最大长度
)

// anthropicSessionRe 提取 Claude Code metadata.user_id 中的 session UUID 段。
// 格式形如 user_<hash>_account_<uuid>_session_<uuid>，格式稳定性需抓包验证（基线方案 §19.1）。
var anthropicSessionRe = regexp.MustCompile(`session_([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})`)

// ResolveSessionKey 会话键解析链（基线方案 §3.1）：
// 显式头 → Anthropic metadata → OpenAI Responses 信号 → 身份四元组回退。
// 纯函数：相同输入永远得到相同输出。
func ResolveSessionKey(p RequestProfile, pol SessionPolicy) SessionKey {
	// 1. 显式头
	if validToken(p.Signals.HeaderSessionID, maxHeaderTokenLen) {
		return makeSessionKey(SourceHeader, p, p.Signals.HeaderSessionID)
	}

	// 2a. Anthropic metadata.user_id（可配置关闭）
	if pol.ParseAnthropicMetadata && validToken(p.Signals.AnthropicUserID, maxProtocolTokenLen) {
		raw := p.Signals.AnthropicUserID
		if m := anthropicSessionRe.FindStringSubmatch(raw); len(m) == 2 {
			raw = m[1] // 提取到 session UUID 段
		}
		// 提取失败则整个 user_id 作为会话键（仍优于身份级）
		return makeSessionKey(SourceAnthropic, p, raw)
	}

	// 2b. OpenAI Responses（可配置关闭）
	if pol.ParseOpenAIResponses {
		if validToken(p.Signals.PreviousResponseID, maxProtocolTokenLen) {
			return makeSessionKey(SourceOpenAI, p, p.Signals.PreviousResponseID)
		}
		if validToken(p.Signals.ConversationID, maxProtocolTokenLen) {
			return makeSessionKey(SourceOpenAI, p, p.Signals.ConversationID)
		}
	}

	// 3. 身份级回退：tenant:user:apiKey:model 四元组
	ident := fmt.Sprintf("%d:%d:%d:%s", p.TenantID, p.UserID, p.APIKeyID, p.Model)
	return makeSessionKey(SourceIdentity, p, ident)
}

// makeSessionKey 生成最终会话键。哈希材料带上租户与模型做命名空间隔离：
// 不同租户携带相同 X-Session-Id 不会互相碰撞；同一会话在不同模型上的绑定互相独立。
func makeSessionKey(src SessionSource, p RequestProfile, raw string) SessionKey {
	sum := sha256.Sum256(fmt.Appendf(nil, "%d|%s|%s|%s", p.TenantID, p.Model, src, raw))
	return SessionKey{
		Source: src,
		Key:    "sk:" + string(src) + ":" + hex.EncodeToString(sum[:]),
	}
}

// validToken 校验会话信号：非空、长度受限、不含控制字符。
// 非法值直接跳过（落入下一级信号），不记录原值。
func validToken(s string, maxLen int) bool {
	if s == "" || len(s) > maxLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}

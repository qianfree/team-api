package aiassistant

import (
	"fmt"

	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/internal/logic/aiassistant/skills"
)

// builtinPrompt 内置系统提示词（自定义提示词为空时使用）。
const builtinPrompt = "你是 team-api 多租户大模型网关平台管理后台的 AI 运营助手，服务对象是平台运营者（管理员）。" +
	"用中文回答，简洁专业。你可以协助运营问答、文案起草、配置排障思路，以及按需激活技能生成计费表达式与 JS 插件代码。" +
	"你不直接操作平台数据；涉及系统操作时，引导管理员到对应页面自行完成。"

// SanitizeAndBuild 清洗客户端消息并拼装最终请求消息：
//   - 丢弃客户端携带的 system 消息（系统提示词只能来自服务端配置，客户端注入面为零）
//   - 条数/单条长度上限校验（防 prompt 膨胀与滥用）
//   - 历史按配置截断（保留最近 HistoryLimit 条）
//   - 系统提示词 = 自定义（或内置）+ 技能注入段
//
// 校验失败返回中文错误（handler 层转为 400 响应）。
func SanitizeAndBuild(cfg *Config, client []ChatMessage, skillKeys []string) ([]ChatMessage, error) {
	if len(client) == 0 {
		return nil, gerror.New("对话消息不能为空")
	}
	if len(client) > MaxMessages {
		return nil, gerror.Newf("单次最多携带 %d 条消息", MaxMessages)
	}

	history := make([]ChatMessage, 0, len(client))
	for _, m := range client {
		if m.Role != "user" && m.Role != "assistant" {
			continue // system 等角色一律丢弃
		}
		if m.Content == "" {
			continue // 空消息（如被中断的生成）不参与
		}
		history = append(history, m)
	}
	if len(history) == 0 {
		return nil, gerror.New("没有可用的对话消息")
	}
	if history[len(history)-1].Role != "user" {
		return nil, gerror.New("最后一条消息必须是用户消息")
	}
	for _, m := range history {
		if len(m.Content) > MaxMessageBytes {
			return nil, gerror.Newf("单条消息过长（上限 %d KB）", MaxMessageBytes>>10)
		}
	}
	if len(history) > cfg.HistoryLimit {
		history = history[len(history)-cfg.HistoryLimit:]
	}

	system := cfg.SystemPrompt
	if system == "" {
		system = builtinPrompt
	}
	if injection := skills.PromptFor(skillKeys); injection != "" {
		system += "\n\n" + injection
	}

	msgs := make([]ChatMessage, 0, len(history)+1)
	msgs = append(msgs, ChatMessage{Role: "system", Content: system})
	msgs = append(msgs, history...)
	return msgs, nil
}

// SkillKeyForFence 按围栏语言标记把代码归属到激活技能：
// 精确匹配优先（js→js_plugin、formula→billing_formula），未知标记回落
// 字母序首个激活技能（多技能场景模型通常仍会带标记；不带也能兜底）。
func SkillKeyForFence(lang string, activeKeys []string) string {
	if len(activeKeys) == 0 {
		return ""
	}
	if lang != "" {
		for _, k := range activeKeys {
			if s, ok := skills.Get(k); ok && s.Fence() == lang {
				return k
			}
		}
		// 常见别名：javascript 归 js_plugin
		if lang == "javascript" {
			for _, k := range activeKeys {
				if s, ok := skills.Get(k); ok && s.Fence() == "js" {
					return k
				}
			}
		}
	}
	return activeKeys[0]
}

// FilterSkillKeys 过滤出已注册的技能 key（未知 key 静默忽略，字母序稳定）。
// 无有效技能时返回 nil。
func FilterSkillKeys(keys []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range keys {
		if _, ok := skills.Get(k); ok && !seen[k] {
			out = append(out, k)
			seen[k] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	// 字母序（与 skills.PromptFor 的拼接顺序一致）
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// RateLimitKey 每管理员限流键。
func RateLimitKey(adminID int64) string {
	return fmt.Sprintf("ai_assistant:rl:%d", adminID)
}

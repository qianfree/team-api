package v1

import "github.com/gogf/gf/v2/frame/g"

// PlaygroundMessage 对话消息（Playground 和 Sandbox 共用）
type PlaygroundMessage struct {
	Role    string `json:"role" v:"required|in:system,user,assistant#请指定角色|角色无效"`
	Content string `json:"content" v:"required#消息内容不能为空"`
}

// ============================================================
// Sandbox（模拟调用，不计费）
// ============================================================

// SandboxChatReq Sandbox 对话请求
type SandboxChatReq struct {
	g.Meta      `path:"/sandbox/chat" method:"post" mime:"json" tags:"租户控制台-Playground" summary:"Sandbox模拟对话"`
	Model       string              `json:"model" v:"required#请选择模型" dc:"模型名称"`
	Messages    []PlaygroundMessage `json:"messages" v:"required|length:1,50#请输入消息|消息数量超出限制" dc:"对话消息列表"`
	Temperature *float64            `json:"temperature" dc:"温度参数 (0-2)"`
	MaxTokens   *int                `json:"max_tokens" dc:"最大输出 Token 数"`
	Stream      bool                `json:"stream" d:"true" dc:"是否流式响应"`
}

type SandboxChatRes struct {
	Content        string `json:"content"`
	IsSandbox      bool   `json:"is_sandbox"`
	RemainingQuota int    `json:"remaining_quota" dc:"本月剩余沙箱额度"`
}

// SandboxQuotaReq 沙箱额度查询
type SandboxQuotaReq struct {
	g.Meta `path:"/sandbox/quota" method:"get" tags:"租户控制台-Playground" summary:"查询沙箱额度"`
}

type SandboxQuotaRes struct {
	TotalQuota     int `json:"total_quota"`
	RemainingQuota int `json:"remaining_quota"`
	UsedQuota      int `json:"used_quota"`
}

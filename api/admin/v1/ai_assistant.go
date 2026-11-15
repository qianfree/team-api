package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// AI 助手（管理后台全局运营助手）：右上角入口、独立配置的 OpenAI 兼容端点、
// 两个领域技能（计费表达式 / JS 插件代码生成）。
// 权限点：ai_assistant:use（对话，SSE 端点走 handler 注册）/ ai_assistant:config（配置）。

// ── 配置 ──

type AiAssistantConfigGetReq struct {
	g.Meta `path:"/ai-assistant/config" method:"get" mime:"json" tags:"管理后台-AI助手" summary:"读取助手配置（敏感字段掩码）"`
}

// AiAssistantConfigGetRes 与设置页读取同构（key/value/type/label/description），
// 敏感字段值为掩码（已配置显示 ******，未配置为空），掩码原样提交表示保持现值。
type AiAssistantConfigGetRes struct {
	List []AdminSettingItem `json:"list"`
}

type AiAssistantConfigUpdateReq struct {
	g.Meta   `path:"/ai-assistant/config" method:"put" mime:"json" tags:"管理后台-AI助手" summary:"更新助手配置"`
	Settings map[string]any `json:"settings" v:"required" dc:"配置键值对；敏感字段掩码/空值=保持现值"`
}

type AiAssistantConfigUpdateRes struct{}

// ── 连通性测试 ──

type AiAssistantTestReq struct {
	g.Meta `path:"/ai-assistant/test" method:"post" mime:"json" tags:"管理后台-AI助手" summary:"测试助手 AI 连通性（用已保存配置发起最小补全）"`
}

type AiAssistantTestRes struct {
	OK         bool   `json:"ok"`
	Model      string `json:"model"`
	LatencyMs  int64  `json:"latency_ms"`
	TokensUsed int    `json:"tokens_used"`
	Reply      string `json:"reply"`
	Error      string `json:"error,omitempty" dc:"失败原因（连通性测试的失败本身就是有效结果）"`
}

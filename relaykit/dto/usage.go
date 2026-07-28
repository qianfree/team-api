// Package dto holds the relay protocol data-transfer objects.
//
// 阶段 1 占位：本文件仅定义 convmeta.ClaudeConvertInfo 与 relayconvert 注册表
// 函数签名所需的 dto.Usage，使基础层与注册表框架可独立编译。
// 阶段 2（DTO 迁移）将扩充为 new-api relaykit 的完整 DTO 集
// （GeneralOpenAIRequest / ClaudeRequest / GeminiChatRequest / Message / 各类 Response 等），
// 届时 Usage 也会同步为 new-api 的完整定义（含 PromptTokensDetails / BillingUsage 等）。
package dto

// Usage 是 token 用量的最小表示。阶段 2 会扩充字段以对齐 new-api。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

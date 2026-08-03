// Package dto — Coze（字节扣子）v3 协议数据结构。
//
// 从 relay/channel/coze 提取的纯数据结构，供 relaykit 转换器与宿主桥接层使用。
// 不依赖任何宿主类型（RelayInfo / GoFrame），仅含协议字段。
package dto

// CozeCreateRequest Coze v3 创建对话请求。
// BotID 取自渠道模型映射后的上游模型名；Query 取最后一条 user 消息文本。
type CozeCreateRequest struct {
	ConversationID string `json:"conversation_id,omitempty"`
	BotID          string `json:"bot_id"`
	User           string `json:"user"`
	Query          string `json:"query"`
	Stream         bool   `json:"stream"`
}

// CozeMessage Coze 流式 SSE 事件中的消息载荷。
//
// Coze SSE 帧形如：
//
//	event: conversation.message.delta
//	data: {"role":"assistant","type":"answer","content":"Hello"}
//
// 仅 type=="answer" 的消息是模型正文输出，其余类型（follow_up/voice 等）在转换时忽略。
type CozeMessage struct {
	Role        string `json:"role"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

// Package relayconvert — 转换器哨兵错误。
//
// internal 转换器包宿主无法 import，哨兵错误必须定义在可导出的顶层包中，
// 宿主才能用 errors.Is 识别并映射为自身的错误语义。
package relayconvert

import "errors"

// ErrStatefulResponsesUnsupported responses 有状态请求（previous_response_id）落在
// 不支持 Responses 协议的 chat-only 渠道上：会话历史存储在上游 Responses 服务侧，
// 降级转换会静默丢失全部上下文，必须快速失败。
//
// 由 internal/oai_responses 的 ResponsesToOpenAIRequestConverter wrap 返回；
// 语义与宿主 relay/constant.ErrStatefulResponsesUnsupported 一致，宿主捕获转换错误后
// 需用 errors.Is 识别本哨兵并按渠道级致命上报调度 FSM 换渠道。
var ErrStatefulResponsesUnsupported = errors.New("previous_response_id requires a responses-native channel")

// ErrContentBlocked 上游以内容安全策略拒绝了本次请求（如 Gemini 的
// promptFeedback.blockReason=SAFETY）。这是**客户端内容**问题而非渠道故障：
// 宿主必须映射为请求类错误（4xx 语义），不得按 5xx 上游故障上报调度 FSM，
// 否则正常渠道会因用户发送违规提示词而被扣健康分乃至熔断。
//
// 由 internal/{claude_gemini,native_responses,oai_gemini} 的 Gemini 出站转换器
// wrap 返回（流式与非流式各 3 处）；宿主用 errors.Is 识别。
var ErrContentBlocked = errors.New("content blocked by upstream safety policy")

// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnDebugLogs is the golang structure of table chn_debug_logs for DAO operations like Where/Data.
type ChnDebugLogs struct {
	g.Meta               `orm:"table:chn_debug_logs, do:true"`
	Id                   any         // 主键ID
	ChannelId            any         // 渠道ID（逻辑关联 chn_channels.id，无外键）
	ChannelName          any         // 渠道名称（冗余存储，渠道删除后仍可辨识）
	ChannelType          any         // 渠道类型（ProviderType 枚举值）
	RequestId            any         // 请求唯一ID（同一请求多次重试共享）
	TenantId             any         // 租户ID
	UserId               any         // 用户ID
	ApiKeyId             any         // API Key ID
	ModelName            any         // 用户请求的模型名
	UpstreamModel        any         // 上游实际使用的模型名（模型映射后）
	RelayMode            any         // 转发模式（chat_completions/claude_messages/embeddings 等）
	InboundPath          any         // 入站端点路径（如 /v1/chat/completions）
	UpstreamUrl          any         // 上游请求 URL（query 中的凭证参数已脱敏）
	IsStream             any         // 是否流式请求
	RetryIndex           any         // 重试轮次（0=首次尝试）
	IsFinal              any         // 是否为产生客户端响应的最终尝试（成功/终止/流中断）
	UpstreamStatusCode   any         // 上游 HTTP 状态码（未发起请求或连接失败为 NULL）
	ClientStatusCode     any         // 返回客户端的状态码
	Error                any         // 本尝试的错误信息（成功为空）
	ClientReqHeaders     any         // 段1 客户端请求头（凭证类头脱敏：前6后4）JSON
	ClientReqBody        any         // 段1 客户端请求体（完整不截断；二进制为 base64，见 encoding 列）
	ClientReqEncoding    any         // 段1 编码：plain / base64
	UpstreamReqHeaders   any         // 段2 发往上游的最终请求头（协议转换+override 后，凭证类脱敏）JSON
	UpstreamReqBody      any         // 段2 发往上游的请求体（实际发送字节，含协议转换；二进制为 base64）
	UpstreamReqEncoding  any         // 段2 编码：plain / base64
	UpstreamRespHeaders  any         // 段3 上游响应头 JSON（Content-Encoding 可能已被 net/http 透明解压移除）
	UpstreamRespBody     any         // 段3 上游响应体（Go 透明解压后的字节；流式为 SSE 原文；二进制为 base64）
	UpstreamRespEncoding any         // 段3 编码：plain / base64
	ClientRespHeaders    any         // 段4 返回客户端的响应头 JSON
	ClientRespBody       any         // 段4 返回客户端的响应体（协议转换后，完整不截断；二进制为 base64）
	ClientRespEncoding   any         // 段4 编码：plain / base64
	UpstreamLatencyMs    any         // 上游往返耗时（RoundTrip 到响应头）毫秒
	TotalLatencyMs       any         // 请求总耗时毫秒
	FirstTokenMs         any         // 首字节延迟毫秒
	ClientReqBytes       any         // 段1 请求体原始字节数（base64 落库膨胀前的真实大小）
	UpstreamReqBytes     any         // 段2 请求体原始字节数
	UpstreamRespBytes    any         // 段3 响应体原始字节数
	ClientRespBytes      any         // 段4 响应体原始字节数
	CreatedAt            *gtime.Time // 创建时间
	Conversion           any         // 协议转换信息 JSON：client_format 客户端协议 / upstream_format 上游协议 / chain 请求侧转换链 / bridge 桥接方式（responses_api|responses_direct|pass_through，空=常规转换）
}

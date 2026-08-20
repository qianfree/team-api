package common

import "context"

// DebugLogRecord 渠道调试日志记录（四段报文 + 元数据），经 SubmitDebugLog 钩子异步落库。
// headers 均已脱敏；body 为原始字节（落库时经 EncodeBody 处理二进制）。
type DebugLogRecord struct {
	ChannelID     int64
	ChannelName   string
	ChannelType   int
	RequestID     string
	TenantID      int64
	UserID        int64
	ApiKeyID      int64
	ModelName     string
	UpstreamModel string
	RelayMode     string
	InboundPath   string
	UpstreamURL   string
	IsStream      bool
	RetryIndex    int
	IsFinal       bool

	UpstreamStatusCode int // 0 = 未发起请求或连接失败（落库为 NULL）
	ClientStatusCode   int
	Error              string

	// 四段报文：段1 客户端请求 / 段2 发往上游最终请求 / 段3 上游响应 / 段4 返回客户端响应
	ClientReqHeaders    map[string]string
	ClientReqBody       []byte
	UpstreamReqHeaders  map[string]string
	UpstreamReqBody     []byte
	UpstreamRespHeaders map[string]string
	UpstreamRespBody    []byte
	ClientRespHeaders   map[string]string
	ClientRespBody      []byte

	UpstreamLatencyMs int64
	TotalLatencyMs    int64
	FirstTokenMs      int64

	// Conversion 协议转换信息（客户端协议 → 上游协议及转换链），buildRelayInfo 后由 CaptureProtocol 快照
	Conversion *DebugConversion
}

// DebugConversion 协议转换信息：追踪客户端协议经哪些格式转换到上游协议。
// 段3/段4 的协议无需单独记录——段3 = upstream_format，段4 = client_format（推导关系）。
type DebugConversion struct {
	ClientFormat   string   `json:"client_format"`    // 客户端原始协议（openai/claude/gemini/responses）
	UpstreamFormat string   `json:"upstream_format"`  // 发往上游的协议
	Chain          []string `json:"chain,omitempty"`  // 请求侧转换链（relaykit 填充；为空且两端不同时兜底两端）
	Bridge         string   `json:"bridge,omitempty"` // 桥接方式：responses_api / responses_direct / pass_through，空=常规转换
}

// SubmitDebugLog 调试日志提交钩子。relay 层不直接依赖 internal/logic（避免反向导入），
// 由 internal/logic/relay 在进程启动时注入实现；nil（未初始化/单测）时调用方静默丢弃。
var SubmitDebugLog func(ctx context.Context, record *DebugLogRecord)

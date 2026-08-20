// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChnDebugLogsDao is the data access object for the table chn_debug_logs.
type ChnDebugLogsDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  ChnDebugLogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// ChnDebugLogsColumns defines and stores column names for the table chn_debug_logs.
type ChnDebugLogsColumns struct {
	Id                   string // 主键ID
	ChannelId            string // 渠道ID（逻辑关联 chn_channels.id，无外键）
	ChannelName          string // 渠道名称（冗余存储，渠道删除后仍可辨识）
	ChannelType          string // 渠道类型（ProviderType 枚举值）
	RequestId            string // 请求唯一ID（同一请求多次重试共享）
	TenantId             string // 租户ID
	UserId               string // 用户ID
	ApiKeyId             string // API Key ID
	ModelName            string // 用户请求的模型名
	UpstreamModel        string // 上游实际使用的模型名（模型映射后）
	RelayMode            string // 转发模式（chat_completions/claude_messages/embeddings 等）
	InboundPath          string // 入站端点路径（如 /v1/chat/completions）
	UpstreamUrl          string // 上游请求 URL（query 中的凭证参数已脱敏）
	IsStream             string // 是否流式请求
	RetryIndex           string // 重试轮次（0=首次尝试）
	IsFinal              string // 是否为产生客户端响应的最终尝试（成功/终止/流中断）
	UpstreamStatusCode   string // 上游 HTTP 状态码（未发起请求或连接失败为 NULL）
	ClientStatusCode     string // 返回客户端的状态码
	Error                string // 本尝试的错误信息（成功为空）
	ClientReqHeaders     string // 段1 客户端请求头（凭证类头脱敏：前6后4）JSON
	ClientReqBody        string // 段1 客户端请求体（完整不截断；二进制为 base64，见 encoding 列）
	ClientReqEncoding    string // 段1 编码：plain / base64
	UpstreamReqHeaders   string // 段2 发往上游的最终请求头（协议转换+override 后，凭证类脱敏）JSON
	UpstreamReqBody      string // 段2 发往上游的请求体（实际发送字节，含协议转换；二进制为 base64）
	UpstreamReqEncoding  string // 段2 编码：plain / base64
	UpstreamRespHeaders  string // 段3 上游响应头 JSON（Content-Encoding 可能已被 net/http 透明解压移除）
	UpstreamRespBody     string // 段3 上游响应体（Go 透明解压后的字节；流式为 SSE 原文；二进制为 base64）
	UpstreamRespEncoding string // 段3 编码：plain / base64
	ClientRespHeaders    string // 段4 返回客户端的响应头 JSON
	ClientRespBody       string // 段4 返回客户端的响应体（协议转换后，完整不截断；二进制为 base64）
	ClientRespEncoding   string // 段4 编码：plain / base64
	UpstreamLatencyMs    string // 上游往返耗时（RoundTrip 到响应头）毫秒
	TotalLatencyMs       string // 请求总耗时毫秒
	FirstTokenMs         string // 首字节延迟毫秒
	ClientReqBytes       string // 段1 请求体原始字节数（base64 落库膨胀前的真实大小）
	UpstreamReqBytes     string // 段2 请求体原始字节数
	UpstreamRespBytes    string // 段3 响应体原始字节数
	ClientRespBytes      string // 段4 响应体原始字节数
	CreatedAt            string // 创建时间
	Conversion           string // 协议转换信息 JSON：client_format 客户端协议 / upstream_format 上游协议 / chain 请求侧转换链 / bridge 桥接方式（responses_api|responses_direct|pass_through，空=常规转换）
}

// chnDebugLogsColumns holds the columns for the table chn_debug_logs.
var chnDebugLogsColumns = ChnDebugLogsColumns{
	Id:                   "id",
	ChannelId:            "channel_id",
	ChannelName:          "channel_name",
	ChannelType:          "channel_type",
	RequestId:            "request_id",
	TenantId:             "tenant_id",
	UserId:               "user_id",
	ApiKeyId:             "api_key_id",
	ModelName:            "model_name",
	UpstreamModel:        "upstream_model",
	RelayMode:            "relay_mode",
	InboundPath:          "inbound_path",
	UpstreamUrl:          "upstream_url",
	IsStream:             "is_stream",
	RetryIndex:           "retry_index",
	IsFinal:              "is_final",
	UpstreamStatusCode:   "upstream_status_code",
	ClientStatusCode:     "client_status_code",
	Error:                "error",
	ClientReqHeaders:     "client_req_headers",
	ClientReqBody:        "client_req_body",
	ClientReqEncoding:    "client_req_encoding",
	UpstreamReqHeaders:   "upstream_req_headers",
	UpstreamReqBody:      "upstream_req_body",
	UpstreamReqEncoding:  "upstream_req_encoding",
	UpstreamRespHeaders:  "upstream_resp_headers",
	UpstreamRespBody:     "upstream_resp_body",
	UpstreamRespEncoding: "upstream_resp_encoding",
	ClientRespHeaders:    "client_resp_headers",
	ClientRespBody:       "client_resp_body",
	ClientRespEncoding:   "client_resp_encoding",
	UpstreamLatencyMs:    "upstream_latency_ms",
	TotalLatencyMs:       "total_latency_ms",
	FirstTokenMs:         "first_token_ms",
	ClientReqBytes:       "client_req_bytes",
	UpstreamReqBytes:     "upstream_req_bytes",
	UpstreamRespBytes:    "upstream_resp_bytes",
	ClientRespBytes:      "client_resp_bytes",
	CreatedAt:            "created_at",
	Conversion:           "conversion",
}

// NewChnDebugLogsDao creates and returns a new DAO object for table data access.
func NewChnDebugLogsDao(handlers ...gdb.ModelHandler) *ChnDebugLogsDao {
	return &ChnDebugLogsDao{
		group:    "default",
		table:    "chn_debug_logs",
		columns:  chnDebugLogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChnDebugLogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChnDebugLogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChnDebugLogsDao) Columns() ChnDebugLogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChnDebugLogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChnDebugLogsDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *ChnDebugLogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnErrorEvents is the golang structure of table chn_error_events for DAO operations like Where/Data.
type ChnErrorEvents struct {
	g.Meta        `orm:"table:chn_error_events, do:true"`
	Id            any         // 主键ID
	ChannelId     any         // 发生错误的渠道ID
	ChannelName   any         // 渠道名称（冗余存储，避免查询时JOIN）
	ChannelType   any         // 渠道类型（ProviderType枚举值）
	Provider      any         // 供应商名称（如 OpenAI, Claude, Zhipu 等）
	ModelName     any         // 请求的模型名
	UpstreamModel any         // 上游实际模型名（模型映射后）
	RequestId     any         // 关联的请求唯一ID
	TenantId      any         // 租户ID
	ErrorCategory any         // 错误分类：rate_limit/auth_error/timeout/upstream_error/server_error/network_error/unknown
	StatusCode    any         // HTTP状态码（来自上游响应或RelayError.StatusCode）
	ErrorType     any         // RelayError.Type原始值（upstream_error/channel_error/auth_error等）
	ErrorMessage  any         // 错误详细信息
	IsRetryable   any         // 是否为可重试错误（429,500,502,503,504）
	Attempt       any         // 重试轮次编号（0=首次）
	IsFinal       any         // 是否为最终错误（非中间重试失败）
	LatencyMs     any         // 请求耗时（毫秒）
	CreatedAt     *gtime.Time // 错误发生时间
}

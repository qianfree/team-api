// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnErrorEvents is the golang structure for table chn_error_events.
type ChnErrorEvents struct {
	Id            int64       `json:"id"             orm:"id"             description:"主键ID"`                                                                                 // 主键ID
	ChannelId     int64       `json:"channel_id"     orm:"channel_id"     description:"发生错误的渠道ID"`                                                                            // 发生错误的渠道ID
	ChannelName   string      `json:"channel_name"   orm:"channel_name"   description:"渠道名称（冗余存储，避免查询时JOIN）"`                                                                 // 渠道名称（冗余存储，避免查询时JOIN）
	ChannelType   int         `json:"channel_type"   orm:"channel_type"   description:"渠道类型（ProviderType枚举值）"`                                                                // 渠道类型（ProviderType枚举值）
	Provider      string      `json:"provider"       orm:"provider"       description:"供应商名称（如 OpenAI, Claude, Zhipu 等）"`                                                     // 供应商名称（如 OpenAI, Claude, Zhipu 等）
	ModelName     string      `json:"model_name"     orm:"model_name"     description:"请求的模型名"`                                                                               // 请求的模型名
	UpstreamModel string      `json:"upstream_model" orm:"upstream_model" description:"上游实际模型名（模型映射后）"`                                                                       // 上游实际模型名（模型映射后）
	RequestId     string      `json:"request_id"     orm:"request_id"     description:"关联的请求唯一ID"`                                                                            // 关联的请求唯一ID
	TenantId      int64       `json:"tenant_id"      orm:"tenant_id"      description:"租户ID"`                                                                                 // 租户ID
	ErrorCategory string      `json:"error_category" orm:"error_category" description:"错误分类：rate_limit/auth_error/timeout/upstream_error/server_error/network_error/unknown"` // 错误分类：rate_limit/auth_error/timeout/upstream_error/server_error/network_error/unknown
	StatusCode    int         `json:"status_code"    orm:"status_code"    description:"HTTP状态码（来自上游响应或RelayError.StatusCode）"`                                                // HTTP状态码（来自上游响应或RelayError.StatusCode）
	ErrorType     string      `json:"error_type"     orm:"error_type"     description:"RelayError.Type原始值（upstream_error/channel_error/auth_error等）"`                         // RelayError.Type原始值（upstream_error/channel_error/auth_error等）
	ErrorMessage  string      `json:"error_message"  orm:"error_message"  description:"错误详细信息"`                                                                               // 错误详细信息
	IsRetryable   bool        `json:"is_retryable"   orm:"is_retryable"   description:"是否为可重试错误（429,500,502,503,504）"`                                                        // 是否为可重试错误（429,500,502,503,504）
	Attempt       int         `json:"attempt"        orm:"attempt"        description:"重试轮次编号（0=首次）"`                                                                         // 重试轮次编号（0=首次）
	IsFinal       bool        `json:"is_final"       orm:"is_final"       description:"是否为最终错误（非中间重试失败）"`                                                                     // 是否为最终错误（非中间重试失败）
	LatencyMs     float64     `json:"latency_ms"     orm:"latency_ms"     description:"请求耗时（毫秒）"`                                                                             // 请求耗时（毫秒）
	CreatedAt     *gtime.Time `json:"created_at"     orm:"created_at"     description:"错误发生时间"`                                                                               // 错误发生时间
}

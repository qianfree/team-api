// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AudContentFilterLogs is the golang structure for table aud_content_filter_logs.
type AudContentFilterLogs struct {
	Id              int64       `json:"id"               orm:"id"               description:"主键ID"`                       // 主键ID
	TenantId        int64       `json:"tenant_id"        orm:"tenant_id"        description:"租户ID"`                       // 租户ID
	UserId          int64       `json:"user_id"          orm:"user_id"          description:"用户ID"`                       // 用户ID
	ApiKeyId        int64       `json:"api_key_id"       orm:"api_key_id"       description:"API Key ID"`                 // API Key ID
	RequestId       string      `json:"request_id"       orm:"request_id"       description:"请求唯一ID"`                     // 请求唯一ID
	Method          string      `json:"method"           orm:"method"           description:"HTTP 方法"`                    // HTTP 方法
	Path            string      `json:"path"             orm:"path"             description:"请求路径"`                       // 请求路径
	ClientIp        string      `json:"client_ip"        orm:"client_ip"        description:"客户端 IP"`                     // 客户端 IP
	FilterMode      string      `json:"filter_mode"      orm:"filter_mode"      description:"过滤模式：log / replace / block"` // 过滤模式：log / replace / block
	MatchedWords    string      `json:"matched_words"    orm:"matched_words"    description:"命中的敏感词列表（JSONB 数组）"`         // 命中的敏感词列表（JSONB 数组）
	OriginalSnippet string      `json:"original_snippet" orm:"original_snippet" description:"原始请求体片段（截断存储，仅 replace 模式）"` // 原始请求体片段（截断存储，仅 replace 模式）
	Blocked         bool        `json:"blocked"          orm:"blocked"          description:"是否被拦截（mode=block 时为 true）"`  // 是否被拦截（mode=block 时为 true）
	CreatedAt       *gtime.Time `json:"created_at"       orm:"created_at"       description:"创建时间"`                       // 创建时间
	ProjectId       int64       `json:"project_id"       orm:"project_id"       description:"项目ID"`                       // 项目ID
}

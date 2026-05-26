// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// AudContentFilterLogs is the golang structure of table aud_content_filter_logs for DAO operations like Where/Data.
type AudContentFilterLogs struct {
	g.Meta          `orm:"table:aud_content_filter_logs, do:true"`
	Id              any         // 主键ID
	TenantId        any         // 租户ID
	UserId          any         // 用户ID
	ApiKeyId        any         // API Key ID
	ProjectId       any         // 项目ID
	RequestId       any         // 请求唯一ID
	Method          any         // HTTP 方法
	Path            any         // 请求路径
	ClientIp        any         // 客户端 IP
	FilterMode      any         // 过滤模式：log / replace / block
	MatchedWords    any         // 命中的敏感词列表（JSONB 数组）
	OriginalSnippet any         // 原始请求体片段（截断存储，仅 replace 模式）
	Blocked         any         // 是否被拦截（mode=block 时为 true）
	CreatedAt       *gtime.Time // 创建时间
}

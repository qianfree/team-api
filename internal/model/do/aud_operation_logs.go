// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// AudOperationLogs is the golang structure of table aud_operation_logs for DAO operations like Where/Data.
type AudOperationLogs struct {
	g.Meta       `orm:"table:aud_operation_logs, do:true"`
	Id           any         // 主键ID
	TenantId     any         // 租户ID（管理后台操作时为 NULL）
	UserId       any         // 操作者用户ID
	UserType     any         // 操作者类型：admin（管理后台）/ tenant（租户控制台）
	Action       any         // 操作动作（如 create_tenant、update_channel、delete_model）
	ResourceType any         // 操作资源类型：tenant / channel / model / user / api_key 等
	ResourceId   any         // 操作资源ID
	Detail       any         // 操作详情（JSONB：变更前后的字段差异等）
	IpAddress    any         // 操作者 IP 地址
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
	ChangesJson  any         // 变更前后数据对比（JSON diff）
}

// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlTenantGroups is the golang structure for table mdl_tenant_groups.
type MdlTenantGroups struct {
	Id        int64       `json:"id"         orm:"id"         description:"主键ID"`                         // 主键ID
	TenantId  int64       `json:"tenant_id"  orm:"tenant_id"  description:"租户ID（关联 tnt_tenants.id）"`      // 租户ID（关联 tnt_tenants.id）
	GroupId   int64       `json:"group_id"   orm:"group_id"   description:"分组ID（关联 mdl_model_groups.id）"` // 分组ID（关联 mdl_model_groups.id）
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:"创建时间"`                         // 创建时间
	UpdatedAt *gtime.Time `json:"updated_at" orm:"updated_at" description:"更新时间"`                         // 更新时间
}

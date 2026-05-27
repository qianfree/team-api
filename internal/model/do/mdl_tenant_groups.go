// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlTenantGroups is the golang structure of table mdl_tenant_groups for DAO operations like Where/Data.
type MdlTenantGroups struct {
	g.Meta    `orm:"table:mdl_tenant_groups, do:true"`
	Id        any         // 主键ID
	TenantId  any         // 租户ID（关联 tnt_tenants.id）
	GroupId   any         // 分组ID（关联 mdl_model_groups.id）
	CreatedAt *gtime.Time // 创建时间
	UpdatedAt *gtime.Time // 更新时间
}

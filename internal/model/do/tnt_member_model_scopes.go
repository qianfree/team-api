// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TntMemberModelScopes is the golang structure of table tnt_member_model_scopes for DAO operations like Where/Data.
type TntMemberModelScopes struct {
	g.Meta    `orm:"table:tnt_member_model_scopes, do:true"`
	Id        any         // 主键ID
	TenantId  any         // 所属租户ID
	UserId    any         // 成员用户ID
	ModelId   any         // 模型ID
	CreatedAt *gtime.Time // 创建时间
}

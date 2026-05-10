// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TntMemberModelScopes is the golang structure for table tnt_member_model_scopes.
type TntMemberModelScopes struct {
	Id        int64       `json:"id"         orm:"id"         description:"主键ID"`   // 主键ID
	TenantId  int64       `json:"tenant_id"  orm:"tenant_id"  description:"所属租户ID"` // 所属租户ID
	UserId    int64       `json:"user_id"    orm:"user_id"    description:"成员用户ID"` // 成员用户ID
	ModelId   int64       `json:"model_id"   orm:"model_id"   description:"模型ID"`   // 模型ID
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:"创建时间"`   // 创建时间
}

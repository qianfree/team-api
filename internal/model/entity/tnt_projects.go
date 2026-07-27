// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// TntProjects is the golang structure for table tnt_projects.
type TntProjects struct {
	Id          int64            `json:"id"          orm:"id"          description:"主键ID"`                                                // 主键ID
	TenantId    int64            `json:"tenant_id"   orm:"tenant_id"   description:"所属租户ID"`                                              // 所属租户ID
	Name        string           `json:"name"        orm:"name"        description:"项目名称"`                                                // 项目名称
	Description string           `json:"description" orm:"description" description:"项目描述"`                                                // 项目描述
	Status      string           `json:"status"      orm:"status"      description:"状态：active（活跃）/ archived（归档）/ budget_exhausted（预算耗尽）"` // 状态：active（活跃）/ archived（归档）/ budget_exhausted（预算耗尽）
	Budget      *decimal.Decimal `json:"budget"      orm:"budget"      description:"项目预算上限（NUMERIC(20,10) 金额，NULL 表示不限制）"`                // 项目预算上限（NUMERIC(20,10) 金额，NULL 表示不限制）
	CreatedBy   int64            `json:"created_by"  orm:"created_by"  description:"创建者用户ID"`                                             // 创建者用户ID
	CreatedAt   *gtime.Time      `json:"created_at"  orm:"created_at"  description:"创建时间"`                                                // 创建时间
	UpdatedAt   *gtime.Time      `json:"updated_at"  orm:"updated_at"  description:"更新时间"`                                                // 更新时间
}

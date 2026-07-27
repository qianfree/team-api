// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// TntProjects is the golang structure of table tnt_projects for DAO operations like Where/Data.
type TntProjects struct {
	g.Meta      `orm:"table:tnt_projects, do:true"`
	Id          any              // 主键ID
	TenantId    any              // 所属租户ID
	Name        any              // 项目名称
	Description any              // 项目描述
	Status      any              // 状态：active（活跃）/ archived（归档）/ budget_exhausted（预算耗尽）
	Budget      *decimal.Decimal // 项目预算上限（NUMERIC(20,10) 金额，NULL 表示不限制）
	CreatedBy   any              // 创建者用户ID
	CreatedAt   *gtime.Time      // 创建时间
	UpdatedAt   *gtime.Time      // 更新时间
}

// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TntMemberImports is the golang structure of table tnt_member_imports for DAO operations like Where/Data.
type TntMemberImports struct {
	g.Meta       `orm:"table:tnt_member_imports, do:true"`
	Id           any         // 主键ID
	TenantId     any         // 所属租户ID
	Filename     any         // 上传文件名
	TotalCount   any         // 总行数
	SuccessCount any         // 成功数
	FailCount    any         // 失败数
	SkipCount    any         // 跳过数（重复）
	Status       any         // 状态：pending/processing/completed/failed
	ErrorMessage any         // 整体错误信息
	ResultJson   any         // 逐行结果 [{row,username,status,error}]
	CreatedBy    any         // 创建者用户ID
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
}

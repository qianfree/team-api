// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TntMemberImports is the golang structure for table tnt_member_imports.
type TntMemberImports struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                                   // 主键ID
	TenantId     int64       `json:"tenant_id"     orm:"tenant_id"     description:"所属租户ID"`                                 // 所属租户ID
	Filename     string      `json:"filename"      orm:"filename"      description:"上传文件名"`                                  // 上传文件名
	TotalCount   int         `json:"total_count"   orm:"total_count"   description:"总行数"`                                    // 总行数
	SuccessCount int         `json:"success_count" orm:"success_count" description:"成功数"`                                    // 成功数
	FailCount    int         `json:"fail_count"    orm:"fail_count"    description:"失败数"`                                    // 失败数
	SkipCount    int         `json:"skip_count"    orm:"skip_count"    description:"跳过数（重复）"`                                // 跳过数（重复）
	Status       string      `json:"status"        orm:"status"        description:"状态：pending/processing/completed/failed"` // 状态：pending/processing/completed/failed
	ErrorMessage string      `json:"error_message" orm:"error_message" description:"整体错误信息"`                                 // 整体错误信息
	ResultJson   string      `json:"result_json"   orm:"result_json"   description:"逐行结果 [{row,username,status,error}]"`     // 逐行结果 [{row,username,status,error}]
	CreatedBy    int64       `json:"created_by"    orm:"created_by"    description:"创建者用户ID"`                                // 创建者用户ID
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                                   // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                                   // 更新时间
}

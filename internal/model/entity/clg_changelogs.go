// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ClgChangelogs is the golang structure for table clg_changelogs.
type ClgChangelogs struct {
	Id          int64       `json:"id"           orm:"id"           description:"主键ID"`                                      // 主键ID
	Version     string      `json:"version"      orm:"version"      description:"版本号"`                                       // 版本号
	Title       string      `json:"title"        orm:"title"        description:"标题"`                                        // 标题
	Content     string      `json:"content"      orm:"content"      description:"Markdown 内容"`                               // Markdown 内容
	Type        string      `json:"type"         orm:"type"         description:"类型：feature / fix / improvement / breaking"` // 类型：feature / fix / improvement / breaking
	Status      string      `json:"status"       orm:"status"       description:"状态：draft / published"`                      // 状态：draft / published
	PublishedAt *gtime.Time `json:"published_at" orm:"published_at" description:"发布时间"`                                      // 发布时间
	CreatedBy   int64       `json:"created_by"   orm:"created_by"   description:"创建的管理员 ID"`                                 // 创建的管理员 ID
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"   description:""`                                          //
	UpdatedAt   *gtime.Time `json:"updated_at"   orm:"updated_at"   description:""`                                          //
}

// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SptAttachments is the golang structure for table spt_attachments.
type SptAttachments struct {
	Id          int64       `json:"id"           orm:"id"           description:"主键ID"`                 // 主键ID
	TicketId    int64       `json:"ticket_id"    orm:"ticket_id"    description:"工单ID"`                 // 工单ID
	ReplyId     int64       `json:"reply_id"     orm:"reply_id"     description:"回复ID（NULL表示工单创建时的附件）"` // 回复ID（NULL表示工单创建时的附件）
	FileName    string      `json:"file_name"    orm:"file_name"    description:"文件名"`                  // 文件名
	FileUrl     string      `json:"file_url"     orm:"file_url"     description:"文件访问地址"`               // 文件访问地址
	FileSize    int         `json:"file_size"    orm:"file_size"    description:"文件大小（字节）"`             // 文件大小（字节）
	ContentType string      `json:"content_type" orm:"content_type" description:"文件MIME类型"`             // 文件MIME类型
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"   description:"上传时间"`                 // 上传时间
}

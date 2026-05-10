// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SptAttachments is the golang structure of table spt_attachments for DAO operations like Where/Data.
type SptAttachments struct {
	g.Meta      `orm:"table:spt_attachments, do:true"`
	Id          any         // 主键ID
	TicketId    any         // 工单ID
	ReplyId     any         // 回复ID（NULL表示工单创建时的附件）
	FileName    any         // 文件名
	FileUrl     any         // 文件访问地址
	FileSize    any         // 文件大小（字节）
	ContentType any         // 文件MIME类型
	CreatedAt   *gtime.Time // 上传时间
}

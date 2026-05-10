// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfReadStatus is the golang structure of table ntf_read_status for DAO operations like Where/Data.
type NtfReadStatus struct {
	g.Meta    `orm:"table:ntf_read_status, do:true"`
	Id        any         // 主键ID
	MessageId any         // 广播消息ID
	UserId    any         // 已读用户ID
	ReadAt    *gtime.Time // 已读时间
}

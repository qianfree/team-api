// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfReadStatus is the golang structure for table ntf_read_status.
type NtfReadStatus struct {
	Id        int64       `json:"id"         orm:"id"         description:"主键ID"`   // 主键ID
	MessageId int64       `json:"message_id" orm:"message_id" description:"广播消息ID"` // 广播消息ID
	UserId    int64       `json:"user_id"    orm:"user_id"    description:"已读用户ID"` // 已读用户ID
	ReadAt    *gtime.Time `json:"read_at"    orm:"read_at"    description:"已读时间"`   // 已读时间
}

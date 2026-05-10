// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// AudSensitiveAccessLogs is the golang structure of table aud_sensitive_access_logs for DAO operations like Where/Data.
type AudSensitiveAccessLogs struct {
	g.Meta       `orm:"table:aud_sensitive_access_logs, do:true"`
	Id           any         // 主键ID
	UserId       any         // 访问者用户ID
	UserType     any         // 访问者类型：admin（管理后台）/ tenant（租户控制台）
	ResourceType any         // 资源类型（如 api_key、channel、wallet 等）
	ResourceId   any         // 资源ID
	Action       any         // 访问动作（如 view、export、download）
	Reason       any         // 访问原因（查看敏感数据时需填写）
	IpAddress    any         // 访问者 IP 地址
	UserAgent    any         // 访问者 User-Agent
	CreatedAt    *gtime.Time // 创建时间
}

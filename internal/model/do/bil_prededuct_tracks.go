// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilPredeductTracks is the golang structure of table bil_prededuct_tracks for DAO operations like Where/Data.
type BilPredeductTracks struct {
	g.Meta    `orm:"table:bil_prededuct_tracks, do:true"`
	Id        any         //
	TenantId  any         // 租户 ID
	RequestId any         // 请求唯一 ID
	Amount    any         // 预扣金额（USD）
	ModelName any         // 模型名称
	Status    any         // frozen=冻结中, settled=已结算, expired=超时自动释放, released=手动释放
	CreatedAt *gtime.Time // 创建时间
	ExpiredAt *gtime.Time // 过期释放时间（仅 status=expired 时有值）
}

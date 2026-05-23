// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// BilPredeductTracks is the golang structure for table bil_prededuct_tracks.
type BilPredeductTracks struct {
	Id        int64       `json:"id"         orm:"id"         description:""`                                                       //
	TenantId  int64       `json:"tenant_id"  orm:"tenant_id"  description:"租户 ID"`                                                  // 租户 ID
	RequestId string      `json:"request_id" orm:"request_id" description:"请求唯一 ID"`                                                // 请求唯一 ID
	Amount    float64     `json:"amount"     orm:"amount"     description:"预扣金额（USD）"`                                              // 预扣金额（USD）
	ModelName string      `json:"model_name" orm:"model_name" description:"模型名称"`                                                   // 模型名称
	Status    string      `json:"status"     orm:"status"     description:"frozen=冻结中, settled=已结算, expired=超时自动释放, released=手动释放"` // frozen=冻结中, settled=已结算, expired=超时自动释放, released=手动释放
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:"创建时间"`                                                   // 创建时间
	ExpiredAt *gtime.Time `json:"expired_at" orm:"expired_at" description:"过期释放时间（仅 status=expired 时有值）"`                           // 过期释放时间（仅 status=expired 时有值）
}

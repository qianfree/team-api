// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdRedemptionUsages is the golang structure for table ord_redemption_usages.
type OrdRedemptionUsages struct {
	Id            int64       `json:"id"             orm:"id"             description:"主键ID"`                             // 主键ID
	RedemptionId  int64       `json:"redemption_id"  orm:"redemption_id"  description:"关联兑换码ID"`                          // 关联兑换码ID
	TenantId      int64       `json:"tenant_id"      orm:"tenant_id"      description:"使用兑换码的租户ID"`                       // 使用兑换码的租户ID
	UserId        int64       `json:"user_id"        orm:"user_id"        description:"执行兑换操作的用户ID"`                      // 执行兑换操作的用户ID
	Type          string      `json:"type"           orm:"type"           description:"兑换类型：quota / plan / duration"`     // 兑换类型：quota / plan / duration
	Value         float64     `json:"value"          orm:"value"          description:"兑换面值（quota类型为金额，plan/duration为0）"` // 兑换面值（quota类型为金额，plan/duration为0）
	TransactionId int64       `json:"transaction_id" orm:"transaction_id" description:"关联的交易流水ID（仅quota类型有值）"`            // 关联的交易流水ID（仅quota类型有值）
	CreatedAt     *gtime.Time `json:"created_at"     orm:"created_at"     description:"创建时间"`                             // 创建时间
	UpdatedAt     *gtime.Time `json:"updated_at"     orm:"updated_at"     description:"更新时间"`                             // 更新时间
}

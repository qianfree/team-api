// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// BilTransactions is the golang structure for table bil_transactions.
type BilTransactions struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                                                                                                                           // 主键ID
	TenantId     int64       `json:"tenant_id"     orm:"tenant_id"     description:"租户ID"`                                                                                                                           // 租户ID
	WalletId     int64       `json:"wallet_id"     orm:"wallet_id"     description:"关联钱包ID"`                                                                                                                         // 关联钱包ID
	Type         string      `json:"type"          orm:"type"          description:"类型：consume（消费）/ recharge（充值）/ adjust（调整）/ pre_deduct（预扣，已废弃）/ settle（结算，已废弃）/ refund（退款，已废弃）/ freeze（冻结，已废弃）/ unfreeze（解冻，已废弃）"` // 类型：consume（消费）/ recharge（充值）/ adjust（调整）/ pre_deduct（预扣，已废弃）/ settle（结算，已废弃）/ refund（退款，已废弃）/ freeze（冻结，已废弃）/ unfreeze（解冻，已废弃）
	Amount       float64     `json:"amount"        orm:"amount"        description:"变动金额（正数=收入，负数=支出）"`                                                                                                              // 变动金额（正数=收入，负数=支出）
	BalanceAfter float64     `json:"balance_after" orm:"balance_after" description:"变动后总余额"`                                                                                                                         // 变动后总余额
	FrozenAfter  float64     `json:"frozen_after"  orm:"frozen_after"  description:"变动后冻结余额"`                                                                                                                        // 变动后冻结余额
	RelatedId    int64       `json:"related_id"    orm:"related_id"    description:"关联业务ID（如计费记录ID、订单ID等）"`                                                                                                          // 关联业务ID（如计费记录ID、订单ID等）
	RelatedType  string      `json:"related_type"  orm:"related_type"  description:"关联业务类型：billing_record / order / refund / adjustment / redemption"`                                                               // 关联业务类型：billing_record / order / refund / adjustment / redemption
	Description  string      `json:"description"   orm:"description"   description:"交易描述"`                                                                                                                           // 交易描述
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                                                                                                                           // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                                                                                                                           // 更新时间
	UserId       int64       `json:"user_id"       orm:"user_id"       description:"关联用户ID（consume 类型为实际消费用户，recharge 类型为操作用户，adjust 类型为空）"`                                                                         // 关联用户ID（consume 类型为实际消费用户，recharge 类型为操作用户，adjust 类型为空）
	RequestId    string      `json:"request_id"    orm:"request_id"    description:"关联请求ID（consume 类型对应 API 调用的 request_id，其他类型为空）"`                                                                                 // 关联请求ID（consume 类型对应 API 调用的 request_id，其他类型为空）
	ModelName    string      `json:"model_name"    orm:"model_name"    description:"关联模型名（consume 类型为调用的模型名，其他类型为空）"`                                                                                                // 关联模型名（consume 类型为调用的模型名，其他类型为空）
	ProjectId    int64       `json:"project_id"    orm:"project_id"    description:"关联项目ID（consume 类型为 API Key 所属项目，个人密钥为空）"`                                                                                        // 关联项目ID（consume 类型为 API Key 所属项目，个人密钥为空）
	ApiKeyId     int64       `json:"api_key_id"    orm:"api_key_id"    description:"关联API密钥ID（consume 类型为发起请求的密钥）"`                                                                                                  // 关联API密钥ID（consume 类型为发起请求的密钥）
	TaskId       string      `json:"task_id"       orm:"task_id"       description:"关联异步任务公开ID（consume+relay_mode=task 时关联 tsk_model_tasks.public_task_id）"`                                                         // 关联异步任务公开ID（consume+relay_mode=task 时关联 tsk_model_tasks.public_task_id）
}

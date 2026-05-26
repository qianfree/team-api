// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TskModelTasks is the golang structure for table tsk_model_tasks.
type TskModelTasks struct {
	Id              int64       `json:"id"                orm:"id"                description:"主键ID"`                                                                   // 主键ID
	PublicTaskId    string      `json:"public_task_id"    orm:"public_task_id"    description:"对外公开的任务ID（task_xxxxx 格式），用于 API 响应和客户端查询"`                               // 对外公开的任务ID（task_xxxxx 格式），用于 API 响应和客户端查询
	Platform        string      `json:"platform"          orm:"platform"          description:"任务所属平台：sora、kling、suno、midjourney、volcengine"`                           // 任务所属平台：sora、kling、suno、midjourney、volcengine
	Action          string      `json:"action"            orm:"action"            description:"任务动作类型。通用：generate；Suno：music、lyrics；"`                                  // 任务动作类型。通用：generate；Suno：music、lyrics；
	Status          string      `json:"status"            orm:"status"            description:"任务状态：NOT_START-未开始、SUBMITTED-已提交、IN_PROGRESS-进行中、SUCCESS-成功、FAILURE-失败"` // 任务状态：NOT_START-未开始、SUBMITTED-已提交、IN_PROGRESS-进行中、SUCCESS-成功、FAILURE-失败
	Progress        string      `json:"progress"          orm:"progress"          description:"任务进度，字符串格式百分比（如 0%、50%、100%）"`                                           // 任务进度，字符串格式百分比（如 0%、50%、100%）
	FailReason      string      `json:"fail_reason"       orm:"fail_reason"       description:"任务失败原因文本"`                                                               // 任务失败原因文本
	TenantId        int64       `json:"tenant_id"         orm:"tenant_id"         description:"发起任务的租户ID"`                                                              // 发起任务的租户ID
	UserId          int64       `json:"user_id"           orm:"user_id"           description:"发起任务的租户用户ID"`                                                            // 发起任务的租户用户ID
	ApiKeyId        int64       `json:"api_key_id"        orm:"api_key_id"        description:"调用方使用的 API Key ID"`                                                      // 调用方使用的 API Key ID
	ChannelId       int64       `json:"channel_id"        orm:"channel_id"        description:"转发请求的渠道ID"`                                                              // 转发请求的渠道ID
	ModelName       string      `json:"model_name"        orm:"model_name"        description:"用户请求的模型名称（如 sora-1.0-turbo、midjourney 等）"`                               // 用户请求的模型名称（如 sora-1.0-turbo、midjourney 等）
	UpstreamModel   string      `json:"upstream_model"    orm:"upstream_model"    description:"上游供应商实际使用的模型名称，可能与请求模型不同"`                                               // 上游供应商实际使用的模型名称，可能与请求模型不同
	PreDeductAmount float64     `json:"pre_deduct_amount" orm:"pre_deduct_amount" description:"提交任务时的预扣金额（USD），任务完成后根据实际用量结算差额"`                                        // 提交任务时的预扣金额（USD），任务完成后根据实际用量结算差额
	ActualCost      float64     `json:"actual_cost"       orm:"actual_cost"       description:"任务实际消费金额（USD），成功时按此结算，失败时退还预扣金额"`                                        // 任务实际消费金额（USD），成功时按此结算，失败时退还预扣金额
	BillingSettled  bool        `json:"billing_settled"   orm:"billing_settled"   description:"计费是否已结算（true 表示已完成预扣与实际金额的多退少补）"`                                        // 计费是否已结算（true 表示已完成预扣与实际金额的多退少补）
	ResultUrl       string      `json:"result_url"        orm:"result_url"        description:"任务完成后的结果资源 URL（如生成的视频/图片/音频的下载地址）"`                                      // 任务完成后的结果资源 URL（如生成的视频/图片/音频的下载地址）
	Data            string      `json:"data"              orm:"data"              description:"上游供应商返回的完整响应数据（JSONB），可返回给用户查看"`                                         // 上游供应商返回的完整响应数据（JSONB），可返回给用户查看
	PrivateData     string      `json:"private_data"      orm:"private_data"      description:"内部私有数据（JSONB），含 upstream_task_id 等敏感字段，用于轮询上游状态，不返回给用户"`                 // 内部私有数据（JSONB），含 upstream_task_id 等敏感字段，用于轮询上游状态，不返回给用户
	SubmitTime      *gtime.Time `json:"submit_time"       orm:"submit_time"       description:"任务提交到上游供应商的时间"`                                                          // 任务提交到上游供应商的时间
	StartTime       *gtime.Time `json:"start_time"        orm:"start_time"        description:"上游供应商开始执行任务的时间"`                                                         // 上游供应商开始执行任务的时间
	FinishTime      *gtime.Time `json:"finish_time"       orm:"finish_time"       description:"任务完成（成功或失败）的时间"`                                                         // 任务完成（成功或失败）的时间
	CreatedAt       *gtime.Time `json:"created_at"        orm:"created_at"        description:"记录创建时间"`                                                                 // 记录创建时间
	UpdatedAt       *gtime.Time `json:"updated_at"        orm:"updated_at"        description:"记录更新时间"`                                                                 // 记录更新时间
	RequestId       string      `json:"request_id"        orm:"request_id"        description:"任务提交时的原始请求 ID（req_xxxxx），关联 aud_request_logs.request_id"`                // 任务提交时的原始请求 ID（req_xxxxx），关联 aud_request_logs.request_id
}

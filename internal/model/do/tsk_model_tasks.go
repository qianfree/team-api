// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TskModelTasks is the golang structure of table tsk_model_tasks for DAO operations like Where/Data.
type TskModelTasks struct {
	g.Meta          `orm:"table:tsk_model_tasks, do:true"`
	Id              any         // 主键ID
	PublicTaskId    any         // 对外公开的任务ID（task_xxxxx 格式），用于 API 响应和客户端查询
	Platform        any         // 任务所属平台：sora、kling、suno、midjourney、volcengine
	Action          any         // 任务动作类型。通用：generate；Suno：music、lyrics；
	Status          any         // 任务状态：NOT_START-未开始、SUBMITTED-已提交、IN_PROGRESS-进行中、SUCCESS-成功、FAILURE-失败
	Progress        any         // 任务进度，字符串格式百分比（如 0%、50%、100%）
	FailReason      any         // 任务失败原因文本
	TenantId        any         // 发起任务的租户ID
	UserId          any         // 发起任务的租户用户ID
	ApiKeyId        any         // 调用方使用的 API Key ID
	ChannelId       any         // 转发请求的渠道ID
	ModelName       any         // 用户请求的模型名称（如 sora-1.0-turbo、midjourney 等）
	UpstreamModel   any         // 上游供应商实际使用的模型名称，可能与请求模型不同
	PreDeductAmount any         // 提交任务时的预扣金额（USD），任务完成后根据实际用量结算差额
	ActualCost      any         // 任务实际消费金额（USD），成功时按此结算，失败时退还预扣金额
	BillingSettled  any         // 计费是否已结算（true 表示已完成预扣与实际金额的多退少补）
	ResultUrl       any         // 任务完成后的结果资源 URL（如生成的视频/图片/音频的下载地址）
	Data            any         // 上游供应商返回的完整响应数据（JSONB），可返回给用户查看
	PrivateData     any         // 内部私有数据（JSONB），含 upstream_task_id 等敏感字段，用于轮询上游状态，不返回给用户
	SubmitTime      *gtime.Time // 任务提交到上游供应商的时间
	StartTime       *gtime.Time // 上游供应商开始执行任务的时间
	FinishTime      *gtime.Time // 任务完成（成功或失败）的时间
	CreatedAt       *gtime.Time // 记录创建时间
	UpdatedAt       *gtime.Time // 记录更新时间
	RequestId       any         // 任务提交时的原始请求 ID（req_xxxxx），关联 aud_request_logs.request_id
}

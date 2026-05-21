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
	Id              any         //
	PublicTaskId    any         //
	Platform        any         //
	Action          any         //
	Status          any         //
	Progress        any         //
	FailReason      any         //
	TenantId        any         //
	UserId          any         //
	ApiKeyId        any         //
	ChannelId       any         //
	ModelName       any         //
	UpstreamModel   any         //
	PreDeductAmount any         //
	ActualCost      any         //
	BillingSettled  any         //
	ResultUrl       any         //
	Data            any         //
	PrivateData     any         //
	SubmitTime      *gtime.Time //
	StartTime       *gtime.Time //
	FinishTime      *gtime.Time //
	CreatedAt       *gtime.Time //
	UpdatedAt       *gtime.Time //
	RequestId       any         // 任务提交时的原始请求 ID（req_xxxxx），关联 aud_request_logs.request_id
}

// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TskModelTasks is the golang structure for table tsk_model_tasks.
type TskModelTasks struct {
	Id              int64       `json:"id"                orm:"id"                description:""`              //
	PublicTaskId    string      `json:"public_task_id"    orm:"public_task_id"    description:""`              //
	RequestId       string      `json:"request_id"        orm:"request_id"        description:"任务提交时的原始请求 ID"` //
	Platform        string      `json:"platform"          orm:"platform"          description:""`              //
	Action          string      `json:"action"            orm:"action"            description:""`              //
	Status          string      `json:"status"            orm:"status"            description:""`              //
	Progress        string      `json:"progress"          orm:"progress"          description:""`              //
	FailReason      string      `json:"fail_reason"       orm:"fail_reason"       description:""`              //
	TenantId        int64       `json:"tenant_id"         orm:"tenant_id"         description:""`              //
	UserId          int64       `json:"user_id"           orm:"user_id"           description:""`              //
	ApiKeyId        int64       `json:"api_key_id"        orm:"api_key_id"        description:""`              //
	ChannelId       int64       `json:"channel_id"        orm:"channel_id"        description:""`              //
	ModelName       string      `json:"model_name"        orm:"model_name"        description:""`              //
	UpstreamModel   string      `json:"upstream_model"    orm:"upstream_model"    description:""`              //
	PreDeductAmount float64     `json:"pre_deduct_amount" orm:"pre_deduct_amount" description:""`              //
	ActualCost      float64     `json:"actual_cost"       orm:"actual_cost"       description:""`              //
	BillingSettled  bool        `json:"billing_settled"   orm:"billing_settled"   description:""`              //
	ResultUrl       string      `json:"result_url"        orm:"result_url"        description:""`              //
	Data            string      `json:"data"              orm:"data"              description:""`              //
	PrivateData     string      `json:"private_data"      orm:"private_data"      description:""`              //
	SubmitTime      *gtime.Time `json:"submit_time"       orm:"submit_time"       description:""`              //
	StartTime       *gtime.Time `json:"start_time"        orm:"start_time"        description:""`              //
	FinishTime      *gtime.Time `json:"finish_time"       orm:"finish_time"       description:""`              //
	CreatedAt       *gtime.Time `json:"created_at"        orm:"created_at"        description:""`              //
	UpdatedAt       *gtime.Time `json:"updated_at"        orm:"updated_at"        description:""`              //
}

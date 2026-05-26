// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TskModelTasksDao is the data access object for the table tsk_model_tasks.
type TskModelTasksDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  TskModelTasksColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// TskModelTasksColumns defines and stores column names for the table tsk_model_tasks.
type TskModelTasksColumns struct {
	Id              string // 主键ID
	PublicTaskId    string // 对外公开的任务ID（task_xxxxx 格式），用于 API 响应和客户端查询
	Platform        string // 任务所属平台：sora、kling、suno、midjourney、volcengine
	Action          string // 任务动作类型。通用：generate；Suno：music、lyrics；
	Status          string // 任务状态：NOT_START-未开始、SUBMITTED-已提交、IN_PROGRESS-进行中、SUCCESS-成功、FAILURE-失败
	Progress        string // 任务进度，字符串格式百分比（如 0%、50%、100%）
	FailReason      string // 任务失败原因文本
	TenantId        string // 发起任务的租户ID
	UserId          string // 发起任务的租户用户ID
	ApiKeyId        string // 调用方使用的 API Key ID
	ChannelId       string // 转发请求的渠道ID
	ModelName       string // 用户请求的模型名称（如 sora-1.0-turbo、midjourney 等）
	UpstreamModel   string // 上游供应商实际使用的模型名称，可能与请求模型不同
	PreDeductAmount string // 提交任务时的预扣金额（USD），任务完成后根据实际用量结算差额
	ActualCost      string // 任务实际消费金额（USD），成功时按此结算，失败时退还预扣金额
	BillingSettled  string // 计费是否已结算（true 表示已完成预扣与实际金额的多退少补）
	ResultUrl       string // 任务完成后的结果资源 URL（如生成的视频/图片/音频的下载地址）
	Data            string // 上游供应商返回的完整响应数据（JSONB），可返回给用户查看
	PrivateData     string // 内部私有数据（JSONB），含 upstream_task_id 等敏感字段，用于轮询上游状态，不返回给用户
	SubmitTime      string // 任务提交到上游供应商的时间
	StartTime       string // 上游供应商开始执行任务的时间
	FinishTime      string // 任务完成（成功或失败）的时间
	CreatedAt       string // 记录创建时间
	UpdatedAt       string // 记录更新时间
	RequestId       string // 任务提交时的原始请求 ID（req_xxxxx），关联 aud_request_logs.request_id
}

// tskModelTasksColumns holds the columns for the table tsk_model_tasks.
var tskModelTasksColumns = TskModelTasksColumns{
	Id:              "id",
	PublicTaskId:    "public_task_id",
	Platform:        "platform",
	Action:          "action",
	Status:          "status",
	Progress:        "progress",
	FailReason:      "fail_reason",
	TenantId:        "tenant_id",
	UserId:          "user_id",
	ApiKeyId:        "api_key_id",
	ChannelId:       "channel_id",
	ModelName:       "model_name",
	UpstreamModel:   "upstream_model",
	PreDeductAmount: "pre_deduct_amount",
	ActualCost:      "actual_cost",
	BillingSettled:  "billing_settled",
	ResultUrl:       "result_url",
	Data:            "data",
	PrivateData:     "private_data",
	SubmitTime:      "submit_time",
	StartTime:       "start_time",
	FinishTime:      "finish_time",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
	RequestId:       "request_id",
}

// NewTskModelTasksDao creates and returns a new DAO object for table data access.
func NewTskModelTasksDao(handlers ...gdb.ModelHandler) *TskModelTasksDao {
	return &TskModelTasksDao{
		group:    "default",
		table:    "tsk_model_tasks",
		columns:  tskModelTasksColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TskModelTasksDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TskModelTasksDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TskModelTasksDao) Columns() TskModelTasksColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TskModelTasksDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TskModelTasksDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *TskModelTasksDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

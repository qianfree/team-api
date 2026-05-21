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
	Id              string //
	PublicTaskId    string //
	Platform        string //
	Action          string //
	Status          string //
	Progress        string //
	FailReason      string //
	TenantId        string //
	UserId          string //
	ApiKeyId        string //
	ChannelId       string //
	ModelName       string //
	UpstreamModel   string //
	PreDeductAmount string //
	ActualCost      string //
	BillingSettled  string //
	ResultUrl       string //
	Data            string //
	PrivateData     string //
	SubmitTime      string //
	StartTime       string //
	FinishTime      string //
	CreatedAt       string //
	UpdatedAt       string //
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

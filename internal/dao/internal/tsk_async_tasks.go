// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TskAsyncTasksDao is the data access object for the table tsk_async_tasks.
type TskAsyncTasksDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  TskAsyncTasksColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// TskAsyncTasksColumns defines and stores column names for the table tsk_async_tasks.
type TskAsyncTasksColumns struct {
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
}

// tskAsyncTasksColumns holds the columns for the table tsk_async_tasks.
var tskAsyncTasksColumns = TskAsyncTasksColumns{
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
}

// NewTskAsyncTasksDao creates and returns a new DAO object for table data access.
func NewTskAsyncTasksDao(handlers ...gdb.ModelHandler) *TskAsyncTasksDao {
	return &TskAsyncTasksDao{
		group:    "default",
		table:    "tsk_async_tasks",
		columns:  tskAsyncTasksColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TskAsyncTasksDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TskAsyncTasksDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TskAsyncTasksDao) Columns() TskAsyncTasksColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TskAsyncTasksDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TskAsyncTasksDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TskAsyncTasksDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AudOperationLogsDao is the data access object for the table aud_operation_logs.
type AudOperationLogsDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  AudOperationLogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// AudOperationLogsColumns defines and stores column names for the table aud_operation_logs.
type AudOperationLogsColumns struct {
	Id           string // 主键ID
	TenantId     string // 租户ID（管理后台操作时为 NULL）
	UserId       string // 操作者用户ID
	UserType     string // 操作者类型：admin（管理后台）/ tenant（租户控制台）
	Action       string // 操作动作（如 create_tenant、update_channel、delete_model）
	ResourceType string // 操作资源类型：tenant / channel / model / user / api_key 等
	ResourceId   string // 操作资源ID
	Detail       string // 操作详情（JSONB：变更前后的字段差异等）
	IpAddress    string // 操作者 IP 地址
	CreatedAt    string // 创建时间
	UpdatedAt    string // 更新时间
	ChangesJson  string // 变更前后数据对比（JSON diff）
}

// audOperationLogsColumns holds the columns for the table aud_operation_logs.
var audOperationLogsColumns = AudOperationLogsColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	UserId:       "user_id",
	UserType:     "user_type",
	Action:       "action",
	ResourceType: "resource_type",
	ResourceId:   "resource_id",
	Detail:       "detail",
	IpAddress:    "ip_address",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
	ChangesJson:  "changes_json",
}

// NewAudOperationLogsDao creates and returns a new DAO object for table data access.
func NewAudOperationLogsDao(handlers ...gdb.ModelHandler) *AudOperationLogsDao {
	return &AudOperationLogsDao{
		group:    "default",
		table:    "aud_operation_logs",
		columns:  audOperationLogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AudOperationLogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AudOperationLogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AudOperationLogsDao) Columns() AudOperationLogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AudOperationLogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AudOperationLogsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AudOperationLogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

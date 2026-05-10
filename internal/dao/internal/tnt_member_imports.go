// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TntMemberImportsDao is the data access object for the table tnt_member_imports.
type TntMemberImportsDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  TntMemberImportsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// TntMemberImportsColumns defines and stores column names for the table tnt_member_imports.
type TntMemberImportsColumns struct {
	Id           string // 主键ID
	TenantId     string // 所属租户ID
	Filename     string // 上传文件名
	TotalCount   string // 总行数
	SuccessCount string // 成功数
	FailCount    string // 失败数
	SkipCount    string // 跳过数（重复）
	Status       string // 状态：pending/processing/completed/failed
	ErrorMessage string // 整体错误信息
	ResultJson   string // 逐行结果 [{row,username,status,error}]
	CreatedBy    string // 创建者用户ID
	CreatedAt    string // 创建时间
	UpdatedAt    string // 更新时间
}

// tntMemberImportsColumns holds the columns for the table tnt_member_imports.
var tntMemberImportsColumns = TntMemberImportsColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	Filename:     "filename",
	TotalCount:   "total_count",
	SuccessCount: "success_count",
	FailCount:    "fail_count",
	SkipCount:    "skip_count",
	Status:       "status",
	ErrorMessage: "error_message",
	ResultJson:   "result_json",
	CreatedBy:    "created_by",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewTntMemberImportsDao creates and returns a new DAO object for table data access.
func NewTntMemberImportsDao(handlers ...gdb.ModelHandler) *TntMemberImportsDao {
	return &TntMemberImportsDao{
		group:    "default",
		table:    "tnt_member_imports",
		columns:  tntMemberImportsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TntMemberImportsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TntMemberImportsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TntMemberImportsDao) Columns() TntMemberImportsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TntMemberImportsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TntMemberImportsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TntMemberImportsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

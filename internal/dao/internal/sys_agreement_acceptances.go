// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysAgreementAcceptancesDao is the data access object for the table sys_agreement_acceptances.
type SysAgreementAcceptancesDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  SysAgreementAcceptancesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// SysAgreementAcceptancesColumns defines and stores column names for the table sys_agreement_acceptances.
type SysAgreementAcceptancesColumns struct {
	Id          string //
	AgreementId string // 关联协议版本ID
	UserType    string // 用户类型
	UserId      string // 用户ID
	IpAddress   string // IP地址
	UserAgent   string // User-Agent
	CreatedAt   string //
}

// sysAgreementAcceptancesColumns holds the columns for the table sys_agreement_acceptances.
var sysAgreementAcceptancesColumns = SysAgreementAcceptancesColumns{
	Id:          "id",
	AgreementId: "agreement_id",
	UserType:    "user_type",
	UserId:      "user_id",
	IpAddress:   "ip_address",
	UserAgent:   "user_agent",
	CreatedAt:   "created_at",
}

// NewSysAgreementAcceptancesDao creates and returns a new DAO object for table data access.
func NewSysAgreementAcceptancesDao(handlers ...gdb.ModelHandler) *SysAgreementAcceptancesDao {
	return &SysAgreementAcceptancesDao{
		group:    "default",
		table:    "sys_agreement_acceptances",
		columns:  sysAgreementAcceptancesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysAgreementAcceptancesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysAgreementAcceptancesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysAgreementAcceptancesDao) Columns() SysAgreementAcceptancesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysAgreementAcceptancesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO.
func (dao *SysAgreementAcceptancesDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
func (dao *SysAgreementAcceptancesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

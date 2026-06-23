// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysAgreementsDao is the data access object for the table sys_agreements.
type SysAgreementsDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  SysAgreementsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// SysAgreementsColumns defines and stores column names for the table sys_agreements.
type SysAgreementsColumns struct {
	Id          string // 主键ID
	Code        string // 协议标识码：terms(用户协议) / privacy(隐私政策)
	Version     string // 版本号，如 1.0、2.0
	Title       string // 协议标题
	Content     string // 协议正文（Markdown）
	Summary     string // 版本变更摘要
	Status      string // 状态：draft(草稿) / published(已发布) / archived(已归档)
	IsCurrent   string // 是否为该标识码的当前生效版本（每个code仅一条）
	ForceAccept string // 是否强制用户接受（true=登录后必须接受才能继续）
	PublishedAt string // 发布时间
	CreatedBy   string // 创建的管理员ID
	CreatedAt   string // 创建时间
	UpdatedAt   string // 更新时间
}

// sysAgreementsColumns holds the columns for the table sys_agreements.
var sysAgreementsColumns = SysAgreementsColumns{
	Id:          "id",
	Code:        "code",
	Version:     "version",
	Title:       "title",
	Content:     "content",
	Summary:     "summary",
	Status:      "status",
	IsCurrent:   "is_current",
	ForceAccept: "force_accept",
	PublishedAt: "published_at",
	CreatedBy:   "created_by",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewSysAgreementsDao creates and returns a new DAO object for table data access.
func NewSysAgreementsDao(handlers ...gdb.ModelHandler) *SysAgreementsDao {
	return &SysAgreementsDao{
		group:    "default",
		table:    "sys_agreements",
		columns:  sysAgreementsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysAgreementsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysAgreementsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysAgreementsDao) Columns() SysAgreementsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysAgreementsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysAgreementsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysAgreementsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SptTicketsDao is the data access object for the table spt_tickets.
type SptTicketsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  SptTicketsColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// SptTicketsColumns defines and stores column names for the table spt_tickets.
type SptTicketsColumns struct {
	Id              string // 主键ID
	TenantId        string // 所属租户ID
	UserId          string // 创建者用户ID
	Category        string // 分类：billing（计费）/ technical（技术）/ feature_request（功能建议）/ other（其他）
	Title           string // 工单标题
	Description     string // 工单描述
	Urgency         string // 紧急程度：low（低）/ normal（普通）/ high（高）/ urgent（紧急）
	Status          string // 状态：pending（待处理）/ processing（处理中）/ replied（已回复）/ closed（已关闭）/ reopened（已重开）
	AssignedAdminId string // 处理人管理员ID
	CreatedAt       string // 创建时间
	UpdatedAt       string // 更新时间
}

// sptTicketsColumns holds the columns for the table spt_tickets.
var sptTicketsColumns = SptTicketsColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	UserId:          "user_id",
	Category:        "category",
	Title:           "title",
	Description:     "description",
	Urgency:         "urgency",
	Status:          "status",
	AssignedAdminId: "assigned_admin_id",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewSptTicketsDao creates and returns a new DAO object for table data access.
func NewSptTicketsDao(handlers ...gdb.ModelHandler) *SptTicketsDao {
	return &SptTicketsDao{
		group:    "default",
		table:    "spt_tickets",
		columns:  sptTicketsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SptTicketsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SptTicketsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SptTicketsDao) Columns() SptTicketsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SptTicketsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SptTicketsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SptTicketsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

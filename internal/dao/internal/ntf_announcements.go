// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NtfAnnouncementsDao is the data access object for the table ntf_announcements.
type NtfAnnouncementsDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  NtfAnnouncementsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// NtfAnnouncementsColumns defines and stores column names for the table ntf_announcements.
type NtfAnnouncementsColumns struct {
	Id              string // 主键ID
	Title           string // 公告标题
	Type            string // 公告类型：info（通知）/ warning（警告）/ important（重要）
	Content         string // 公告内容
	Status          string // 状态：draft（草稿）/ published（已发布）/ archived（已归档）
	IsPinned        string // 是否置顶：0=否, 1=是
	DisplayPosition string // 展示位置：login（登录页）/ console（控制台）/ both（双位置）
	EffectiveAt     string // 生效时间（NULL=立即生效）
	ExpiresAt       string // 过期时间（NULL=永不过期）
	CreatedBy       string // 创建者（管理员ID）
	CreatedAt       string // 创建时间
	UpdatedAt       string // 更新时间
}

// ntfAnnouncementsColumns holds the columns for the table ntf_announcements.
var ntfAnnouncementsColumns = NtfAnnouncementsColumns{
	Id:              "id",
	Title:           "title",
	Type:            "type",
	Content:         "content",
	Status:          "status",
	IsPinned:        "is_pinned",
	DisplayPosition: "display_position",
	EffectiveAt:     "effective_at",
	ExpiresAt:       "expires_at",
	CreatedBy:       "created_by",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewNtfAnnouncementsDao creates and returns a new DAO object for table data access.
func NewNtfAnnouncementsDao(handlers ...gdb.ModelHandler) *NtfAnnouncementsDao {
	return &NtfAnnouncementsDao{
		group:    "default",
		table:    "ntf_announcements",
		columns:  ntfAnnouncementsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NtfAnnouncementsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NtfAnnouncementsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NtfAnnouncementsDao) Columns() NtfAnnouncementsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NtfAnnouncementsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NtfAnnouncementsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NtfAnnouncementsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

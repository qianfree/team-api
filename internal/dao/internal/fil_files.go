// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// FilFilesDao is the data access object for the table fil_files.
type FilFilesDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  FilFilesColumns    // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// FilFilesColumns defines and stores column names for the table fil_files.
type FilFilesColumns struct {
	Id              string // 主键ID
	TenantId        string // 所属租户ID（系统文件为 NULL）
	UserId          string // 上传者用户ID
	Filename        string // 存储文件名（UUID 或哈希值命名）
	OriginalName    string // 用户上传的原始文件名
	MimeType        string // MIME 类型（如 image/png、application/pdf）
	Size            string // 文件大小（字节）
	StorageProvider string // 存储供应商：s3 / minio / oss / cos
	StoragePath     string // 存储桶中的完整路径
	VirusScanStatus string // 病毒扫描状态：pending（待扫描）/ scanning（扫描中）/ clean（安全）/ infected（感染）
	Checksum        string // 文件 SHA-256 校验和
	CreatedAt       string // 创建时间
	UpdatedAt       string // 更新时间
}

// filFilesColumns holds the columns for the table fil_files.
var filFilesColumns = FilFilesColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	UserId:          "user_id",
	Filename:        "filename",
	OriginalName:    "original_name",
	MimeType:        "mime_type",
	Size:            "size",
	StorageProvider: "storage_provider",
	StoragePath:     "storage_path",
	VirusScanStatus: "virus_scan_status",
	Checksum:        "checksum",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewFilFilesDao creates and returns a new DAO object for table data access.
func NewFilFilesDao(handlers ...gdb.ModelHandler) *FilFilesDao {
	return &FilFilesDao{
		group:    "default",
		table:    "fil_files",
		columns:  filFilesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *FilFilesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *FilFilesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *FilFilesDao) Columns() FilFilesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *FilFilesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *FilFilesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *FilFilesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

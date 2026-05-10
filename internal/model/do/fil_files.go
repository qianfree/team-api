// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// FilFiles is the golang structure of table fil_files for DAO operations like Where/Data.
type FilFiles struct {
	g.Meta          `orm:"table:fil_files, do:true"`
	Id              any         // 主键ID
	TenantId        any         // 所属租户ID（系统文件为 NULL）
	UserId          any         // 上传者用户ID
	Filename        any         // 存储文件名（UUID 或哈希值命名）
	OriginalName    any         // 用户上传的原始文件名
	MimeType        any         // MIME 类型（如 image/png、application/pdf）
	Size            any         // 文件大小（字节）
	StorageProvider any         // 存储供应商：s3 / minio / oss / cos
	StoragePath     any         // 存储桶中的完整路径
	VirusScanStatus any         // 病毒扫描状态：pending（待扫描）/ scanning（扫描中）/ clean（安全）/ infected（感染）
	Checksum        any         // 文件 SHA-256 校验和
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
}

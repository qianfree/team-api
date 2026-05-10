// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// FilFiles is the golang structure for table fil_files.
type FilFiles struct {
	Id              int64       `json:"id"                orm:"id"                description:"主键ID"`                                                        // 主键ID
	TenantId        int64       `json:"tenant_id"         orm:"tenant_id"         description:"所属租户ID（系统文件为 NULL）"`                                          // 所属租户ID（系统文件为 NULL）
	UserId          int64       `json:"user_id"           orm:"user_id"           description:"上传者用户ID"`                                                     // 上传者用户ID
	Filename        string      `json:"filename"          orm:"filename"          description:"存储文件名（UUID 或哈希值命名）"`                                          // 存储文件名（UUID 或哈希值命名）
	OriginalName    string      `json:"original_name"     orm:"original_name"     description:"用户上传的原始文件名"`                                                  // 用户上传的原始文件名
	MimeType        string      `json:"mime_type"         orm:"mime_type"         description:"MIME 类型（如 image/png、application/pdf）"`                        // MIME 类型（如 image/png、application/pdf）
	Size            int64       `json:"size"              orm:"size"              description:"文件大小（字节）"`                                                    // 文件大小（字节）
	StorageProvider string      `json:"storage_provider"  orm:"storage_provider"  description:"存储供应商：s3 / minio / oss / cos"`                                // 存储供应商：s3 / minio / oss / cos
	StoragePath     string      `json:"storage_path"      orm:"storage_path"      description:"存储桶中的完整路径"`                                                   // 存储桶中的完整路径
	VirusScanStatus string      `json:"virus_scan_status" orm:"virus_scan_status" description:"病毒扫描状态：pending（待扫描）/ scanning（扫描中）/ clean（安全）/ infected（感染）"` // 病毒扫描状态：pending（待扫描）/ scanning（扫描中）/ clean（安全）/ infected（感染）
	Checksum        string      `json:"checksum"          orm:"checksum"          description:"文件 SHA-256 校验和"`                                              // 文件 SHA-256 校验和
	CreatedAt       *gtime.Time `json:"created_at"        orm:"created_at"        description:"创建时间"`                                                        // 创建时间
	UpdatedAt       *gtime.Time `json:"updated_at"        orm:"updated_at"        description:"更新时间"`                                                        // 更新时间
}

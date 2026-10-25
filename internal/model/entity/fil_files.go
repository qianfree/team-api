// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// FilFiles is the golang structure for table fil_files.
type FilFiles struct {
	Id              int64       `json:"id"`                // 主键ID
	TenantId        int64       `json:"tenant_id"`         // 所属租户ID（系统文件为 NULL）
	UserId          int64       `json:"user_id"`           // 上传者用户ID
	Filename        string      `json:"filename"`          // 存储文件名（UUID 或哈希值命名）
	OriginalName    string      `json:"original_name"`     // 用户上传的原始文件名
	MimeType        string      `json:"mime_type"`         // MIME 类型（如 image/png、application/pdf）
	Size            int64       `json:"size"`              // 文件大小（字节）
	StorageProvider string      `json:"storage_provider"`  // 存储供应商：s3 / minio / oss / cos
	StoragePath     string      `json:"storage_path"`      // 存储桶中的完整路径
	VirusScanStatus string      `json:"virus_scan_status"` // 病毒扫描状态：pending（待扫描）/ scanning（扫描中）/ clean（安全）/ infected（感染）
	Checksum        string      `json:"checksum"`          // 文件 SHA-256 校验和
	ExtId           string      `json:"ext_id"`            // 对外文件 ID（file-xxx，OpenAI 兼容 /v1/files 专用，NULL=非用户上传素材）
	Purpose         string      `json:"purpose"`           // OpenAI 上传用途声明（vision/user_data 等），NULL=非 /v1/files 上传
	CreatedAt       *gtime.Time `json:"created_at"`        // 创建时间
	UpdatedAt       *gtime.Time `json:"updated_at"`        // 更新时间
}

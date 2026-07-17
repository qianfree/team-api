package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 文件存储管理（管理后台） ===

// FileListReq 文件列表（分页 + 筛选）。
type FileListReq struct {
	g.Meta    `path:"/files" method:"get" mime:"json" tags:"管理后台-文件管理" summary:"文件列表"`
	Page      int    `json:"page" in:"query" d:"1"`
	PageSize  int    `json:"page_size" in:"query" d:"20"`
	TenantId  int64  `json:"tenant_id" in:"query" dc:"按租户筛选"`
	UserId    int64  `json:"user_id" in:"query" dc:"按上传者筛选"`
	Provider  string `json:"provider" in:"query" dc:"存储供应商 s3/minio/r2/oss/cos"`
	Category  string `json:"category" in:"query" v:"in:|image|export|other" dc:"分类：image/export/other"`
	Keyword   string `json:"keyword" in:"query" dc:"按原始文件名 / 存储路径模糊匹配"`
	StartDate string `json:"start_date" in:"query" dc:"创建时间起 YYYY-MM-DD"`
	EndDate   string `json:"end_date" in:"query" dc:"创建时间止 YYYY-MM-DD"`
}

type FileItem struct {
	Id              int64       `json:"id"`
	TenantId        int64       `json:"tenant_id"`
	UserId          int64       `json:"user_id"`
	OriginalName    string      `json:"original_name"`
	MimeType        string      `json:"mime_type"`
	Size            int64       `json:"size"`
	StorageProvider string      `json:"storage_provider"`
	StoragePath     string      `json:"storage_path"`
	Category        string      `json:"category"` // image | export | other
	VirusScanStatus string      `json:"virus_scan_status"`
	CreatedAt       *gtime.Time `json:"created_at"`
}

type FileListRes struct {
	List     []*FileItem `json:"list"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// FileStatsReq 存储占用统计（用于 KPI 卡片）。
type FileStatsReq struct {
	g.Meta `path:"/files/stats" method:"get" mime:"json" tags:"管理后台-文件管理" summary:"文件存储统计"`
	TopN   int `json:"top_n" in:"query" d:"10" v:"min:1|max:50" dc:"Top 租户数量"`
}

type FileProviderStat struct {
	Provider string `json:"provider"`
	Count    int64  `json:"count"`
	Bytes    int64  `json:"bytes"`
}

type FileCategoryStat struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
	Bytes    int64  `json:"bytes"`
}

type FileTenantStat struct {
	TenantId int64 `json:"tenant_id"`
	Count    int64 `json:"count"`
	Bytes    int64 `json:"bytes"`
}

type FileStatsRes struct {
	TotalCount int64              `json:"total_count"`
	TotalBytes int64              `json:"total_bytes"`
	ByProvider []FileProviderStat `json:"by_provider"`
	ByCategory []FileCategoryStat `json:"by_category"`
	TopTenants []FileTenantStat   `json:"top_tenants"`
}

// FileDownloadReq 生成临时预览/下载链接。variant=thumb 返回缩略图（仅图片，OSS/COS 服务端裁剪，其余供应商回退原图），空/original 返回原图。
type FileDownloadReq struct {
	g.Meta  `path:"/files/{id}/download" method:"get" mime:"json" tags:"管理后台-文件管理" summary:"生成文件预览/下载链接"`
	Id      int64  `json:"id" in:"path" v:"required|min:1"`
	Variant string `json:"variant" in:"query" v:"in:thumb,original" dc:"thumb=缩略图预览，空/original=原图"`
	Width   int    `json:"width" in:"query" d:"400" v:"min:0|max:2048" dc:"缩略图宽度像素，仅 variant=thumb 生效"`
}

type FileDownloadRes struct {
	Url string `json:"url"`
}

// FileDeleteReq 删除单个文件（对象 + 记录）。
type FileDeleteReq struct {
	g.Meta `path:"/files/{id}" method:"delete" mime:"json" tags:"管理后台-文件管理" summary:"删除文件"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type FileDeleteRes struct{}

// FileCleanupReq 手动触发一次保留期清理（导出 + 图片）。
type FileCleanupReq struct {
	g.Meta `path:"/files/cleanup" method:"post" mime:"json" tags:"管理后台-文件管理" summary:"手动清理过期文件"`
}

type FileCleanupRes struct {
	ExportsDeleted int `json:"exports_deleted"`
	ImagesDeleted  int `json:"images_deleted"`
}

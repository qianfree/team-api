package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 系统在线升级（管理后台） ===

// UpdateCheck 检查系统更新
type UpdateCheckReq struct {
	g.Meta `path:"/update/check" method:"get" mime:"json" tags:"管理后台-系统更新" summary:"检查系统更新"`
	Force  bool `json:"force" in:"query" d:"false" dc:"是否强制跳过缓存检查"`
}

type UpdateCheckRes struct {
	CurrentVersion string      `json:"current_version"`
	LatestVersion  string      `json:"latest_version"`
	HasUpdate      bool        `json:"has_update"`
	ReleaseNotes   string      `json:"release_notes"`
	ReleaseURL     string      `json:"release_url"`
	PublishedAt    *gtime.Time `json:"published_at,omitempty"`
	CheckedAt      *gtime.Time `json:"checked_at"`
	DeploymentMode string      `json:"deployment_mode"` // binary | docker
}

// UpdateStatus 获取更新状态（升级进行中时前端轮询）
type UpdateStatusReq struct {
	g.Meta `path:"/update/status" method:"get" mime:"json" tags:"管理后台-系统更新" summary:"获取更新状态"`
}

type UpdateStatusRes struct {
	CurrentVersion    string          `json:"current_version"`
	DeploymentMode    string          `json:"deployment_mode"` // binary | docker
	Updating          bool            `json:"updating"`
	LastCheck         *UpdateCheckRes `json:"last_check,omitempty"`
	UpdateProgress    *UpdateProgress `json:"update_progress,omitempty"`
	RollbackAvailable bool            `json:"rollback_available"`
	BackupVersion     string          `json:"backup_version,omitempty"`
}

type UpdateProgress struct {
	Phase      string `json:"phase"`           // downloading | verifying | backing_up | replacing | restarting | complete | failed
	Message    string `json:"message"`         // 当前操作描述
	Percentage int    `json:"percentage"`      // 0-100
	Error      string `json:"error,omitempty"` // 失败时的错误信息
}

// UpdateExecute 触发系统升级
type UpdateExecuteReq struct {
	g.Meta  `path:"/update/execute" method:"post" mime:"json" tags:"管理后台-系统更新" summary:"触发系统升级"`
	Version string `json:"version" v:"required" dc:"目标版本号"`
}

type UpdateExecuteRes struct {
	Message string `json:"message"`
}

// UpdateRollback 回滚到上一版本（仅二进制模式）
type UpdateRollbackReq struct {
	g.Meta `path:"/update/rollback" method:"post" mime:"json" tags:"管理后台-系统更新" summary:"回滚到上一版本"`
}

type UpdateRollbackRes struct {
	Message string `json:"message"`
}

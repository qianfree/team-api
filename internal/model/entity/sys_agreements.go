// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAgreements is the golang structure for table sys_agreements.
type SysAgreements struct {
	Id          int64       `json:"id"           orm:"id"           description:"主键ID"`                                          // 主键ID
	Code        string      `json:"code"         orm:"code"         description:"协议标识码：terms(用户协议) / privacy(隐私政策)"`             // 协议标识码：terms(用户协议) / privacy(隐私政策)
	Version     string      `json:"version"      orm:"version"      description:"版本号，如 1.0、2.0"`                                 // 版本号，如 1.0、2.0
	Title       string      `json:"title"        orm:"title"        description:"协议标题"`                                          // 协议标题
	Content     string      `json:"content"      orm:"content"      description:"协议正文（Markdown）"`                                // 协议正文（Markdown）
	Summary     string      `json:"summary"      orm:"summary"      description:"版本变更摘要"`                                        // 版本变更摘要
	Status      string      `json:"status"       orm:"status"       description:"状态：draft(草稿) / published(已发布) / archived(已归档)"` // 状态：draft(草稿) / published(已发布) / archived(已归档)
	IsCurrent   bool        `json:"is_current"   orm:"is_current"   description:"是否为该标识码的当前生效版本（每个code仅一条）"`                     // 是否为该标识码的当前生效版本（每个code仅一条）
	ForceAccept bool        `json:"force_accept" orm:"force_accept" description:"是否强制用户接受（true=登录后必须接受才能继续）"`                    // 是否强制用户接受（true=登录后必须接受才能继续）
	PublishedAt *gtime.Time `json:"published_at" orm:"published_at" description:"发布时间"`                                          // 发布时间
	CreatedBy   int64       `json:"created_by"   orm:"created_by"   description:"创建的管理员ID"`                                      // 创建的管理员ID
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"   description:"创建时间"`                                          // 创建时间
	UpdatedAt   *gtime.Time `json:"updated_at"   orm:"updated_at"   description:"更新时间"`                                          // 更新时间
}

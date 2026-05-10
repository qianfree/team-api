package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 更新日志（管理后台） ===

type ChangelogCreateReq struct {
	g.Meta  `path:"/changelogs" method:"post" mime:"json" tags:"管理后台-更新日志" summary:"创建更新日志"`
	Version string `json:"version" v:"required|length:1,50" dc:"版本号"`
	Title   string `json:"title" v:"required|length:1,200" dc:"标题"`
	Content string `json:"content" v:"required" dc:"Markdown 内容"`
	Type    string `json:"type" d:"feature" v:"in:feature,fix,improvement,breaking" dc:"类型"`
}

type ChangelogCreateRes struct {
	Id int64 `json:"id"`
}

type ChangelogListReq struct {
	g.Meta   `path:"/changelogs" method:"get" mime:"json" tags:"管理后台-更新日志" summary:"更新日志列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
	Type     string `json:"type" in:"query"`
}

type ChangelogItem struct {
	Id          int64       `json:"id"`
	Version     string      `json:"version"`
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	Type        string      `json:"type"`
	Status      string      `json:"status"`
	PublishedAt *gtime.Time `json:"published_at,omitempty"`
	CreatedAt   *gtime.Time `json:"created_at"`
	UpdatedAt   *gtime.Time `json:"updated_at"`
}

type ChangelogListRes struct {
	List     []*ChangelogItem `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type ChangelogUpdateReq struct {
	g.Meta  `path:"/changelogs/{id}" method:"put" mime:"json" tags:"管理后台-更新日志" summary:"更新更新日志"`
	Id      int64  `json:"id" in:"path" v:"required|min:1"`
	Version string `json:"version" v:"required|length:1,50" dc:"版本号"`
	Title   string `json:"title" v:"required|length:1,200" dc:"标题"`
	Content string `json:"content" v:"required" dc:"Markdown 内容"`
	Type    string `json:"type" v:"in:feature,fix,improvement,breaking" dc:"类型"`
	Status  string `json:"status" v:"in:draft,published" dc:"状态"`
}

type ChangelogUpdateRes struct{}

type ChangelogDeleteReq struct {
	g.Meta `path:"/changelogs/{id}" method:"delete" mime:"json" tags:"管理后台-更新日志" summary:"删除更新日志"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type ChangelogDeleteRes struct{}

type ChangelogPublishReq struct {
	g.Meta `path:"/changelogs/{id}/publish" method:"post" mime:"json" tags:"管理后台-更新日志" summary:"发布更新日志"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type ChangelogPublishRes struct{}

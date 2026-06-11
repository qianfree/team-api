package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 用户协议管理（管理后台 CRUD） ===

type AgreementCreateReq struct {
	g.Meta      `path:"/agreements" method:"post" mime:"json" tags:"管理后台-用户协议" summary:"创建协议版本"`
	Code        string `json:"code" v:"required|length:1,50" dc:"协议标识码"`
	Version     string `json:"version" v:"required|length:1,50" dc:"版本号"`
	Title       string `json:"title" v:"required|length:1,200" dc:"协议标题"`
	Content     string `json:"content" v:"required" dc:"协议正文（Markdown）"`
	Summary     string `json:"summary" v:"max-length:500" dc:"版本变更摘要"`
	ForceAccept *bool  `json:"force_accept" d:"true" dc:"是否强制用户接受"`
}

type AgreementCreateRes struct {
	Id int64 `json:"id"`
}

type AgreementListReq struct {
	g.Meta   `path:"/agreements" method:"get" mime:"json" tags:"管理后台-用户协议" summary:"协议版本列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Code     string `json:"code" in:"query" dc:"协议标识码筛选"`
	Status   string `json:"status" in:"query" dc:"状态筛选"`
}

type AgreementItem struct {
	Id          int64       `json:"id"`
	Code        string      `json:"code"`
	Version     string      `json:"version"`
	Title       string      `json:"title"`
	Summary     string      `json:"summary"`
	Status      string      `json:"status"`
	IsCurrent   bool        `json:"is_current"`
	ForceAccept bool        `json:"force_accept"`
	PublishedAt *gtime.Time `json:"published_at,omitempty"`
	CreatedAt   *gtime.Time `json:"created_at"`
	UpdatedAt   *gtime.Time `json:"updated_at"`
}

type AgreementListRes struct {
	List     []*AgreementItem `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type AgreementGetReq struct {
	g.Meta `path:"/agreements/{id}" method:"get" mime:"json" tags:"管理后台-用户协议" summary:"协议版本详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AgreementGetRes struct {
	Id          int64       `json:"id"`
	Code        string      `json:"code"`
	Version     string      `json:"version"`
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	Summary     string      `json:"summary"`
	Status      string      `json:"status"`
	IsCurrent   bool        `json:"is_current"`
	ForceAccept bool        `json:"force_accept"`
	PublishedAt *gtime.Time `json:"published_at,omitempty"`
	CreatedBy   int64       `json:"created_by"`
	CreatedAt   *gtime.Time `json:"created_at"`
	UpdatedAt   *gtime.Time `json:"updated_at"`
}

type AgreementUpdateReq struct {
	g.Meta      `path:"/agreements/{id}" method:"put" mime:"json" tags:"管理后台-用户协议" summary:"更新协议版本"`
	Id          int64  `json:"id" in:"path" v:"required|min:1"`
	Title       string `json:"title" v:"required|length:1,200" dc:"协议标题"`
	Content     string `json:"content" v:"required" dc:"协议正文（Markdown）"`
	Summary     string `json:"summary" v:"max-length:500" dc:"版本变更摘要"`
	ForceAccept *bool  `json:"force_accept" dc:"是否强制用户接受"`
}

type AgreementUpdateRes struct{}

type AgreementDeleteReq struct {
	g.Meta `path:"/agreements/{id}" method:"delete" mime:"json" tags:"管理后台-用户协议" summary:"删除协议版本"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AgreementDeleteRes struct{}

type AgreementPublishReq struct {
	g.Meta `path:"/agreements/{id}/publish" method:"post" mime:"json" tags:"管理后台-用户协议" summary:"发布协议版本"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AgreementPublishRes struct{}

// === 协议接受记录（管理后台审计） ===

type AgreementAcceptanceListReq struct {
	g.Meta   `path:"/agreements/{id}/acceptances" method:"get" mime:"json" tags:"管理后台-用户协议" summary:"协议接受记录"`
	Id       int64 `json:"id" in:"path" v:"required|min:1"`
	Page     int   `json:"page" in:"query" d:"1"`
	PageSize int   `json:"page_size" in:"query" d:"20"`
}

type AgreementAcceptanceItem struct {
	Id        int64       `json:"id"`
	UserType  string      `json:"user_type"`
	UserId    int64       `json:"user_id"`
	IpAddress string      `json:"ip_address"`
	CreatedAt *gtime.Time `json:"created_at"`
}

type AgreementAcceptanceListRes struct {
	List     []*AgreementAcceptanceItem `json:"list"`
	Total    int                        `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
}

// === 管理员待接受协议 ===

type AdminAgreementPendingReq struct {
	g.Meta `path:"/agreements/pending" method:"get" mime:"json" tags:"管理后台-用户协议" summary:"当前管理员待接受的协议"`
}

type PendingAgreementItem struct {
	Id      int64  `json:"id"`
	Code    string `json:"code"`
	Title   string `json:"title"`
	Version string `json:"version"`
	Content string `json:"content"`
}

type AdminAgreementPendingRes struct {
	List []*PendingAgreementItem `json:"list"`
}

type AdminAgreementAcceptReq struct {
	g.Meta       `path:"/agreements/accept" method:"post" mime:"json" tags:"管理后台-用户协议" summary:"管理员接受协议"`
	AgreementIds []int64 `json:"agreement_ids" v:"required|min-length:1" dc:"协议版本ID列表"`
}

type AdminAgreementAcceptRes struct{}

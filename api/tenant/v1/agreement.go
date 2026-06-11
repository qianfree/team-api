package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 用户协议（租户控制台） ===

// --- 租户用户待接受协议 ---

type TenantAgreementPendingReq struct {
	g.Meta `path:"/agreements/pending" method:"get" mime:"json" tags:"租户控制台-用户协议" summary:"当前用户待接受的协议"`
}

type TenantPendingAgreementItem struct {
	Id      int64  `json:"id"`
	Code    string `json:"code"`
	Title   string `json:"title"`
	Version string `json:"version"`
	Content string `json:"content"`
}

type TenantAgreementPendingRes struct {
	List []*TenantPendingAgreementItem `json:"list"`
}

type TenantAgreementAcceptReq struct {
	g.Meta       `path:"/agreements/accept" method:"post" mime:"json" tags:"租户控制台-用户协议" summary:"用户接受协议"`
	AgreementIds []int64 `json:"agreement_ids" v:"required|min-length:1" dc:"协议版本ID列表"`
}

type TenantAgreementAcceptRes struct{}

// --- 公开端点（登录/注册页面使用，无需认证） ---

type AgreementCurrentListReq struct {
	g.Meta `path:"/agreements/current" method:"get" mime:"json" tags:"公开-用户协议" summary:"所有当前生效的协议列表" middleware:"-"`
}

type PublicAgreementItem struct {
	Id          int64       `json:"id"`
	Code        string      `json:"code"`
	Title       string      `json:"title"`
	Version     string      `json:"version"`
	ForceAccept bool        `json:"force_accept"`
	PublishedAt *gtime.Time `json:"published_at,omitempty"`
}

type AgreementCurrentListRes struct {
	List []*PublicAgreementItem `json:"list"`
}

type AgreementCurrentGetReq struct {
	g.Meta `path:"/agreements/current/{code}" method:"get" mime:"json" tags:"公开-用户协议" summary:"按标识码获取当前协议详情" middleware:"-"`
	Code   string `json:"code" in:"path" v:"required|length:1,50"`
}

type AgreementCurrentGetRes struct {
	Id          int64       `json:"id"`
	Code        string      `json:"code"`
	Title       string      `json:"title"`
	Version     string      `json:"version"`
	Content     string      `json:"content"`
	ForceAccept bool        `json:"force_accept"`
	PublishedAt *gtime.Time `json:"published_at,omitempty"`
}

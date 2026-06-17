package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

// === 邮件发送记录（ntf_send_log）===

type EmailSendLogListReq struct {
	g.Meta       `path:"/email/send-logs" method:"get" mime:"json" tags:"管理后台-邮件" summary:"邮件发送记录列表"`
	Page         int    `json:"page" d:"1"`
	PageSize     int    `json:"page_size" d:"20"`
	Status       string `json:"status" dc:"状态：sent/failed"`
	Recipient    string `json:"recipient" dc:"收件人（模糊匹配）"`
	TemplateCode string `json:"template_code" dc:"模板编码"`
	StartDate    string `json:"start_date" dc:"开始日期 YYYY-MM-DD"`
	EndDate      string `json:"end_date" dc:"结束日期 YYYY-MM-DD"`
}

type EmailSendLogListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type EmailSendLogDetailReq struct {
	g.Meta `path:"/email/send-logs/{id}" method:"get" mime:"json" tags:"管理后台-邮件" summary:"邮件发送记录详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type EmailSendLogDetailRes struct {
	Data map[string]any `json:"-"`
}

func (r *EmailSendLogDetailRes) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Data)
}

// === 验证码申请记录（sys_email_verify_codes）===

type EmailVerifyCodeListReq struct {
	g.Meta    `path:"/email/verify-codes" method:"get" mime:"json" tags:"管理后台-邮件" summary:"验证码申请记录列表"`
	Page      int    `json:"page" d:"1"`
	PageSize  int    `json:"page_size" d:"20"`
	Email     string `json:"email" dc:"邮箱（模糊匹配）"`
	Purpose   string `json:"purpose" dc:"用途：register/reset_password/change_email"`
	Used      string `json:"used" dc:"是否已使用：true/false"`
	StartDate string `json:"start_date" dc:"开始日期 YYYY-MM-DD"`
	EndDate   string `json:"end_date" dc:"结束日期 YYYY-MM-DD"`
}

type EmailVerifyCodeListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

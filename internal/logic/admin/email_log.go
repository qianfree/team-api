package admin

import (
	"context"
	"encoding/json"
	"strings"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/entity"
)

// ListEmailSendLogs 分页查询邮件发送记录（ntf_send_log，主库）。
// 列表不返回 body（HTML 正文较大），由详情接口返回完整记录。
func (s *sAdmin) ListEmailSendLogs(ctx context.Context, req *v1.EmailSendLogListReq) (*v1.EmailSendLogListRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var conditions []string
	var args []any
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}
	if req.Recipient != "" {
		conditions = append(conditions, "recipient LIKE ?")
		args = append(args, "%"+req.Recipient+"%")
	}
	if req.TemplateCode != "" {
		conditions = append(conditions, "template_code = ?")
		args = append(args, req.TemplateCode)
	}
	if req.StartDate != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, req.EndDate+" 23:59:59")
	}
	where := strings.Join(conditions, " AND ")

	countM := dao.NtfSendLog.Ctx(ctx).Safe()
	if where != "" {
		countM = countM.Where(where, args...)
	}
	total, err := countM.Count()
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return &v1.EmailSendLogListRes{List: []map[string]any{}, Total: 0, Page: page, PageSize: pageSize}, nil
	}

	dataM := dao.NtfSendLog.Ctx(ctx).Safe().
		Fields("id, tenant_id, user_id, template_code, channel, recipient, subject, status, error_message, retry_count, sent_at, created_at").
		OrderDesc("created_at").
		Page(page, pageSize)
	if where != "" {
		dataM = dataM.Where(where, args...)
	}
	result, err := dataM.All()
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(result))
	for _, row := range result {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v.Val()
		}
		items = append(items, m)
	}

	return &v1.EmailSendLogListRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetEmailSendLogDetail 查询单条邮件发送记录详情（含 body 正文）。
func (s *sAdmin) GetEmailSendLogDetail(ctx context.Context, req *v1.EmailSendLogDetailReq) (*v1.EmailSendLogDetailRes, error) {
	var record *entity.NtfSendLog
	err := dao.NtfSendLog.Ctx(ctx).Where("id", req.Id).Scan(&record)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if record == nil {
		return nil, common.NewNotFoundError("邮件发送记录")
	}
	b, _ := json.Marshal(record)
	var detail map[string]any
	_ = json.Unmarshal(b, &detail)
	return &v1.EmailSendLogDetailRes{Data: detail}, nil
}

// ListEmailVerifyCodes 分页查询验证码申请记录（sys_email_verify_codes，主库）。
func (s *sAdmin) ListEmailVerifyCodes(ctx context.Context, req *v1.EmailVerifyCodeListReq) (*v1.EmailVerifyCodeListRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var conditions []string
	var args []any
	if req.Email != "" {
		conditions = append(conditions, "email LIKE ?")
		args = append(args, "%"+req.Email+"%")
	}
	if req.Purpose != "" {
		conditions = append(conditions, "purpose = ?")
		args = append(args, req.Purpose)
	}
	if req.Used == "true" {
		conditions = append(conditions, "used_at IS NOT NULL")
	} else if req.Used == "false" {
		conditions = append(conditions, "used_at IS NULL")
	}
	if req.StartDate != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, req.EndDate+" 23:59:59")
	}
	where := strings.Join(conditions, " AND ")

	countM := dao.SysEmailVerifyCodes.Ctx(ctx).Safe()
	if where != "" {
		countM = countM.Where(where, args...)
	}
	total, err := countM.Count()
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return &v1.EmailVerifyCodeListRes{List: []map[string]any{}, Total: 0, Page: page, PageSize: pageSize}, nil
	}

	dataM := dao.SysEmailVerifyCodes.Ctx(ctx).Safe().
		Fields("id, email, code, purpose, expires_at, used_at, created_at").
		OrderDesc("created_at").
		Page(page, pageSize)
	if where != "" {
		dataM = dataM.Where(where, args...)
	}
	result, err := dataM.All()
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(result))
	for _, row := range result {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v.Val()
		}
		items = append(items, m)
	}

	return &v1.EmailVerifyCodeListRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

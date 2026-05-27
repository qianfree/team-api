package admin

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

// ListTemplates 获取通知模板列表（分页）
func (s *sAdmin) ListTemplates(ctx context.Context, req *v1.TemplateListReq) (*v1.TemplateListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var total int
	items := make([]*v1.TemplateItem, 0)
	err := dao.NtfTemplates.Ctx(ctx).
		OrderAsc("code").
		Page(page, pageSize).
		ScanAndCount(&items, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.TemplateListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetTemplate 获取单个通知模板
func (s *sAdmin) GetTemplate(ctx context.Context, req *v1.TemplateGetReq) (*v1.TemplateGetRes, error) {
	var tpl map[string]any
	err := dao.NtfTemplates.Ctx(ctx).
		Where("code", req.Code).
		Scan(&tpl)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, common.NewNotFoundError("template")
	}
	return &v1.TemplateGetRes{Data: tpl}, nil
}

// UpdateTemplate 更新通知模板
func (s *sAdmin) UpdateTemplate(ctx context.Context, req *v1.TemplateUpdateReq) (*v1.TemplateUpdateRes, error) {
	data := do.NtfTemplates{}
	if req.Subject != "" {
		data.Subject = req.Subject
	}
	if req.BodyTemplate != "" {
		data.BodyTemplate = req.BodyTemplate
	}
	if req.Channel != "" {
		data.Channel = req.Channel
	}

	_, err := dao.NtfTemplates.Ctx(ctx).
		Where("code", req.Code).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// TestTemplate 用测试变量渲染模板，返回渲染结果（不发送）
func (s *sAdmin) TestTemplate(ctx context.Context, req *v1.TemplateTestReq) (*v1.TemplateTestRes, error) {
	var tpl struct {
		Subject      string `json:"subject"`
		BodyTemplate string `json:"body_template"`
		Channel      string `json:"channel"`
	}
	err := dao.NtfTemplates.Ctx(ctx).
		Where("code", req.Code).
		Scan(&tpl)
	if err != nil {
		return nil, err
	}
	if tpl.BodyTemplate == "" {
		return nil, common.NewNotFoundError("template")
	}

	type renderResult struct {
		Original string `json:"original"`
		Rendered string `json:"rendered"`
		Error    string `json:"error,omitempty"`
	}

	subjectRendered, subjectErr := renderGoTemplate(tpl.Subject, req.Variables)
	bodyRendered, bodyErr := renderGoTemplate(tpl.BodyTemplate, req.Variables)

	result := map[string]any{
		"subject": renderResult{
			Original: tpl.Subject,
			Rendered: subjectRendered,
		},
		"body": renderResult{
			Original: tpl.BodyTemplate,
			Rendered: bodyRendered,
		},
		"channel": tpl.Channel,
	}

	if subjectErr != nil {
		result["subject"] = renderResult{
			Original: tpl.Subject,
			Error:    subjectErr.Error(),
		}
	}
	if bodyErr != nil {
		result["body"] = renderResult{
			Original: tpl.BodyTemplate,
			Error:    bodyErr.Error(),
		}
	}

	return &v1.TemplateTestRes{Data: result}, nil
}

// SendMessage 创建手动站内消息并推送 WebSocket 通知
func (s *sAdmin) SendMessage(ctx context.Context, req *v1.MessageSendReq) (*v1.MessageSendRes, error) {
	if req.Title == "" || req.Content == "" {
		return nil, common.NewBadRequestError("title and content are required")
	}

	engine := common.NewNotificationEngine()
	if req.UserID > 0 {
		return nil, engine.SendMessage(ctx, req.TenantID, req.UserID, "system", req.Title, req.Content)
	}
	// 未指定用户时发送广播
	return nil, engine.SendBroadcastMessage(ctx, req.TenantID, "system", req.Title, req.Content, "")
}

// SendBroadcast 创建广播消息并推送 WebSocket 通知
func (s *sAdmin) SendBroadcast(ctx context.Context, req *v1.MessageBroadcastReq) (*v1.MessageBroadcastRes, error) {
	if req.Title == "" || req.Content == "" {
		return nil, common.NewBadRequestError("title and content are required")
	}

	engine := common.NewNotificationEngine()
	return nil, engine.SendBroadcastMessage(ctx, req.TenantID, "system", req.Title, req.Content, req.TargetRoles)
}

// ListMessages 获取所有消息列表（管理后台，支持过滤）
func (s *sAdmin) ListMessages(ctx context.Context, req *v1.MessageListReq) (*v1.MessageListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.NtfMessages.Ctx(ctx).
		LeftJoin("tnt_tenants t", "t.id = ntf_messages.tenant_id").
		LeftJoin("tnt_users u", "u.id = ntf_messages.user_id").
		Fields("ntf_messages.*, t.name as tenant_name, u.display_name as user_name")
	if req.TenantID > 0 {
		query = query.Where("ntf_messages.tenant_id", req.TenantID)
	}
	if req.Type != "" {
		query = query.Where("ntf_messages.type", req.Type)
	}

	var total int
	items := make([]*v1.MessageItem, 0)
	err := query.OrderDesc("ntf_messages.created_at").
		Page(page, pageSize).
		ScanAndCount(&items, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.MessageListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetMessageReadStats 获取广播消息的已读统计
func GetMessageReadStats(ctx context.Context, messageID int64) (map[string]any, error) {
	var msg struct {
		IsBroadcast int   `json:"is_broadcast"`
		TenantID    int64 `json:"tenant_id"`
	}
	err := dao.NtfMessages.Ctx(ctx).
		Where("id", messageID).
		Fields("is_broadcast, tenant_id").
		Scan(&msg)
	if err != nil {
		return nil, err
	}
	if msg.IsBroadcast != 1 {
		return nil, common.NewBadRequestError("message is not a broadcast")
	}

	totalMembers, err := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", msg.TenantID).
		Where("status", "active").
		Count()
	if err != nil {
		return nil, err
	}

	readCount, err := dao.NtfReadStatus.Ctx(ctx).
		Where("message_id", messageID).
		Count()
	if err != nil {
		return nil, err
	}

	unreadCount := totalMembers - readCount
	if unreadCount < 0 {
		unreadCount = 0
	}

	readRate := float64(0)
	if totalMembers > 0 {
		readRate = float64(readCount) / float64(totalMembers) * 100
	}

	return map[string]any{
		"message_id":    messageID,
		"total_members": totalMembers,
		"read_count":    readCount,
		"unread_count":  unreadCount,
		"read_rate":     fmt.Sprintf("%.1f%%", readRate),
	}, nil
}

// CreateAnnouncement 创建公告
func (s *sAdmin) CreateAnnouncement(ctx context.Context, req *v1.AnnouncementCreateReq) (*v1.AnnouncementCreateRes, error) {
	if req.Title == "" || req.Content == "" {
		return nil, common.NewBadRequestError("title and content are required")
	}
	annType := req.Type
	if annType == "" {
		annType = "info"
	}
	status := req.Status
	if status == "" {
		status = "draft"
	}
	displayPosition := req.DisplayPosition
	if displayPosition == "" {
		displayPosition = "console"
	}

	// Get admin user ID from context
	adminUserID := common.GetCtxUserID(ctx)

	data := do.NtfAnnouncements{
		Title:           req.Title,
		Type:            annType,
		Content:         req.Content,
		Status:          status,
		IsPinned:        req.IsPinned,
		DisplayPosition: displayPosition,
		CreatedBy:       adminUserID,
	}
	if req.EffectiveAt != "" {
		if t, err := time.Parse(time.RFC3339, req.EffectiveAt); err == nil {
			data.EffectiveAt = gtime.NewFromTime(t)
		}
	}
	if req.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, req.ExpiresAt); err == nil {
			data.ExpiresAt = gtime.NewFromTime(t)
		}
	}

	result, err := dao.NtfAnnouncements.Ctx(ctx).Insert(data)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &v1.AnnouncementCreateRes{ID: id}, nil
}

// UpdateAnnouncement 更新公告
func (s *sAdmin) UpdateAnnouncement(ctx context.Context, req *v1.AnnouncementUpdateReq) (*v1.AnnouncementUpdateRes, error) {
	data := do.NtfAnnouncements{}

	if req.Title != "" {
		data.Title = req.Title
	}
	if req.Type != "" {
		data.Type = req.Type
	}
	if req.Content != "" {
		data.Content = req.Content
	}
	if req.Status != "" {
		data.Status = req.Status
	}
	if req.IsPinned != nil {
		data.IsPinned = *req.IsPinned
	}
	if req.DisplayPosition != "" {
		data.DisplayPosition = req.DisplayPosition
	}
	if req.EffectiveAt != "" {
		if t, err := gtime.StrToTime(req.EffectiveAt); err == nil {
			data.EffectiveAt = t
		}
	}
	if req.ExpiresAt != "" {
		if t, err := gtime.StrToTime(req.ExpiresAt); err == nil {
			data.ExpiresAt = t
		}
	}

	_, err := dao.NtfAnnouncements.Ctx(ctx).
		Where("id", req.Id).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ListAnnouncements 获取公告列表（分页）
func (s *sAdmin) ListAnnouncements(ctx context.Context, req *v1.AnnouncementListReq) (*v1.AnnouncementListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.NtfAnnouncements.Ctx(ctx)
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}

	var total int
	items := make([]*v1.AnnouncementItem, 0)
	err := query.OrderDesc("is_pinned").
		OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&items, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.AnnouncementListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// PublishAnnouncement 发布公告
func (s *sAdmin) PublishAnnouncement(ctx context.Context, req *v1.AnnouncementPublishReq) (*v1.AnnouncementPublishRes, error) {
	_, err := dao.NtfAnnouncements.Ctx(ctx).
		Where("id", req.Id).
		Data(do.NtfAnnouncements{
			Status: "published",
		}).Update()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ArchiveAnnouncement 归档公告
func (s *sAdmin) ArchiveAnnouncement(ctx context.Context, req *v1.AnnouncementArchiveReq) (*v1.AnnouncementArchiveRes, error) {
	_, err := dao.NtfAnnouncements.Ctx(ctx).
		Where("id", req.Id).
		Data(do.NtfAnnouncements{
			Status: "archived",
		}).Update()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// renderGoTemplate renders a Go template string with variables.
func renderGoTemplate(tplStr string, vars map[string]any) (string, error) {
	if tplStr == "" {
		return "", nil
	}
	t, err := template.New("").Parse(tplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

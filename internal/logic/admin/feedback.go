package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ListAllFeedbacks 管理后台反馈列表
func (s *sAdmin) ListAllFeedbacks(ctx context.Context, req *v1.FeedbackListAllReq) (*v1.FeedbackListAllRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := g.DB().Model("spt_feedbacks f").Ctx(ctx)
	if req.Status != "" {
		query = query.Where("f.status", req.Status)
	}
	if req.Category != "" {
		query = query.Where("f.category", req.Category)
	}
	if req.TenantID > 0 {
		query = query.Where("f.tenant_id", req.TenantID)
	}
	if req.Priority != "" {
		query = query.Where("f.priority", req.Priority)
	}

	type feedbackRow struct {
		Id              int64       `json:"id" orm:"id"`
		TenantId        int64       `json:"tenant_id" orm:"tenant_id"`
		UserId          int64       `json:"user_id" orm:"user_id"`
		Category        string      `json:"category" orm:"category"`
		Title           string      `json:"title" orm:"title"`
		Description     string      `json:"description" orm:"description"`
		Status          string      `json:"status" orm:"status"`
		Priority        string      `json:"priority" orm:"priority"`
		AdminReply      string      `json:"admin_reply" orm:"admin_reply"`
		Resolution      string      `json:"resolution" orm:"resolution"`
		CreatedAt       *gtime.Time `json:"created_at" orm:"created_at"`
		UpdatedAt       *gtime.Time `json:"updated_at" orm:"updated_at"`
		TenantName      string      `json:"tenant_name" orm:"tenant_name"`
		UserDisplayName string      `json:"user_display_name" orm:"user_display_name"`
	}

	var total int
	rows := make([]feedbackRow, 0)
	err := query.
		LeftJoin("tnt_tenants tt", "f.tenant_id = tt.id").
		LeftJoin("tnt_users tu", "f.user_id = tu.id").
		Fields("f.*, COALESCE(tt.name, '') as tenant_name, COALESCE(tu.display_name, '') as user_display_name").
		OrderDesc("f.created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}

	items := make([]*v1.FeedbackAdminItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, &v1.FeedbackAdminItem{
			Id:              r.Id,
			TenantId:        r.TenantId,
			TenantName:      r.TenantName,
			UserId:          r.UserId,
			UserDisplayName: r.UserDisplayName,
			Category:        r.Category,
			Title:           r.Title,
			Description:     r.Description,
			Status:          r.Status,
			Priority:        r.Priority,
			AdminReply:      r.AdminReply,
			Resolution:      r.Resolution,
			CreatedAt:       r.CreatedAt,
			UpdatedAt:       r.UpdatedAt,
		})
	}

	return &v1.FeedbackListAllRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ReplyToFeedback 管理员回复反馈
func (s *sAdmin) ReplyToFeedback(ctx context.Context, req *v1.FeedbackReplyReq) (*v1.FeedbackReplyRes, error) {
	var fb *struct {
		Id       int64  `json:"id"`
		TenantId int64  `json:"tenant_id"`
		UserId   int64  `json:"user_id"`
		Title    string `json:"title"`
	}
	err := dao.SptFeedbacks.Ctx(ctx).Where("id", req.Id).Scan(&fb)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if fb == nil {
		return nil, common.NewBusinessError(10063, "反馈不存在")
	}

	updateData := g.Map{
		"admin_reply":    req.Reply,
		"admin_reply_by": common.GetCtxUserID(ctx),
		"admin_reply_at": gtime.Now(),
	}
	if req.Status != "" {
		updateData["status"] = req.Status
	}
	if req.Resolution != "" {
		updateData["resolution"] = req.Resolution
	}

	_, err = dao.SptFeedbacks.Ctx(ctx).
		Where("id", req.Id).
		Data(updateData).
		Update()
	if err != nil {
		return nil, err
	}

	// 通知用户
	if req.Status == "acknowledged" || req.Status == "resolved" {
		templateCode := "feedback_acknowledged"
		if req.Status == "resolved" {
			templateCode = "feedback_resolved"
		}
		engine := common.NewNotificationEngine()
		_ = engine.SendNotification(ctx, fb.TenantId, fb.UserId, templateCode, map[string]any{
			"title":      fb.Title,
			"resolution": req.Resolution,
		})
	}

	return &v1.FeedbackReplyRes{}, nil
}

// UpdateFeedbackStatus 更新反馈状态
func (s *sAdmin) UpdateFeedbackStatus(ctx context.Context, req *v1.FeedbackUpdateStatusReq) (*v1.FeedbackUpdateStatusRes, error) {
	var fb *struct {
		Id int64 `json:"id"`
	}
	err := dao.SptFeedbacks.Ctx(ctx).Where("id", req.Id).Scan(&fb)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if fb == nil {
		return nil, common.NewBusinessError(10063, "反馈不存在")
	}

	updateData := g.Map{"status": req.Status}
	if req.Priority != "" {
		updateData["priority"] = req.Priority
	}

	_, err = dao.SptFeedbacks.Ctx(ctx).
		Where("id", req.Id).
		Data(updateData).
		Update()
	if err != nil {
		return nil, err
	}

	return &v1.FeedbackUpdateStatusRes{}, nil
}

// GetFeedbackStats 反馈统计
func (s *sAdmin) GetFeedbackStats(ctx context.Context, req *v1.FeedbackStatsReq) (*v1.FeedbackStatsRes, error) {
	type countRow struct {
		Status string `json:"status" orm:"status"`
		Count  int    `json:"count" orm:"count"`
	}

	var statusCounts []countRow
	err := dao.SptFeedbacks.Ctx(ctx).
		Fields("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	stats := &v1.FeedbackStatsRes{ByCategory: make(map[string]int)}
	for _, r := range statusCounts {
		switch r.Status {
		case "pending":
			stats.Pending = r.Count
		case "acknowledged":
			stats.Acknowledged = r.Count
		case "in_progress":
			stats.InProgress = r.Count
		case "resolved":
			stats.Resolved = r.Count
		case "closed":
			stats.Closed = r.Count
		}
		stats.Total += r.Count
	}

	type catRow struct {
		Category string `json:"category" orm:"category"`
		Count    int    `json:"count" orm:"count"`
	}
	var catCounts []catRow
	dao.SptFeedbacks.Ctx(ctx).
		Fields("category, COUNT(*) as count").
		Group("category").
		Scan(&catCounts)
	for _, r := range catCounts {
		stats.ByCategory[r.Category] = r.Count
	}

	type trendRow struct {
		Date  string `json:"date" orm:"date"`
		Count int    `json:"count" orm:"count"`
	}
	dao.SptFeedbacks.Ctx(ctx).
		Fields("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= NOW() - INTERVAL '30 days'").
		Group("DATE(created_at)").
		OrderAsc("date").
		Scan(&stats.RecentTrend)

	return stats, nil
}

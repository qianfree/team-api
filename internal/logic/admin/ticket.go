package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
)

// ListAllTickets 获取全部工单列表（管理后台）
func (s *sAdmin) ListAllTickets(ctx context.Context, req *v1.TicketListReq) (*v1.TicketListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.SptTickets.Ctx(ctx)
	if req.Status != "" {
		query = query.Where("spt_tickets.status", req.Status)
	}
	if req.Category != "" {
		query = query.Where("spt_tickets.category", req.Category)
	}
	if req.TenantID > 0 {
		query = query.Where("spt_tickets.tenant_id", req.TenantID)
	}
	if req.AssignedAdminID > 0 {
		query = query.Where("spt_tickets.assigned_admin_id", req.AssignedAdminID)
	}

	var total int
	type ticketRow struct {
		Id                int64       `json:"id" orm:"id"`
		TenantId          int64       `json:"tenant_id" orm:"tenant_id"`
		UserId            int64       `json:"user_id" orm:"user_id"`
		Category          string      `json:"category" orm:"category"`
		Title             string      `json:"title" orm:"title"`
		Description       string      `json:"description" orm:"description"`
		Urgency           string      `json:"urgency" orm:"urgency"`
		Status            string      `json:"status" orm:"status"`
		AssignedAdminId   int64       `json:"assigned_admin_id" orm:"assigned_admin_id"`
		CreatedAt         *gtime.Time `json:"created_at" orm:"created_at"`
		UpdatedAt         *gtime.Time `json:"updated_at" orm:"updated_at"`
		TenantName        string      `json:"tenant_name" orm:"tenant_name"`
		UserDisplayName   string      `json:"user_display_name" orm:"user_display_name"`
		AssignedAdminName string      `json:"assigned_admin_name" orm:"assigned_admin_name"`
	}
	rows := make([]ticketRow, 0)
	err := query.
		LeftJoin("tnt_tenants tt", "spt_tickets.tenant_id = tt.id").
		LeftJoin("tnt_users tu", "spt_tickets.user_id = tu.id").
		LeftJoin("sys_admin_users sa", "spt_tickets.assigned_admin_id = sa.id").
		Fields("spt_tickets.*, COALESCE(tt.name, '') as tenant_name, COALESCE(tu.display_name, '') as user_display_name, COALESCE(sa.username, '') as assigned_admin_name").
		OrderDesc("spt_tickets.created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}

	items := make([]*v1.TicketItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, &v1.TicketItem{
			Id:                r.Id,
			TenantId:          r.TenantId,
			UserId:            r.UserId,
			Category:          r.Category,
			Title:             r.Title,
			Description:       r.Description,
			Urgency:           r.Urgency,
			Status:            r.Status,
			AssignedAdminId:   r.AssignedAdminId,
			TenantName:        r.TenantName,
			UserDisplayName:   r.UserDisplayName,
			AssignedAdminName: r.AssignedAdminName,
			CreatedAt:         r.CreatedAt,
			UpdatedAt:         r.UpdatedAt,
		})
	}

	return &v1.TicketListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetTicketAdmin 获取工单详情（管理后台，含回复）
func (s *sAdmin) GetTicketAdmin(ctx context.Context, req *v1.TicketGetReq) (*v1.TicketGetRes, error) {
	type ticketDetail struct {
		entity.SptTickets
		TenantName        string `json:"tenant_name"`
		UserDisplayName   string `json:"user_display_name"`
		AssignedAdminName string `json:"assigned_admin_name"`
	}
	var ticket *ticketDetail
	err := dao.SptTickets.Ctx(ctx).
		LeftJoin("tnt_tenants tt", "spt_tickets.tenant_id = tt.id").
		LeftJoin("tnt_users tu", "spt_tickets.user_id = tu.id").
		LeftJoin("sys_admin_users sa", "spt_tickets.assigned_admin_id = sa.id").
		Fields("spt_tickets.*, COALESCE(tt.name, '') as tenant_name, COALESCE(tu.display_name, '') as user_display_name, COALESCE(sa.username, '') as assigned_admin_name").
		Where("spt_tickets.id", req.Id).
		Scan(&ticket)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, common.NewNotFoundError("工单")
	}

	type replyItem struct {
		entity.SptReplies
		UserName string `json:"user_name"`
	}
	var replies []*replyItem
	err = dao.SptReplies.Ctx(ctx).
		LeftJoin("sys_admin_users sa", "spt_replies.user_type = 'admin' AND spt_replies.user_id = sa.id").
		LeftJoin("tnt_users tu", "spt_replies.user_type = 'tenant' AND spt_replies.user_id = tu.id").
		Fields("spt_replies.*, CASE WHEN spt_replies.user_type = 'admin' THEN COALESCE(sa.username, '') ELSE COALESCE(tu.display_name, '') END as user_name").
		Where("spt_replies.ticket_id", req.Id).
		OrderAsc("spt_replies.created_at").
		Scan(&replies)
	if err != nil {
		return nil, err
	}

	var attachments []*entity.SptAttachments
	err = dao.SptAttachments.Ctx(ctx).
		Where("ticket_id", req.Id).
		OrderAsc("created_at").
		Scan(&attachments)
	if err != nil {
		return nil, err
	}

	data := g.Map{
		"id":                  ticket.Id,
		"tenant_id":           ticket.TenantId,
		"user_id":             ticket.UserId,
		"category":            ticket.Category,
		"title":               ticket.Title,
		"description":         ticket.Description,
		"urgency":             ticket.Urgency,
		"status":              ticket.Status,
		"assigned_admin_id":   ticket.AssignedAdminId,
		"tenant_name":         ticket.TenantName,
		"user_display_name":   ticket.UserDisplayName,
		"assigned_admin_name": ticket.AssignedAdminName,
		"created_at":          ticket.CreatedAt,
		"updated_at":          ticket.UpdatedAt,
		"replies":             replies,
		"attachments":         attachments,
	}
	return &v1.TicketGetRes{Data: data}, nil
}

// AssignTicket 分配工单给管理员
func (s *sAdmin) AssignTicket(ctx context.Context, req *v1.TicketAssignReq) (*v1.TicketAssignRes, error) {
	var ticket struct {
		Status string `json:"status"`
	}
	err := dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Fields("status").
		Scan(&ticket)
	if err != nil {
		return nil, err
	}
	if ticket.Status == "" {
		return nil, common.NewNotFoundError("工单")
	}

	newStatus := ticket.Status
	if ticket.Status == "pending" {
		newStatus = "processing"
	}

	_, err = dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Data(do.SptTickets{
			AssignedAdminId: req.AdminID,
			Status:          newStatus,
		}).Update()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// ReplyToTicketAdmin 管理员回复工单
func (s *sAdmin) ReplyToTicketAdmin(ctx context.Context, req *v1.TicketReplyReq) (*v1.TicketReplyRes, error) {
	if req.Content == "" {
		return nil, common.NewBadRequestError("回复内容不能为空")
	}

	adminID := getCtxUserID(ctx)

	var ticket struct {
		Status          string `json:"status"`
		AssignedAdminID *int64 `json:"assigned_admin_id"`
	}
	err := dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Fields("status, assigned_admin_id").
		Scan(&ticket)
	if err != nil {
		return nil, err
	}
	if ticket.Status == "" {
		return nil, common.NewNotFoundError("工单")
	}
	if ticket.Status == "closed" {
		return nil, common.NewBadRequestError("工单已关闭，无法回复")
	}

	_, err = dao.SptReplies.Ctx(ctx).Insert(do.SptReplies{
		TicketId: req.Id,
		UserId:   adminID,
		UserType: "admin",
		Content:  req.Content,
	})
	if err != nil {
		return nil, err
	}

	data := do.SptTickets{
		Status: "replied",
	}
	if ticket.AssignedAdminID == nil {
		data.AssignedAdminId = adminID
	}

	_, err = dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Data(data).Update()
	return nil, err
}

// UpdateTicketStatus 更新工单状态
func (s *sAdmin) UpdateTicketStatus(ctx context.Context, req *v1.TicketStatusUpdateReq) (*v1.TicketStatusUpdateRes, error) {
	validStatuses := map[string]bool{
		"pending": true, "processing": true, "replied": true,
		"closed": true, "reopened": true,
	}
	if !validStatuses[req.Status] {
		return nil, common.NewBadRequestError("无效的工单状态")
	}

	var ticket struct {
		Status string `json:"status"`
	}
	err := dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Fields("status").
		Scan(&ticket)
	if err != nil {
		return nil, err
	}
	if ticket.Status == "" {
		return nil, common.NewNotFoundError("工单")
	}

	validTransitions := map[string][]string{
		"pending":    {"processing", "closed"},
		"processing": {"replied", "closed"},
		"replied":    {"processing", "closed"},
		"closed":     {"reopened"},
		"reopened":   {"processing", "closed"},
	}

	allowed, ok := validTransitions[ticket.Status]
	if !ok {
		return nil, common.NewBadRequestError("当前状态不允许变更")
	}

	valid := false
	for _, st := range allowed {
		if st == req.Status {
			valid = true
			break
		}
	}
	if !valid {
		return nil, gerror.Newf("不允许从 %s 变更为 %s", ticket.Status, req.Status)
	}

	_, err = dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Data(do.SptTickets{
			Status: req.Status,
		}).Update()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

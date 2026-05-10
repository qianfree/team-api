package tenant

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/logic/common"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/qianfree/team-api/internal/utility/export"
)

// TicketCreate 创建工单
func (s *sTenant) TicketCreate(ctx context.Context, req *v1.TenantTicketCreateReq) (*v1.TenantTicketCreateRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	result, err := dao.SptTickets.Ctx(ctx).Insert(do.SptTickets{
		TenantId:    tenantID,
		UserId:      userID,
		Category:    req.Category,
		Title:       req.Title,
		Description: req.Description,
		Urgency:     req.Urgency,
		Status:      "pending",
	})
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.TenantTicketCreateRes{ID: id}, nil
}

// TicketList 获取租户工单列表
func (s *sTenant) TicketList(ctx context.Context, req *v1.TenantTicketListReq) (*v1.TenantTicketListRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	items := make([]*v1.TenantTicketItem, 0)
	listQuery := dao.SptTickets.Ctx(ctx).
		LeftJoin("sys_admin_users sa", "spt_tickets.assigned_admin_id = sa.id").
		Fields("spt_tickets.*, COALESCE(sa.username, '') as assigned_admin_name").
		Where("spt_tickets.tenant_id", tenantID).
		Where("spt_tickets.user_id", userID)
	if req.Status != "" {
		listQuery = listQuery.Where("spt_tickets.status", req.Status)
	}

	var total int
	err := listQuery.OrderDesc("spt_tickets.created_at").
		Page(page, pageSize).
		ScanAndCount(&items, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.TenantTicketListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// TicketGet 获取工单详情（含回复）
func (s *sTenant) TicketGet(ctx context.Context, req *v1.TenantTicketGetReq) (*v1.TenantTicketGetRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	var ticket map[string]any
	err := dao.SptTickets.Ctx(ctx).
		LeftJoin("sys_admin_users sa", "spt_tickets.assigned_admin_id = sa.id").
		Fields("spt_tickets.*, COALESCE(sa.username, '') as assigned_admin_name").
		Where("spt_tickets.id", req.Id).
		Where("spt_tickets.tenant_id", tenantID).
		Where("spt_tickets.user_id", userID).
		Scan(&ticket)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, common.NewNotFoundError("工单")
	}

	replies := make([]map[string]any, 0)
	err = dao.SptReplies.Ctx(ctx).
		Where("ticket_id", req.Id).
		OrderAsc("created_at").
		Scan(&replies)
	if err != nil {
		return nil, err
	}

	attachments := make([]map[string]any, 0)
	err = dao.SptAttachments.Ctx(ctx).
		Where("ticket_id", req.Id).
		OrderAsc("created_at").
		Scan(&attachments)
	if err != nil {
		return nil, err
	}

	ticket["replies"] = replies
	ticket["attachments"] = attachments
	return &v1.TenantTicketGetRes{Data: ticket}, nil
}

// TicketReply 租户用户回复工单
func (s *sTenant) TicketReply(ctx context.Context, req *v1.TenantTicketReplyReq) (*v1.TenantTicketReplyRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	if req.Content == "" {
		return nil, common.NewBadRequestError("回复内容不能为空")
	}

	var ticket struct {
		TenantID int64  `json:"tenant_id"`
		UserID   int64  `json:"user_id"`
		Status   string `json:"status"`
	}
	err := dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Fields("tenant_id, user_id, status").
		Scan(&ticket)
	if err != nil {
		return nil, err
	}
	if ticket.TenantID == 0 {
		return nil, common.NewNotFoundError("工单")
	}
	if ticket.TenantID != tenantID || ticket.UserID != userID {
		return nil, common.NewForbiddenError("无权操作此工单")
	}
	if ticket.Status == "closed" {
		return nil, common.NewBadRequestError("工单已关闭，请重新打开后再回复")
	}

	_, err = dao.SptReplies.Ctx(ctx).Insert(do.SptReplies{
		TicketId: req.Id,
		UserId:   userID,
		UserType: "tenant",
		Content:  req.Content,
	})
	if err != nil {
		return nil, err
	}

	newStatus := ticket.Status
	if ticket.Status == "processing" || ticket.Status == "pending" {
		newStatus = "replied"
	}

	_, err = dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Data(do.SptTickets{
			Status: newStatus,
		}).Update()
	if err != nil {
		return nil, err
	}

	return &v1.TenantTicketReplyRes{}, nil
}

// TicketClose 租户用户关闭工单
func (s *sTenant) TicketClose(ctx context.Context, req *v1.TenantTicketCloseReq) (*v1.TenantTicketCloseRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	result, err := dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Where("status != ?", "closed").
		Data(do.SptTickets{
			Status: "closed",
		}).Update()
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, common.NewNotFoundError("工单")
	}
	return &v1.TenantTicketCloseRes{}, nil
}

// TicketReopen 租户用户重新打开工单
func (s *sTenant) TicketReopen(ctx context.Context, req *v1.TenantTicketReopenReq) (*v1.TenantTicketReopenRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	result, err := dao.SptTickets.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Where("status", "closed").
		Data(do.SptTickets{
			Status: "reopened",
		}).Update()
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, common.NewBadRequestError("工单不存在或无法重新打开")
	}
	return &v1.TenantTicketReopenRes{}, nil
}

// ExportTickets exports the tenant ticket list as CSV or Excel.
func (s *sTenant) ExportTickets(ctx context.Context, req *v1.TenantTicketExportReq) (*v1.TenantTicketExportRes, error) {
	r := g.RequestFromCtx(ctx)
	format := req.Format
	if format == "" {
		format = "csv"
	}

	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "category", Header: "分类"},
		{Field: "title", Header: "标题"},
		{Field: "urgency", Header: "紧急程度"},
		{Field: "status", Header: "状态"},
		{Field: "assigned_admin_name", Header: "处理人"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   format,
		Filename: "工单列表_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	buildQuery := func() *gdb.Model {
		query := dao.SptTickets.Ctx(ctx).
			LeftJoin("sys_admin_users sa", "spt_tickets.assigned_admin_id = sa.id").
			Fields("spt_tickets.id, spt_tickets.category, spt_tickets.title, spt_tickets.urgency, spt_tickets.status, COALESCE(sa.username, '') as assigned_admin_name, spt_tickets.created_at").
			Where("spt_tickets.tenant_id", tenantID).
			Where("spt_tickets.user_id", userID)
		if req.Status != "" {
			query = query.Where("spt_tickets.status", req.Status)
		}
		return query
	}

	if format == "xlsx" {
		var items []*v1.TenantTicketItem
		err := buildQuery().OrderDesc("spt_tickets.created_at").Scan(&items)
		if err != nil {
			return nil, err
		}

		data := make([]map[string]any, 0, len(items))
		for _, item := range items {
			data = append(data, map[string]any{
				"id":                  item.Id,
				"category":            item.Category,
				"title":               item.Title,
				"urgency":             item.Urgency,
				"status":              item.Status,
				"assigned_admin_name": item.AssignedAdminName,
				"created_at":          item.CreatedAt,
			})
		}
		return nil, export.WriteExcel(r, config, data)
	}

	return nil, export.StreamCSV(r, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			var items []*v1.TenantTicketItem
			err := buildQuery().OrderDesc("spt_tickets.created_at").Limit(1000).Offset(offset).Scan(&items)
			if err != nil {
				return
			}
			for _, item := range items {
				if !yield(map[string]any{
					"id":                  item.Id,
					"category":            item.Category,
					"title":               item.Title,
					"urgency":             item.Urgency,
					"status":              item.Status,
					"assigned_admin_name": item.AssignedAdminName,
					"created_at":          item.CreatedAt,
				}) {
					return
				}
			}
			if len(items) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

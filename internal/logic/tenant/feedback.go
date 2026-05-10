package tenant

import (
	"context"
	"encoding/json"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/logic/common"

	"github.com/gogf/gf/v2/frame/g"
)

// CreateFeedback 提交反馈
func (s *sTenant) CreateFeedback(ctx context.Context, req *v1.FeedbackCreateReq) (*v1.FeedbackCreateRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	metadata := "{}"
	if req.Metadata != nil {
		if b, err := json.Marshal(req.Metadata); err == nil {
			metadata = string(b)
		}
	}

	result, err := g.DB().Model("spt_feedbacks").Ctx(ctx).Data(g.Map{
		"tenant_id":   tenantID,
		"user_id":     userID,
		"category":    req.Category,
		"title":       req.Title,
		"description": req.Description,
		"metadata":    metadata,
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.FeedbackCreateRes{Id: id}, nil
}

// ListFeedbacks 我的反馈列表
func (s *sTenant) ListFeedbacks(ctx context.Context, req *v1.FeedbackListReq) (*v1.FeedbackListRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	page, pageSize := normalizePagination(req.Page, req.PageSize)

	query := g.DB().Model("spt_feedbacks").Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", userID)
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}
	if req.Category != "" {
		query = query.Where("category", req.Category)
	}

	var total int
	rows := make([]*v1.FeedbackItem, 0)
	err := query.OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.FeedbackListRes{
		List:     rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetFeedback 反馈详情
func (s *sTenant) GetFeedback(ctx context.Context, req *v1.FeedbackGetReq) (*v1.FeedbackGetRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	var row v1.FeedbackGetRes
	err := g.DB().Model("spt_feedbacks").Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Scan(&row)
	if err != nil {
		return nil, err
	}
	if row.Id == 0 {
		return nil, common.NewBusinessError(10063, "反馈不存在")
	}

	return &row, nil
}

func normalizePagination(page, pageSize int) (int, int) {
	return common.NormalizePagination(page, pageSize)
}

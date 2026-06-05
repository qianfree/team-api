package admin

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/utility/export"
)

// ListPlans 获取套餐列表
func (s *sAdmin) ListPlans(ctx context.Context, req *v1.PlanListReq) (*v1.PlanListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.PlnPlans.Ctx(ctx)
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}

	var total int
	plans := make([]*v1.PlanItem, 0)
	err := query.OrderAsc("sort_order").
		Page(page, pageSize).
		ScanAndCount(&plans, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.PlanListRes{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		List:     plans,
	}, nil
}

// GetPlan 获取套餐详情
func (s *sAdmin) GetPlan(ctx context.Context, req *v1.PlanDetailReq) (*v1.PlanDetailRes, error) {
	var plan *v1.PlanItem
	err := dao.PlnPlans.Ctx(ctx).
		Where("id", req.Id).
		Scan(&plan)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, common.NewNotFoundError("套餐")
	}
	return &v1.PlanDetailRes{Data: plan}, nil
}

// CreatePlan 创建套餐
func (s *sAdmin) CreatePlan(ctx context.Context, req *v1.PlanCreateReq) (*v1.PlanCreateRes, error) {
	result, err := dao.PlnPlans.Ctx(ctx).Insert(do.PlnPlans{
		Name:               req.Name,
		Identifier:         req.Identifier,
		Description:        req.Description,
		MonthlyPrice:       req.MonthlyPrice,
		YearlyPrice:        req.YearlyPrice,
		MonthlyQuotaTokens: req.MonthlyQuotaTokens,
		AllowedModels:      req.AllowedModels,
		IsRecommended:      req.IsRecommended,
		SortOrder:          req.SortOrder,
	})
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &v1.PlanCreateRes{ID: id}, nil
}

// UpdatePlan 更新套餐
func (s *sAdmin) UpdatePlan(ctx context.Context, req *v1.PlanUpdateReq) (*v1.PlanUpdateRes, error) {
	data := do.PlnPlans{}
	allowedFields := map[string]bool{
		"name": true, "identifier": true, "description": true,
		"monthly_price": true, "yearly_price": true,
		"monthly_quota_tokens": true, "allowed_models": true,
		"is_recommended": true, "sort_order": true, "status": true,
	}
	for k, v := range req.Update {
		if !allowedFields[k] {
			continue
		}
		switch k {
		case "name":
			data.Name = v.(string)
		case "identifier":
			data.Identifier = v.(string)
		case "description":
			data.Description = v.(string)
		case "monthly_price":
			data.MonthlyPrice = v
		case "yearly_price":
			data.YearlyPrice = v
		case "monthly_quota_tokens":
			data.MonthlyQuotaTokens = v
		case "allowed_models":
			if models, ok := v.([]string); ok {
				data.AllowedModels = models
			} else if models, ok := v.([]interface{}); ok {
				result := make([]string, len(models))
				for i, m := range models {
					result[i] = m.(string)
				}
				data.AllowedModels = result
			}
		case "is_recommended":
			data.IsRecommended = v
		case "sort_order":
			data.SortOrder = v
		case "status":
			data.Status = v.(string)
		}
	}

	result, err := dao.PlnPlans.Ctx(ctx).
		Where("id", req.Id).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, common.NewNotFoundError("套餐")
	}
	return &v1.PlanUpdateRes{}, nil
}

// ArchivePlan 下架套餐（软删除）
func (s *sAdmin) ArchivePlan(ctx context.Context, req *v1.PlanArchiveReq) (*v1.PlanArchiveRes, error) {
	result, err := dao.PlnPlans.Ctx(ctx).
		Where("id", req.Id).
		Where("status", "active").
		Data(do.PlnPlans{
			Status: "archived",
		}).Update()
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, common.NewNotFoundError("plan")
	}
	return &v1.PlanArchiveRes{}, nil
}

// ToggleRecommend 切换推荐标记
func (s *sAdmin) ToggleRecommend(ctx context.Context, req *v1.PlanToggleRecommendReq) (*v1.PlanToggleRecommendRes, error) {
	val, err := dao.PlnPlans.Ctx(ctx).
		Where("id", req.Id).
		Fields("is_recommended").
		Value()
	if err != nil {
		return nil, err
	}
	isRecommended := val.Bool()

	_, err = dao.PlnPlans.Ctx(ctx).
		Where("id", req.Id).
		Data(do.PlnPlans{
			IsRecommended: !isRecommended,
		}).Update()
	if err != nil {
		return nil, err
	}
	return &v1.PlanToggleRecommendRes{}, nil
}

// ExportPlans exports plan list to CSV or Excel.
func (s *sAdmin) ExportPlans(ctx context.Context, req *v1.PlanExportReq) (*v1.PlanExportRes, error) {
	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "name", Header: "名称"},
		{Field: "identifier", Header: "标识"},
		{Field: "monthly_price", Header: "月价"},
		{Field: "yearly_price", Header: "年价"},
		{Field: "status", Header: "状态"},
		{Field: "monthly_quota_tokens", Header: "月配额Token"},
		{Field: "is_recommended", Header: "推荐标记"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "套餐_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	planFields := "id, name, identifier, monthly_price, yearly_price, status, monthly_quota_tokens, is_recommended, created_at"

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			query := dao.PlnPlans.Ctx(ctx)
			if req.Status != "" {
				query = query.Where("status", req.Status)
			}
			var batch []struct {
				Id                 int64       `json:"id"`
				Name               string      `json:"name"`
				Identifier         string      `json:"identifier"`
				MonthlyPrice       float64     `json:"monthly_price"`
				YearlyPrice        float64     `json:"yearly_price"`
				Status             string      `json:"status"`
				MonthlyQuotaTokens int64       `json:"monthly_quota_tokens"`
				IsRecommended      bool        `json:"is_recommended"`
				CreatedAt          *gtime.Time `json:"created_at"`
			}
			if err := query.Fields(planFields).OrderAsc("sort_order").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				return
			}
			for _, p := range batch {
				isRec := "否"
				if p.IsRecommended {
					isRec = "是"
				}
				if !yield(map[string]any{
					"id":                   p.Id,
					"name":                 p.Name,
					"identifier":           p.Identifier,
					"monthly_price":        p.MonthlyPrice,
					"yearly_price":         p.YearlyPrice,
					"status":               p.Status,
					"monthly_quota_tokens": p.MonthlyQuotaTokens,
					"is_recommended":       isRec,
					"created_at":           p.CreatedAt.String(),
				}) {
					return
				}
			}
			if len(batch) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

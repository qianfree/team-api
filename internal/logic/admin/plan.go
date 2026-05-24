package admin

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
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

	for _, item := range plans {
		if item.AllowedModels == nil {
			item.AllowedModels = []string{}
		}
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
	var plan v1.PlanItem
	err := dao.PlnPlans.Ctx(ctx).
		Where("id", req.Id).
		Scan(&plan)
	if err != nil {
		return nil, err
	}
	if plan.Id == 0 {
		return nil, common.NewNotFoundError("plan")
	}

	if plan.AllowedModels == nil {
		plan.AllowedModels = []string{}
	}

	return &v1.PlanDetailRes{Data: &plan}, nil
}

// CreatePlan 创建套餐
func (s *sAdmin) CreatePlan(ctx context.Context, req *v1.PlanCreateReq) (*v1.PlanCreateRes, error) {
	result, err := dao.PlnPlans.Ctx(ctx).Data(g.Map{
		"name":                  req.Name,
		"identifier":            req.Identifier,
		"description":           req.Description,
		"price":                 req.Price,
		"credit_amount":         req.CreditAmount,
		"bonus_amount":          req.BonusAmount,
		"validity_days":         req.ValidityDays,
		"allowed_models":        req.AllowedModels,
		"purchase_limit":        req.PurchaseLimit,
		"purchase_limit_period": req.PurchaseLimitPeriod,
		"stock":                 req.Stock,
		"is_recommended":        req.IsRecommended,
		"sort_order":            req.SortOrder,
	}).Insert()
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
	result, err := dao.PlnPlans.Ctx(ctx).
		Where("id", req.Id).
		Data(req.Update).
		Update()
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, common.NewNotFoundError("plan")
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
	var row struct {
		IsRecommended bool `orm:"is_recommended"`
	}
	err := dao.PlnPlans.Ctx(ctx).
		Where("id", req.Id).
		Fields("is_recommended").
		Scan(&row)
	if err != nil {
		return nil, err
	}

	_, err = dao.PlnPlans.Ctx(ctx).
		Where("id", req.Id).
		Data(do.PlnPlans{
			IsRecommended: !row.IsRecommended,
		}).Update()
	if err != nil {
		return nil, err
	}
	return &v1.PlanToggleRecommendRes{}, nil
}

// ExportPlans exports plan list to CSV or Excel.
func (s *sAdmin) ExportPlans(ctx context.Context, req *v1.PlanExportReq) (*v1.PlanExportRes, error) {
	r := g.RequestFromCtx(ctx)
	format := req.Format
	if format == "" {
		format = "csv"
	}
	if format == "excel" {
		format = "xlsx"
	}

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "name", Header: "名称"},
		{Field: "identifier", Header: "标识"},
		{Field: "price", Header: "价格(CNY)"},
		{Field: "status", Header: "状态"},
		{Field: "credit_amount", Header: "额度"},
		{Field: "bonus_amount", Header: "赠送额度"},
		{Field: "validity_days", Header: "有效天数"},
		{Field: "is_recommended", Header: "推荐标记"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   format,
		Filename: "套餐_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	planFields := "id, name, identifier, price, status, credit_amount, bonus_amount, validity_days, is_recommended, created_at"

	if format == "xlsx" {
		query := dao.PlnPlans.Ctx(ctx)
		if req.Status != "" {
			query = query.Where("status", req.Status)
		}
		var plans []struct {
			Id            int64       `json:"id"`
			Name          string      `json:"name"`
			Identifier    string      `json:"identifier"`
			Price         float64     `json:"price"`
			Status        string      `json:"status"`
			CreditAmount  float64     `json:"credit_amount"`
			BonusAmount   float64     `json:"bonus_amount"`
			ValidityDays  int         `json:"validity_days"`
			IsRecommended bool        `json:"is_recommended"`
			CreatedAt     *gtime.Time `json:"created_at"`
		}
		if err := query.Fields(planFields).OrderAsc("sort_order").Scan(&plans); err != nil {
			return nil, err
		}
		data := make([]map[string]any, len(plans))
		for i, p := range plans {
			isRec := "否"
			if p.IsRecommended {
				isRec = "是"
			}
			data[i] = map[string]any{
				"id":             p.Id,
				"name":           p.Name,
				"identifier":     p.Identifier,
				"price":          p.Price,
				"status":         p.Status,
				"credit_amount":  p.CreditAmount,
				"bonus_amount":   p.BonusAmount,
				"validity_days":  p.ValidityDays,
				"is_recommended": isRec,
				"created_at":     p.CreatedAt.String(),
			}
		}
		return nil, export.WriteExcel(r, config, data)
	}

	return nil, export.StreamCSV(r, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			query := dao.PlnPlans.Ctx(ctx)
			if req.Status != "" {
				query = query.Where("status", req.Status)
			}
			var batch []struct {
				Id            int64       `json:"id"`
				Name          string      `json:"name"`
				Identifier    string      `json:"identifier"`
				Price         float64     `json:"price"`
				Status        string      `json:"status"`
				CreditAmount  float64     `json:"credit_amount"`
				BonusAmount   float64     `json:"bonus_amount"`
				ValidityDays  int         `json:"validity_days"`
				IsRecommended bool        `json:"is_recommended"`
				CreatedAt     *gtime.Time `json:"created_at"`
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
					"id":             p.Id,
					"name":           p.Name,
					"identifier":     p.Identifier,
					"price":          p.Price,
					"status":         p.Status,
					"credit_amount":  p.CreditAmount,
					"bonus_amount":   p.BonusAmount,
					"validity_days":  p.ValidityDays,
					"is_recommended": isRec,
					"created_at":     p.CreatedAt.String(),
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

package admin

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/grand"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/utility/export"
)

// ListPromoCodes 获取优惠码列表
func (s *sAdmin) ListPromoCodes(ctx context.Context, req *v1.PromoCodeListReq) (*v1.PromoCodeListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var total int
	items := make([]*v1.PromoCodeItem, 0)
	err := dao.OrdPromoCodes.Ctx(ctx).
		OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&items, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.PromoCodeListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// validatePromoDiscountBound percentage 类型折扣值边界校验：必须 ∈ (0,100]。
// 超过 100 时折扣会超过订单金额产生负数实付（fixed 类型由消费侧封顶到订单金额保证不为负）。
func validatePromoDiscountBound(typ string, value float64) error {
	if typ == "percentage" && (value <= 0 || value > 100) {
		return common.NewBusinessError(422, "百分比类型的折扣值必须在 (0, 100] 之间")
	}
	return nil
}

// CreatePromoCode 创建优惠码
func (s *sAdmin) CreatePromoCode(ctx context.Context, req *v1.PromoCodeCreateReq) (*v1.PromoCodeCreateRes, error) {
	if err := validatePromoDiscountBound(req.Type, req.DiscountValue); err != nil {
		return nil, err
	}
	// code 留空时自动生成 12 位随机码（与前端「留空自动生成」提示对应）
	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = strings.ToUpper(grand.S(12))
	}
	data := do.OrdPromoCodes{
		Code:          code,
		Name:          req.Name,
		Type:          req.Type,
		DiscountValue: req.DiscountValue,
		MinAmount:     req.MinAmount,
		MaxDiscount:   req.MaxDiscount,
		TotalCount:    req.TotalCount,
		PerUserLimit:  req.PerUserLimit,
		Status:        req.Status,
		ValidFrom:     req.ValidFrom,
		ValidTo:       req.ValidTo,
		PlanIds:       req.PlanIds,
	}

	result, err := dao.OrdPromoCodes.Ctx(ctx).Insert(data)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &v1.PromoCodeCreateRes{ID: id}, nil
}

// UpdatePromoCode 更新优惠码
func (s *sAdmin) UpdatePromoCode(ctx context.Context, req *v1.PromoCodeUpdateReq) (*v1.PromoCodeUpdateRes, error) {
	// 读取现值：存在性校验与折扣值边界校验共用一次查询
	var existing *struct {
		Type          string  `json:"type"`
		DiscountValue float64 `json:"discount_value"`
	}
	err := dao.OrdPromoCodes.Ctx(ctx).
		Where("id", req.Id).
		Fields("type", "discount_value").
		Scan(&existing)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, common.NewNotFoundError("优惠码")
	}

	// percentage 折扣值边界（创建侧同款）：类型/折扣值可能只传其一，
	// 与库中现值合成生效值后校验，防止「fixed 200 仅改 type 为 percentage」这类绕过边界的更新
	if req.Type != nil || req.DiscountValue != nil {
		typ, value := existing.Type, existing.DiscountValue
		if req.Type != nil {
			typ = *req.Type
		}
		if req.DiscountValue != nil {
			value = *req.DiscountValue
		}
		if err := validatePromoDiscountBound(typ, value); err != nil {
			return nil, err
		}
	}

	// 指针字段非 nil 才更新（留空不更新），code 创建后不可改，
	// 可更新字段即 Req 结构体定义，天然防越权注入
	data := do.OrdPromoCodes{}
	if req.Name != nil {
		data.Name = *req.Name
	}
	if req.Type != nil {
		data.Type = *req.Type
	}
	if req.DiscountValue != nil {
		data.DiscountValue = *req.DiscountValue
	}
	if req.MinAmount != nil {
		data.MinAmount = *req.MinAmount
	}
	if req.MaxDiscount != nil {
		data.MaxDiscount = *req.MaxDiscount
	}
	if req.TotalCount != nil {
		data.TotalCount = *req.TotalCount
	}
	if req.PerUserLimit != nil {
		data.PerUserLimit = *req.PerUserLimit
	}
	if req.Status != nil {
		data.Status = *req.Status
	}
	if req.ValidFrom != nil {
		data.ValidFrom = req.ValidFrom
	}
	if req.ValidTo != nil {
		data.ValidTo = req.ValidTo
	}
	if len(req.PlanIds) > 0 {
		data.PlanIds = req.PlanIds
	}

	_, err = dao.OrdPromoCodes.Ctx(ctx).
		Where("id", req.Id).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}
	return &v1.PromoCodeUpdateRes{}, nil
}

// GetPromoCodeUsages 获取优惠码使用记录
func (s *sAdmin) GetPromoCodeUsages(ctx context.Context, req *v1.PromoCodeUsagesReq) (*v1.PromoCodeUsagesRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var total int
	items := make([]*v1.PromoCodeUsageItem, 0)
	err := dao.OrdPromoCodeUsages.Ctx(ctx).
		Where("promo_code_id", req.Id).
		OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&items, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.PromoCodeUsagesRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ExportPromoCodes exports promo code list to CSV or Excel.
func (s *sAdmin) ExportPromoCodes(ctx context.Context, req *v1.PromoCodeExportReq) (*v1.PromoCodeExportRes, error) {
	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "code", Header: "优惠码"},
		{Field: "name", Header: "名称"},
		{Field: "type", Header: "类型"},
		{Field: "discount_value", Header: "折扣值"},
		{Field: "min_amount", Header: "最低金额"},
		{Field: "total_count", Header: "总数"},
		{Field: "used_count", Header: "已用数"},
		{Field: "status", Header: "状态"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "优惠码_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	promoFields := "id, code, name, type, discount_value, min_amount, total_count, used_count, status, created_at"

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			var batch []struct {
				Id            int64       `json:"id"`
				Code          string      `json:"code"`
				Name          string      `json:"name"`
				Type          string      `json:"type"`
				DiscountValue float64     `json:"discount_value"`
				MinAmount     float64     `json:"min_amount"`
				TotalCount    int         `json:"total_count"`
				UsedCount     int         `json:"used_count"`
				Status        string      `json:"status"`
				CreatedAt     *gtime.Time `json:"created_at"`
			}
			if err := dao.OrdPromoCodes.Ctx(ctx).Fields(promoFields).OrderDesc("created_at").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				return
			}
			for _, item := range batch {
				if !yield(map[string]any{
					"id":             item.Id,
					"code":           item.Code,
					"name":           item.Name,
					"type":           item.Type,
					"discount_value": item.DiscountValue,
					"min_amount":     item.MinAmount,
					"total_count":    item.TotalCount,
					"used_count":     item.UsedCount,
					"status":         item.Status,
					"created_at":     item.CreatedAt.String(),
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

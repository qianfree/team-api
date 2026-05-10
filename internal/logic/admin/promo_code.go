package admin

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

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

// CreatePromoCode 创建优惠码
func (s *sAdmin) CreatePromoCode(ctx context.Context, req *v1.PromoCodeCreateReq) (*v1.PromoCodeCreateRes, error) {
	result, err := dao.OrdPromoCodes.Ctx(ctx).Insert(req.Data)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &v1.PromoCodeCreateRes{ID: id}, nil
}

// UpdatePromoCode 更新优惠码
func (s *sAdmin) UpdatePromoCode(ctx context.Context, req *v1.PromoCodeUpdateReq) (*v1.PromoCodeUpdateRes, error) {
	_, err := dao.OrdPromoCodes.Ctx(ctx).
		Where("id", req.Id).
		Data(req.Update).
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
		Format:   format,
		Filename: "优惠码_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	promoFields := "id, code, name, type, discount_value, min_amount, total_count, used_count, status, created_at"

	if format == "xlsx" {
		var items []struct {
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
		if err := dao.OrdPromoCodes.Ctx(ctx).Fields(promoFields).OrderDesc("created_at").Scan(&items); err != nil {
			return nil, err
		}
		data := make([]map[string]any, len(items))
		for i, item := range items {
			data[i] = map[string]any{
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
			}
		}
		return nil, export.WriteExcel(r, config, data)
	}

	return nil, export.StreamCSV(r, config, func(yield func(map[string]any) bool) {
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

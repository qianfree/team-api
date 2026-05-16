package admin

import (
	"context"
	"encoding/json"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

// ListTenantModels 列出租户已分配的模型
func (s *sAdmin) ListTenantModels(ctx context.Context, req *v1.TenantModelListReq) (*v1.TenantModelListRes, error) {
	var results []struct {
		ID                       int64    `json:"id"`
		TenantID                 int64    `json:"tenant_id"`
		ModelID                  int64    `json:"model_id"`
		Enabled                  bool     `json:"enabled"`
		BillingMode              *string  `json:"billing_mode"`
		PerRequestPrice          *float64 `json:"per_request_price"`
		DiscountRatio            *float64 `json:"discount_ratio"`
		MaxConcurrency           *int     `json:"max_concurrency"`
		ChannelScope             string   `json:"channel_scope"`
		CustomInputPrice         *float64 `json:"custom_input_price"`
		CustomOutputPrice        *float64 `json:"custom_output_price"`
		CustomCacheReadPrice     *float64 `json:"custom_cache_read_price"`
		CustomCacheCreationPrice *float64 `json:"custom_cache_creation_price"`
		CustomPricingTiers       string   `json:"custom_pricing_tiers"`
		Multiplier               float64  `json:"multiplier"`
		ModelCode                string   `json:"model_code"`
		ModelName                string   `json:"model_name"`
		Category                 string   `json:"category"`
	}

	err := dao.MdlTenantModels.Ctx(ctx).
		LeftJoin("mdl_models ON mdl_tenant_models.model_id = mdl_models.id").
		Where("mdl_tenant_models.tenant_id", req.TenantID).
		Fields("mdl_tenant_models.id, mdl_tenant_models.tenant_id, mdl_tenant_models.model_id, mdl_tenant_models.enabled, mdl_tenant_models.billing_mode, mdl_tenant_models.per_request_price, mdl_tenant_models.discount_ratio, mdl_tenant_models.max_concurrency, mdl_tenant_models.channel_scope, mdl_tenant_models.custom_input_price, mdl_tenant_models.custom_output_price, mdl_tenant_models.custom_cache_read_price, mdl_tenant_models.custom_cache_creation_price, mdl_tenant_models.custom_pricing_tiers, mdl_tenant_models.multiplier, mdl_models.model_id as model_code, mdl_models.model_name, mdl_models.category").
		OrderAsc("mdl_models.category").
		OrderAsc("mdl_models.model_id").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	list := make([]v1.TenantModelItem, 0, len(results))
	for _, r := range results {
		item := v1.TenantModelItem{
			ID:                       r.ID,
			TenantID:                 r.TenantID,
			ModelID:                  r.ModelID,
			ModelCode:                r.ModelCode,
			ModelName:                r.ModelName,
			Category:                 r.Category,
			Enabled:                  r.Enabled,
			BillingMode:              r.BillingMode,
			PerRequestPrice:          r.PerRequestPrice,
			DiscountRatio:            r.DiscountRatio,
			MaxConcurrency:           r.MaxConcurrency,
			ChannelScope:             r.ChannelScope,
			CustomInputPrice:         r.CustomInputPrice,
			CustomOutputPrice:        r.CustomOutputPrice,
			CustomCacheReadPrice:     r.CustomCacheReadPrice,
			CustomCacheCreationPrice: r.CustomCacheCreationPrice,
			Multiplier:               r.Multiplier,
		}
		if r.CustomPricingTiers != "" && r.CustomPricingTiers != "null" && r.CustomPricingTiers != "[]" {
			var tiers []*v1.PricingTier
			if err := json.Unmarshal([]byte(r.CustomPricingTiers), &tiers); err == nil {
				item.CustomPricingTiers = tiers
			}
		}
		list = append(list, item)
	}

	return &v1.TenantModelListRes{List: list}, nil
}

// BatchAssignModels 批量分配模型给租户
func (s *sAdmin) BatchAssignModels(ctx context.Context, req *v1.TenantModelBatchAssignReq) (*v1.TenantModelBatchAssignRes, error) {
	assigned := 0

	for _, a := range req.Assignments {
		var model *struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		}
		err := dao.MdlModels.Ctx(ctx).
			Where("id", a.ModelID).
			Scan(&model)
		if err != nil || model == nil || model.Status != "active" {
			continue
		}

		count, _ := dao.MdlTenantModels.Ctx(ctx).
			Where("tenant_id", req.TenantID).
			Where("model_id", a.ModelID).
			Count()
		if count > 0 {
			continue
		}

		maxConc := a.MaxConcurrency
		if maxConc == nil {
			concurrency := 5
			maxConc = &concurrency
		}

		var tiersJSON string
		if len(a.CustomPricingTiers) > 0 {
			b, _ := json.Marshal(a.CustomPricingTiers)
			tiersJSON = string(b)
		}

		_, err = dao.MdlTenantModels.Ctx(ctx).Insert(do.MdlTenantModels{
			TenantId:                 req.TenantID,
			ModelId:                  a.ModelID,
			Enabled:                  a.Enabled,
			BillingMode:              a.BillingMode,
			PerRequestPrice:          a.PerRequestPrice,
			DiscountRatio:            a.DiscountRatio,
			MaxConcurrency:           maxConc,
			ChannelScope:             a.ChannelScope,
			CustomInputPrice:         a.CustomInputPrice,
			CustomOutputPrice:        a.CustomOutputPrice,
			CustomCacheReadPrice:     a.CustomCacheReadPrice,
			CustomCacheCreationPrice: a.CustomCacheCreationPrice,
			CustomPricingTiers:       tiersJSON,
			Multiplier:               1.0,
		})
		if err == nil {
			assigned++
		}
	}

	billing.ClearTenantPriceCache(ctx, req.TenantID)

	return &v1.TenantModelBatchAssignRes{Assigned: assigned}, nil
}

// UpdateTenantModel 更新租户模型配置
func (s *sAdmin) UpdateTenantModel(ctx context.Context, req *v1.TenantModelUpdateReq) (*v1.TenantModelUpdateRes, error) {
	data := do.MdlTenantModels{}

	if req.Enabled != nil {
		data.Enabled = *req.Enabled
	}
	if req.BillingMode != nil {
		data.BillingMode = *req.BillingMode
	}
	if req.PerRequestPrice != nil {
		data.PerRequestPrice = *req.PerRequestPrice
	}
	if req.DiscountRatio != nil {
		data.DiscountRatio = *req.DiscountRatio
	}
	if req.MaxConcurrency != nil {
		data.MaxConcurrency = *req.MaxConcurrency
	}
	if req.ChannelScope != nil {
		data.ChannelScope = *req.ChannelScope
	}
	if req.CustomInputPrice != nil {
		data.CustomInputPrice = *req.CustomInputPrice
	}
	if req.CustomCacheReadPrice != nil {
		data.CustomCacheReadPrice = *req.CustomCacheReadPrice
	}
	if req.CustomCacheCreationPrice != nil {
		data.CustomCacheCreationPrice = *req.CustomCacheCreationPrice
	}
	if req.CustomOutputPrice != nil {
		data.CustomOutputPrice = *req.CustomOutputPrice
	}
	if req.CustomPricingTiers != nil {
		if len(*req.CustomPricingTiers) > 0 {
			b, _ := json.Marshal(*req.CustomPricingTiers)
			data.CustomPricingTiers = string(b)
		} else {
			data.CustomPricingTiers = "[]"
		}
	}

	_, err := dao.MdlTenantModels.Ctx(ctx).
		Where("tenant_id", req.TenantID).
		Where("model_id", req.ModelID).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}

	billing.ClearTenantPriceCache(ctx, req.TenantID)

	return nil, nil
}

// DeleteTenantModel 移除租户模型分配
func (s *sAdmin) DeleteTenantModel(ctx context.Context, req *v1.TenantModelDeleteReq) (*v1.TenantModelDeleteRes, error) {
	result, err := dao.MdlTenantModels.Ctx(ctx).
		Where("tenant_id", req.TenantID).
		Where("model_id", req.ModelID).
		Delete()
	if err != nil {
		return nil, err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, common.NewNotFoundError("模型分配")
	}

	billing.ClearTenantPriceCache(ctx, req.TenantID)

	return nil, nil
}

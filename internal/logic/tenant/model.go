package tenant

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
)

// pricingTierRow 阶梯定价行（与 billing.pricingTierRow 结构一致）
type pricingTierRow struct {
	MinTokens   int64   `json:"min_tokens"`
	MaxTokens   *int64  `json:"max_tokens"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
}

// ListAvailableModels 获取租户可用的模型列表
func (s *sTenant) ListAvailableModels(ctx context.Context, req *v1.TenantAvailableModelsReq) (*v1.TenantAvailableModelsRes, error) {
	tenantID := ctxTenantID(ctx)

	query := dao.MdlTenantModels.Ctx(ctx).
		LeftJoin("mdl_models m ON mdl_tenant_models.model_id = m.id").
		LeftJoin("mdl_pricing p ON p.model_id = mdl_tenant_models.model_id AND p.min_tokens = 0").
		Where("mdl_tenant_models.tenant_id", tenantID).
		Where("mdl_tenant_models.enabled", true).
		Where("m.status", "active")

	if req.Category != "" {
		query = query.Where("m.category", req.Category)
	}
	if req.Search != "" {
		query = query.Where("m.model_id LIKE ? OR m.model_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}

	var results []struct {
		ID                       int64    `json:"id"`
		ModelDBID                int64    `json:"model_db_id"`
		ModelId                  string   `json:"model_id"`
		ModelName                string   `json:"model_name"`
		Category                 string   `json:"category"`
		MaxContextTokens         int      `json:"max_context_tokens"`
		MaxOutputTokens          int      `json:"max_output_tokens"`
		Description              string   `json:"description"`
		Tags                     string   `json:"tags"`
		Capabilities             string   `json:"capabilities"`
		BillingMode              *string  `json:"billing_mode"`
		PerRequestPrice          *float64 `json:"per_request_price"`
		DiscountRatio            *float64 `json:"discount_ratio"`
		MaxConcurrency           *int     `json:"max_concurrency"`
		BaseInputPrice           float64  `json:"base_input_price"`
		BaseOutputPrice          float64  `json:"base_output_price"`
		BaseCacheReadPrice       float64  `json:"base_cache_read_price"`
		BaseCacheCreationPrice   float64  `json:"base_cache_creation_price"`
		BaseBillingMode          string   `json:"base_billing_mode"`
		CustomInputPrice         *float64 `json:"custom_input_price"`
		CustomOutputPrice        *float64 `json:"custom_output_price"`
		CustomCacheReadPrice     *float64 `json:"custom_cache_read_price"`
		CustomCacheCreationPrice *float64 `json:"custom_cache_creation_price"`
		CustomPricingTiers       string   `json:"custom_pricing_tiers"`
	}

	err := query.
		Fields("mdl_tenant_models.id, mdl_tenant_models.model_id AS model_db_id, m.model_id, m.model_name, m.category, m.max_context_tokens, m.max_output_tokens, m.description, m.tags, m.capabilities, mdl_tenant_models.billing_mode, mdl_tenant_models.per_request_price, mdl_tenant_models.discount_ratio, mdl_tenant_models.max_concurrency, p.input_price AS base_input_price, p.output_price AS base_output_price, p.cache_read_price AS base_cache_read_price, p.cache_creation_price AS base_cache_creation_price, p.billing_mode AS base_billing_mode, mdl_tenant_models.custom_input_price, mdl_tenant_models.custom_output_price, mdl_tenant_models.custom_cache_read_price, mdl_tenant_models.custom_cache_creation_price, mdl_tenant_models.custom_pricing_tiers").
		OrderAsc("m.category").
		OrderAsc("m.model_id").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	// 收集需要查询 base 阶梯定价的 model 内部 ID（mdl_models.id）
	tieredModelDBIDs := make([]int64, 0)
	for _, r := range results {
		effectiveBillingMode := resolveBillingMode(r.BillingMode, r.BaseBillingMode)
		if effectiveBillingMode == "tiered" && r.CustomPricingTiers == "" {
			tieredModelDBIDs = append(tieredModelDBIDs, r.ModelDBID)
		}
	}

	// 批量查询 base 阶梯定价（min_tokens > 0 的阶梯）
	baseTiersMap := make(map[int64][]v1.PricingTierItem)
	if len(tieredModelDBIDs) > 0 {
		var baseTiers []struct {
			ModelId     int64   `json:"model_id"`
			MinTokens   int64   `json:"min_tokens"`
			MaxTokens   *int64  `json:"max_tokens"`
			InputPrice  float64 `json:"input_price"`
			OutputPrice float64 `json:"output_price"`
		}
		err = dao.MdlPricing.Ctx(ctx).
			WhereIn("model_id", tieredModelDBIDs).
			Where("billing_mode", "tiered").
			Where("min_tokens > 0").
			Fields("model_id, min_tokens, max_tokens, input_price, output_price").
			OrderAsc("model_id").
			OrderAsc("min_tokens").
			Scan(&baseTiers)
		if err == nil {
			for _, t := range baseTiers {
				baseTiersMap[t.ModelId] = append(baseTiersMap[t.ModelId], v1.PricingTierItem{
					MinTokens:   t.MinTokens,
					MaxTokens:   t.MaxTokens,
					InputPrice:  t.InputPrice,
					OutputPrice: t.OutputPrice,
				})
			}
		}
	}

	list := make([]v1.TenantAvailableModelItem, 0, len(results))
	for _, r := range results {
		effectiveBillingMode := resolveBillingMode(r.BillingMode, r.BaseBillingMode)

		inputPrice := effectivePrice(r.CustomInputPrice, r.BaseInputPrice)
		outputPrice := effectivePrice(r.CustomOutputPrice, r.BaseOutputPrice)
		cacheReadPrice := effectivePrice(r.CustomCacheReadPrice, r.BaseCacheReadPrice)
		cacheCreationPrice := effectivePrice(r.CustomCacheCreationPrice, r.BaseCacheCreationPrice)

		item := v1.TenantAvailableModelItem{
			ID:                 r.ID,
			ModelId:            r.ModelId,
			ModelName:          r.ModelName,
			Category:           r.Category,
			MaxContext:         r.MaxContextTokens,
			MaxOutput:          r.MaxOutputTokens,
			Description:        r.Description,
			Tags:               r.Tags,
			Capabilities:       r.Capabilities,
			BillingMode:        &effectiveBillingMode,
			PerRequestPrice:    r.PerRequestPrice,
			DiscountRatio:      r.DiscountRatio,
			MaxConcurrency:     r.MaxConcurrency,
			InputPrice:         inputPrice,
			OutputPrice:        outputPrice,
			CacheReadPrice:     cacheReadPrice,
			CacheCreationPrice: cacheCreationPrice,
		}

		// 阶梯定价处理
		if effectiveBillingMode == "tiered" {
			var tiers []v1.PricingTierItem

			// 优先使用租户自定义阶梯
			if r.CustomPricingTiers != "" && r.CustomPricingTiers != "null" && r.CustomPricingTiers != "[]" {
				var raw []pricingTierRow
				if json.Unmarshal([]byte(r.CustomPricingTiers), &raw) == nil && len(raw) > 0 {
					for _, t := range raw {
						tiers = append(tiers, v1.PricingTierItem{
							MinTokens:   t.MinTokens,
							MaxTokens:   t.MaxTokens,
							InputPrice:  t.InputPrice,
							OutputPrice: t.OutputPrice,
						})
					}
				}
			}

			// 使用 base 阶梯定价
			if len(tiers) == 0 {
				// 第一阶梯（min_tokens=0，来自主查询的 base_input/output_price）
				if r.BaseInputPrice > 0 || r.BaseOutputPrice > 0 {
					tiers = append(tiers, v1.PricingTierItem{
						MinTokens:   0,
						MaxTokens:   nil,
						InputPrice:  r.BaseInputPrice,
						OutputPrice: r.BaseOutputPrice,
					})
				}
				// 后续阶梯
				if rest, ok := baseTiersMap[r.ModelDBID]; ok {
					// 设置第一阶梯的 max_tokens
					if len(tiers) > 0 && len(rest) > 0 {
						tiers[0].MaxTokens = &rest[0].MinTokens
					}
					tiers = append(tiers, rest...)
				}
			}

			item.PricingTiers = tiers
		}

		list = append(list, item)
	}
	// 追加分组来源的模型（不在显式分配中的模型）
	explicitModelIDs := make(map[int64]bool, len(results))
	for _, r := range results {
		explicitModelIDs[r.ModelDBID] = true
	}

	var groupModels []struct {
		ModelDBID        int64   `json:"model_db_id"`
		ModelId          string  `json:"model_id"`
		ModelName        string  `json:"model_name"`
		Category         string  `json:"category"`
		MaxContextTokens int     `json:"max_context_tokens"`
		MaxOutputTokens  int     `json:"max_output_tokens"`
		Description      string  `json:"description"`
		Tags             string  `json:"tags"`
		Capabilities     string  `json:"capabilities"`
		BaseBillingMode  string  `json:"base_billing_mode"`
		BaseInputPrice   float64 `json:"base_input_price"`
		BaseOutputPrice  float64 `json:"base_output_price"`
	}

	groupQuery := g.DB().Model("mdl_group_models gm").Ctx(ctx).
		InnerJoin("mdl_model_groups g ON gm.group_id = g.id").
		InnerJoin("mdl_models m ON gm.model_id = m.id").
		InnerJoin("mdl_tenant_groups tg ON tg.group_id = g.id").
		LeftJoin("mdl_pricing p ON p.model_id = m.id AND p.min_tokens = 0").
		Where("tg.tenant_id", tenantID).
		Where("g.status", "active").
		Where("m.status", "active")

	if len(explicitModelIDs) > 0 {
		ids := make([]int64, 0, len(explicitModelIDs))
		for id := range explicitModelIDs {
			ids = append(ids, id)
		}
		groupQuery = groupQuery.WhereNotIn("m.id", ids)
	}

	if req.Category != "" {
		groupQuery = groupQuery.Where("m.category", req.Category)
	}
	if req.Search != "" {
		groupQuery = groupQuery.Where("m.model_id LIKE ? OR m.model_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}

	_ = groupQuery.
		Fields("DISTINCT m.id AS model_db_id, m.model_id, m.model_name, m.category, m.max_context_tokens, m.max_output_tokens, m.description, m.tags, m.capabilities, p.billing_mode AS base_billing_mode, p.input_price AS base_input_price, p.output_price AS base_output_price").
		OrderAsc("m.category").
		OrderAsc("m.model_id").
		Scan(&groupModels)

	for _, gm := range groupModels {
		billingMode := gm.BaseBillingMode
		if billingMode == "" {
			billingMode = "token"
		}

		item := v1.TenantAvailableModelItem{
			ModelId:      gm.ModelId,
			ModelName:    gm.ModelName,
			Category:     gm.Category,
			MaxContext:   gm.MaxContextTokens,
			MaxOutput:    gm.MaxOutputTokens,
			Description:  gm.Description,
			Tags:         gm.Tags,
			Capabilities: gm.Capabilities,
			BillingMode:  &billingMode,
			InputPrice:   &gm.BaseInputPrice,
			OutputPrice:  &gm.BaseOutputPrice,
		}
		list = append(list, item)
	}

	return &v1.TenantAvailableModelsRes{List: list}, nil
}

// resolveBillingMode 解析有效计费模式
func resolveBillingMode(tenantMode *string, baseMode string) string {
	if tenantMode != nil && *tenantMode != "" {
		return *tenantMode
	}
	if baseMode != "" {
		return baseMode
	}
	return "token"
}

// effectivePrice 计算有效价格：自定义价优先，否则用基础价
func effectivePrice(custom *float64, base float64) *float64 {
	if custom != nil && *custom > 0 {
		return custom
	}
	if base > 0 {
		return &base
	}
	return nil
}

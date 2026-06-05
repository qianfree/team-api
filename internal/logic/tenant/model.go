package tenant

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
)

// pricingTierRow 阶梯定价行（与 billing.pricingTierRow 结构一致）
type pricingTierRow struct {
	MinTokens   int64   `json:"min_tokens"`
	MaxTokens   *int64  `json:"max_tokens"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
}

// memberModelScopeNoAccess 表示成员无权访问任何模型的哨兵值
const memberModelScopeNoAccess = -1

// tenantModelPriceRow 显式分配模型的价格查询结果
type tenantModelPriceRow struct {
	ModelDBID                int64    `json:"model_db_id"`
	ID                       int64    `json:"id"`
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

// groupPriceRow 分组模型的 base 价格查询结果
type groupPriceRow struct {
	ModelID         int64   `json:"model_id"`
	BaseBillingMode string  `json:"base_billing_mode"`
	BaseInputPrice  float64 `json:"base_input_price"`
	BaseOutputPrice float64 `json:"base_output_price"`
}

// baseTierRow 阶梯定价查询结果
type baseTierRow struct {
	ModelId     int64   `json:"model_id"`
	MinTokens   int64   `json:"min_tokens"`
	MaxTokens   *int64  `json:"max_tokens"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
}

// memberScopeRow 成员模型范围查询结果
type memberScopeRow struct {
	ModelID   int64  `json:"model_id"`
	ModelName string `json:"model_name"`
}

// priceInfo 显式模型的完整价格信息
type priceInfo struct {
	ID                       int64
	BillingMode              *string
	PerRequestPrice          *float64
	DiscountRatio            *float64
	MaxConcurrency           *int
	BaseInputPrice           float64
	BaseOutputPrice          float64
	BaseCacheReadPrice       float64
	BaseCacheCreationPrice   float64
	BaseBillingMode          string
	CustomInputPrice         *float64
	CustomOutputPrice        *float64
	CustomCacheReadPrice     *float64
	CustomCacheCreationPrice *float64
	CustomPricingTiers       string
}

// groupPriceInfo 分组模型的价格信息
type groupPriceInfo struct {
	BaseBillingMode string
	BaseInputPrice  float64
	BaseOutputPrice float64
}

// ListAvailableModels 获取租户可用的模型列表
func (s *sTenant) ListAvailableModels(ctx context.Context, req *v1.TenantAvailableModelsReq) (*v1.TenantAvailableModelsRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	models, err := lcommon.GetTenantAvailableModels(ctx, tenantID, req.Category, req.Search)
	if err != nil {
		return nil, err
	}

	if len(models) == 0 {
		return &v1.TenantAvailableModelsRes{List: nil}, nil
	}

	// 查询显式分配模型的完整价格信息
	var priceResults []tenantModelPriceRow

	explicitDBIDs := make([]int64, 0)
	for _, m := range models {
		if m.Source == "explicit" {
			explicitDBIDs = append(explicitDBIDs, m.ModelDBID)
		}
	}

	if len(explicitDBIDs) > 0 {
		err = dao.MdlTenantModels.Ctx(ctx).
			LeftJoin("mdl_pricing p ON p.model_id = mdl_tenant_models.model_id AND p.min_tokens = 0").
			Where("mdl_tenant_models.tenant_id", tenantID).
			WhereIn("mdl_tenant_models.model_id", explicitDBIDs).
			Fields("mdl_tenant_models.model_id AS model_db_id, mdl_tenant_models.id, mdl_tenant_models.billing_mode, mdl_tenant_models.per_request_price, mdl_tenant_models.discount_ratio, mdl_tenant_models.max_concurrency, p.input_price AS base_input_price, p.output_price AS base_output_price, p.cache_read_price AS base_cache_read_price, p.cache_creation_price AS base_cache_creation_price, p.billing_mode AS base_billing_mode, mdl_tenant_models.custom_input_price, mdl_tenant_models.custom_output_price, mdl_tenant_models.custom_cache_read_price, mdl_tenant_models.custom_cache_creation_price, mdl_tenant_models.custom_pricing_tiers").
			Scan(&priceResults)
		if err != nil {
			return nil, err
		}
	}

	// 构建显式模型价格映射
	priceMap := make(map[int64]*priceInfo, len(priceResults))
	for _, r := range priceResults {
		priceMap[r.ModelDBID] = &priceInfo{
			ID:                       r.ID,
			BillingMode:              r.BillingMode,
			PerRequestPrice:          r.PerRequestPrice,
			DiscountRatio:            r.DiscountRatio,
			MaxConcurrency:           r.MaxConcurrency,
			BaseInputPrice:           r.BaseInputPrice,
			BaseOutputPrice:          r.BaseOutputPrice,
			BaseCacheReadPrice:       r.BaseCacheReadPrice,
			BaseCacheCreationPrice:   r.BaseCacheCreationPrice,
			BaseBillingMode:          r.BaseBillingMode,
			CustomInputPrice:         r.CustomInputPrice,
			CustomOutputPrice:        r.CustomOutputPrice,
			CustomCacheReadPrice:     r.CustomCacheReadPrice,
			CustomCacheCreationPrice: r.CustomCacheCreationPrice,
			CustomPricingTiers:       r.CustomPricingTiers,
		}
	}

	// 批量查询分组模型的 base 价格
	groupDBIDs := make([]int64, 0)
	for _, m := range models {
		if m.Source == "group" {
			groupDBIDs = append(groupDBIDs, m.ModelDBID)
		}
	}

	groupPriceMap := make(map[int64]*groupPriceInfo)
	if len(groupDBIDs) > 0 {
		var groupPrices []groupPriceRow
		err = dao.MdlPricing.Ctx(ctx).
			WhereIn("model_id", groupDBIDs).
			Where("min_tokens", 0).
			Fields("model_id, billing_mode AS base_billing_mode, input_price AS base_input_price, output_price AS base_output_price").
			Scan(&groupPrices)
		if err != nil {
			return nil, err
		}

		for _, gp := range groupPrices {
			groupPriceMap[gp.ModelID] = &groupPriceInfo{
				BaseBillingMode: gp.BaseBillingMode,
				BaseInputPrice:  gp.BaseInputPrice,
				BaseOutputPrice: gp.BaseOutputPrice,
			}
		}
	}

	// 收集需要查询阶梯定价的模型
	tieredModelDBIDs := make([]int64, 0)
	for _, m := range models {
		if m.Source == "explicit" {
			if pi, ok := priceMap[m.ModelDBID]; ok {
				effectiveBillingMode := resolveBillingMode(pi.BillingMode, pi.BaseBillingMode)
				if effectiveBillingMode == "tiered" && pi.CustomPricingTiers == "" {
					tieredModelDBIDs = append(tieredModelDBIDs, m.ModelDBID)
				}
			}
		}
	}

	baseTiersMap := make(map[int64][]v1.PricingTierItem)
	if len(tieredModelDBIDs) > 0 {
		var baseTiers []baseTierRow
		err = dao.MdlPricing.Ctx(ctx).
			WhereIn("model_id", tieredModelDBIDs).
			Where("billing_mode", "tiered").
			Where("min_tokens > 0").
			Fields("model_id, min_tokens, max_tokens, input_price, output_price").
			OrderAsc("model_id").
			OrderAsc("min_tokens").
			Scan(&baseTiers)
		if err != nil {
			return nil, err
		}

		for _, t := range baseTiers {
			baseTiersMap[t.ModelId] = append(baseTiersMap[t.ModelId], v1.PricingTierItem{
				MinTokens:   t.MinTokens,
				MaxTokens:   t.MaxTokens,
				InputPrice:  t.InputPrice,
				OutputPrice: t.OutputPrice,
			})
		}
	}

	// 组装最终列表
	list := make([]v1.TenantAvailableModelItem, 0, len(models))
	for _, m := range models {
		if m.Source == "explicit" {
			pi, ok := priceMap[m.ModelDBID]
			if !ok {
				continue
			}

			effectiveBillingMode := resolveBillingMode(pi.BillingMode, pi.BaseBillingMode)
			inputPrice := effectivePrice(pi.CustomInputPrice, pi.BaseInputPrice)
			outputPrice := effectivePrice(pi.CustomOutputPrice, pi.BaseOutputPrice)
			cacheReadPrice := effectivePrice(pi.CustomCacheReadPrice, pi.BaseCacheReadPrice)
			cacheCreationPrice := effectivePrice(pi.CustomCacheCreationPrice, pi.BaseCacheCreationPrice)

			item := v1.TenantAvailableModelItem{
				ID:                 m.ModelDBID,
				ModelId:            m.ModelId,
				ModelName:          m.ModelName,
				Category:           m.Category,
				MaxContext:         m.MaxContextTokens,
				MaxOutput:          m.MaxOutputTokens,
				Description:        m.Description,
				Tags:               m.Tags,
				Capabilities:       m.Capabilities,
				BillingMode:        &effectiveBillingMode,
				PerRequestPrice:    pi.PerRequestPrice,
				DiscountRatio:      pi.DiscountRatio,
				MaxConcurrency:     pi.MaxConcurrency,
				InputPrice:         inputPrice,
				OutputPrice:        outputPrice,
				CacheReadPrice:     cacheReadPrice,
				CacheCreationPrice: cacheCreationPrice,
			}

			if effectiveBillingMode == "tiered" {
				var tiers []v1.PricingTierItem
				if pi.CustomPricingTiers != "" && pi.CustomPricingTiers != "null" && pi.CustomPricingTiers != "[]" {
					var raw []pricingTierRow
					if json.Unmarshal([]byte(pi.CustomPricingTiers), &raw) == nil && len(raw) > 0 {
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
				if len(tiers) == 0 {
					if pi.BaseInputPrice > 0 || pi.BaseOutputPrice > 0 {
						tiers = append(tiers, v1.PricingTierItem{
							MinTokens:   0,
							MaxTokens:   nil,
							InputPrice:  pi.BaseInputPrice,
							OutputPrice: pi.BaseOutputPrice,
						})
					}
					if rest, ok := baseTiersMap[m.ModelDBID]; ok {
						if len(tiers) > 0 && len(rest) > 0 {
							tiers[0].MaxTokens = &rest[0].MinTokens
						}
						tiers = append(tiers, rest...)
					}
				}
				item.PricingTiers = tiers
			}

			list = append(list, item)
		} else {
			// group 来源的模型：使用 base 价格
			billingMode := "token"
			var inputPrice, outputPrice *float64
			if gp, ok := groupPriceMap[m.ModelDBID]; ok {
				billingMode = gp.BaseBillingMode
				if billingMode == "" {
					billingMode = "token"
				}
				inputPrice = effectivePrice(nil, gp.BaseInputPrice)
				outputPrice = effectivePrice(nil, gp.BaseOutputPrice)
			}

			item := v1.TenantAvailableModelItem{
				ID:           m.ModelDBID,
				ModelId:      m.ModelId,
				ModelName:    m.ModelName,
				Category:     m.Category,
				MaxContext:   m.MaxContextTokens,
				MaxOutput:    m.MaxOutputTokens,
				Description:  m.Description,
				Tags:         m.Tags,
				Capabilities: m.Capabilities,
				BillingMode:  &billingMode,
				InputPrice:   inputPrice,
				OutputPrice:  outputPrice,
			}
			list = append(list, item)
		}
	}

	// 按成员模型范围过滤
	userID := middleware.GetUserID(ctx)
	if userID > 0 {
		var memberScopes []memberScopeRow
		err = g.DB().Model("tnt_member_model_scopes ms").Ctx(ctx).
			LeftJoin("mdl_models m ON ms.model_id = m.id").
			Where("ms.tenant_id", tenantID).
			Where("ms.user_id", userID).
			Fields("ms.model_id, m.model_id as model_name").
			Scan(&memberScopes)
		if err != nil {
			return nil, err
		}

		if len(memberScopes) > 0 {
			// 检查是否存在"无权访问任何模型"的哨兵值
			hasNoAccess := false
			for _, s := range memberScopes {
				if s.ModelID == memberModelScopeNoAccess {
					hasNoAccess = true
					break
				}
			}
			if hasNoAccess {
				return &v1.TenantAvailableModelsRes{List: nil}, nil
			}

			allowed := make(map[string]bool, len(memberScopes))
			for _, s := range memberScopes {
				if s.ModelName != "" {
					allowed[s.ModelName] = true
				}
			}
			// allowed 为空说明没有有效的范围约束，不进行过滤，返回租户全部可用模型
			if len(allowed) > 0 {
				filtered := make([]v1.TenantAvailableModelItem, 0, len(list))
				for _, item := range list {
					if allowed[item.ModelId] {
						filtered = append(filtered, item)
					}
				}
				list = filtered
			}
		}
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

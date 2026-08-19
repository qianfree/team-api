package tenant

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
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
	BasePerRequestPrice      *float64 `json:"base_per_request_price"`
	CustomInputPrice         *float64 `json:"custom_input_price"`
	CustomOutputPrice        *float64 `json:"custom_output_price"`
	CustomCacheReadPrice     *float64 `json:"custom_cache_read_price"`
	CustomCacheCreationPrice *float64 `json:"custom_cache_creation_price"`
	CustomPricingTiers       string   `json:"custom_pricing_tiers"`
}

// groupPriceRow 分组模型的 base 价格查询结果
type groupPriceRow struct {
	ModelID                int64    `json:"model_id"`
	BaseBillingMode        string   `json:"base_billing_mode"`
	BaseInputPrice         float64  `json:"base_input_price"`
	BaseOutputPrice        float64  `json:"base_output_price"`
	BaseCacheReadPrice     float64  `json:"base_cache_read_price"`
	BaseCacheCreationPrice float64  `json:"base_cache_creation_price"`
	BasePerRequestPrice    *float64 `json:"base_per_request_price"`
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
	BasePerRequestPrice      *float64
	CustomInputPrice         *float64
	CustomOutputPrice        *float64
	CustomCacheReadPrice     *float64
	CustomCacheCreationPrice *float64
	CustomPricingTiers       string
}

// groupPriceInfo 分组模型的价格信息
type groupPriceInfo struct {
	BaseBillingMode        string
	BaseInputPrice         float64
	BaseOutputPrice        float64
	BaseCacheReadPrice     float64
	BaseCacheCreationPrice float64
	BasePerRequestPrice    *float64
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
			Fields("mdl_tenant_models.model_id AS model_db_id, mdl_tenant_models.id, mdl_tenant_models.billing_mode, mdl_tenant_models.per_request_price, mdl_tenant_models.discount_ratio, mdl_tenant_models.max_concurrency, p.input_price AS base_input_price, p.output_price AS base_output_price, p.cache_read_price AS base_cache_read_price, p.cache_creation_price AS base_cache_creation_price, p.billing_mode AS base_billing_mode, p.per_request_price AS base_per_request_price, mdl_tenant_models.custom_input_price, mdl_tenant_models.custom_output_price, mdl_tenant_models.custom_cache_read_price, mdl_tenant_models.custom_cache_creation_price, mdl_tenant_models.custom_pricing_tiers").
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
			BasePerRequestPrice:      r.BasePerRequestPrice,
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
			Fields("model_id, billing_mode AS base_billing_mode, input_price AS base_input_price, output_price AS base_output_price, cache_read_price AS base_cache_read_price, cache_creation_price AS base_cache_creation_price, per_request_price AS base_per_request_price").
			Scan(&groupPrices)
		if err != nil {
			return nil, err
		}

		for _, gp := range groupPrices {
			groupPriceMap[gp.ModelID] = &groupPriceInfo{
				BaseBillingMode:        gp.BaseBillingMode,
				BaseInputPrice:         gp.BaseInputPrice,
				BaseOutputPrice:        gp.BaseOutputPrice,
				BaseCacheReadPrice:     gp.BaseCacheReadPrice,
				BaseCacheCreationPrice: gp.BaseCacheCreationPrice,
				BasePerRequestPrice:    gp.BasePerRequestPrice,
			}
		}
	}

	// 收集需要查询阶梯定价的模型：显式模型（无自定义阶梯）与分组模型（走 base 阶梯）都纳入
	tieredModelDBIDs := make([]int64, 0)
	for _, m := range models {
		if m.Source == "explicit" {
			if pi, ok := priceMap[m.ModelDBID]; ok {
				effectiveBillingMode := resolveBillingMode(pi.BillingMode, pi.BaseBillingMode)
				if effectiveBillingMode == "tiered" && pi.CustomPricingTiers == "" {
					tieredModelDBIDs = append(tieredModelDBIDs, m.ModelDBID)
				}
			}
		} else {
			// 分组模型使用 base 定价，tiered 时同样需要返回阶梯明细
			if gp, ok := groupPriceMap[m.ModelDBID]; ok && gp.BaseBillingMode == "tiered" {
				tieredModelDBIDs = append(tieredModelDBIDs, m.ModelDBID)
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

	// 时段定价批量查询（锚点行 JSONB；显式与分组来源模型统一处理，展示换算价目用）
	allDBIDs := make([]int64, 0, len(models))
	for _, m := range models {
		allDBIDs = append(allDBIDs, m.ModelDBID)
	}
	timeSegmentsMap := make(map[int64][]billing.TimeSegment)
	if len(allDBIDs) > 0 {
		var segRows []struct {
			ModelId      int64  `json:"model_id"`
			TimeSegments string `json:"time_segments"`
		}
		if err := dao.MdlPricing.Ctx(ctx).
			WhereIn("model_id", allDBIDs).
			Where("min_tokens", 0).
			WhereNotNull("time_segments").
			Fields("model_id, time_segments").
			Scan(&segRows); err != nil {
			return nil, err
		}
		for _, r := range segRows {
			if r.TimeSegments == "" || r.TimeSegments == "null" {
				continue
			}
			var segs []billing.TimeSegment
			if err := json.Unmarshal([]byte(r.TimeSegments), &segs); err == nil && len(segs) > 0 {
				timeSegmentsMap[r.ModelId] = segs
			}
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
			// 按次计费：租户未设置覆盖单价时回退 base 单价
			perRequestPrice := pi.PerRequestPrice
			if effectiveBillingMode == "per_request" && (perRequestPrice == nil || *perRequestPrice <= 0) {
				perRequestPrice = pi.BasePerRequestPrice
			}
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
				PerRequestPrice:    perRequestPrice,
				DiscountRatio:      pi.DiscountRatio,
				MaxConcurrency:     pi.MaxConcurrency,
				InputPrice:         inputPrice,
				OutputPrice:        outputPrice,
				CacheReadPrice:     cacheReadPrice,
				CacheCreationPrice: cacheCreationPrice,
			}

			if effectiveBillingMode == "tiered" {
				item.PricingTiers = buildTiers(pi.CustomPricingTiers, pi.BaseInputPrice, pi.BaseOutputPrice, baseTiersMap[m.ModelDBID])
			}

			item.TimePrices = buildTimePrices(timeSegmentsMap[m.ModelDBID], effectiveBillingMode,
				inputPrice, outputPrice, perRequestPrice, item.PricingTiers)

			list = append(list, item)
		} else {
			// group 来源的模型：使用 base 定价，与显式模型展示一致（含按次单价 / 缓存价 / 阶梯明细）
			billingMode := "token"
			var inputPrice, outputPrice, cacheReadPrice, cacheCreationPrice, perRequestPrice *float64
			var baseInputPrice, baseOutputPrice float64
			gp, ok := groupPriceMap[m.ModelDBID]
			if ok {
				billingMode = gp.BaseBillingMode
				if billingMode == "" {
					billingMode = "token"
				}
				baseInputPrice = gp.BaseInputPrice
				baseOutputPrice = gp.BaseOutputPrice
				inputPrice = effectivePrice(nil, baseInputPrice)
				outputPrice = effectivePrice(nil, baseOutputPrice)
				cacheReadPrice = effectivePrice(nil, gp.BaseCacheReadPrice)
				cacheCreationPrice = effectivePrice(nil, gp.BaseCacheCreationPrice)
				perRequestPrice = gp.BasePerRequestPrice
			}

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
				BillingMode:        &billingMode,
				PerRequestPrice:    perRequestPrice,
				InputPrice:         inputPrice,
				OutputPrice:        outputPrice,
				CacheReadPrice:     cacheReadPrice,
				CacheCreationPrice: cacheCreationPrice,
			}
			if billingMode == "tiered" && ok {
				item.PricingTiers = buildTiers("", baseInputPrice, baseOutputPrice, baseTiersMap[m.ModelDBID])
			}
			item.TimePrices = buildTimePrices(timeSegmentsMap[m.ModelDBID], billingMode,
				inputPrice, outputPrice, perRequestPrice, item.PricingTiers)
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

	// 按 API Key 模型范围过滤（与 relay /v1/models 列表口径一致：Key 配置了范围才过滤，无记录不限）。
	// 通过 JOIN api_keys 校验 Key 归属：仅当 Key 属于当前租户时才应用其模型范围，
	// 避免传入他人 Key ID 时泄露其模型范围。
	if req.ApiKeyID > 0 {
		var keyScopes []struct {
			ModelName string `json:"model_name"`
		}
		err = dao.ApiKeyModelScopes.Ctx(ctx).As("sc").
			InnerJoin("api_keys k ON k.id = sc.api_key_id").
			Where("sc.api_key_id", req.ApiKeyID).
			Where("k.tenant_id", tenantID).
			Fields("sc.model_name").
			Scan(&keyScopes)
		if err != nil {
			return nil, err
		}

		if len(keyScopes) > 0 {
			allowed := make(map[string]bool, len(keyScopes))
			for _, s := range keyScopes {
				allowed[s.ModelName] = true
			}
			filtered := make([]v1.TenantAvailableModelItem, 0, len(list))
			for _, item := range list {
				if allowed[item.ModelId] {
					filtered = append(filtered, item)
				}
			}
			list = filtered
		}
	}

	// 标记图片模型可用的调用模式（同步端点 / 异步端点），供在线体验决定同步/异步开关。
	// 判定与同步端点拦截 gate 同源（constant.IsAsyncImageModel），保证在线体验示例与后端实际
	// 行为一致。仅对图片分类模型跑一次批量查询。
	imageModelIDs := make([]string, 0)
	for _, item := range list {
		if item.Category == "image" {
			imageModelIDs = append(imageModelIDs, item.ModelId)
		}
	}
	if len(imageModelIDs) > 0 {
		modes, err := lcommon.GetImageModelModes(ctx, imageModelIDs)
		if err != nil {
			return nil, err
		}
		for i := range list {
			if list[i].Category != "image" {
				continue
			}
			m, ok := modes[list[i].ModelId]
			if !ok {
				continue
			}
			list[i].AsyncImage = m.AsyncSupported
			list[i].ImageSyncSupported = m.SyncSupported
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

// buildTiers 组装阶梯定价明细：自定义阶梯优先，否则用 base 首档价格（min_tokens=0）+ 基础阶梯其余档。
// 显式模型与分组模型共用；baseTiers 为 min_tokens>0 的其余档，为空时仅返回首档。
func buildTiers(customTiersJSON string, baseInputPrice, baseOutputPrice float64, baseTiers []v1.PricingTierItem) []v1.PricingTierItem {
	if customTiersJSON != "" && customTiersJSON != "null" && customTiersJSON != "[]" {
		var raw []pricingTierRow
		if json.Unmarshal([]byte(customTiersJSON), &raw) == nil && len(raw) > 0 {
			tiers := make([]v1.PricingTierItem, 0, len(raw))
			for _, t := range raw {
				tiers = append(tiers, v1.PricingTierItem{
					MinTokens:   t.MinTokens,
					MaxTokens:   t.MaxTokens,
					InputPrice:  t.InputPrice,
					OutputPrice: t.OutputPrice,
				})
			}
			return tiers
		}
	}

	tiers := make([]v1.PricingTierItem, 0, len(baseTiers)+1)
	if baseInputPrice > 0 || baseOutputPrice > 0 {
		tiers = append(tiers, v1.PricingTierItem{
			MinTokens:   0,
			MaxTokens:   nil,
			InputPrice:  baseInputPrice,
			OutputPrice: baseOutputPrice,
		})
	}
	if len(baseTiers) > 0 {
		if len(tiers) > 0 {
			// 首档与第二档衔接：首档 max_tokens = 第二档 min_tokens
			tiers[0].MaxTokens = &baseTiers[0].MinTokens
		}
		tiers = append(tiers, baseTiers...)
	}
	return tiers
}

// buildTimePrices 构建时段展示价目：每个时段 = 模型当前有效价 × 时段乘数（后端换算，前端直接渲染）。
// token 模式换算输入/输出价；per_request 换算按次价；tiered 用首档价换算（起价，前端标注「起」）。
func buildTimePrices(segments []billing.TimeSegment, billingMode string,
	inputPrice, outputPrice, perRequestPrice *float64, tiers []v1.PricingTierItem) []v1.TimePriceItem {
	if len(segments) == 0 {
		return nil
	}

	// tiered 模式无阶梯数据时无法换算，不展示时段价目（防御）
	var tierInput, tierOutput *float64
	if billingMode == "tiered" {
		if len(tiers) == 0 {
			return nil
		}
		tierInput = &tiers[0].InputPrice
		tierOutput = &tiers[0].OutputPrice
	}

	result := make([]v1.TimePriceItem, 0, len(segments))
	for _, seg := range segments {
		tp := v1.TimePriceItem{
			Name:       seg.Name,
			Days:       seg.Days,
			StartTime:  seg.StartTime,
			EndTime:    seg.EndTime,
			ValidFrom:  seg.ValidFrom,
			ValidTo:    seg.ValidTo,
			Multiplier: seg.Multiplier,
		}
		switch billingMode {
		case "per_request":
			if perRequestPrice != nil {
				tp.PerRequestPrice = mulDisplayPrice(*perRequestPrice, seg.Multiplier)
			}
		case "tiered":
			if tierInput != nil {
				tp.InputPrice = mulDisplayPrice(*tierInput, seg.Multiplier)
			}
			if tierOutput != nil {
				tp.OutputPrice = mulDisplayPrice(*tierOutput, seg.Multiplier)
			}
		default: // token
			if inputPrice != nil {
				tp.InputPrice = mulDisplayPrice(*inputPrice, seg.Multiplier)
			}
			if outputPrice != nil {
				tp.OutputPrice = mulDisplayPrice(*outputPrice, seg.Multiplier)
			}
		}
		result = append(result, tp)
	}
	return result
}

// mulDisplayPrice 展示价换算：单价 × 时段乘数（decimal 单步乘法，出口转 float64 供前端渲染）
func mulDisplayPrice(price, multiplier float64) *float64 {
	v := billing.InexactFloat64(billing.NewFromFloat(price).Mul(billing.NewFromFloat(multiplier)))
	return &v
}

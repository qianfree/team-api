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

// tenantModelPriceRow 显式分配模型的价格查询结果（平台基础价统一从 pricing JSONB 解析）
type tenantModelPriceRow struct {
	ModelDBID                int64    `json:"model_db_id"`
	ID                       int64    `json:"id"`
	BillingMode              *string  `json:"billing_mode"`
	PerRequestPrice          *float64 `json:"per_request_price"`
	DiscountRatio            *float64 `json:"discount_ratio"`
	MaxConcurrency           *int     `json:"max_concurrency"`
	BaseBillingMode          string   `json:"base_billing_mode"`
	BaseDiscountLabel        *string  `json:"base_discount_label"`
	BasePriceChangeNote      *string  `json:"base_price_change_note"`
	CustomInputPrice         *float64 `json:"custom_input_price"`
	CustomOutputPrice        *float64 `json:"custom_output_price"`
	CustomCacheReadPrice     *float64 `json:"custom_cache_read_price"`
	CustomCacheCreationPrice *float64 `json:"custom_cache_creation_price"`
	CustomPricingTiers       string   `json:"custom_pricing_tiers"`
	BasePricing              string   `json:"base_pricing"` // pricing JSONB（唯一真相）
}

// groupPriceRow 分组模型的 base 价格查询结果
type groupPriceRow struct {
	ModelID             int64   `json:"model_id"`
	BaseBillingMode     string  `json:"base_billing_mode"`
	BaseDiscountLabel   *string `json:"base_discount_label"`
	BasePriceChangeNote *string `json:"base_price_change_note"`
	BasePricing         string  `json:"base_pricing"`
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

// priceInfo 显式模型的完整价格信息（基础价字段从平台 pricing JSONB 解析而来）
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
	BaseDiscountLabel        *string
	BasePriceChangeNote      *string
	CustomInputPrice         *float64
	CustomOutputPrice        *float64
	CustomCacheReadPrice     *float64
	CustomCacheCreationPrice *float64
	CustomPricingTiers       string
	BaseBlob                 *billing.PricingBlob // 平台 pricing JSONB 解析结果（tiered 阶梯/时段/per_second 矩阵）
}

// BasePerSecondPrices per_second 矩阵（无 blob 时为 nil）
func (p *priceInfo) BasePerSecondPrices() map[string]float64 {
	if p.BaseBlob == nil {
		return nil
	}
	return p.BaseBlob.Prices
}

// groupPriceInfo 分组模型的价格信息
type groupPriceInfo struct {
	BaseBillingMode        string
	BaseInputPrice         float64
	BaseOutputPrice        float64
	BaseCacheReadPrice     float64
	BaseCacheCreationPrice float64
	BasePerRequestPrice    *float64
	BaseDiscountLabel      *string
	BasePriceChangeNote    *string
	BaseBlob               *billing.PricingBlob
}

// fillFromBlob 从平台 pricing JSONB 提取展示用基础价（tiered 取首档价，与列表口径一致）
func fillFromBlob(blob *billing.PricingBlob) (input, output, cacheRead, cacheCreation float64, perRequest *float64) {
	if blob == nil {
		return
	}
	if len(blob.Tiers) > 0 {
		input = blob.Tiers[0].InputPrice
		output = blob.Tiers[0].OutputPrice
	}
	if blob.InputPrice != nil {
		input = *blob.InputPrice
	}
	if blob.OutputPrice != nil {
		output = *blob.OutputPrice
	}
	if blob.CacheReadPrice != nil {
		cacheRead = *blob.CacheReadPrice
	}
	if blob.CacheCreationPrice != nil {
		cacheCreation = *blob.CacheCreationPrice
	}
	perRequest = blob.Price
	return
}

// ListAvailableModels 获取租户可用的模型列表
func (s *sTenant) ListAvailableModels(ctx context.Context, req *v1.TenantAvailableModelsReq) (*v1.TenantAvailableModelsRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	models, err := lcommon.GetTenantAvailableModels(ctx, tenantID, req.Category, req.Vendor, req.Search)
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
			LeftJoin("mdl_pricing p ON p.model_id = mdl_tenant_models.model_id").
			Where("mdl_tenant_models.tenant_id", tenantID).
			WhereIn("mdl_tenant_models.model_id", explicitDBIDs).
			Fields("mdl_tenant_models.model_id AS model_db_id, mdl_tenant_models.id, mdl_tenant_models.billing_mode, mdl_tenant_models.per_request_price, mdl_tenant_models.discount_ratio, mdl_tenant_models.max_concurrency, p.billing_mode AS base_billing_mode, p.pricing AS base_pricing, p.discount_label AS base_discount_label, p.price_change_note AS base_price_change_note, mdl_tenant_models.custom_input_price, mdl_tenant_models.custom_output_price, mdl_tenant_models.custom_cache_read_price, mdl_tenant_models.custom_cache_creation_price, mdl_tenant_models.custom_pricing_tiers").
			Scan(&priceResults)
		if err != nil {
			return nil, err
		}
	}

	// 构建显式模型价格映射（基础价从 pricing JSONB 解析）
	priceMap := make(map[int64]*priceInfo, len(priceResults))
	for _, r := range priceResults {
		pi := &priceInfo{
			ID:                       r.ID,
			BillingMode:              r.BillingMode,
			PerRequestPrice:          r.PerRequestPrice,
			DiscountRatio:            r.DiscountRatio,
			MaxConcurrency:           r.MaxConcurrency,
			BaseBillingMode:          r.BaseBillingMode,
			BaseDiscountLabel:        r.BaseDiscountLabel,
			BasePriceChangeNote:      r.BasePriceChangeNote,
			CustomInputPrice:         r.CustomInputPrice,
			CustomOutputPrice:        r.CustomOutputPrice,
			CustomCacheReadPrice:     r.CustomCacheReadPrice,
			CustomCacheCreationPrice: r.CustomCacheCreationPrice,
			CustomPricingTiers:       r.CustomPricingTiers,
			BaseBlob:                 billing.ParsePricingBlob(r.BasePricing),
		}
		pi.BaseInputPrice, pi.BaseOutputPrice, pi.BaseCacheReadPrice, pi.BaseCacheCreationPrice, pi.BasePerRequestPrice =
			fillFromBlob(pi.BaseBlob)
		priceMap[r.ModelDBID] = pi
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
			Fields("model_id, billing_mode AS base_billing_mode, pricing AS base_pricing, discount_label AS base_discount_label, price_change_note AS base_price_change_note").
			Scan(&groupPrices)
		if err != nil {
			return nil, err
		}

		for _, gp := range groupPrices {
			gpi := &groupPriceInfo{
				BaseBillingMode:     gp.BaseBillingMode,
				BaseDiscountLabel:   gp.BaseDiscountLabel,
				BasePriceChangeNote: gp.BasePriceChangeNote,
				BaseBlob:            billing.ParsePricingBlob(gp.BasePricing),
			}
			gpi.BaseInputPrice, gpi.BaseOutputPrice, gpi.BaseCacheReadPrice, gpi.BaseCacheCreationPrice, gpi.BasePerRequestPrice =
				fillFromBlob(gpi.BaseBlob)
			groupPriceMap[gp.ModelID] = gpi
		}
	}

	// 平台阶梯与时段定价：从已解析的 pricing JSONB 提取（tiered 阶梯随 blob.Tiers，
	// 时段随 blob.TimeSegments；显式与分组来源统一处理）
	baseTiersMap := make(map[int64][]v1.PricingTierItem)
	timeSegmentsMap := make(map[int64][]billing.TimeSegment)
	for _, m := range models {
		var blob *billing.PricingBlob
		if m.Source == "explicit" {
			if pi, ok := priceMap[m.ModelDBID]; ok {
				blob = pi.BaseBlob
			}
		} else if gp, ok := groupPriceMap[m.ModelDBID]; ok {
			blob = gp.BaseBlob
		}
		if blob == nil {
			continue
		}
		if len(blob.Tiers) > 0 {
			for _, tier := range blob.Tiers {
				baseTiersMap[m.ModelDBID] = append(baseTiersMap[m.ModelDBID], v1.PricingTierItem{
					MinTokens:   tier.MinTokens,
					MaxTokens:   tier.MaxTokens,
					InputPrice:  tier.InputPrice,
					OutputPrice: tier.OutputPrice,
				})
			}
		}
		if len(blob.TimeSegments) > 0 {
			timeSegmentsMap[m.ModelDBID] = blob.TimeSegments
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
				Vendor:             m.Vendor,
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
				DiscountLabel:      pi.BaseDiscountLabel,
				PriceChangeNote:    pi.BasePriceChangeNote,
			}

			if effectiveBillingMode == "tiered" {
				item.PricingTiers = buildTiers(pi.CustomPricingTiers, pi.BaseInputPrice, pi.BaseOutputPrice, baseTiersMap[m.ModelDBID])
			}
			// 按秒/特殊计费：矩阵来自平台定价（租户不逐格覆盖，倍率在计费时作用）；
			// special 的矩阵是输出生成组件的参考单价（素材组件单价在方案配置中）
			if effectiveBillingMode == "per_second" || effectiveBillingMode == billing.BillingModeSpecial {
				item.PerSecondPrices = pi.BasePerSecondPrices()
			}

			item.TimePrices = buildTimePrices(timeSegmentsMap[m.ModelDBID], effectiveBillingMode,
				inputPrice, outputPrice, perRequestPrice, item.PricingTiers, item.PerSecondPrices)

			list = append(list, item)
		} else {
			// group 来源的模型：使用 base 定价，与显式模型展示一致（含按次单价 / 缓存价 / 阶梯明细）
			billingMode := "token"
			var inputPrice, outputPrice, cacheReadPrice, cacheCreationPrice, perRequestPrice *float64
			var baseInputPrice, baseOutputPrice float64
			var discountLabel, priceChangeNote *string
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
				discountLabel = gp.BaseDiscountLabel
				priceChangeNote = gp.BasePriceChangeNote
			}

			item := v1.TenantAvailableModelItem{
				ID:                 m.ModelDBID,
				ModelId:            m.ModelId,
				ModelName:          m.ModelName,
				Category:           m.Category,
				Vendor:             m.Vendor,
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
				DiscountLabel:      discountLabel,
				PriceChangeNote:    priceChangeNote,
			}
			if billingMode == "tiered" && ok {
				item.PricingTiers = buildTiers("", baseInputPrice, baseOutputPrice, baseTiersMap[m.ModelDBID])
			}
			if (billingMode == "per_second" || billingMode == billing.BillingModeSpecial) && ok && gp.BaseBlob != nil {
				item.PerSecondPrices = gp.BaseBlob.Prices
			}
			item.TimePrices = buildTimePrices(timeSegmentsMap[m.ModelDBID], billingMode,
				inputPrice, outputPrice, perRequestPrice, item.PricingTiers, item.PerSecondPrices)
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

// resolveBillingMode 解析有效计费模式。
// 特殊计费方案模型的模式由平台方案决定（special），租户级覆盖无效
// （与 billing.GetModelPriceAt 的读侧守卫、admin.UpdateTenantModel 的写侧守卫同口径）
func resolveBillingMode(tenantMode *string, baseMode string) string {
	if baseMode == billing.BillingModeSpecial {
		return billing.BillingModeSpecial
	}
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

// buildTiers 组装阶梯定价明细：自定义阶梯优先，否则用平台阶梯（pricing JSONB tiers 数组，含首档）。
// 显式模型与分组模型共用；baseTiers 为空时按锚点价合成单档兜底。
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

	// 平台阶梯已含首档（min_tokens=0）：直接返回
	if len(baseTiers) > 0 {
		return baseTiers
	}
	// 无阶梯数据：按锚点价合成单档
	if baseInputPrice > 0 || baseOutputPrice > 0 {
		return []v1.PricingTierItem{{
			MinTokens:   0,
			MaxTokens:   nil,
			InputPrice:  baseInputPrice,
			OutputPrice: baseOutputPrice,
		}}
	}
	return nil
}

// buildTimePrices 构建时段展示价目：每个时段 = 模型当前有效价 × 时段乘数（后端换算，前端直接渲染）。
// token 模式换算输入/输出价；per_request 换算按次价；tiered 用首档价换算（起价，前端标注「起」）。
func buildTimePrices(segments []billing.TimeSegment, billingMode string,
	inputPrice, outputPrice, perRequestPrice *float64, tiers []v1.PricingTierItem, perSecondPrices map[string]float64) []v1.TimePriceItem {
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
	// per_second / special 模式换算基准：兜底档 "*" 单价，无 "*" 取矩阵最低正价
	var perSecondBase *float64
	if billingMode == "per_second" || billingMode == billing.BillingModeSpecial {
		if base := billing.LookupPerSecondPrice(perSecondPrices, "*"); base > 0 {
			perSecondBase = &base
		}
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
		case "per_second", billing.BillingModeSpecial:
			if perSecondBase != nil {
				tp.PerSecondPrice = mulDisplayPrice(*perSecondBase, seg.Multiplier)
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

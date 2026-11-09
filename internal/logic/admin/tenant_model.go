package admin

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	tenantLogic "github.com/qianfree/team-api/internal/logic/common"
	relay "github.com/qianfree/team-api/internal/logic/relay"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/shopspring/decimal"
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
		CustomPricing            string   `json:"custom_pricing"`
		Multiplier               float64  `json:"multiplier"`
		PriceNote                *string  `json:"price_note"`
		DiscountLabel            *string  `json:"discount_label"`
		PriceChangeNote          *string  `json:"price_change_note"`
		ModelCode                string   `json:"model_code"`
		ModelName                string   `json:"model_name"`
		Category                 string   `json:"category"`
	}

	err := dao.MdlTenantModels.Ctx(ctx).
		LeftJoin("mdl_models ON mdl_tenant_models.model_id = mdl_models.id").
		Where("mdl_tenant_models.tenant_id", req.TenantID).
		Fields("mdl_tenant_models.id, mdl_tenant_models.tenant_id, mdl_tenant_models.model_id, mdl_tenant_models.enabled, mdl_tenant_models.billing_mode, mdl_tenant_models.per_request_price, mdl_tenant_models.discount_ratio, mdl_tenant_models.max_concurrency, mdl_tenant_models.channel_scope, mdl_tenant_models.custom_input_price, mdl_tenant_models.custom_output_price, mdl_tenant_models.custom_cache_read_price, mdl_tenant_models.custom_cache_creation_price, mdl_tenant_models.custom_pricing_tiers, mdl_tenant_models.custom_pricing, mdl_tenant_models.multiplier, mdl_tenant_models.price_note, mdl_tenant_models.discount_label, mdl_tenant_models.price_change_note, mdl_models.model_id as model_code, mdl_models.model_name, mdl_models.category").
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
			PriceNote:                r.PriceNote,
			DiscountLabel:            r.DiscountLabel,
			PriceChangeNote:          r.PriceChangeNote,
		}
		if r.CustomPricingTiers != "" && r.CustomPricingTiers != "null" && r.CustomPricingTiers != "[]" {
			var tiers []*v1.PricingTier
			if err := json.Unmarshal([]byte(r.CustomPricingTiers), &tiers); err == nil {
				item.CustomPricingTiers = tiers
			}
		}
		// custom_pricing 补丁展开回显：三态保真（nil/null=继承、空数组=显式关闭、非空=覆盖）。
		// 空切片必须手工构造非 nil 空切片，否则序列化退化为 null 丢失「关闭」标记
		if patch := billing.ParseTenantPricingPatch(r.CustomPricing); patch != nil {
			if len(patch.Prices) > 0 {
				// 解析自本轮 JSON，patch 为循环内新建副本，直接取地址无共享问题
				item.CustomPerSecondPrices = &patch.Prices
			}
			if patch.TimeSegments != nil {
				segs := timeSegmentsToAPI(*patch.TimeSegments)
				if segs == nil {
					segs = make([]v1.TimeSegmentItem, 0)
				}
				item.CustomTimeSegments = segs
			}
			if patch.ParamMultipliers != nil {
				rules := paramMultipliersToAPI(*patch.ParamMultipliers)
				if rules == nil {
					rules = make([]v1.ParamMultiplierItem, 0)
				}
				item.CustomParamMultipliers = rules
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
		g.Log().Debugf(ctx, "BatchAssignModels: tenant_id=%d, model_id=%d, enabled=%v", req.TenantID, a.ModelID, a.Enabled)

		var model *struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		}
		err := dao.MdlModels.Ctx(ctx).
			Where("id", a.ModelID).
			Scan(&model)
		if err != nil || model == nil || model.Status != "active" {
			g.Log().Warningf(ctx, "BatchAssignModels: skip model_id=%d, err=%v, nil=%v", a.ModelID, err, model == nil)
			continue
		}

		count, countErr := dao.MdlTenantModels.Ctx(ctx).
			Where("tenant_id", req.TenantID).
			Where("model_id", a.ModelID).
			Count()
		g.Log().Debugf(ctx, "BatchAssignModels: existing count=%d, countErr=%v", count, countErr)
		if count > 0 {
			g.Log().Debugf(ctx, "BatchAssignModels: skip already assigned, model_id=%d", a.ModelID)
			continue
		}

		maxConc := a.MaxConcurrency
		if maxConc == nil {
			concurrency := 5
			maxConc = &concurrency
		}

		var tiersJSON any
		if len(a.CustomPricingTiers) > 0 {
			b, _ := json.Marshal(a.CustomPricingTiers)
			tiersJSON = string(b)
		}

		// API 边界 *float64 → *decimal.Decimal 转换
		var perRequestPriceDecimal, discountRatioDecimal, customInputPriceDecimal, customOutputPriceDecimal, customCacheReadPriceDecimal, customCacheCreationPriceDecimal *decimal.Decimal
		if a.PerRequestPrice != nil {
			d := billing.NewFromFloat(*a.PerRequestPrice)
			perRequestPriceDecimal = &d
		}
		if a.DiscountRatio != nil {
			d := billing.NewFromFloat(*a.DiscountRatio)
			discountRatioDecimal = &d
		}
		if a.CustomInputPrice != nil {
			d := billing.NewFromFloat(*a.CustomInputPrice)
			customInputPriceDecimal = &d
		}
		if a.CustomOutputPrice != nil {
			d := billing.NewFromFloat(*a.CustomOutputPrice)
			customOutputPriceDecimal = &d
		}
		if a.CustomCacheReadPrice != nil {
			d := billing.NewFromFloat(*a.CustomCacheReadPrice)
			customCacheReadPriceDecimal = &d
		}
		if a.CustomCacheCreationPrice != nil {
			d := billing.NewFromFloat(*a.CustomCacheCreationPrice)
			customCacheCreationPriceDecimal = &d
		}

		insertData := do.MdlTenantModels{
			TenantId:                 req.TenantID,
			ModelId:                  a.ModelID,
			Enabled:                  a.Enabled,
			BillingMode:              a.BillingMode,
			PerRequestPrice:          perRequestPriceDecimal,
			DiscountRatio:            discountRatioDecimal,
			MaxConcurrency:           maxConc,
			ChannelScope:             a.ChannelScope,
			CustomInputPrice:         customInputPriceDecimal,
			CustomOutputPrice:        customOutputPriceDecimal,
			CustomCacheReadPrice:     customCacheReadPriceDecimal,
			CustomCacheCreationPrice: customCacheCreationPriceDecimal,
			CustomPricingTiers:       tiersJSON,
			Multiplier:               1.0,
		}
		g.Log().Debugf(ctx, "BatchAssignModels: inserting data tenant_id=%v, model_id=%v, enabled=%v, max_concurrency=%v", insertData.TenantId, insertData.ModelId, insertData.Enabled, *maxConc)
		_, err = dao.MdlTenantModels.Ctx(ctx).Insert(insertData)
		if err != nil {
			g.Log().Errorf(ctx, "BatchAssignModels: insert failed, model_id=%d, err=%v", a.ModelID, err)
		} else {
			assigned++
			g.Log().Debugf(ctx, "BatchAssignModels: insert success, model_id=%d, assigned=%d", a.ModelID, assigned)
		}
	}

	billing.ClearTenantPriceCache(ctx, req.TenantID)
	relay.ClearTenantModelAccessCache(ctx, req.TenantID)

	return &v1.TenantModelBatchAssignRes{Assigned: assigned}, nil
}

// buildTenantPricingPatchUpdate 旧补丁 + 非 nil 请求键 → 合并后的补丁 JSON（读改写合并）。
// 请求键四态：外层 nil=不动该键；内层 nil（或矩阵空 map）=清除恢复继承；
// 时段/倍率空数组=显式关闭平台配置；非空=整体替换。
// 校验：时段/倍率复用平台 fromAPI 转换（内含 Validate*）；矩阵复用 BuildPricingBlob 同规则。
// 返回 (jsonStr —— 空串表示补丁全空应写 NULL, patch —— 合并后补丁供模式守卫, err)
func buildTenantPricingPatchUpdate(oldRaw string,
	prices **map[string]float64,
	segs **[]v1.TimeSegmentItem,
	rules **[]v1.ParamMultiplierItem) (string, *billing.TenantPricingPatch, error) {
	// 全部请求键省略 = 不动：原样返回旧补丁（真实调用方在至少一键非 nil 时才进入，
	// 此短路保证纯函数语义完整，重写同值无副作用）
	if prices == nil && segs == nil && rules == nil {
		return oldRaw, billing.ParseTenantPricingPatch(oldRaw), nil
	}

	patch := billing.ParseTenantPricingPatch(oldRaw)
	if patch == nil {
		patch = &billing.TenantPricingPatch{}
	}

	if prices != nil {
		// 矩阵无「关闭」态：null / 空 map 统一按清除恢复继承处理
		if *prices == nil || len(**prices) == 0 {
			patch.Prices = nil
		} else {
			patch.Prices = **prices
		}
	}
	if segs != nil {
		if *segs == nil {
			patch.TimeSegments = nil
		} else {
			converted, err := timeSegmentsFromAPI(**segs)
			if err != nil {
				return "", nil, err
			}
			// 空数组=显式关闭：fromAPI 空入参返回 nil，归一为非 nil 空切片保住三态语义
			if converted == nil {
				converted = []billing.TimeSegment{}
			}
			patch.TimeSegments = &converted
		}
	}
	if rules != nil {
		if *rules == nil {
			patch.ParamMultipliers = nil
		} else {
			converted, err := paramMultipliersFromAPI(**rules)
			if err != nil {
				return "", nil, err
			}
			if converted == nil {
				converted = []billing.ParamRule{}
			}
			patch.ParamMultipliers = &converted
		}
	}

	// 矩阵校验与平台同规则（无空规格/非负价/至少一档正价），经 BuildPricingBlob 单行构造复用
	if len(patch.Prices) > 0 {
		if _, err := billing.BuildPricingBlob([]billing.PricingItemInput{{
			BillingMode:     "per_second",
			PerSecondPrices: patch.Prices,
		}}); err != nil {
			return "", nil, err
		}
	}

	// 全键空 = 无覆盖，写 NULL
	if len(patch.Prices) == 0 && patch.TimeSegments == nil && patch.ParamMultipliers == nil {
		return "", nil, nil
	}
	b, err := json.Marshal(patch)
	if err != nil {
		return "", nil, err
	}
	return string(b), patch, nil
}

// assertTenantMatrixModeAllowed 按秒矩阵覆盖守卫：最终生效计费模式必须是 per_second/special，
// 防止租户覆盖模式为 token 等却留下永不消费的矩阵死配置。模式解析优先级：
// 平台特殊方案（scheme）锁定 special；否则 本次请求覆盖 > 租户行覆盖 > 平台模式
func assertTenantMatrixModeAllowed(ctx context.Context, modelID int64, reqMode, rowMode *string) error {
	var pricingRow struct {
		BillingMode string `json:"billing_mode"`
		Pricing     string `json:"pricing"`
	}
	if err := dao.MdlPricing.Ctx(ctx).Where("model_id", modelID).Fields("billing_mode, pricing").Scan(&pricingRow); err != nil {
		return gerror.Wrap(err, "查询模型定价")
	}
	if blob := billing.ParsePricingBlob(pricingRow.Pricing); blob != nil && blob.Scheme != "" {
		return nil // 特殊方案锁定 special，矩阵是方案定价依据，允许覆盖
	}
	finalMode := pricingRow.BillingMode
	if rowMode != nil && *rowMode != "" {
		finalMode = *rowMode
	}
	if reqMode != nil {
		if *reqMode == "" {
			finalMode = pricingRow.BillingMode // 本次置空 = 恢复平台模式
		} else {
			finalMode = *reqMode
		}
	}
	if finalMode != "per_second" && finalMode != billing.BillingModeSpecial {
		return gerror.New("按秒矩阵覆盖仅适用于按秒/特殊计费模型；请勿在该模型上覆盖为其他计费模式")
	}
	return nil
}

// assertTenantPatchFormulaAllowed 表达式计费（formula）模型拒绝租户级时段定价与参数倍率覆盖：
// 与平台写侧（SetModelPricing）同口径，时段/参数条件只能在公式内用 hour()/param() 表达，
// 否则乘数会与公式叠加错价。显式关闭（空切片）不算覆盖，允许保存以便清理存量补丁。
// formula 模式锁定为平台模式（租户行无法改模式），只需看平台定价
func assertTenantPatchFormulaAllowed(ctx context.Context, modelID int64, merged *billing.TenantPricingPatch) error {
	if merged == nil {
		return nil
	}
	hasSegments := merged.TimeSegments != nil && len(*merged.TimeSegments) > 0
	hasParamRules := merged.ParamMultipliers != nil && len(*merged.ParamMultipliers) > 0
	if !hasSegments && !hasParamRules {
		return nil
	}
	var pricingRow struct {
		BillingMode string `json:"billing_mode"`
	}
	if err := dao.MdlPricing.Ctx(ctx).Where("model_id", modelID).Fields("billing_mode").Scan(&pricingRow); err != nil {
		return gerror.Wrap(err, "查询模型定价")
	}
	if pricingRow.BillingMode == billing.BillingModeFormula {
		return gerror.New("表达式计费（formula）模型不支持租户级时段定价与参数倍率：时段/请求参数条件请在平台公式的请求规则中表达")
	}
	return nil
}

// displayColValue 展示字段列值：trim 后空串=清除恢复继承（NULL），非空=覆盖。
// 与平台 writePricingDisplayFieldsForModel 的口径一致
func displayColValue(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return gdb.Raw("NULL")
	}
	return s
}

// UpdateTenantModel 更新租户模型配置
func (s *sAdmin) UpdateTenantModel(ctx context.Context, req *v1.TenantModelUpdateReq) (*v1.TenantModelUpdateRes, error) {
	data := do.MdlTenantModels{}

	if req.Enabled != nil {
		data.Enabled = *req.Enabled
	}
	if req.BillingMode != nil {
		// 特殊计费方案模型不允许租户级 billing_mode 覆盖：计费模式由平台方案决定（special），
		// 租户改模式只会造成日志标签与计费口径错位（读取侧 GetModelPriceAt 同口径忽略覆盖）。
		// 置空（恢复平台默认）不受限
		if *req.BillingMode != "" {
			var pricingRow struct {
				Pricing string `json:"pricing"`
			}
			if err := dao.MdlPricing.Ctx(ctx).Where("model_id", req.ModelID).Fields("pricing").Scan(&pricingRow); err != nil {
				return nil, gerror.Wrap(err, "查询模型定价")
			}
			if blob := billing.ParsePricingBlob(pricingRow.Pricing); blob != nil && blob.Scheme != "" {
				return nil, gerror.New("该模型使用特殊计费方案，计费模式由平台方案决定，不允许租户级覆盖")
			}
		}
		data.BillingMode = *req.BillingMode
	}
	if req.PerRequestPrice != nil {
		d := billing.NewFromFloat(**req.PerRequestPrice)
		data.PerRequestPrice = &d
	}
	if req.DiscountRatio != nil {
		d := billing.NewFromFloat(**req.DiscountRatio)
		data.DiscountRatio = &d
	}
	if req.MaxConcurrency != nil {
		data.MaxConcurrency = *req.MaxConcurrency
	}
	if req.ChannelScope != nil {
		data.ChannelScope = *req.ChannelScope
	}
	if req.CustomInputPrice != nil {
		d := billing.NewFromFloat(**req.CustomInputPrice)
		data.CustomInputPrice = &d
	}
	if req.CustomCacheReadPrice != nil {
		d := billing.NewFromFloat(**req.CustomCacheReadPrice)
		data.CustomCacheReadPrice = &d
	}
	if req.CustomCacheCreationPrice != nil {
		d := billing.NewFromFloat(**req.CustomCacheCreationPrice)
		data.CustomCacheCreationPrice = &d
	}
	if req.CustomOutputPrice != nil {
		d := billing.NewFromFloat(**req.CustomOutputPrice)
		data.CustomOutputPrice = &d
	}
	if req.CustomPricingTiers != nil {
		if len(*req.CustomPricingTiers) > 0 {
			b, _ := json.Marshal(*req.CustomPricingTiers)
			data.CustomPricingTiers = string(b)
		} else {
			data.CustomPricingTiers = "[]"
		}
	}

	// 扩展计费覆盖与展示字段走列直写（do 结构 nil 字段不更新，无法表达「清除恢复继承」）
	cols := g.Map{}
	if req.CustomPerSecondPrices != nil || req.CustomTimeSegments != nil || req.CustomParamMultipliers != nil {
		// 读旧行：旧补丁供合并、billing_mode 供矩阵模式守卫
		var oldRow struct {
			CustomPricing string  `json:"custom_pricing"`
			BillingMode   *string `json:"billing_mode"`
		}
		if err := dao.MdlTenantModels.Ctx(ctx).
			Where("tenant_id", req.TenantID).
			Where("model_id", req.ModelID).
			Fields("custom_pricing, billing_mode").
			Scan(&oldRow); err != nil {
			return nil, gerror.Wrap(err, "查询租户模型覆盖补丁")
		}
		patchJSON, merged, err := buildTenantPricingPatchUpdate(oldRow.CustomPricing,
			req.CustomPerSecondPrices, req.CustomTimeSegments, req.CustomParamMultipliers)
		if err != nil {
			return nil, err
		}
		if merged != nil && len(merged.Prices) > 0 {
			if err := assertTenantMatrixModeAllowed(ctx, req.ModelID, req.BillingMode, oldRow.BillingMode); err != nil {
				return nil, err
			}
		}
		if err := assertTenantPatchFormulaAllowed(ctx, req.ModelID, merged); err != nil {
			return nil, err
		}
		if patchJSON == "" {
			cols["custom_pricing"] = gdb.Raw("NULL")
		} else {
			cols["custom_pricing"] = patchJSON
		}
	}
	if req.PriceNote != nil {
		cols["price_note"] = displayColValue(**req.PriceNote)
	}
	if req.DiscountLabel != nil {
		cols["discount_label"] = displayColValue(**req.DiscountLabel)
	}
	if req.PriceChangeNote != nil {
		cols["price_change_note"] = displayColValue(**req.PriceChangeNote)
	}

	_, err := dao.MdlTenantModels.Ctx(ctx).
		Where("tenant_id", req.TenantID).
		Where("model_id", req.ModelID).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}
	if len(cols) > 0 {
		if _, err := dao.MdlTenantModels.Ctx(ctx).
			Where("tenant_id", req.TenantID).
			Where("model_id", req.ModelID).
			Data(cols).
			Update(); err != nil {
			return nil, err
		}
	}

	billing.ClearTenantPriceCache(ctx, req.TenantID)
	relay.ClearTenantModelAccessCache(ctx, req.TenantID)

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
	relay.ClearTenantModelAccessCache(ctx, req.TenantID)

	return nil, nil
}

// ListTenantAvailableModels 预览租户实际可用的所有模型（显式分配 + 分组来源，去重）
func (s *sAdmin) ListTenantAvailableModels(ctx context.Context, req *v1.TenantAvailableModelsPreviewReq) (*v1.TenantAvailableModelsPreviewRes, error) {
	models, err := tenantLogic.GetTenantAvailableModels(ctx, req.TenantID, "", "", "")
	if err != nil {
		return nil, err
	}

	list := make([]v1.TenantAvailableModelPreview, 0, len(models))
	for _, m := range models {
		list = append(list, v1.TenantAvailableModelPreview{
			ModelId:          m.ModelId,
			ModelName:        m.ModelName,
			Category:         m.Category,
			MaxContextTokens: m.MaxContextTokens,
			MaxOutputTokens:  m.MaxOutputTokens,
			Description:      m.Description,
			Source:           m.Source,
		})
	}

	return &v1.TenantAvailableModelsPreviewRes{List: list}, nil
}

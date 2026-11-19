package admin

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
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
		ID              int64    `json:"id"`
		TenantID        int64    `json:"tenant_id"`
		ModelID         int64    `json:"model_id"`
		Enabled         bool     `json:"enabled"`
		DiscountRatio   *float64 `json:"discount_ratio"`
		MaxConcurrency  *int     `json:"max_concurrency"`
		ChannelScope    string   `json:"channel_scope"`
		CustomPricing   string   `json:"custom_pricing"`
		Multiplier      float64  `json:"multiplier"`
		PriceNote       *string  `json:"price_note"`
		DiscountLabel   *string  `json:"discount_label"`
		PriceChangeNote *string  `json:"price_change_note"`
		ModelCode       string   `json:"model_code"`
		ModelName       string   `json:"model_name"`
		Category        string   `json:"category"`
	}

	err := dao.MdlTenantModels.Ctx(ctx).
		LeftJoin("mdl_models ON mdl_tenant_models.model_id = mdl_models.id").
		Where("mdl_tenant_models.tenant_id", req.TenantID).
		Fields("mdl_tenant_models.id, mdl_tenant_models.tenant_id, mdl_tenant_models.model_id, mdl_tenant_models.enabled, mdl_tenant_models.discount_ratio, mdl_tenant_models.max_concurrency, mdl_tenant_models.channel_scope, mdl_tenant_models.custom_pricing, mdl_tenant_models.multiplier, mdl_tenant_models.price_note, mdl_tenant_models.discount_label, mdl_tenant_models.price_change_note, mdl_models.model_id as model_code, mdl_models.model_name, mdl_models.category").
		OrderAsc("mdl_models.category").
		OrderAsc("mdl_models.model_id").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	list := make([]v1.TenantModelItem, 0, len(results))
	for _, r := range results {
		item := v1.TenantModelItem{
			ID:              r.ID,
			TenantID:        r.TenantID,
			ModelID:         r.ModelID,
			ModelCode:       r.ModelCode,
			ModelName:       r.ModelName,
			Category:        r.Category,
			Enabled:         r.Enabled,
			DiscountRatio:   r.DiscountRatio,
			MaxConcurrency:  r.MaxConcurrency,
			ChannelScope:    r.ChannelScope,
			Multiplier:      r.Multiplier,
			PriceNote:       r.PriceNote,
			DiscountLabel:   r.DiscountLabel,
			PriceChangeNote: r.PriceChangeNote,
		}
		// 定价覆盖回显：custom_pricing blob（PricingBlob 形状）→ API 覆盖对象，
		// 三态保真（nil/null=继承、空数组=显式关闭、非空=覆盖）
		if blob := billing.ParsePricingBlob(r.CustomPricing); blob != nil && blob.HasAnyOverride() {
			item.Pricing = blobToPricingOverride(blob)
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

		// 定价覆盖：Pricing 对象 → custom_pricing blob（全量写入，键缺失=继承）
		var pricingJSON any
		if a.Pricing != nil {
			blob, err := pricingOverrideToBlob(a.Pricing)
			if err != nil {
				return nil, gerror.Wrapf(err, "模型 %d 定价覆盖格式错误", a.ModelID)
			}
			if err := billing.ValidateTenantPricingBlob(blob, a.DiscountRatio); err != nil {
				return nil, gerror.Wrapf(err, "模型 %d 定价覆盖校验失败", a.ModelID)
			}
			if len(blob.Prices) > 0 {
				if err := assertTenantMatrixModeAllowed(ctx, a.ModelID, blob.BillingMode, nil); err != nil {
					return nil, gerror.Wrapf(err, "模型 %d", a.ModelID)
				}
			}
			if blob.HasAnyOverride() {
				b, err := json.Marshal(blob)
				if err != nil {
					return nil, gerror.Wrapf(err, "模型 %d 序列化定价覆盖", a.ModelID)
				}
				pricingJSON = string(b)
			}
		}

		// API 边界 *float64 → *decimal.Decimal 转换（折扣）
		var discountRatioDecimal *decimal.Decimal
		if a.DiscountRatio != nil {
			d := billing.NewFromFloat(*a.DiscountRatio)
			discountRatioDecimal = &d
		}

		insertData := do.MdlTenantModels{
			TenantId:       req.TenantID,
			ModelId:        a.ModelID,
			Enabled:        a.Enabled,
			DiscountRatio:  discountRatioDecimal,
			MaxConcurrency: maxConc,
			ChannelScope:   a.ChannelScope,
			CustomPricing:  pricingJSON,
			Multiplier:     1.0,
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
func assertTenantPatchFormulaAllowed(ctx context.Context, modelID int64, blob *billing.PricingBlob) error {
	if blob == nil {
		return nil
	}
	hasSegments := blob.TimeSegments != nil && len(*blob.TimeSegments) > 0
	hasParamRules := blob.ParamMultipliers != nil && len(*blob.ParamMultipliers) > 0
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

// nullKeysFromRequest 提取请求原始 JSON body 中显式置 null 的顶层键集合。
// 用途：还原「null=清除」语义——GoFrame 的结构体解析（gconv）把 JSON null 与
// 省略键都折叠成 nil 指针，**类型的三态契约在解析产物中不可区分，只能回读
// 原始 body（json.Unmarshal 到 map 时 null 值保留键存在性）。
// 非 HTTP 上下文 / 非 JSON body / 解析失败返回空集合（调用方回退保守语义：视为省略）。
func nullKeysFromRequest(ctx context.Context) map[string]bool {
	r := ghttp.RequestFromCtx(ctx)
	if r == nil {
		return nil
	}
	return nullKeysFromBody(r.GetBody())
}

// nullKeysFromBody 从 JSON body 字节提取显式置 null 的顶层键（nullKeysFromRequest
// 的纯函数核心，供单测）。非 JSON / 解析失败返回 nil。
func nullKeysFromBody(body []byte) map[string]bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	keys := make(map[string]bool, len(raw))
	for k, v := range raw {
		if strings.TrimSpace(string(v)) == "null" {
			keys[k] = true
		}
	}
	return keys
}

// UpdateTenantModel 更新租户模型配置
func (s *sAdmin) UpdateTenantModel(ctx context.Context, req *v1.TenantModelUpdateReq) (*v1.TenantModelUpdateRes, error) {
	data := do.MdlTenantModels{}

	// 「null=清除」键集合（gconv 把 null 与省略都折叠为 nil 指针，三态只能回读原始
	// body 还原，见 nullKeysFromRequest）。折扣与展示字段的三态契约：
	// 省略=不动；null=清除恢复继承；值=设置
	explicitNulls := nullKeysFromRequest(ctx)
	// 定价覆盖/展示字段/折扣清除走列直写（do 结构 nil 字段不更新，无法表达「清除恢复继承」）
	cols := g.Map{}

	if req.Enabled != nil {
		data.Enabled = *req.Enabled
	}
	if req.DiscountRatio != nil && *req.DiscountRatio != nil {
		d := billing.NewFromFloat(**req.DiscountRatio)
		data.DiscountRatio = &d
	} else if explicitNulls["discount_ratio"] {
		cols["discount_ratio"] = gdb.Raw("NULL") // null=清除折扣恢复继承
	}
	if req.MaxConcurrency != nil {
		data.MaxConcurrency = *req.MaxConcurrency
	}
	if req.ChannelScope != nil {
		data.ChannelScope = *req.ChannelScope
	}

	if req.Pricing != nil {
		blob, err := pricingOverrideToBlob(req.Pricing)
		if err != nil {
			return nil, err
		}
		// 旧行（折扣供互斥校验、旧覆盖供矩阵模式守卫与价格键清除审计）
		var oldRow struct {
			DiscountRatio *float64 `json:"discount_ratio"`
			CustomPricing string   `json:"custom_pricing"`
		}
		if err := dao.MdlTenantModels.Ctx(ctx).
			Where("tenant_id", req.TenantID).
			Where("model_id", req.ModelID).
			Fields("discount_ratio, custom_pricing").
			Scan(&oldRow); err != nil {
			return nil, gerror.Wrap(err, "查询租户模型覆盖")
		}
		oldBlob := billing.ParsePricingBlob(oldRow.CustomPricing)

		// 互斥校验的折扣口径（本次动作三态：值=null 外的数值 / 显式 null=清除后无折扣 /
		// 省略=读行内现有值），防「先设折扣再补价格键」的两步绕过
		var effectiveRatio *float64
		if req.DiscountRatio != nil && *req.DiscountRatio != nil {
			effectiveRatio = *req.DiscountRatio
		} else if !explicitNulls["discount_ratio"] {
			effectiveRatio = oldRow.DiscountRatio
		}
		if err := billing.ValidateTenantPricingBlob(blob, effectiveRatio); err != nil {
			return nil, err
		}
		// 按秒矩阵覆盖守卫：最终生效计费模式必须是 per_second/special
		if len(blob.Prices) > 0 {
			var rowMode *string
			if oldBlob != nil {
				rowMode = oldBlob.BillingMode
			}
			if err := assertTenantMatrixModeAllowed(ctx, req.ModelID, blob.BillingMode, rowMode); err != nil {
				return nil, err
			}
		}
		// 专属价被整体清除的审计日志：全量替换语义下，管理员主动清除是合法操作，
		// 但前端漏传价格键也表现为同一形态（曾因此静默丢价），留痕便于事后追查
		if oldBlob.HasPriceOverride() && !blob.HasPriceOverride() {
			g.Log().Warningf(ctx, "UpdateTenantModel: 租户专属价被清除 tenant_id=%d model_id=%d old_custom_pricing=%s",
				req.TenantID, req.ModelID, oldRow.CustomPricing)
		}
		// formula 模型拒绝租户级时段/参数覆盖（与平台写侧同口径）
		if err := assertTenantPatchFormulaAllowed(ctx, req.ModelID, blob); err != nil {
			return nil, err
		}
		if blob.HasAnyOverride() {
			b, err := json.Marshal(blob)
			if err != nil {
				return nil, gerror.Wrap(err, "序列化定价覆盖")
			}
			cols["custom_pricing"] = string(b)
		} else {
			cols["custom_pricing"] = gdb.Raw("NULL") // 空对象=清除全部覆盖恢复继承
		}
	}
	if req.PriceNote != nil && *req.PriceNote != nil {
		cols["price_note"] = displayColValue(**req.PriceNote)
	} else if explicitNulls["price_note"] {
		cols["price_note"] = gdb.Raw("NULL") // null=清除恢复继承（等价空串语义）
	}
	if req.DiscountLabel != nil && *req.DiscountLabel != nil {
		cols["discount_label"] = displayColValue(**req.DiscountLabel)
	} else if explicitNulls["discount_label"] {
		cols["discount_label"] = gdb.Raw("NULL")
	}
	if req.PriceChangeNote != nil && *req.PriceChangeNote != nil {
		cols["price_change_note"] = displayColValue(**req.PriceChangeNote)
	} else if explicitNulls["price_change_note"] {
		cols["price_change_note"] = gdb.Raw("NULL")
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

// pricingOverrideToBlob API 覆盖对象 → billing.PricingBlob（custom_pricing JSONB 的内存形态）。
// tiers 逐字段转换（v1.PricingTier → billing.PricingTierRow）；时段/参数复用平台
// fromAPI 转换（内含校验），空数组归一为非 nil 空切片保住「显式关闭」三态
func pricingOverrideToBlob(p *v1.TenantPricingOverride) (*billing.PricingBlob, error) {
	if p == nil {
		return nil, nil
	}
	blob := &billing.PricingBlob{
		BillingMode:          p.BillingMode,
		InputPrice:           p.InputPrice,
		OutputPrice:          p.OutputPrice,
		CacheReadPrice:       p.CacheReadPrice,
		CacheCreationPrice:   p.CacheCreationPrice,
		CacheCreation1hPrice: p.CacheCreation1hPrice,
		Price:                p.Price,
	}
	if p.Prices != nil && len(*p.Prices) > 0 {
		blob.Prices = *p.Prices
	}
	for _, t := range p.Tiers {
		if t == nil {
			continue
		}
		blob.Tiers = append(blob.Tiers, billing.PricingTierRow{
			MinTokens:            t.MinTokens,
			MaxTokens:            t.MaxTokens,
			InputPrice:           t.InputPrice,
			OutputPrice:          t.OutputPrice,
			CacheReadPrice:       positivePricePtr(t.CacheReadPrice),
			CacheCreationPrice:   positivePricePtr(t.CacheCreationPrice),
			CacheCreation1hPrice: positivePricePtr(t.CacheCreation1hPrice),
		})
	}
	if p.TimeSegments != nil {
		segs, err := timeSegmentsFromAPI(*p.TimeSegments)
		if err != nil {
			return nil, err
		}
		if segs == nil {
			segs = []billing.TimeSegment{} // 空数组=显式关闭：归一非 nil 空切片
		}
		blob.TimeSegments = &segs
	}
	if p.ParamMultipliers != nil {
		rules, err := paramMultipliersFromAPI(*p.ParamMultipliers)
		if err != nil {
			return nil, err
		}
		if rules == nil {
			rules = []billing.ParamRule{}
		}
		blob.ParamMultipliers = &rules
	}
	return blob, nil
}

// positivePricePtr 阶梯缓存价正值取指针（0/负=未配置 nil，回退锚点语义）
func positivePricePtr(v float64) *float64 {
	if v > 0 {
		return &v
	}
	return nil
}

// blobToPricingOverride blob → API 覆盖对象（回显，三态保真：空切片手工构造
// 非 nil，避免序列化退化为 null 丢失「关闭」标记）
func blobToPricingOverride(b *billing.PricingBlob) *v1.TenantPricingOverride {
	if b == nil {
		return nil
	}
	p := &v1.TenantPricingOverride{
		BillingMode:          b.BillingMode,
		InputPrice:           b.InputPrice,
		OutputPrice:          b.OutputPrice,
		CacheReadPrice:       b.CacheReadPrice,
		CacheCreationPrice:   b.CacheCreationPrice,
		CacheCreation1hPrice: b.CacheCreation1hPrice,
		Price:                b.Price,
	}
	if len(b.Prices) > 0 {
		prices := b.Prices
		p.Prices = &prices
	}
	for _, t := range b.Tiers {
		tier := t
		p.Tiers = append(p.Tiers, &v1.PricingTier{
			MinTokens:            tier.MinTokens,
			MaxTokens:            tier.MaxTokens,
			InputPrice:           tier.InputPrice,
			OutputPrice:          tier.OutputPrice,
			CacheReadPrice:       tierVal(tier.CacheReadPrice),
			CacheCreationPrice:   tierVal(tier.CacheCreationPrice),
			CacheCreation1hPrice: tierVal(tier.CacheCreation1hPrice),
		})
	}
	if b.TimeSegments != nil {
		segs := timeSegmentsToAPI(*b.TimeSegments)
		if segs == nil {
			segs = make([]v1.TimeSegmentItem, 0)
		}
		p.TimeSegments = &segs
	}
	if b.ParamMultipliers != nil {
		rules := paramMultipliersToAPI(*b.ParamMultipliers)
		if rules == nil {
			rules = make([]v1.ParamMultiplierItem, 0)
		}
		p.ParamMultipliers = &rules
	}
	return p
}

// tierVal 阶梯缓存价指针 → 值（nil=0，回显端 0 即未配置）
func tierVal(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

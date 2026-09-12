package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/export"
)

// ListModelOptions 获取模型选项列表（不分页，用于下拉选择）
func (s *sAdmin) ListModelOptions(ctx context.Context, req *v1.ModelOptionsReq) (*v1.ModelOptionsRes, error) {
	query := dao.MdlModels.Ctx(ctx)

	if req.Status != "" {
		query = query.Where("status", req.Status)
	} else {
		query = query.Where("status", "active")
	}
	if req.Category != "" {
		query = query.Where("category", req.Category)
	}

	var models []struct {
		ID        int64  `json:"id"`
		ModelId   string `json:"model_id"`
		ModelName string `json:"model_name"`
		Category  string `json:"category"`
	}

	err := query.Fields("id, model_id, model_name, category").
		OrderAsc("category").OrderAsc("model_id").
		Scan(&models)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	list := make([]v1.ModelOptionItem, 0, len(models))
	for _, m := range models {
		list = append(list, v1.ModelOptionItem{
			ID:        m.ID,
			ModelId:   m.ModelId,
			ModelName: m.ModelName,
			Category:  m.Category,
		})
	}

	return &v1.ModelOptionsRes{List: list}, nil
}

// ListModels 获取模型列表（含定价摘要）
func (s *sAdmin) ListModels(ctx context.Context, req *v1.ModelListReq) (*v1.ModelListRes, error) {
	query := dao.MdlModels.Ctx(ctx)

	if req.Category != "" {
		query = query.Where("category", req.Category)
	}
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}
	if req.Search != "" {
		query = query.Where("model_id LIKE ? OR model_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}
	// 定价状态筛选（pricing JSONB 唯一真相：任一模式有正价即已定价）
	// 已配价谓词：token 价>0 / per_request 价>0 / per_second 矩阵非空 / tiered 任一档价>0
	const pricedPredicate = `EXISTS (SELECT 1 FROM mdl_pricing WHERE mdl_pricing.model_id = mdl_models.id AND (
		(pricing->>'input_price')::numeric > 0
		OR (pricing->>'output_price')::numeric > 0
		OR (pricing->>'price')::numeric > 0
		OR (pricing->'prices' IS NOT NULL AND pricing->'prices' <> '{}'::jsonb)
		OR EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(pricing->'tiers', '[]'::jsonb)) tx WHERE (tx->>'input_price')::numeric > 0 OR (tx->>'output_price')::numeric > 0)
	))`
	if req.PricingStatus == "priced" {
		query = query.Where(pricedPredicate)
	} else if req.PricingStatus == "unpriced" {
		query = query.Where("EXISTS (SELECT 1 FROM mdl_pricing WHERE mdl_pricing.model_id = mdl_models.id) AND NOT " + pricedPredicate)
	}

	var total int
	var models []struct {
		ID               int64       `json:"id"`
		ModelId          string      `json:"model_id"`
		ModelName        string      `json:"model_name"`
		Category         string      `json:"category"`
		Status           string      `json:"status"`
		MaxContextTokens int         `json:"max_context_tokens"`
		MaxOutputTokens  int         `json:"max_output_tokens"`
		Capabilities     string      `json:"capabilities"`
		Description      string      `json:"description"`
		Tags             []string    `json:"tags"`
		CreatedAt        *gtime.Time `json:"created_at"`
		UpdatedAt        *gtime.Time `json:"updated_at"`
		DeprecatedAt     *gtime.Time `json:"deprecated_at"`
		SunsetDate       *gtime.Time `json:"sunset_date"`
		ReplacementModel string      `json:"replacement_model"`
	}

	err := query.Fields("id, model_id, model_name, category, status, max_context_tokens, max_output_tokens, capabilities, description, tags, created_at, updated_at, deprecated_at, sunset_date, replacement_model").
		OrderDesc("id").
		Page(req.Page, req.PageSize).
		ScanAndCount(&models, &total, false)
	if err != nil {
		return nil, err
	}

	// 批量查询定价摘要（min_tokens=0 的基准行）
	modelIDs := make([]int64, 0, len(models))
	for _, m := range models {
		modelIDs = append(modelIDs, m.ID)
	}
	type pricingRow struct {
		ModelId     int64  `json:"model_id"`
		BillingMode string `json:"billing_mode"`
		Pricing     string `json:"pricing"`
	}
	var pricingRows []pricingRow
	if len(modelIDs) > 0 {
		err = dao.MdlPricing.Ctx(ctx).
			Fields("model_id, billing_mode, pricing").
			Where("model_id IN (?)", modelIDs).
			Scan(&pricingRows)
		if err = common.IgnoreScanNoRows(err); err != nil {
			return nil, err
		}
	}
	pricingMap := make(map[int64]pricingRow, len(pricingRows))
	for _, p := range pricingRows {
		pricingMap[p.ModelId] = p
	}

	// 批量查询渠道支持情况
	type channelAbilityRow struct {
		ModelName   string `json:"model_name"`
		ChannelID   int64  `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		Type        int    `json:"type"`
	}
	modelIdStrs := make([]string, 0, len(models))
	for _, m := range models {
		modelIdStrs = append(modelIdStrs, m.ModelId)
	}
	var channelRows []channelAbilityRow
	if len(modelIdStrs) > 0 {
		err = dao.ChnAbilities.Ctx(ctx).
			LeftJoin("chn_channels", "chn_channels.id = chn_abilities.channel_id").
			Fields("chn_abilities.model_name, chn_abilities.channel_id, chn_channels.name AS channel_name, chn_channels.type").
			Where("chn_abilities.model_name IN (?)", modelIdStrs).
			Where("chn_abilities.enabled", true).
			Where("chn_channels.status", "active").
			Scan(&channelRows)
		if err = common.IgnoreScanNoRows(err); err != nil {
			return nil, err
		}
	}
	channelMap := make(map[string][]v1.ModelChannelInfo, len(channelRows))
	for _, r := range channelRows {
		channelMap[r.ModelName] = append(channelMap[r.ModelName], v1.ModelChannelInfo{
			ChannelID:   r.ChannelID,
			ChannelName: r.ChannelName,
			Type:        r.Type,
		})
	}

	list := make([]v1.ModelItem, 0, len(models))
	for _, m := range models {
		item := v1.ModelItem{
			ID:           m.ID,
			ModelId:      m.ModelId,
			ModelName:    m.ModelName,
			Category:     m.Category,
			Status:       m.Status,
			MaxContext:   m.MaxContextTokens,
			MaxOutput:    m.MaxOutputTokens,
			Capabilities: parseCapabilities(m.Capabilities),
			Description:  m.Description,
			Tags:         m.Tags,
			CreatedAt:    m.CreatedAt.String(),
			UpdatedAt:    m.UpdatedAt.String(),
		}
		if m.DeprecatedAt != nil {
			s := m.DeprecatedAt.String()
			item.DeprecatedAt = &s
		}
		if m.SunsetDate != nil {
			s := m.SunsetDate.Format("Y-m-d")
			item.SunsetDate = &s
		}
		item.ReplacementModel = m.ReplacementModel
		// 填充定价摘要（从 pricing JSONB 解析；tiered 取首档价，与旧列表口径一致）
		if p, ok := pricingMap[m.ID]; ok {
			item.PricingMode = p.BillingMode
			if blob := billing.ParsePricingBlob(p.Pricing); blob != nil {
				switch {
				case len(blob.Tiers) > 0:
					item.InputPrice = blob.Tiers[0].InputPrice
					item.OutputPrice = blob.Tiers[0].OutputPrice
				default:
					if blob.InputPrice != nil {
						item.InputPrice = *blob.InputPrice
					}
					if blob.OutputPrice != nil {
						item.OutputPrice = *blob.OutputPrice
					}
					if blob.Price != nil {
						item.PerRequestPrice = *blob.Price
					}
					if len(blob.Prices) > 0 {
						item.PerSecondPrices = blob.Prices
					}
				}
			}
		}
		// 填充可用渠道
		if chs, ok := channelMap[m.ModelId]; ok {
			item.Channels = chs
		}
		list = append(list, item)
	}

	return &v1.ModelListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// CreateModel 创建模型（自动创建默认 token 定价记录）
func (s *sAdmin) CreateModel(ctx context.Context, req *v1.ModelCreateReq) (*v1.ModelCreateRes, error) {
	count, err := dao.MdlModels.Ctx(ctx).Where(do.MdlModels{ModelId: req.ModelId}).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, gerror.NewCode(gcode.New(consts.CodeModelNameExists, consts.MsgModelNameExists, nil), consts.MsgModelNameExists)
	}

	insertData := do.MdlModels{
		ModelId:          req.ModelId,
		ModelName:        req.ModelName,
		Category:         req.Category,
		Status:           "active",
		MaxContextTokens: req.MaxContext,
		MaxOutputTokens:  req.MaxOutput,
		Description:      req.Description,
	}
	if req.Tags != nil {
		insertData.Tags = req.Tags
	} else {
		insertData.Tags = []string{}
	}
	if req.Capabilities != nil {
		capJson, _ := json.Marshal(req.Capabilities)
		insertData.Capabilities = string(capJson)
	}

	id, err := dao.MdlModels.Ctx(ctx).InsertAndGetId(insertData)
	if err != nil {
		return nil, err
	}

	// 默认 token 模式空定价行（未配价，SetModelPricing 时覆盖 pricing JSONB）
	_, err = dao.MdlPricing.Ctx(ctx).Insert(do.MdlPricing{
		ModelId:     id,
		BillingMode: "token",
		Pricing:     "{}",
	})
	if err != nil {
		return nil, err
	}

	return &v1.ModelCreateRes{ID: id}, nil
}

// UpdateModel 更新模型（含弃用状态管理）
func (s *sAdmin) UpdateModel(ctx context.Context, req *v1.ModelUpdateReq) (*v1.ModelUpdateRes, error) {
	var oldModel *struct {
		ModelId string `json:"model_id"`
		Status  string `json:"status"`
	}
	err := dao.MdlModels.Ctx(ctx).Where("id", req.ID).Fields("model_id, status").Scan(&oldModel)
	if err != nil || oldModel == nil {
		return nil, common.NewBusinessError(404, "模型不存在")
	}

	data := do.MdlModels{}
	hasUpdate := false
	var capUpdate *string // capabilities 需要单独处理

	if req.ModelName != "" {
		data.ModelName = req.ModelName
		hasUpdate = true
	}
	if req.Category != "" {
		data.Category = req.Category
		hasUpdate = true
	}
	data.MaxContextTokens = req.MaxContext
	data.MaxOutputTokens = req.MaxOutput
	hasUpdate = true
	if req.Capabilities != nil {
		capJson, _ := json.Marshal(req.Capabilities)
		s := string(capJson)
		capUpdate = &s
		hasUpdate = true
	}
	if req.Description != "" {
		data.Description = req.Description
		hasUpdate = true
	}
	if req.Tags != nil {
		data.Tags = req.Tags
		hasUpdate = true
	}

	statusChanged := req.Status != "" && req.Status != oldModel.Status

	if req.Status == "deprecated" {
		data.Status = "deprecated"
		data.DeprecatedAt = gtime.Now()
		hasUpdate = true
		if req.SunsetDate != nil && *req.SunsetDate != "" {
			data.SunsetDate = gtime.NewFromStr(*req.SunsetDate)
			hasUpdate = true
		}
		if req.ReplacementModel != "" {
			data.ReplacementModel = req.ReplacementModel
			hasUpdate = true
		}
	} else if req.Status == "active" {
		data.Status = "active"
		hasUpdate = true
	} else if req.Status == "offline" {
		data.Status = "offline"
		hasUpdate = true
	} else if req.Status != "" {
		data.Status = req.Status
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, nil
	}

	if capUpdate != nil {
		data.Capabilities = *capUpdate
	}

	_, err = dao.MdlModels.Ctx(ctx).Where("id", req.ID).Data(data).Update()
	if err != nil {
		return nil, err
	}

	if statusChanged {
		// 状态变更时清除模型缓存和租户分组缓存
		relay.NewDataProvider().InvalidateModelCache(oldModel.ModelId)
		invalidateTenantsForModel(ctx, req.ID)

		// 状态从 active → deprecated 时额外发送通知
		if req.Status == "deprecated" && oldModel.Status == "active" {
			go func() {
				bgCtx := context.Background()
				variables := g.Map{
					"model_name":        oldModel.ModelId,
					"sunset_date":       "",
					"replacement_model": req.ReplacementModel,
				}
				if req.SunsetDate != nil {
					variables["sunset_date"] = *req.SunsetDate
				}
				engine := common.NewNotificationEngine()
				if err := engine.SendToAllTenants(bgCtx, "model_deprecated", variables, ""); err != nil {
					g.Log().Errorf(bgCtx, "[ModelDeprecation] send notification for %s failed: %v", oldModel.ModelId, err)
				}
			}()
		}
	}

	return nil, nil
}

// DeleteModel 删除模型（同时删除定价记录、租户分配记录和分组关联）
func (s *sAdmin) DeleteModel(ctx context.Context, req *v1.ModelDeleteReq) (*v1.ModelDeleteRes, error) {
	// 先查出受影响的分组，供事务提交后清理缓存（缓存失效不可回滚，故放在事务外）
	var affectedGroups []struct {
		GroupId int64 `json:"group_id"`
	}
	dao.MdlGroupModels.Ctx(ctx).Where("model_id", req.ID).Fields("group_id").Scan(&affectedGroups)

	// 查出模型编码，供事务提交后按模型清除所有租户的价格缓存
	var model struct {
		ModelId string `json:"model_id"`
	}
	if err := dao.MdlModels.Ctx(ctx).Where("id", req.ID).Fields("model_id").Scan(&model); err != nil {
		if err = common.IgnoreScanNoRows(err); err != nil {
			return nil, err
		}
	}

	// 四表删除（定价 / 租户分配 / 分组关联 / 模型本体）放入同一事务，
	// 避免中途失败留下「模型已删但定价/分配记录残留」的孤儿数据。
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := dao.MdlPricing.Ctx(ctx).Where("model_id", req.ID).Delete(); err != nil {
			return gerror.Wrapf(err, "delete pricing tiers for model %d", req.ID)
		}
		if _, err := dao.MdlTenantModels.Ctx(ctx).Where("model_id", req.ID).Delete(); err != nil {
			return gerror.Wrapf(err, "delete tenant models for model %d", req.ID)
		}
		if _, err := dao.MdlGroupModels.Ctx(ctx).Where("model_id", req.ID).Delete(); err != nil {
			return gerror.Wrapf(err, "delete group models for model %d", req.ID)
		}
		if _, err := dao.MdlModels.Ctx(ctx).Where("id", req.ID).Delete(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后再清除受影响租户的缓存
	for _, ag := range affectedGroups {
		invalidateTenantsInGroup(ctx, ag.GroupId)
	}

	// 清除该模型在所有租户下的价格缓存，避免删除后旧价格残留最多 600s
	if model.ModelId != "" {
		billing.ClearModelPriceCache(ctx, model.ModelId)
	}

	return nil, nil
}

func parseCapabilities(raw string) map[string]bool {
	if raw == "" || raw == "{}" {
		return nil
	}
	var caps map[string]bool
	if err := json.Unmarshal([]byte(raw), &caps); err != nil {
		return nil
	}
	return caps
}

// GetModelPricing 获取模型定价（pricing JSONB 单行还原为 PricingItem 列表，响应结构与旧多行结构保持兼容）
func (s *sAdmin) GetModelPricing(ctx context.Context, req *v1.PricingGetReq) (*v1.PricingGetRes, error) {
	var row *struct {
		BillingMode     string  `json:"billing_mode" orm:"billing_mode"`
		Pricing         string  `json:"pricing" orm:"pricing"`
		PriceNote       *string `json:"price_note" orm:"price_note"`
		DiscountLabel   *string `json:"discount_label" orm:"discount_label"`
		PriceChangeNote *string `json:"price_change_note" orm:"price_change_note"`
	}

	err := dao.MdlPricing.Ctx(ctx).
		Where("model_id", req.ModelID).
		Scan(&row)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	result := make([]v1.PricingItem, 0, 1)
	var timeSegments []billing.TimeSegment
	var paramRules []billing.ParamRule
	var priceNote, discountLabel, priceChangeNote string
	if row != nil {
		mode := row.BillingMode
		blob := billing.ParsePricingBlob(row.Pricing)
		if blob != nil {
			paramRules = blob.ParamMultipliers
		}
		switch {
		case blob != nil && len(blob.Tiers) > 0:
			// tiered：tiers 数组还原为多行 PricingItem（与旧档位行结构一致）
			for _, tier := range blob.Tiers {
				result = append(result, v1.PricingItem{
					BillingMode: mode,
					MinTokens:   tier.MinTokens,
					MaxTokens:   tier.MaxTokens,
					InputPrice:  tier.InputPrice,
					OutputPrice: tier.OutputPrice,
				})
			}
		default:
			item := v1.PricingItem{BillingMode: mode}
			if blob != nil {
				if blob.InputPrice != nil {
					item.InputPrice = *blob.InputPrice
				}
				if blob.OutputPrice != nil {
					item.OutputPrice = *blob.OutputPrice
				}
				if blob.CacheReadPrice != nil {
					item.CacheReadPrice = *blob.CacheReadPrice
				}
				if blob.CacheCreationPrice != nil {
					item.CacheCreationPrice = *blob.CacheCreationPrice
				}
				if blob.Price != nil {
					p := *blob.Price
					item.PerRequestPrice = &p
				}
				if len(blob.Prices) > 0 {
					item.PerSecondPrices = blob.Prices
				}
				timeSegments = blob.TimeSegments
			}
			result = append(result, item)
		}
		priceNote = derefStr(row.PriceNote)
		discountLabel = derefStr(row.DiscountLabel)
		priceChangeNote = derefStr(row.PriceChangeNote)
	}
	return &v1.PricingGetRes{
		List:             result,
		TimeSegments:     timeSegmentsToAPI(timeSegments),
		ParamMultipliers: paramMultipliersToAPI(paramRules),
		PriceNote:        priceNote,
		DiscountLabel:    discountLabel,
		PriceChangeNote:  priceChangeNote,
	}, nil
}

// derefStr 字符串指针安全解引用（NULL → 空串）
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// timeSegmentsToAPI billing.TimeSegment → API 时段项
func timeSegmentsToAPI(segments []billing.TimeSegment) []v1.TimeSegmentItem {
	if len(segments) == 0 {
		return nil
	}
	result := make([]v1.TimeSegmentItem, 0, len(segments))
	for _, seg := range segments {
		result = append(result, v1.TimeSegmentItem{
			Name:       seg.Name,
			Days:       seg.Days,
			StartTime:  seg.StartTime,
			EndTime:    seg.EndTime,
			ValidFrom:  seg.ValidFrom,
			ValidTo:    seg.ValidTo,
			Multiplier: seg.Multiplier,
		})
	}
	return result
}

// timeSegmentsFromAPI API 时段项 → billing.TimeSegment（含校验）
func timeSegmentsFromAPI(items []v1.TimeSegmentItem) ([]billing.TimeSegment, error) {
	if len(items) == 0 {
		return nil, nil
	}
	result := make([]billing.TimeSegment, 0, len(items))
	for _, item := range items {
		result = append(result, billing.TimeSegment{
			Name:       item.Name,
			Days:       item.Days,
			StartTime:  item.StartTime,
			EndTime:    item.EndTime,
			ValidFrom:  item.ValidFrom,
			ValidTo:    item.ValidTo,
			Multiplier: item.Multiplier,
		})
	}
	if err := billing.ValidateTimeSegments(result); err != nil {
		return nil, err
	}
	return result, nil
}

// paramMultipliersFromAPI API 参数倍率规则 → billing 规则（含校验）
func paramMultipliersFromAPI(items []v1.ParamMultiplierItem) ([]billing.ParamRule, error) {
	if len(items) == 0 {
		return nil, nil
	}
	rules := make([]billing.ParamRule, 0, len(items))
	for _, item := range items {
		rule := billing.ParamRule{
			Multiplier: item.Multiplier,
			Note:       strings.TrimSpace(item.Note),
		}
		for _, c := range item.Conditions {
			rule.Conditions = append(rule.Conditions, billing.ParamCondition{
				Path:  strings.TrimSpace(c.Path),
				Match: c.Match,
				Value: c.Value,
			})
		}
		rules = append(rules, rule)
	}
	if err := billing.ValidateParamMultipliers(rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// paramMultipliersToAPI billing 规则 → API 参数倍率项
func paramMultipliersToAPI(rules []billing.ParamRule) []v1.ParamMultiplierItem {
	if len(rules) == 0 {
		return nil
	}
	items := make([]v1.ParamMultiplierItem, 0, len(rules))
	for _, rule := range rules {
		item := v1.ParamMultiplierItem{
			Multiplier: rule.Multiplier,
			Note:       rule.Note,
		}
		for _, c := range rule.Conditions {
			item.Conditions = append(item.Conditions, v1.ParamConditionItem{
				Path:  c.Path,
				Match: c.Match,
				Value: c.Value,
			})
		}
		items = append(items, item)
	}
	return items
}

// pricingItemsToInput API 定价项 → billing 构造输入
func pricingItemsToInput(items []v1.PricingItem) []billing.PricingItemInput {
	result := make([]billing.PricingItemInput, 0, len(items))
	for _, item := range items {
		result = append(result, billing.PricingItemInput{
			BillingMode:        item.BillingMode,
			MinTokens:          item.MinTokens,
			MaxTokens:          item.MaxTokens,
			InputPrice:         item.InputPrice,
			OutputPrice:        item.OutputPrice,
			PerRequestPrice:    item.PerRequestPrice,
			CacheReadPrice:     item.CacheReadPrice,
			CacheCreationPrice: item.CacheCreationPrice,
			PerSecondPrices:    item.PerSecondPrices,
		})
	}
	return result
}

// writePricingForModel 全量替换模型定价：单行写 billing_mode + pricing JSONB（每模型一行，
// uk_mdl_pricing_model 唯一）。SetModelPricing 与模型导入共用；时段定价随后经
// writeTimeSegmentsForModel 的 jsonb_set 并入 JSON 顶层；参数倍率规则随 blob 顶层写入。
// 调用方需保证事务 ctx 传播。
func writePricingForModel(ctx context.Context, modelDBID int64, items []v1.PricingItem, paramRules []billing.ParamRule) error {
	blob, err := billing.BuildPricingBlob(pricingItemsToInput(items))
	if err != nil {
		return err
	}
	blob.ParamMultipliers = paramRules
	blobJSON, err := json.Marshal(blob)
	if err != nil {
		return err
	}

	// 全量替换语义：删旧行插新行（保留展示字段列随 insert 重置为默认，随后由调用方回写）
	if _, err := dao.MdlPricing.Ctx(ctx).Where("model_id", modelDBID).Delete(); err != nil {
		return err
	}
	if _, err := dao.MdlPricing.Ctx(ctx).Insert(do.MdlPricing{
		ModelId:     modelDBID,
		BillingMode: items[0].BillingMode,
		Pricing:     string(blobJSON),
	}); err != nil {
		return err
	}
	return nil
}

// writeTimeSegmentsForModel 把时段定价并入模型定价行的 pricing JSONB 顶层（time_segments 键），
// 空列表=清除（全量替换语义，写 "[]" 而非删除键：nil 切片会序列化成 "null"，语义不明确）。
// 定价保存与模型导入共用。jsonb_set 只更新目标键，不覆盖 JSON 内其他计费字段。
// 单引号需 SQL 转义（json.Marshal 不转义内容中的单引号）。调用方需保证事务 ctx 传播。
func writeTimeSegmentsForModel(ctx context.Context, modelDBID int64, items []v1.TimeSegmentItem) error {
	segs, err := timeSegmentsFromAPI(items)
	if err != nil {
		return err
	}
	segmentsJSON := "[]"
	if len(segs) > 0 {
		data, mErr := json.Marshal(segs)
		if mErr != nil {
			return mErr
		}
		segmentsJSON = string(data)
	}
	escapedJSON := strings.ReplaceAll(segmentsJSON, "'", "''")
	_, err = dao.MdlPricing.Ctx(ctx).
		Where("model_id", modelDBID).
		Data(g.Map{
			"pricing": gdb.Raw(fmt.Sprintf(
				`jsonb_set(COALESCE(pricing, '{}'::jsonb), '{time_segments}', '%s'::jsonb)`, escapedJSON)),
		}).
		Update()
	return err
}

// writePricingDisplayFieldsForModel 把展示字段（价格说明/折扣标签/价格调整说明）写到模型定价行。
// 全量替换语义：空串=清除（写 NULL）。与 writeTimeSegmentsForModel 同理用列名直写而非 do 字段，
// 与 gf gen dao 生成物解耦。调用方需保证事务 ctx 传播。
func writePricingDisplayFieldsForModel(ctx context.Context, modelDBID int64, priceNote, discountLabel, priceChangeNote string) error {
	trimOrNull := func(s string) any {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		return s
	}
	_, err := dao.MdlPricing.Ctx(ctx).
		Where("model_id", modelDBID).
		Data(g.Map{
			"price_note":        trimOrNull(priceNote),
			"discount_label":    trimOrNull(discountLabel),
			"price_change_note": trimOrNull(priceChangeNote),
		}).
		Update()
	return err
}

// SetModelPricing 设置模型定价（全量替换）
func (s *sAdmin) SetModelPricing(ctx context.Context, req *v1.PricingSetReq) (*v1.PricingSetRes, error) {
	// 时段定价校验（乘数范围/时间格式/星期/日期边界），fail-fast 返回用户可读错误
	if _, err := timeSegmentsFromAPI(req.TimeSegments); err != nil {
		return nil, err
	}

	// 参数倍率规则校验（路径/匹配方式/值形态/倍率区间），fail-fast
	paramRules, err := paramMultipliersFromAPI(req.ParamMultipliers)
	if err != nil {
		return nil, err
	}

	// 先查模型编码，供事务提交后按模型清除所有租户的价格缓存
	var model struct {
		ModelId string `json:"model_id"`
	}
	if err := dao.MdlModels.Ctx(ctx).Where("id", req.ModelID).Fields("model_id").Scan(&model); err != nil {
		if err = common.IgnoreScanNoRows(err); err != nil {
			return nil, err
		}
	}

	// 全量替换：单行写 pricing JSONB（计费详情 + 参数倍率随 blob 顶层），同一事务。
	// 否则中途失败会残留「无定价」状态，导致计费失败或回退到默认价。
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := writePricingForModel(ctx, req.ModelID, req.Items, paramRules); err != nil {
			return err
		}

		// 时段定价并入 JSON 顶层（空=清除，全量替换语义；需在定价行插入之后执行）
		if err := writeTimeSegmentsForModel(ctx, req.ModelID, req.TimeSegments); err != nil {
			return err
		}

		// 展示字段写定价行（空=清除，全量替换语义；需在定价行插入之后执行）
		if err := writePricingDisplayFieldsForModel(ctx, req.ModelID, req.PriceNote, req.DiscountLabel, req.PriceChangeNote); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后再清价格缓存：模型基础定价变更影响所有租户，按模型编码全量失效，
	// 否则各租户缓存的旧价格（如新建时的零价）最多残留 600s，出现「已设价仍报未配置定价」。
	if model.ModelId != "" {
		billing.ClearModelPriceCache(ctx, model.ModelId)
	}

	return nil, nil
}

// FetchOfficialPricing 拉取模型官方定价（来自 LiteLLM + models.dev 双数据源）
func (s *sAdmin) FetchOfficialPricing(ctx context.Context, req *v1.PricingFetchOfficialReq) (*v1.PricingFetchOfficialRes, error) {
	var model *struct {
		ModelId string `json:"model_id"`
	}
	err := dao.MdlModels.Ctx(ctx).Where("id", req.ModelID).Fields("model_id").Scan(&model)
	if err != nil || model == nil || model.ModelId == "" {
		return nil, common.NewBusinessError(404, "模型不存在")
	}

	sources := make([]*v1.OfficialPricingSource, 0, 4)

	// Source 1: LiteLLM
	litellmSource := fetchLiteLLMSource(ctx, model.ModelId)
	sources = append(sources, litellmSource)

	// Source 2: models.dev
	modelsDevSource := fetchModelsDevSource(ctx, model.ModelId)
	sources = append(sources, modelsDevSource)

	// Source 3: BaseLLM
	baseLLMSource := fetchBaseLLMSource(ctx, model.ModelId)
	sources = append(sources, baseLLMSource)

	// Source 4: OpenRouter
	openRouterSource := fetchOpenRouterSource(ctx, model.ModelId)
	sources = append(sources, openRouterSource)

	return &v1.PricingFetchOfficialRes{
		ModelName: model.ModelId,
		Sources:   sources,
	}, nil
}

func fetchLiteLLMSource(ctx context.Context, modelName string) *v1.OfficialPricingSource {
	source := &v1.OfficialPricingSource{Source: "litellm"}

	data, err := common.FetchLiteLLMPricing(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialPricing] litellm fetch failed for %s: %v", modelName, err)
		source.Error = err.Error()
		return source
	}

	matchedName, entry := common.FindLiteLLMModel(data, modelName)
	if entry == nil {
		return source
	}

	// 修复定价单位转换：用 decimal 避免裸浮点乘法，将每 token 价格转换为每百万 token 价格
	million := decimal.NewFromInt(1_000_000)
	inputPrice := billing.RoundMoney(decimal.NewFromFloat(entry.InputCostPerToken).Mul(million))
	outputPrice := billing.RoundMoney(decimal.NewFromFloat(entry.OutputCostPerToken).Mul(million))
	cacheReadPrice := billing.RoundMoney(decimal.NewFromFloat(entry.CacheReadInputTokenCost).Mul(million))
	cacheCreationPrice := billing.RoundMoney(decimal.NewFromFloat(entry.CacheCreationInputTokenCost).Mul(million))

	billingMode := "token"
	if entry.Mode == "image_generation" && entry.OutputCostPerImage > 0 && inputPrice.IsZero() && outputPrice.IsZero() {
		billingMode = "per_request"
	}

	source.Found = true
	source.Provider = entry.LitellmProvider
	source.Mode = entry.Mode
	source.MaxContext = entry.MaxInputTokens
	source.MaxOutput = entry.MaxOutputTokens
	source.Pricing = &v1.OfficialPricingItem{
		InputPrice:         billing.InexactFloat64(inputPrice),
		OutputPrice:        billing.InexactFloat64(outputPrice),
		CacheReadPrice:     billing.InexactFloat64(cacheReadPrice),
		CacheCreationPrice: billing.InexactFloat64(cacheCreationPrice),
		BillingMode:        billingMode,
	}

	g.Log().Debugf(ctx, "[FetchOfficialPricing] litellm matched %s -> %s", modelName, matchedName)
	return source
}

func fetchModelsDevSource(ctx context.Context, modelName string) *v1.OfficialPricingSource {
	source := &v1.OfficialPricingSource{Source: "models.dev"}

	data, err := common.FetchModelsDevPricing(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialPricing] models.dev fetch failed for %s: %v", modelName, err)
		source.Error = err.Error()
		return source
	}

	matchedName, entry := common.FindModelsDevModel(data, modelName)
	if entry == nil {
		return source
	}

	cacheReadPrice := 0.0
	if entry.CacheRead != nil {
		cacheReadPrice = *entry.CacheRead
	}

	source.Found = true
	source.Provider = entry.Provider
	source.Pricing = &v1.OfficialPricingItem{
		InputPrice: roundTo2(entry.Input),
		OutputPrice: roundTo2(func() float64 {
			if entry.Output != nil {
				return *entry.Output
			}
			return 0
		}()),
		CacheReadPrice: roundTo2(cacheReadPrice),
		BillingMode:    "token",
	}

	g.Log().Debugf(ctx, "[FetchOfficialPricing] models.dev matched %s -> %s", modelName, matchedName)
	return source
}

func fetchBaseLLMSource(ctx context.Context, modelName string) *v1.OfficialPricingSource {
	source := &v1.OfficialPricingSource{Source: "basellm"}

	data, err := common.FetchBaseLLMPricing(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialPricing] basellm fetch failed for %s: %v", modelName, err)
		source.Error = err.Error()
		return source
	}

	matchedName, entry := common.FindBaseLLMModel(data, modelName)
	if entry == nil {
		return source
	}

	cacheReadPrice := 0.0
	if entry.PricePerMCacheRead != nil {
		cacheReadPrice = *entry.PricePerMCacheRead
	}
	cacheWritePrice := 0.0
	if entry.PricePerMCacheWrite != nil {
		cacheWritePrice = *entry.PricePerMCacheWrite
	}

	source.Found = true
	source.Provider = entry.VendorName
	source.MaxContext = common.ParseBaseLLMContext(entry.Tags)
	source.Pricing = &v1.OfficialPricingItem{
		InputPrice:         roundTo2(entry.PricePerMInput),
		OutputPrice:        roundTo2(entry.PricePerMOutput),
		CacheReadPrice:     roundTo2(cacheReadPrice),
		CacheCreationPrice: roundTo2(cacheWritePrice),
		BillingMode:        "token",
	}

	g.Log().Debugf(ctx, "[FetchOfficialPricing] basellm matched %s -> %s", modelName, matchedName)
	return source
}

func fetchOpenRouterSource(ctx context.Context, modelName string) *v1.OfficialPricingSource {
	source := &v1.OfficialPricingSource{Source: "openrouter"}

	data, err := common.FetchOpenRouterModels(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialPricing] openrouter fetch failed for %s: %v", modelName, err)
		source.Error = err.Error()
		return source
	}

	matchedName, entry := common.FindOpenRouterModel(data, modelName)
	if entry == nil || entry.Pricing == nil {
		return source
	}

	inputPrice := common.OpenRouterPricePerM(entry.Pricing.Prompt)
	outputPrice := common.OpenRouterPricePerM(entry.Pricing.Completion)
	cacheReadPrice := common.OpenRouterPricePerM(entry.Pricing.InputCacheRead)
	cacheWritePrice := common.OpenRouterPricePerM(entry.Pricing.InputCacheWrite)

	source.Found = true
	if idx := strings.Index(entry.ID, "/"); idx >= 0 {
		source.Provider = entry.ID[:idx]
	}
	source.MaxContext = entry.ContextLength
	if entry.TopProvider != nil && entry.TopProvider.MaxCompletionTokens > 0 {
		source.MaxOutput = entry.TopProvider.MaxCompletionTokens
	}
	source.Pricing = &v1.OfficialPricingItem{
		InputPrice:         roundTo2(inputPrice),
		OutputPrice:        roundTo2(outputPrice),
		CacheReadPrice:     roundTo2(cacheReadPrice),
		CacheCreationPrice: roundTo2(cacheWritePrice),
		BillingMode:        "token",
	}

	g.Log().Debugf(ctx, "[FetchOfficialPricing] openrouter matched %s -> %s", modelName, matchedName)
	return source
}

func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}

// FetchOfficialModelInfo 按模型名称拉取官方模型信息（上下文长度+能力特性）
// 按优先级尝试 LiteLLM → OpenRouter → BaseLLM，返回第一个匹配的结果
func (s *sAdmin) FetchOfficialModelInfo(ctx context.Context, req *v1.ModelFetchOfficialInfoReq) (*v1.ModelFetchOfficialInfoRes, error) {
	var lastError string

	// Source 1: LiteLLM
	if data, err := common.FetchLiteLLMPricing(ctx); err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialModelInfo] litellm fetch failed for %s: %v", req.ModelName, err)
		lastError = err.Error()
	} else if _, entry := common.FindLiteLLMModel(data, req.ModelName); entry != nil {
		return &v1.ModelFetchOfficialInfoRes{
			Found:            true,
			Provider:         entry.LitellmProvider,
			MaxContextTokens: entry.MaxInputTokens,
			MaxOutputTokens:  entry.MaxOutputTokens,
			Capabilities: map[string]bool{
				"vision":                    entry.SupportsVision,
				"function_calling":          entry.SupportsFunctionCalling,
				"parallel_function_calling": entry.SupportsParallelFuncCalling,
				"tool_choice":               entry.SupportsToolChoice,
				"response_schema":           entry.SupportsResponseSchema,
				"system_messages":           entry.SupportsSystemMessages,
				"prompt_caching":            entry.SupportsPromptCaching,
				"audio_input":               entry.SupportsAudioInput,
				"audio_output":              entry.SupportsAudioOutput,
				"pdf_input":                 entry.SupportsPdfInput,
				"embedding_image":           entry.SupportsEmbeddingImage,
				"reasoning":                 entry.SupportsReasoning,
				"web_search":                entry.SupportsWebSearch,
			},
		}, nil
	}

	// Source 2: OpenRouter
	if data, err := common.FetchOpenRouterModels(ctx); err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialModelInfo] openrouter fetch failed for %s: %v", req.ModelName, err)
		lastError = err.Error()
	} else if _, entry := common.FindOpenRouterModel(data, req.ModelName); entry != nil {
		maxOutput := 0
		if entry.TopProvider != nil {
			maxOutput = entry.TopProvider.MaxCompletionTokens
		}
		provider := ""
		if idx := strings.Index(entry.ID, "/"); idx >= 0 {
			provider = entry.ID[:idx]
		}
		return &v1.ModelFetchOfficialInfoRes{
			Found:            true,
			Provider:         provider,
			MaxContextTokens: entry.ContextLength,
			MaxOutputTokens:  maxOutput,
			Capabilities:     common.ParseOpenRouterCapabilities(entry),
		}, nil
	}

	// Source 3: BaseLLM
	if data, err := common.FetchBaseLLMPricing(ctx); err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialModelInfo] basellm fetch failed for %s: %v", req.ModelName, err)
		lastError = err.Error()
	} else if _, entry := common.FindBaseLLMModel(data, req.ModelName); entry != nil {
		return &v1.ModelFetchOfficialInfoRes{
			Found:            true,
			Provider:         entry.VendorName,
			MaxContextTokens: common.ParseBaseLLMContext(entry.Tags),
			Capabilities:     common.ParseBaseLLMCapabilities(entry.Tags),
		}, nil
	}

	res := &v1.ModelFetchOfficialInfoRes{Found: false}
	if lastError != "" {
		res.Error = lastError
	}
	return res, nil
}

// ExportModels exports model list to CSV or Excel.
func (s *sAdmin) ExportModels(ctx context.Context, req *v1.ModelExportReq) (*v1.ModelExportRes, error) {
	modelFields := "id, model_id, model_name, category, status, max_context_tokens, max_output_tokens, created_at"

	config := export.Config{
		Format:   req.Format,
		Filename: "模型_" + gtime.Now().Format("Ymd_His"),
		Columns: []export.Column{
			{Field: "id", Header: "ID"},
			{Field: "model_id", Header: "模型标识"},
			{Field: "model_name", Header: "显示名称"},
			{Field: "category", Header: "分类"},
			{Field: "status", Header: "状态"},
			{Field: "max_context_tokens", Header: "最大上下文"},
			{Field: "max_output_tokens", Header: "最大输出"},
			{Field: "created_at", Header: "创建时间"},
		},
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			query := dao.MdlModels.Ctx(ctx)
			if req.Category != "" {
				query = query.Where("category", req.Category)
			}
			if req.Status != "" {
				query = query.Where("status", req.Status)
			}
			if req.Search != "" {
				query = query.Where("model_id LIKE ? OR model_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
			}
			var batch []struct {
				ID               int64       `json:"id"`
				ModelId          string      `json:"model_id"`
				ModelName        string      `json:"model_name"`
				Category         string      `json:"category"`
				Status           string      `json:"status"`
				MaxContextTokens int         `json:"max_context_tokens"`
				MaxOutputTokens  int         `json:"max_output_tokens"`
				CreatedAt        *gtime.Time `json:"created_at"`
			}
			if err := query.Fields(modelFields).OrderDesc("id").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				g.Log().Errorf(ctx, "ExportModels: query batch at offset %d failed: %v", offset, err)
				return
			}
			for _, m := range batch {
				if !yield(map[string]any{
					"id":                 m.ID,
					"model_id":           m.ModelId,
					"model_name":         m.ModelName,
					"category":           m.Category,
					"status":             m.Status,
					"max_context_tokens": m.MaxContextTokens,
					"max_output_tokens":  m.MaxOutputTokens,
					"created_at":         m.CreatedAt.String(),
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

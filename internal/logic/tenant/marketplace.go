package tenant

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/model/entity"
)

// marketplacePricingRow 定价锚点行（min_tokens=0）：全部计费模式的基础价载体
// （token 默认价 / tiered 首档价 / per_request 单价），另带时段定价与对外展示字段。
// 显式列名扫描进独立结构体，与 do/entity 生成物解耦（新展示列需迁移落库 + gf gen dao 后才进生成物）。
type marketplacePricingRow struct {
	ModelID            int64    `json:"model_id"`
	BillingMode        string   `json:"billing_mode"`
	InputPrice         float64  `json:"input_price"`
	OutputPrice        float64  `json:"output_price"`
	PerRequestPrice    *float64 `json:"per_request_price"`
	CacheReadPrice     float64  `json:"cache_read_price"`
	CacheCreationPrice float64  `json:"cache_creation_price"`
	TimeSegments       string   `json:"time_segments"`
	DiscountLabel      *string  `json:"discount_label"`
	PriceChangeNote    *string  `json:"price_change_note"`
}

// GetModelList 获取模型广场列表（从默认模型分组加载）
func (s *sTenant) GetModelList(ctx context.Context, req *v1.MarketplaceListReq) (*v1.MarketplaceListRes, error) {
	// 1. 查找默认模型分组
	var defaultGroup *entity.MdlModelGroups
	err := dao.MdlModelGroups.Ctx(ctx).
		Where("is_default", true).
		Where("status", "active").
		Scan(&defaultGroup)
	if err != nil {
		return nil, err
	}
	if defaultGroup == nil {
		// 没有默认分组，返回空列表
		return &v1.MarketplaceListRes{
			List:     []v1.MarketplaceModelItem{},
			Total:    0,
			Page:     req.Page,
			PageSize: req.PageSize,
		}, nil
	}

	// 2. 构建查询：通过分组关联查询模型
	m := dao.MdlModels.Ctx(ctx).
		LeftJoin("mdl_group_models", "mdl_group_models.model_id = mdl_models.id").
		Where("mdl_group_models.group_id", defaultGroup.Id).
		Where("mdl_models.status", "active")

	// 搜索关键词
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		m = m.Where("(mdl_models.model_id LIKE ? OR mdl_models.model_name LIKE ? OR mdl_models.description LIKE ?)", keyword, keyword, keyword)
	}

	// 分类筛选
	if req.Category != "" {
		m = m.Where("mdl_models.category", req.Category)
	}

	// 排序：按类别、模型名称
	m = m.OrderAsc("mdl_models.category").OrderAsc("mdl_models.model_id")

	// 3. 查询总数
	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	// 4. 分页查询模型
	var models []*entity.MdlModels
	err = m.Fields("mdl_models.*").
		Page(req.Page, req.PageSize).
		Scan(&models)
	if err != nil {
		return nil, err
	}

	// 5. 批量查询价格（从 mdl_pricing 表）
	if len(models) == 0 {
		return &v1.MarketplaceListRes{
			List:     []v1.MarketplaceModelItem{},
			Total:    0,
			Page:     req.Page,
			PageSize: req.PageSize,
		}, nil
	}

	modelIds := make([]int64, len(models))
	for i, model := range models {
		modelIds[i] = model.Id
	}

	var pricings []marketplacePricingRow
	err = dao.MdlPricing.Ctx(ctx).
		WhereIn("model_id", modelIds).
		Where("min_tokens", 0). // 锚点行承载全部计费模式的基础价与展示字段
		Scan(&pricings)
	if err != nil {
		return nil, err
	}

	// 构建价格映射（model_id -> pricing）
	priceMap := make(map[int64]*marketplacePricingRow, len(pricings))
	for i := range pricings {
		priceMap[pricings[i].ModelID] = &pricings[i]
	}

	// 6. 转换为响应格式
	list := make([]v1.MarketplaceModelItem, 0, len(models))
	for _, model := range models {
		item := s.convertToMarketplaceItem(ctx, model, priceMap[model.Id])
		list = append(list, item)
	}

	return &v1.MarketplaceListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetModelDetail 获取模型详情
func (s *sTenant) GetModelDetail(ctx context.Context, req *v1.MarketplaceDetailReq) (*v1.MarketplaceDetailRes, error) {
	// 1. 检查模型是否在默认分组中
	var defaultGroup *entity.MdlModelGroups
	err := dao.MdlModelGroups.Ctx(ctx).
		Where("is_default", true).
		Where("status", "active").
		Scan(&defaultGroup)
	if err != nil {
		return nil, err
	}
	if defaultGroup == nil {
		return nil, fmt.Errorf("默认模型分组不存在")
	}

	// 2. 查询模型（需在默认分组中）
	var model *entity.MdlModels
	err = dao.MdlModels.Ctx(ctx).
		LeftJoin("mdl_group_models", "mdl_group_models.model_id = mdl_models.id").
		Where("mdl_group_models.group_id", defaultGroup.Id).
		Where("mdl_models.model_id", req.ModelId).
		Where("mdl_models.status", "active").
		Fields("mdl_models.*").
		Scan(&model)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, fmt.Errorf("模型不存在或未公开")
	}

	// 3. 查询价格（锚点行：全部计费模式的基础价 + 时段/展示字段）
	var pricing *marketplacePricingRow
	err = dao.MdlPricing.Ctx(ctx).
		Where("model_id", model.Id).
		Where("min_tokens", 0).
		Scan(&pricing)
	if err != nil {
		return nil, err
	}

	// 4. 转换为响应格式
	item := s.convertToMarketplaceItem(ctx, model, pricing)

	return &v1.MarketplaceDetailRes{
		MarketplaceModelItem: item,
	}, nil
}

// convertToMarketplaceItem 转换模型实体为响应格式
func (s *sTenant) convertToMarketplaceItem(ctx context.Context, model *entity.MdlModels, pricing *marketplacePricingRow) v1.MarketplaceModelItem {
	item := v1.MarketplaceModelItem{
		ModelId:          model.ModelId,
		ModelName:        model.ModelName,
		Category:         model.Category,
		Description:      model.Description,
		MaxContextTokens: model.MaxContextTokens,
		MaxOutputTokens:  model.MaxOutputTokens,
		Tags:             []string{},
		Capabilities:     g.Map{},
	}

	// 标签
	if model.Tags != nil && len(model.Tags) > 0 {
		item.Tags = model.Tags
	}

	// 能力标签（从 capabilities JSONB 解析）
	if model.Capabilities != "" {
		var capabilities map[string]interface{}
		if err := gconv.Struct(model.Capabilities, &capabilities); err == nil {
			item.Capabilities = capabilities
		}
	}

	if pricing == nil {
		return item
	}

	billingMode := pricing.BillingMode
	if billingMode == "" {
		billingMode = "token"
	}
	item.BillingMode = billingMode
	// 锚点行价即对外展示价（数据库存的就是每 1M token 美元价，直接返回）：
	// token=默认价，tiered=首档价（前端标「起」），per_request=按次单价
	item.InputPrice = pricing.InputPrice
	item.OutputPrice = pricing.OutputPrice
	item.CacheReadPrice = pricing.CacheReadPrice
	item.CacheCreationPrice = pricing.CacheCreationPrice
	if pricing.PerRequestPrice != nil {
		item.PerRequestPrice = *pricing.PerRequestPrice
	}
	if pricing.DiscountLabel != nil {
		item.DiscountLabel = *pricing.DiscountLabel
	}
	if pricing.PriceChangeNote != nil {
		item.PriceChangeNote = *pricing.PriceChangeNote
	}

	// 时段价目：平台基础价 × 时段乘数换算（tiered 用锚点首档价换算，与 buildTiers 首档口径一致）
	if pricing.TimeSegments != "" && pricing.TimeSegments != "null" {
		var segments []billing.TimeSegment
		if err := json.Unmarshal([]byte(pricing.TimeSegments), &segments); err == nil && len(segments) > 0 {
			var tiers []v1.PricingTierItem
			if billingMode == "tiered" {
				tiers = append(tiers, v1.PricingTierItem{InputPrice: pricing.InputPrice, OutputPrice: pricing.OutputPrice})
			}
			item.TimePrices = buildTimePrices(segments, billingMode, &pricing.InputPrice, &pricing.OutputPrice, pricing.PerRequestPrice, tiers)
		}
	}

	return item
}

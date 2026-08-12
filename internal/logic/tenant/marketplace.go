package tenant

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/model/entity"
)

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

	var pricings []*entity.MdlPricing
	err = dao.MdlPricing.Ctx(ctx).
		WhereIn("model_id", modelIds).
		Where("billing_mode", "token"). // 只取 token 计费模式的基础价格
		Scan(&pricings)
	if err != nil {
		return nil, err
	}

	// 构建价格映射（model_id -> pricing）
	priceMap := make(map[int64]*entity.MdlPricing)
	for _, pricing := range pricings {
		priceMap[pricing.ModelId] = pricing
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

	// 3. 查询价格
	var pricing *entity.MdlPricing
	err = dao.MdlPricing.Ctx(ctx).
		Where("model_id", model.Id).
		Where("billing_mode", "token").
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
func (s *sTenant) convertToMarketplaceItem(ctx context.Context, model *entity.MdlModels, pricing *entity.MdlPricing) v1.MarketplaceModelItem {
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

	// 价格（数据库已存储每百万 token 的美元价格，直接返回）
	if pricing != nil {
		// input_price/output_price 数据库字段存储的就是每 1M token 价格（USD）
		// 无需转换，直接使用
		item.InputPrice = billing.InexactFloat64(pricing.InputPrice)
		item.OutputPrice = billing.InexactFloat64(pricing.OutputPrice)
	}

	return item
}

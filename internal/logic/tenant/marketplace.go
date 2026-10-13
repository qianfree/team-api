package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/entity"
)

// marketplacePricingRow 模型定价行（每模型一行）：billing_mode + pricing JSONB 为计费真相，
// 另带对外展示字段。显式列名扫描进独立结构体，与 do/entity 生成物解耦。
type marketplacePricingRow struct {
	ModelID         int64   `json:"model_id"`
	BillingMode     string  `json:"billing_mode"`
	Pricing         string  `json:"pricing"` // pricing JSONB（唯一真相）
	DiscountLabel   *string `json:"discount_label"`
	PriceChangeNote *string `json:"price_change_note"`
}

// marketplaceDefaultGroup 查找启用中的默认模型分组（模型广场数据源）
func (s *sTenant) marketplaceDefaultGroup(ctx context.Context) (*entity.MdlModelGroups, error) {
	var defaultGroup *entity.MdlModelGroups
	err := dao.MdlModelGroups.Ctx(ctx).
		Where("is_default", true).
		Where("status", "active").
		Scan(&defaultGroup)
	if err != nil {
		return nil, err
	}
	return defaultGroup, nil
}

// splitCSV 逗号分隔参数拆分为去空格、去重的值列表
func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := make(map[string]struct{})
	values := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

// mergeCSV 多选参数与旧单选参数合并（单选值并入多选集合，保持旧参数兼容）
func mergeCSV(multi string, single string) []string {
	values := splitCSV(multi)
	if single == "" {
		return values
	}
	for _, value := range values {
		if value == single {
			return values
		}
	}
	return append(values, single)
}

// boolKeySet 值列表 → 键集合
func boolKeySet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

// applyMarketplaceSort 列表排序分支（白名单已由 API 层 v:"in:" 校验）。
// 价格排序走 mdl_pricing.sort_price（定价写入路径维护的基准价列，NULL=未配价落尾部），
// 经标量子查询取值避免 join 破坏 entity 扫描
func applyMarketplaceSort(m *gdb.Model, sort string) *gdb.Model {
	switch sort {
	case "newest":
		return m.Order("mdl_models.created_at DESC, mdl_models.model_id ASC")
	case "price_asc":
		return m.Order("(SELECT sp.sort_price FROM mdl_pricing sp WHERE sp.model_id = mdl_models.id) ASC NULLS LAST, mdl_models.model_id ASC")
	case "price_desc":
		return m.Order("(SELECT sp.sort_price FROM mdl_pricing sp WHERE sp.model_id = mdl_models.id) DESC NULLS LAST, mdl_models.model_id ASC")
	case "context_desc":
		return m.Order("mdl_models.max_context_tokens DESC, mdl_models.model_id ASC")
	default:
		return m.OrderAsc("mdl_models.category").OrderAsc("mdl_models.model_id")
	}
}

// GetModelList 获取模型广场列表（从默认模型分组加载）
func (s *sTenant) GetModelList(ctx context.Context, req *v1.MarketplaceListReq) (*v1.MarketplaceListRes, error) {
	defaultGroup, err := s.marketplaceDefaultGroup(ctx)
	if err != nil {
		return nil, err
	}
	if defaultGroup == nil {
		return &v1.MarketplaceListRes{
			List:     []v1.MarketplaceModelItem{},
			Total:    0,
			Page:     req.Page,
			PageSize: req.PageSize,
			Vendors:  []string{},
			Facets:   emptyMarketplaceFacets(),
		}, nil
	}

	// 多选筛选解析（品类/厂商旧单选参数并入多选集合）
	categories := mergeCSV(req.Categories, req.Category)
	vendors := mergeCSV(req.Vendors, req.Vendor)
	capabilities := splitCSV(req.Capabilities)

	// 基础查询：默认分组 + active + 关键词。主列表与分面统计共用同一套基础条件
	// （闭包各自构建链路，避免 gf Model 链式分支共享底层状态的歧义）。
	// 计费筛选与价格排序经 mdl_pricing 的标量子查询表达而非 LEFT JOIN：主查询保持
	// 无 pricing join（mdl_group_models 之外不引入新 join 面），子查询写法与 admin
	// 侧 pricedPredicate 惯用法一致
	newQuery := func() *gdb.Model {
		q := dao.MdlModels.Ctx(ctx).
			LeftJoin("mdl_group_models", "mdl_group_models.model_id = mdl_models.id").
			Where("mdl_group_models.group_id", defaultGroup.Id).
			Where("mdl_models.status", "active")
		if req.Keyword != "" {
			keyword := "%" + common.EscapeLikePattern(req.Keyword) + "%"
			q = q.Where("(mdl_models.model_id LIKE ? OR mdl_models.model_name LIKE ? OR mdl_models.description LIKE ?)", keyword, keyword, keyword)
		}
		return q
	}

	// 维度筛选（对主列表与分面统计的「其余维度谓词」共用同一语义）
	applyDimensionFilters := func(q *gdb.Model) *gdb.Model {
		if len(categories) > 0 {
			q = q.WhereIn("mdl_models.category", categories)
		}
		if len(vendors) > 0 {
			q = q.WhereIn("mdl_models.vendor", vendors)
		}
		if len(capabilities) > 0 {
			// JSONB 包含匹配，AND 语义（需全部具备）。能力值由管理后台写入为布尔，
			// @> 匹配与前端 truthy 判定（true/1/"true"）在布尔形态下一致
			contain := make(map[string]bool, len(capabilities))
			for _, key := range capabilities {
				contain[key] = true
			}
			raw, _ := json.Marshal(contain)
			q = q.Where("mdl_models.capabilities @> ?::jsonb", string(raw))
		}
		if req.BillingMode != "" {
			// 计费模式经标量子查询归一：无定价行/空模式兜底 token（与展示侧口径一致）
			q = q.Where(
				"COALESCE(NULLIF((SELECT sp.billing_mode FROM mdl_pricing sp WHERE sp.model_id = mdl_models.id), ''), 'token') = ?",
				req.BillingMode,
			)
		}
		return q
	}

	m := applyMarketplaceSort(applyDimensionFilters(newQuery()), req.Sort)

	// 查询总数
	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	// 分面统计：一条投影查询拉回 keyword 基础条件下的全量候选行，Go 内存析取聚合。
	// 目录超过阈值时跳过统计（增长阀），计数返回空、vendors 列表返回空，前端隐藏计数
	facets := emptyMarketplaceFacets()
	vendorOptions := []string{}
	if total <= marketplaceFacetRowLimit {
		var facetRows []marketplaceFacetRow
		err = newQuery().
			Fields("mdl_models.category, mdl_models.vendor, mdl_models.capabilities, (SELECT sp.billing_mode FROM mdl_pricing sp WHERE sp.model_id = mdl_models.id) AS billing_mode").
			Scan(&facetRows)
		if err != nil {
			return nil, err
		}
		facets, vendorOptions = aggregateMarketplaceFacets(facetRows, marketplaceFacetFilters{
			Categories:   boolKeySet(categories),
			Vendors:      boolKeySet(vendors),
			Capabilities: boolKeySet(capabilities),
			BillingMode:  req.BillingMode,
		})
	}

	// 分页查询模型
	var models []*entity.MdlModels
	err = m.Fields("mdl_models.*").
		Page(req.Page, req.PageSize).
		Scan(&models)
	if err != nil {
		return nil, err
	}

	if len(models) == 0 {
		return &v1.MarketplaceListRes{
			List:     []v1.MarketplaceModelItem{},
			Total:    total,
			Page:     req.Page,
			PageSize: req.PageSize,
			Vendors:  vendorOptions,
			Facets:   facets,
		}, nil
	}

	// 批量查询价格并转换为响应格式
	list, err := s.convertMarketplaceModels(ctx, models)
	if err != nil {
		return nil, err
	}

	return &v1.MarketplaceListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Vendors:  vendorOptions,
		Facets:   facets,
	}, nil
}

// GetMarketplaceShowcase 获取模型广场展示带：推荐位（运营精选，display_order 排序）
// 与上新位（运营标记，created_at 倒序）。均为编辑推荐位，不参与用户筛选
func (s *sTenant) GetMarketplaceShowcase(ctx context.Context, req *v1.MarketplaceShowcaseReq) (*v1.MarketplaceShowcaseRes, error) {
	res := &v1.MarketplaceShowcaseRes{
		Recommended: []v1.MarketplaceModelItem{},
		Newest:      []v1.MarketplaceModelItem{},
	}

	defaultGroup, err := s.marketplaceDefaultGroup(ctx)
	if err != nil {
		return nil, err
	}
	if defaultGroup == nil {
		return res, nil
	}

	baseQuery := func() *gdb.Model {
		return dao.MdlModels.Ctx(ctx).
			LeftJoin("mdl_group_models", "mdl_group_models.model_id = mdl_models.id").
			Where("mdl_group_models.group_id", defaultGroup.Id).
			Where("mdl_models.status", "active")
	}

	var recommended []*entity.MdlModels
	err = baseQuery().
		Where("mdl_models.is_recommended", true).
		OrderAsc("mdl_models.display_order").
		OrderAsc("mdl_models.model_id").
		Limit(8).
		Fields("mdl_models.*").
		Scan(&recommended)
	if err != nil {
		return nil, err
	}

	var newest []*entity.MdlModels
	err = baseQuery().
		Where("mdl_models.is_new", true).
		OrderDesc("mdl_models.created_at").
		OrderAsc("mdl_models.model_id").
		Limit(8).
		Fields("mdl_models.*").
		Scan(&newest)
	if err != nil {
		return nil, err
	}

	if len(recommended) > 0 {
		items, err := s.convertMarketplaceModels(ctx, recommended)
		if err != nil {
			return nil, err
		}
		res.Recommended = items
	}
	if len(newest) > 0 {
		items, err := s.convertMarketplaceModels(ctx, newest)
		if err != nil {
			return nil, err
		}
		res.Newest = items
	}
	return res, nil
}

// convertMarketplaceModels 批量查询定价并转换为响应格式（列表/展示带共用）
func (s *sTenant) convertMarketplaceModels(ctx context.Context, models []*entity.MdlModels) ([]v1.MarketplaceModelItem, error) {
	modelIds := make([]int64, len(models))
	for i, model := range models {
		modelIds[i] = model.Id
	}

	var pricings []marketplacePricingRow
	err := dao.MdlPricing.Ctx(ctx).
		WhereIn("model_id", modelIds).
		Scan(&pricings)
	if err != nil {
		return nil, err
	}
	priceMap := make(map[int64]*marketplacePricingRow, len(pricings))
	for i := range pricings {
		priceMap[pricings[i].ModelID] = &pricings[i]
	}

	list := make([]v1.MarketplaceModelItem, 0, len(models))
	for _, model := range models {
		list = append(list, s.convertToMarketplaceItem(ctx, model, priceMap[model.Id]))
	}
	return list, nil
}

// GetModelDetail 获取模型详情
func (s *sTenant) GetModelDetail(ctx context.Context, req *v1.MarketplaceDetailReq) (*v1.MarketplaceDetailRes, error) {
	defaultGroup, err := s.marketplaceDefaultGroup(ctx)
	if err != nil {
		return nil, err
	}
	if defaultGroup == nil {
		return nil, fmt.Errorf("默认模型分组不存在")
	}

	// 查询模型（需在默认分组中）
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

	// 查询价格（每模型一行：billing_mode + pricing JSONB + 展示字段）
	var pricing *marketplacePricingRow
	err = dao.MdlPricing.Ctx(ctx).
		Where("model_id", model.Id).
		Scan(&pricing)
	if err != nil {
		return nil, err
	}

	return &v1.MarketplaceDetailRes{
		MarketplaceModelItem: s.convertToMarketplaceItem(ctx, model, pricing),
	}, nil
}

// convertToMarketplaceItem 转换模型实体为响应格式
func (s *sTenant) convertToMarketplaceItem(ctx context.Context, model *entity.MdlModels, pricing *marketplacePricingRow) v1.MarketplaceModelItem {
	item := v1.MarketplaceModelItem{
		ModelId:          model.ModelId,
		ModelName:        model.ModelName,
		Category:         model.Category,
		Vendor:           model.Vendor,
		Description:      model.Description,
		MaxContextTokens: model.MaxContextTokens,
		MaxOutputTokens:  model.MaxOutputTokens,
		IsRecommended:    model.IsRecommended,
		IsNew:            model.IsNew,
		Tags:             []string{},
		Capabilities:     g.Map{},
	}

	// 上架日期（展示与上新条排序参考）
	if model.CreatedAt != nil {
		item.CreatedAt = model.CreatedAt.Format("Y-m-d")
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

	// pricing JSONB 解析（唯一真相）：token=默认价，tiered=首档价（前端标「起」），per_request=按次单价
	blob := billing.ParsePricingBlob(pricing.Pricing)
	baseInput, baseOutput, baseCacheRead, baseCacheCreation, basePerRequest := fillFromBlob(blob)
	item.InputPrice = baseInput
	item.OutputPrice = baseOutput
	item.CacheReadPrice = baseCacheRead
	item.CacheCreationPrice = baseCacheCreation
	if basePerRequest != nil {
		item.PerRequestPrice = *basePerRequest
	}
	// 按秒/特殊计费：矩阵来自平台 pricing JSONB（展示用）；special 的矩阵是
	// 输出生成组件的参考单价（素材组件单价在方案配置中，不随广场下发）
	if (billingMode == "per_second" || billingMode == billing.BillingModeSpecial) && blob != nil {
		item.PerSecondPrices = blob.Prices
	}
	if pricing.DiscountLabel != nil {
		item.DiscountLabel = *pricing.DiscountLabel
	}
	if pricing.PriceChangeNote != nil {
		item.PriceChangeNote = *pricing.PriceChangeNote
	}

	// 时段价目：平台基础价 × 时段乘数换算（tiered 用首档价换算，与 buildTiers 首档口径一致）
	if blob != nil && len(blob.TimeSegments) > 0 {
		var tiers []v1.PricingTierItem
		if billingMode == "tiered" && len(blob.Tiers) > 0 {
			tiers = append(tiers, v1.PricingTierItem{InputPrice: blob.Tiers[0].InputPrice, OutputPrice: blob.Tiers[0].OutputPrice})
		}
		in, out := baseInput, baseOutput
		item.TimePrices = buildTimePrices(blob.TimeSegments, billingMode, &in, &out, basePerRequest, tiers, item.PerSecondPrices)
	}

	return item
}

// emptyMarketplaceFacets 空分面（各维度返回空 map，前端按「无计数」渲染）
func emptyMarketplaceFacets() v1.MarketplaceFacets {
	return v1.MarketplaceFacets{
		Categories:   map[string]int{},
		Vendors:      map[string]int{},
		Capabilities: map[string]int{},
		BillingModes: map[string]int{},
	}
}

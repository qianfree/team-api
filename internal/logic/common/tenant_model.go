package common

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/relay/constant"
)

// AvailableModelItem 租户可用模型基本信息（公共结构体，不含价格）
type AvailableModelItem struct {
	ModelDBID        int64  `json:"model_db_id"`
	ModelId          string `json:"model_id"`
	ModelName        string `json:"model_name"`
	Category         string `json:"category"`
	MaxContextTokens int    `json:"max_context_tokens"`
	MaxOutputTokens  int    `json:"max_output_tokens"`
	Description      string `json:"description"`
	Tags             string `json:"tags"`
	Capabilities     string `json:"capabilities"`
	Source           string `json:"source"` // "explicit" 或 "group"
}

// GetTenantAvailableModels 计算租户实际可用的所有模型（显式分配 + 分组来源，去重）
// 返回模型基本信息列表，不含价格。可被管理后台和租户控制台共用。
func GetTenantAvailableModels(ctx context.Context, tenantID int64, category, search string) ([]AvailableModelItem, error) {
	// 阶段1：获取显式分配的模型
	var explicitModels []struct {
		ModelDBID        int64  `json:"model_db_id"`
		ModelId          string `json:"model_id"`
		ModelName        string `json:"model_name"`
		Category         string `json:"category"`
		MaxContextTokens int    `json:"max_context_tokens"`
		MaxOutputTokens  int    `json:"max_output_tokens"`
		Description      string `json:"description"`
		Tags             string `json:"tags"`
		Capabilities     string `json:"capabilities"`
	}

	explicitQuery := dao.MdlTenantModels.Ctx(ctx).
		LeftJoin("mdl_models m ON mdl_tenant_models.model_id = m.id").
		Where("mdl_tenant_models.tenant_id", tenantID).
		Where("mdl_tenant_models.enabled", true).
		Where("m.status", "active")

	if category != "" {
		explicitQuery = explicitQuery.Where("m.category", category)
	}
	if search != "" {
		explicitQuery = explicitQuery.Where("m.model_id LIKE ? OR m.model_name LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	err := explicitQuery.
		Fields("mdl_tenant_models.model_id AS model_db_id, m.model_id, m.model_name, m.category, m.max_context_tokens, m.max_output_tokens, m.description, m.tags, m.capabilities").
		OrderAsc("m.category").
		OrderAsc("m.model_id").
		Scan(&explicitModels)
	if err != nil {
		return nil, err
	}

	result := make([]AvailableModelItem, 0, len(explicitModels))
	explicitModelIDs := make(map[int64]bool, len(explicitModels))
	for _, r := range explicitModels {
		explicitModelIDs[r.ModelDBID] = true
		result = append(result, AvailableModelItem{
			ModelDBID:        r.ModelDBID,
			ModelId:          r.ModelId,
			ModelName:        r.ModelName,
			Category:         r.Category,
			MaxContextTokens: r.MaxContextTokens,
			MaxOutputTokens:  r.MaxOutputTokens,
			Description:      r.Description,
			Tags:             r.Tags,
			Capabilities:     r.Capabilities,
			Source:           "explicit",
		})
	}

	// 阶段2：获取分组来源的模型（排除已在显式分配中的）
	var groupModels []struct {
		ModelDBID        int64  `json:"model_db_id"`
		ModelId          string `json:"model_id"`
		ModelName        string `json:"model_name"`
		Category         string `json:"category"`
		MaxContextTokens int    `json:"max_context_tokens"`
		MaxOutputTokens  int    `json:"max_output_tokens"`
		Description      string `json:"description"`
		Tags             string `json:"tags"`
		Capabilities     string `json:"capabilities"`
	}

	groupQuery := g.DB().Model("mdl_group_models gm").Ctx(ctx).
		InnerJoin("mdl_model_groups g ON gm.group_id = g.id").
		InnerJoin("mdl_models m ON gm.model_id = m.id").
		InnerJoin("mdl_tenant_groups tg ON tg.group_id = g.id").
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

	if category != "" {
		groupQuery = groupQuery.Where("m.category", category)
	}
	if search != "" {
		groupQuery = groupQuery.Where("m.model_id LIKE ? OR m.model_name LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	_ = groupQuery.
		Fields("DISTINCT m.id AS model_db_id, m.model_id, m.model_name, m.category, m.max_context_tokens, m.max_output_tokens, m.description, m.tags, m.capabilities").
		OrderAsc("m.category").
		OrderAsc("m.model_id").
		Scan(&groupModels)

	for _, gm := range groupModels {
		result = append(result, AvailableModelItem{
			ModelDBID:        gm.ModelDBID,
			ModelId:          gm.ModelId,
			ModelName:        gm.ModelName,
			Category:         gm.Category,
			MaxContextTokens: gm.MaxContextTokens,
			MaxOutputTokens:  gm.MaxOutputTokens,
			Description:      gm.Description,
			Tags:             gm.Tags,
			Capabilities:     gm.Capabilities,
			Source:           "group",
		})
	}

	return result, nil
}

// GetAsyncImageModelSet 判定给定图片模型（按 model_id 字符串）是否「走异步图片端点」。
//
// 真相源是服务该模型的渠道 provider 类型：桥接 mdl_models.model_id = chn_abilities.model_name
// → chn_channels.type，用 constant.IsAsyncImageModel 分类（与同步端点拦截 gate 同源）。
//   - 当某模型的全部在役服务渠道都是异步图片 provider（如 DashScope wanx/qwen-image）时，同步
//     /v1/images/generations 必失败，必须走 /v1/images/generations/async 提交+轮询 → true。
//   - 此外若「同步图片厂商异步化」总开关（sync_image_async_enabled，默认开启）打开，同步阻塞
//     返回的图片厂商（OpenAI/DALL·E 等）也由 worker 池在异步端点处理，故所有有渠道的图片模型
//     一并走异步端点 → true。开关关闭时，同步厂商回落同步端点 → false。
//
// 一次批量查询（非 N+1）；仅对图片分类模型调用。modelIDs 为空时返回空 map。
func GetAsyncImageModelSet(ctx context.Context, modelIDs []string) (map[string]bool, error) {
	result := make(map[string]bool)
	if len(modelIDs) == 0 {
		return result, nil
	}

	var rows []struct {
		ModelName string `json:"model_name"`
		Type      int    `json:"type"`
	}
	err := dao.ChnAbilities.Ctx(ctx).As("a").
		LeftJoin("chn_channels c ON a.channel_id = c.id").
		WhereIn("a.model_name", modelIDs).
		Where("a.enabled", true).
		Where("c.status", "active").
		Fields("DISTINCT a.model_name, c.type").
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	// 聚合每个模型的服务渠道 provider 类型：有渠道且全部为异步图片模型 → true。
	// 判定按 provider + 模型名（constant.IsAsyncImageModel），与同步端点拦截 gate 同源，
	// 因此 qwen-image-2.x 等同步 multimodal 模型即使渠道为 Ali 也判为非异步。
	type aggState struct {
		hasChannel bool
		allAsync   bool
	}
	agg := make(map[string]*aggState, len(modelIDs))
	for _, r := range rows {
		st := agg[r.ModelName]
		if st == nil {
			st = &aggState{allAsync: true}
			agg[r.ModelName] = st
		}
		st.hasChannel = true
		if !constant.IsAsyncImageModel(constant.ProviderType(r.Type), r.ModelName) {
			st.allAsync = false
		}
	}
	// 「同步图片厂商异步化」总开关（默认开启）：开启时，同步阻塞返回的图片厂商（OpenAI/DALL·E
	// 等）也能走 /v1/images/generations/async 由 worker 池异步处理，因此任何有在役渠道的图片
	// 模型都应走异步端点（在线体验用提交+轮询）。关闭时，仅「全部在役渠道均为异步图片 provider」
	// 的模型才异步（此时同步端点必失败）；同步厂商回落同步端点。
	syncImageEnabled := Config().GetBool(ctx, "sync_image_async_enabled")
	for name, st := range agg {
		result[name] = st.hasChannel && (st.allAsync || syncImageEnabled)
	}
	return result, nil
}
